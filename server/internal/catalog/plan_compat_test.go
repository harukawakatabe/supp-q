package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"suppq.local/server/migrations"
)

func TestNormalizeCreateRequiresOneUniqueReminderPerDose(t *testing.T) {
	now := time.Date(2026, time.August, 2, 9, 0, 0, 0, time.UTC)
	valid := CreateProductInput{
		Name:                      "测试产品",
		Unit:                      "粒",
		DoseQuantity:              1,
		DoseTimesPerDay:           2,
		IngredientServingQuantity: 1,
		OpeningBatch:              BatchInput{Quantity: 10},
		Schedule: ScheduleInput{
			StartDate:     "2026-08-02",
			Weekdays:      []int{0, 1, 2, 3, 4, 5, 6},
			ReminderTimes: []string{"08:00", "20:00"},
		},
	}
	if _, _, err := normalizeCreate(valid, now); err != nil {
		t.Fatalf("valid two-slot schedule rejected: %v", err)
	}

	tests := []struct {
		name      string
		reminders []string
	}{
		{name: "missing slot", reminders: []string{"08:00"}},
		{name: "empty slots", reminders: nil},
		{name: "duplicate slots", reminders: []string{"08:00", "08:00"}},
		{name: "invalid local time", reminders: []string{"08:00", "24:00"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			input.Schedule.ReminderTimes = test.reminders
			_, _, err := normalizeCreate(input, now)
			var catalogErr *Error
			if !errors.As(err, &catalogErr) || catalogErr.Code != "invalid_schedule" {
				t.Fatalf("expected invalid_schedule, got %v", err)
			}
		})
	}
}

