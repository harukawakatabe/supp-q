package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"suppq.local/server/internal/core"
)

const applicationTimezoneRuleset = "go_runtime_unversioned"

type planCompatibilityContext struct {
	planID                  string
	currentScheduleID       string
	productProfileVersionID string
	timezoneVersionID       string
	ianaTimezone            string
}

type currentTargetSchedule struct {
	id                string
	businessVersion   int
	effectiveFrom     time.Time
	timezoneVersionID string
	ianaTimezone      string
	planStartDate     time.Time
	weekdays          []int16
	dayCycleEnabled   bool
	dayCycleDays      int
	dayTakeDays       int
	dayAnchorDate     time.Time
	longCycleEnabled  bool
	longTakeWeeks     int
	longRestWeeks     int
	longAnchorDate    time.Time
	slots             []targetDoseSlot
}

type targetDoseSlot struct {
	localTime    string
	quantity     core.Quantity
	mealRelation string
}

func createInitialPlanCompatibilityTx(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	productID string,
	input CreateProductInput,
	schedule core.Schedule,
	dose core.Quantity,
	now time.Time,
) error {
	var productProfileVersionID, timezoneVersionID, ianaTimezone string
	err := tx.QueryRow(ctx, `
		SELECT p.current_product_profile_version_id,w.current_timezone_version_id,tz.iana_timezone
		FROM products p
		JOIN workspaces w
		  ON w.id=p.workspace_id AND w.owner_user_id=p.user_id
		JOIN workspace_timezone_versions tz
		  ON tz.id=w.current_timezone_version_id
		 AND tz.user_id=p.user_id AND tz.workspace_id=p.workspace_id
		WHERE p.id=$1 AND p.user_id=$2 AND p.workspace_id=$3`,
		productID, scope.UserID, scope.WorkspaceID).Scan(
		&productProfileVersionID, &timezoneVersionID, &ianaTimezone)
	if err != nil {
		return err
	}
	planID, err := newUUID()
	if err != nil {
		return err
	}
	scheduleID, err := newUUID()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO product_plans (
			id,user_id,workspace_id,product_id,aggregate_version,current_schedule_version_id,
			state_history_completeness,created_at,updated_at
		) VALUES ($1,$2,$3,$4,1,NULL,'complete',$5,$5)`,
		planID, scope.UserID, scope.WorkspaceID, productID, now); err != nil {
		return err
	}
	source := "initial"
	if input.SourceRecognitionSetID != "" {
		source = "capture_confirmed"
	}
	context := planCompatibilityContext{
		planID:                  planID,
		productProfileVersionID: productProfileVersionID,
		timezoneVersionID:       timezoneVersionID,
		ianaTimezone:            ianaTimezone,
	}
	if err = insertTargetScheduleVersion(ctx, tx, scope, productID, context, scheduleID, 1, schedule.StartDate, input.WithFood, schedule, dose, source, now); err != nil {
		return err
	}
	stateID, err := newUUID()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO plan_state_intervals (
			id,user_id,workspace_id,product_id,product_plan_id,state,reason,source,
			history_completeness,effective_from,created_at
		) VALUES ($1,$2,$3,$4,$5,'active','initial','application','complete',$6,$6)`,
		stateID, scope.UserID, scope.WorkspaceID, productID, planID, now); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE product_plans
		SET current_schedule_version_id=$1
		WHERE id=$2 AND product_id=$3 AND user_id=$4 AND workspace_id=$5`,
		scheduleID, planID, productID, scope.UserID, scope.WorkspaceID)
	return err
}

func syncPlanCompatibilityTx(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	productID string,
	input UpdateProductInput,
	schedule core.Schedule,
	dose core.Quantity,
	effectiveDate time.Time,
	requestedStatus string,
	now time.Time,
) error {
	planContext, err := loadPlanCompatibilityContext(ctx, tx, scope, productID)
	if err != nil {
		return err
	}
	current, err := loadCurrentTargetSchedule(ctx, tx, scope, productID, planContext)
	if err != nil {
		return err
	}
	scheduleChanged := !targetScheduleMatches(current, planContext, input.WithFood, schedule, dose)
	stateChanged, err := syncPlanStateInterval(ctx, tx, scope, productID, planContext.planID, requestedStatus, now)
	if err != nil {
		return err
	}
	if scheduleChanged {
		if err = appendTargetScheduleVersion(ctx, tx, scope, productID, planContext, current, input.WithFood, schedule, dose, effectiveDate, now); err != nil {
			return err
		}
	}
	if scheduleChanged || stateChanged {
		_, err = tx.Exec(ctx, `
			UPDATE product_plans
			SET aggregate_version=aggregate_version+1,updated_at=$1
			WHERE id=$2 AND product_id=$3 AND user_id=$4 AND workspace_id=$5`,
			now, planContext.planID, productID, scope.UserID, scope.WorkspaceID)
	}
	return err
}

func loadPlanCompatibilityContext(ctx context.Context, tx pgx.Tx, scope Scope, productID string) (planCompatibilityContext, error) {
	var result planCompatibilityContext
	err := tx.QueryRow(ctx, `
		SELECT plan.id,plan.current_schedule_version_id,p.current_product_profile_version_id,
		       w.current_timezone_version_id,tz.iana_timezone
		FROM product_plans plan
		JOIN products p
		  ON p.id=plan.product_id AND p.user_id=plan.user_id AND p.workspace_id=plan.workspace_id
		JOIN workspaces w
		  ON w.id=plan.workspace_id AND w.owner_user_id=plan.user_id
		JOIN workspace_timezone_versions tz
		  ON tz.id=w.current_timezone_version_id
		 AND tz.user_id=plan.user_id AND tz.workspace_id=plan.workspace_id
		WHERE plan.product_id=$1 AND plan.user_id=$2 AND plan.workspace_id=$3
		FOR UPDATE OF plan`, productID, scope.UserID, scope.WorkspaceID).Scan(
		&result.planID, &result.currentScheduleID, &result.productProfileVersionID,
		&result.timezoneVersionID, &result.ianaTimezone)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, fmt.Errorf("product plan compatibility row is missing for product %s", productID)
	}
	return result, err
}

func loadCurrentTargetSchedule(ctx context.Context, tx pgx.Tx, scope Scope, productID string, plan planCompatibilityContext) (currentTargetSchedule, error) {
	var result currentTargetSchedule
	err := tx.QueryRow(ctx, `
		SELECT id,business_version,effective_from,timezone_version_id,iana_timezone,
		       plan_start_date,weekdays,day_cycle_enabled,day_cycle_days,day_take_days,
		       day_anchor_date,long_cycle_enabled,long_take_weeks,long_rest_weeks,long_anchor_date
		FROM schedule_versions
		WHERE id=$1 AND product_plan_id=$2 AND product_id=$3 AND user_id=$4 AND workspace_id=$5
		  AND version_state='active'`,
		plan.currentScheduleID, plan.planID, productID, scope.UserID, scope.WorkspaceID).Scan(
		&result.id, &result.businessVersion, &result.effectiveFrom, &result.timezoneVersionID,
		&result.ianaTimezone, &result.planStartDate, &result.weekdays, &result.dayCycleEnabled,
		&result.dayCycleDays, &result.dayTakeDays, &result.dayAnchorDate, &result.longCycleEnabled,
		&result.longTakeWeeks, &result.longRestWeeks, &result.longAnchorDate)
	if err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `
		SELECT to_char(local_time,'HH24:MI'),quantity::text,meal_relation
		FROM dose_slots
		WHERE schedule_version_id=$1 AND product_plan_id=$2 AND product_id=$3
		  AND user_id=$4 AND workspace_id=$5
		ORDER BY sort_order`,
		result.id, plan.planID, productID, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var slot targetDoseSlot
		var quantityText string
		if err = rows.Scan(&slot.localTime, &quantityText, &slot.mealRelation); err != nil {
			return result, err
		}
		slot.quantity, err = core.ParseQuantity(quantityText)
		if err != nil {
			return result, err
		}
		result.slots = append(result.slots, slot)
	}
	return result, rows.Err()
}

func targetScheduleMatches(current currentTargetSchedule, plan planCompatibilityContext, withFood *bool, schedule core.Schedule, dose core.Quantity) bool {
	if current.timezoneVersionID != plan.timezoneVersionID || current.ianaTimezone != plan.ianaTimezone ||
		core.DateKey(current.planStartDate) != core.DateKey(schedule.StartDate) ||
		current.dayCycleEnabled != schedule.DayCycle.Enabled || current.dayCycleDays != schedule.DayCycle.CycleDays ||
		current.dayTakeDays != schedule.DayCycle.TakeDays || core.DateKey(current.dayAnchorDate) != core.DateKey(schedule.DayCycle.AnchorDate) ||
		current.longCycleEnabled != schedule.LongCycle.Enabled || current.longTakeWeeks != schedule.LongCycle.TakeWeeks ||
		current.longRestWeeks != schedule.LongCycle.RestWeeks || core.DateKey(current.longAnchorDate) != core.DateKey(schedule.LongCycle.StartDate) ||
		len(current.weekdays) != len(schedule.Weekdays) || len(current.slots) != len(schedule.ReminderTimes) {
		return false
	}
	for index, weekday := range schedule.Weekdays {
		if current.weekdays[index] != int16(weekday) {
			return false
		}
	}
	mealRelation := targetMealRelation(withFood)
	for index, reminderTime := range schedule.ReminderTimes {
		slot := current.slots[index]
		if slot.localTime != reminderTime || slot.quantity != dose || slot.mealRelation != mealRelation {
			return false
		}
	}
	return true
}

func appendTargetScheduleVersion(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	productID string,
	plan planCompatibilityContext,
	current currentTargetSchedule,
	withFood *bool,
	schedule core.Schedule,
	dose core.Quantity,
	effectiveDate time.Time,
	now time.Time,
) error {
	if effectiveDate.Before(current.effectiveFrom) {
		return NewError("schedule_version_conflict", "新计划不能早于当前计划版本的生效日期。")
	}
	location, err := time.LoadLocation(plan.ianaTimezone)
	if err != nil {
		return fmt.Errorf("load workspace timezone %q: %w", plan.ianaTimezone, err)
	}
	localNow := now.In(location)
	localToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	if current.effectiveFrom.After(localToday) && !effectiveDate.Equal(current.effectiveFrom) {
		return NewError("schedule_version_conflict", "已有待生效计划只能在相同生效日期替换。")
	}
	if effectiveDate.Equal(current.effectiveFrom) {
		command, updateErr := tx.Exec(ctx, `
			UPDATE schedule_versions
			SET version_state='cancelled',effective_to=$1
			WHERE id=$2 AND product_plan_id=$3 AND product_id=$4 AND user_id=$5 AND workspace_id=$6
			  AND version_state='active' AND effective_to IS NULL`,
			effectiveDate, current.id, plan.planID, productID, scope.UserID, scope.WorkspaceID)
		if updateErr != nil {
			return updateErr
		}
		if command.RowsAffected() != 1 {
			return NewError("schedule_version_conflict", "当前计划版本已变化，请刷新后重试。")
		}
	} else {
		command, updateErr := tx.Exec(ctx, `
			UPDATE schedule_versions
			SET effective_to=$1
			WHERE id=$2 AND product_plan_id=$3 AND product_id=$4 AND user_id=$5 AND workspace_id=$6
			  AND version_state='active' AND effective_to IS NULL`,
			effectiveDate, current.id, plan.planID, productID, scope.UserID, scope.WorkspaceID)
		if updateErr != nil {
			return updateErr
		}
		if command.RowsAffected() != 1 {
			return NewError("schedule_version_conflict", "当前计划版本已变化，请刷新后重试。")
		}
	}
	newScheduleID, err := newUUID()
	if err != nil {
		return err
	}
	if err = insertTargetScheduleVersion(ctx, tx, scope, productID, plan, newScheduleID, current.businessVersion+1, effectiveDate, withFood, schedule, dose, "user_edit", now); err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `
		UPDATE product_plans
		SET current_schedule_version_id=$1
		WHERE id=$2 AND product_id=$3 AND user_id=$4 AND workspace_id=$5
		  AND current_schedule_version_id=$6`,
		newScheduleID, plan.planID, productID, scope.UserID, scope.WorkspaceID, current.id)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return NewError("schedule_version_conflict", "当前计划版本已变化，请刷新后重试。")
	}
	return nil
}

func insertTargetScheduleVersion(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	productID string,
	plan planCompatibilityContext,
	scheduleID string,
	businessVersion int,
	effectiveFrom time.Time,
	withFood *bool,
	schedule core.Schedule,
	dose core.Quantity,
	source string,
	now time.Time,
) error {
	weekdays := make([]int16, len(schedule.Weekdays))
	for index, weekday := range schedule.Weekdays {
		weekdays[index] = int16(weekday)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO schedule_versions (
			id,user_id,workspace_id,product_id,product_plan_id,product_profile_version_id,
			business_version,version_state,effective_from,timezone_version_id,iana_timezone,
			timezone_ruleset,plan_start_date,weekdays,day_cycle_enabled,
			day_cycle_days,day_take_days,day_anchor_date,long_cycle_enabled,long_take_weeks,
			long_rest_weeks,long_anchor_date,source,history_completeness,source_updated_at,
			created_by,created_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,'active',$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,
			$18,$19,$20,$21,$22,'complete',$23,$2,$23
		)`,
		scheduleID, scope.UserID, scope.WorkspaceID, productID, plan.planID,
		plan.productProfileVersionID, businessVersion, effectiveFrom, plan.timezoneVersionID,
		plan.ianaTimezone, applicationTimezoneRuleset, schedule.StartDate,
		weekdays, schedule.DayCycle.Enabled, schedule.DayCycle.CycleDays, schedule.DayCycle.TakeDays,
		schedule.DayCycle.AnchorDate, schedule.LongCycle.Enabled, schedule.LongCycle.TakeWeeks,
		schedule.LongCycle.RestWeeks, schedule.LongCycle.StartDate, source, now); err != nil {
		return err
	}
	mealRelation := targetMealRelation(withFood)
	for index, reminderTime := range schedule.ReminderTimes {
		slotID, createErr := newUUID()
		if createErr != nil {
			return createErr
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO dose_slots (
				id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
				slot_key,sort_order,local_time,quantity,meal_relation,label,created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'',$12)`,
			slotID, scope.UserID, scope.WorkspaceID, productID, plan.planID, scheduleID,
			fmt.Sprintf("slot-%d", index+1), index, reminderTime, dose.DatabaseString(), mealRelation, now); err != nil {
			return err
		}
	}
	return nil
}

func syncPlanStateInterval(
	ctx context.Context,
	tx pgx.Tx,
	scope Scope,
	productID string,
	planID string,
	requestedStatus string,
	now time.Time,
) (bool, error) {
	if requestedStatus == "depleted" {
		return false, nil
	}
	desiredState := "active"
	desiredReason := "user_resume"
	if requestedStatus == "paused" || requestedStatus == "archived" {
		desiredState = "paused"
		if requestedStatus == "archived" {
			desiredReason = "archived"
		} else {
			desiredReason = "user_pause"
		}
	}
	var currentID, currentState string
	var effectiveFrom time.Time
	err := tx.QueryRow(ctx, `
		SELECT id,state,effective_from
		FROM plan_state_intervals
		WHERE product_plan_id=$1 AND product_id=$2 AND user_id=$3 AND workspace_id=$4
		  AND effective_to IS NULL
		FOR UPDATE`, planID, productID, scope.UserID, scope.WorkspaceID).Scan(
		&currentID, &currentState, &effectiveFrom)
	if errors.Is(err, pgx.ErrNoRows) {
		stateID, createErr := newUUID()
		if createErr != nil {
			return false, createErr
		}
		_, createErr = tx.Exec(ctx, `
			INSERT INTO plan_state_intervals (
				id,user_id,workspace_id,product_id,product_plan_id,state,reason,source,
				history_completeness,effective_from,created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,'application','complete',$8,$8)`,
			stateID, scope.UserID, scope.WorkspaceID, productID, planID, desiredState, desiredReason, now)
		return createErr == nil, createErr
	}
	if err != nil {
		return false, err
	}
	if currentState == desiredState {
		return false, nil
	}
	transitionAt := now
	if !transitionAt.After(effectiveFrom) {
		transitionAt = effectiveFrom.Add(time.Microsecond)
	}
	command, err := tx.Exec(ctx, `
		UPDATE plan_state_intervals SET effective_to=$1
		WHERE id=$2 AND product_plan_id=$3 AND product_id=$4 AND user_id=$5 AND workspace_id=$6
		  AND effective_to IS NULL`,
		transitionAt, currentID, planID, productID, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return false, err
	}
	if command.RowsAffected() != 1 {
		return false, NewError("plan_state_conflict", "当前计划状态已变化，请刷新后重试。")
	}
	stateID, err := newUUID()
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO plan_state_intervals (
			id,user_id,workspace_id,product_id,product_plan_id,state,reason,source,
			history_completeness,effective_from,created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'application','complete',$8,$8)`,
		stateID, scope.UserID, scope.WorkspaceID, productID, planID, desiredState, desiredReason, transitionAt)
	return err == nil, err
}

func targetMealRelation(withFood *bool) string {
	if withFood != nil && *withFood {
		return "with_meal"
	}
	return "unspecified"
}