func TestPlanCompatibilityDualWrite(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	schema := fmt.Sprintf("suppq_plan_compat_test_%d", time.Now().UnixNano())
	if _, err = adminPool.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`) })
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	if err = migrations.Up(ctx, parsed.String()); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	now := time.Date(2026, time.August, 2, 9, 0, 0, 0, time.UTC)
	owner := createCatalogOwner(t, ctx, pool, "00000000-0000-4000-8000-000000000101", "00000000-0000-4000-8000-000000000111", now)
	service := New(pool)
	service.now = func() time.Time { return now }
	withFood := true
	createInput := CreateProductInput{
		Name:                      "双写测试",
		Brand:                     "初始品牌",
		Unit:                      "粒",
		DoseQuantity:              1,
		DoseTimesPerDay:           1,
		IngredientServingQuantity: 1,
		WithFood:                  &withFood,
		RestockThresholdDays:      7,
		ExpiryReminderDays:        30,
		Schedule: ScheduleInput{
			StartDate:     "2026-08-01",
			Weekdays:      []int{0, 1, 2, 3, 4, 5, 6},
			ReminderTimes: []string{"08:00"},
		},
		OpeningBatch: BatchInput{Quantity: 2},
	}
	product, err := service.CreateProduct(ctx, owner, createInput)
	if err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `
		SELECT count(*)
		FROM product_plans plan
		JOIN products p
		  ON p.id=plan.product_id AND p.user_id=plan.user_id AND p.workspace_id=plan.workspace_id
		JOIN schedule_versions sv
		  ON sv.id=plan.current_schedule_version_id
		 AND sv.product_profile_version_id=p.current_product_profile_version_id
		JOIN workspace_timezone_versions tz
		  ON tz.id=sv.timezone_version_id AND tz.user_id=sv.user_id AND tz.workspace_id=sv.workspace_id
		JOIN dose_slots slot
		  ON slot.schedule_version_id=sv.id
		JOIN plan_state_intervals state
		  ON state.product_plan_id=plan.id AND state.effective_to IS NULL
		WHERE plan.product_id=$1
		  AND plan.state_history_completeness='complete'
		  AND sv.business_version=1 AND sv.source='initial' AND sv.history_completeness='complete'
		  AND sv.timezone_ruleset='go_runtime_unversioned'
		  AND slot.sort_order=0 AND slot.local_time='08:00' AND slot.quantity=1
		  AND slot.meal_relation='with_meal'
		  AND state.state='active' AND state.reason='initial' AND state.source='application'`, 1, product.ID)

	update := UpdateProductInput{
		Name:                      "仅改名称",
		Brand:                     "初始品牌",
		ProductType:               "supplement",
		Status:                    "active",
		Unit:                      "粒",
		DoseQuantity:              1,
		DoseTimesPerDay:           1,
		IngredientServingQuantity: 1,
		WithFood:                  &withFood,
		RestockThresholdDays:      7,
		ExpiryReminderDays:        30,
		Schedule: ScheduleInput{
			StartDate:     "2026-08-01",
			Weekdays:      []int{0, 1, 2, 3, 4, 5, 6},
			DayCycle:      DayCycleInput{CycleDays: 28, TakeDays: 28, AnchorDate: "2026-08-01"},
			LongCycle:     LongCycleInput{TakeWeeks: 6, StartDate: "2026-08-01"},
			ReminderTimes: []string{"08:00"},
		},
		EffectiveDate: "2026-08-03",
	}
	if _, err = service.UpdateProduct(ctx, owner, product.ID, update); err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM schedule_versions WHERE product_id=$1`, 1, product.ID)

	update.Schedule.ReminderTimes = []string{"20:00"}
	if _, err = service.UpdateProduct(ctx, owner, product.ID, update); err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM schedule_versions WHERE product_id=$1`, 2, product.ID)
	assertCatalogCount(t, ctx, pool, `
		SELECT count(*)
		FROM product_plans plan
		JOIN schedule_versions sv ON sv.id=plan.current_schedule_version_id
		JOIN products p ON p.id=plan.product_id
		JOIN dose_slots slot ON slot.schedule_version_id=sv.id
		WHERE plan.product_id=$1 AND sv.business_version=2 AND sv.source='user_edit'
		  AND sv.product_profile_version_id=p.current_product_profile_version_id
		  AND slot.local_time='20:00' AND slot.quantity=1`, 1, product.ID)

	if _, err = service.UpdateProduct(ctx, owner, product.ID, update); err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM schedule_versions WHERE product_id=$1`, 2, product.ID)

	update.Status = "paused"
	if _, err = service.UpdateProduct(ctx, owner, product.ID, update); err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdateProduct(ctx, owner, product.ID, update); err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM schedule_versions WHERE product_id=$1`, 2, product.ID)
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM plan_state_intervals WHERE product_id=$1`, 2, product.ID)
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM plan_state_intervals WHERE product_id=$1 AND effective_to IS NULL AND state='paused' AND reason='user_pause'`, 1, product.ID)

	update.Status = "active"
	if _, err = service.UpdateProduct(ctx, owner, product.ID, update); err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM plan_state_intervals WHERE product_id=$1`, 3, product.ID)
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM plan_state_intervals WHERE product_id=$1 AND effective_to IS NULL AND state='active' AND reason='user_resume'`, 1, product.ID)

	intake, _, err := service.CreateIntake(ctx, owner, CreateIntakeInput{
		ProductID: product.ID, Date: "2026-08-02", Time: "08:00", Quantity: 2, Source: "scheduled",
	}, "plan-compat-deplete")
	if err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM schedule_versions WHERE product_id=$1`, 2, product.ID)
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM plan_state_intervals WHERE product_id=$1`, 3, product.ID)
	if _, err = service.AddBatch(ctx, owner, product.ID, BatchInput{Quantity: 1}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.UndoIntake(ctx, owner, intake.ID); err != nil {
		t.Fatal(err)
	}
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM schedule_versions WHERE product_id=$1`, 2, product.ID)
	assertCatalogCount(t, ctx, pool, `SELECT count(*) FROM plan_state_intervals WHERE product_id=$1`, 3, product.ID)
}
