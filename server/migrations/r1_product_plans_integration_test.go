package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

const r1ProductProfilesVersion int64 = 202609200001

func TestR1ProductPlansFreshAndCurrentSnapshotUpgrade(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}

	t.Run("fresh", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyAll(t, ctx, db)
			for _, table := range []string{
				"product_plans",
				"schedule_versions",
				"dose_slots",
				"plan_state_intervals",
				"scheduled_occurrences",
				"r1_product_plan_backfill_state",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx,
					`SELECT to_regclass(current_schema() || '.' || $1) IS NOT NULL`, table,
				).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected E3 table %s", table)
				}
			}

			var batchFunctionExists bool
			if err := db.QueryRowContext(ctx,
				`SELECT to_regprocedure('r1_backfill_product_plans_batch(integer)') IS NOT NULL`,
			).Scan(&batchFunctionExists); err != nil {
				t.Fatal(err)
			}
			if !batchFunctionExists {
				t.Fatal("expected resumable E3 product-plan backfill function")
			}
		})
	})

	t.Run("current_snapshot", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyProductProfileSnapshot(t, ctx, db)
			configureGoose(t)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("upgrade E2 current snapshot through E3: %v", err)
			}

			assertCount(t, ctx, db, `SELECT count(*) FROM products`, 7)
			assertCount(t, ctx, db, `SELECT count(*) FROM product_schedules`, 5)
			assertCount(t, ctx, db, `SELECT count(*) FROM product_plans`, 4)
			assertCount(t, ctx, db, `SELECT count(*) FROM schedule_versions`, 4)
			assertCount(t, ctx, db, `SELECT count(*) FROM dose_slots`, 5)
			assertCount(t, ctx, db, `SELECT count(*) FROM plan_state_intervals`, 4)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM plan_state_intervals
				WHERE effective_to IS NULL AND state='active'`, 3)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM plan_state_intervals
				WHERE effective_to IS NULL AND state='paused'`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM scheduled_occurrences`, 0)

			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM product_plans pp
				JOIN schedule_versions sv
				  ON sv.id=pp.current_schedule_version_id
				 AND sv.product_plan_id=pp.id
				 AND sv.product_id=pp.product_id
				 AND sv.user_id=pp.user_id
				 AND sv.workspace_id=pp.workspace_id
				JOIN products p
				  ON p.id=sv.product_id
				 AND p.user_id=sv.user_id
				 AND p.workspace_id=sv.workspace_id
				JOIN product_profile_versions pv
				  ON pv.id=sv.product_profile_version_id
				 AND pv.product_id=sv.product_id
				 AND pv.user_id=sv.user_id
				 AND pv.workspace_id=sv.workspace_id
				JOIN workspace_timezone_versions tz
				  ON tz.id=sv.timezone_version_id
				 AND tz.user_id=sv.user_id
				 AND tz.workspace_id=sv.workspace_id
				WHERE sv.source='legacy_migration'
				  AND p.current_product_profile_version_id=pv.id
				  AND sv.history_completeness='legacy_current_only'
				  AND sv.timezone_ruleset='legacy_unversioned'
				  AND sv.iana_timezone='Etc/UTC'`, 4)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM dose_slots ds
				JOIN schedule_versions sv
				  ON sv.id=ds.schedule_version_id
				 AND sv.product_plan_id=ds.product_plan_id
				 AND sv.product_id=ds.product_id
				 AND sv.user_id=ds.user_id
				 AND sv.workspace_id=ds.workspace_id
				WHERE ds.quantity <= 0`, 0)

			assertSlotMismatchQuarantine(t, ctx, db)
			assertBatchResumeAndIdempotency(t, ctx, db)
			assertLegacyPlanDriftAppendsVersion(t, ctx, db)
			assertPlanConstraints(t, ctx, db)
			assertPlanParentCascade(t, ctx, db)
		})
	})

	t.Run("down_before_target_writes", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyProductProfileSnapshot(t, ctx, db)
			configureGoose(t)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("apply E3 before rollback rehearsal: %v", err)
			}
			if err := goose.DownToContext(ctx, db, ".", r1ProductProfilesVersion); err != nil {
				t.Fatalf("rollback E3 before target writes: %v", err)
			}

			assertCount(t, ctx, db, `SELECT count(*) FROM products`, 7)
			assertCount(t, ctx, db, `SELECT count(*) FROM product_schedules`, 5)
			var planTableExists bool
			if err := db.QueryRowContext(ctx,
				`SELECT to_regclass(current_schema() || '.product_plans') IS NOT NULL`,
			).Scan(&planTableExists); err != nil {
				t.Fatal(err)
			}
			if planTableExists {
				t.Fatal("pre-write E3 rollback left product_plans behind")
			}
			for _, signature := range []string{
				"r1_scheduled_occurrence_id(uuid,date,uuid)",
				"r1_validate_workspace_timezone_version()",
				"r1_block_timezone_history_delete()",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx, `SELECT to_regprocedure($1) IS NOT NULL`, signature).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if exists {
					t.Fatalf("pre-write E3 rollback left function %s behind", signature)
				}
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM pg_trigger
				WHERE tgrelid='workspace_timezone_versions'::regclass
				  AND tgname IN ('workspace_timezone_versions_validate','workspace_timezone_versions_block_delete')
				  AND NOT tgisinternal`, 0)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM pg_constraint
				WHERE conrelid='workspaces'::regclass
				  AND conname='workspaces_current_timezone_version_fk'
				  AND contype='f' AND cardinality(conkey)=1`, 1)
		})
	})

	t.Run("down_refuses_target_writes", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyProductProfileSnapshot(t, ctx, db)
			configureGoose(t)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("apply E3 before guarded rollback: %v", err)
			}

			insertOccurrence(t, ctx, db, fixtureProductID, 2)
			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", r1ProductProfilesVersion); err == nil {
				t.Fatal("E3 Down accepted target occurrence writes")
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM scheduled_occurrences
				WHERE product_id='10000000-0000-4000-8000-000000000101'
				  AND local_date=CURRENT_DATE + 2`, 1)
		})
	})
}

const fixtureProductID = "10000000-0000-4000-8000-000000000101"

func applyProductProfileSnapshot(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	configureGoose(t)
	if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
		t.Fatalf("apply launch snapshot: %v", err)
	}
	seedLaunchSnapshot(t, ctx, db)
	seedProductPlanSnapshot(t, ctx, db)
	if err := goose.UpToContext(ctx, db, ".", r1ProductProfilesVersion); err != nil {
		t.Fatalf("upgrade fixture through E2: %v", err)
	}
}

func seedProductPlanSnapshot(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createdAt := time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC)
	seedScheduledProduct(t, ctx, db, scheduledProductFixture{
		productID: "10000000-0000-4000-8000-000000000104",
		cycleID:   "10000000-0000-4000-8000-000000000504",
		name:      "fixture paused split schedule",
		status:    "paused",
		doseTimes: 2,
		withFood:  true,
		reminders: "{08:00,20:00}",
		createdAt: createdAt,
	})
	seedScheduledProduct(t, ctx, db, scheduledProductFixture{
		productID: "10000000-0000-4000-8000-000000000105",
		cycleID:   "10000000-0000-4000-8000-000000000505",
		name:      "fixture slot mismatch",
		status:    "active",
		doseTimes: 2,
		reminders: "{09:00}",
		createdAt: createdAt.Add(time.Minute),
	})
	seedScheduledProduct(t, ctx, db, scheduledProductFixture{
		productID: "10000000-0000-4000-8000-000000000106",
		cycleID:   "10000000-0000-4000-8000-000000000506",
		name:      "fixture cascade schedule",
		status:    "active",
		doseTimes: 1,
		reminders: "{07:30}",
		createdAt: createdAt.Add(2 * time.Minute),
	})

	secondUserID := "20000000-0000-4000-8000-000000000001"
	secondWorkspaceID := "20000000-0000-4000-8000-000000000011"
	execSeed(t, ctx, db, `
		INSERT INTO users (id,kind,status,role,last_activity_at,created_at)
		VALUES ($1,'registered','active','member',$2,$2)`, secondUserID, createdAt)
	execSeed(t, ctx, db, `
		INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,created_at)
		VALUES ($1,$2,'registered','second migration fixture',$3,$3)`,
		secondWorkspaceID, secondUserID, createdAt)
	execSeed(t, ctx, db, `
		INSERT INTO workspace_members (workspace_id,user_id,role,created_at)
		VALUES ($1,$2,'owner',$3)`, secondWorkspaceID, secondUserID, createdAt)
	seedScheduledProduct(t, ctx, db, scheduledProductFixture{
		userID:      secondUserID,
		workspaceID: secondWorkspaceID,
		productID:   "20000000-0000-4000-8000-000000000101",
		cycleID:     "20000000-0000-4000-8000-000000000501",
		name:        "fixture other tenant schedule",
		status:      "active",
		doseTimes:   1,
		reminders:   "{21:00}",
		createdAt:   createdAt,
	})
}

type scheduledProductFixture struct {
	userID      string
	workspaceID string
	productID   string
	cycleID     string
	name        string
	status      string
	doseTimes   int
	withFood    bool
	reminders   string
	createdAt   time.Time
}

func seedScheduledProduct(t *testing.T, ctx context.Context, db *sql.DB, fixture scheduledProductFixture) {
	t.Helper()
	if fixture.userID == "" {
		fixture.userID = "10000000-0000-4000-8000-000000000001"
	}
	if fixture.workspaceID == "" {
		fixture.workspaceID = "10000000-0000-4000-8000-000000000011"
	}
	execSeed(t, ctx, db, `
		INSERT INTO products (
			id,user_id,workspace_id,name,brand,product_type,status,unit,
			dose_quantity,dose_times_per_day,ingredient_serving_quantity,with_food,
			restock_threshold_days,expiry_reminder_days,created_at,updated_at
		) VALUES ($1,$2,$3,$4,'fixture','supplement',$5,'capsule',1,$6,1,$7,7,30,$8,$8)
		`, fixture.productID, fixture.userID, fixture.workspaceID, fixture.name,
		fixture.status, fixture.doseTimes, fixture.withFood, fixture.createdAt)
	execSeed(t, ctx, db, `
		INSERT INTO product_schedules (
			product_id,user_id,workspace_id,version,start_date,weekdays,
			day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,
			long_cycle_enabled,long_take_weeks,long_rest_weeks,long_start_date,
			reminder_times,created_at,updated_at
		) VALUES ($1,$2,$3,1,'2026-09-01',ARRAY[0,1,2,3,4,5,6]::smallint[],
			false,28,28,'2026-09-01',false,6,4,'2026-09-01',$4::text[],$5,$5)`,
		fixture.productID, fixture.userID, fixture.workspaceID, fixture.reminders,
		fixture.createdAt)
	execSeed(t, ctx, db, `
		INSERT INTO day_cycle_versions (
			id,user_id,workspace_id,product_id,effective_date,enabled,cycle_days,take_days,anchor_date,created_at
		) VALUES ($1,$2,$3,$4,'2026-09-01',false,28,28,'2026-09-01',$5)`,
		fixture.cycleID, fixture.userID, fixture.workspaceID, fixture.productID,
		fixture.createdAt)
}

func assertSlotMismatchQuarantine(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	mismatchProductID := "10000000-0000-4000-8000-000000000105"
	assertCount(t, ctx, db, `
		SELECT count(*) FROM migration_quarantines
		WHERE source_table='product_schedules'
		  AND source_id='10000000-0000-4000-8000-000000000105'
		  AND reason_code='schedule_slot_count_mismatch'
		  AND state='open'`, 1)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM product_plans
		WHERE product_id='10000000-0000-4000-8000-000000000105'`, 0)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM schedule_versions
		WHERE product_id='10000000-0000-4000-8000-000000000105'`, 0)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM dose_slots
		WHERE product_id='10000000-0000-4000-8000-000000000105'`, 0)

	var targetPlanCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM product_plans WHERE product_id=$1`, mismatchProductID,
	).Scan(&targetPlanCount); err != nil {
		t.Fatal(err)
	}
	if targetPlanCount != 0 {
		t.Fatalf("slot-mismatch source left %d target plans", targetPlanCount)
	}
}

func assertBatchResumeAndIdempotency(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	versionsBefore := queryCount(t, ctx, db, `SELECT count(*) FROM schedule_versions`)
	slotsBefore := queryCount(t, ctx, db, `SELECT count(*) FROM dose_slots`)
	statesBefore := queryCount(t, ctx, db, `SELECT count(*) FROM plan_state_intervals`)
	quarantinesBefore := queryCount(t, ctx, db, `SELECT count(*) FROM migration_quarantines`)

	var cycleComplete bool
	if err := db.QueryRowContext(ctx, `SELECT r1_backfill_product_plans_batch(1)`).Scan(&cycleComplete); err != nil {
		t.Fatalf("start one-row E3 backfill cycle: %v", err)
	}
	if cycleComplete {
		t.Fatal("one-row E3 batch unexpectedly completed a five-product cycle")
	}
	assertCount(t, ctx, db, `
		SELECT count(*) FROM r1_product_plan_backfill_state
		WHERE singleton AND cycle_state='running'
		  AND cursor_product_id IS NOT NULL AND processed_count=1`, 1)

	// Force database/sql to discard the first session so the durable cursor is
	// resumed by a fresh PostgreSQL connection.
	db.SetMaxIdleConns(0)
	for attempts := 0; attempts < 10 && !cycleComplete; attempts++ {
		if err := db.QueryRowContext(ctx, `SELECT r1_backfill_product_plans_batch(1)`).Scan(&cycleComplete); err != nil {
			t.Fatalf("resume E3 backfill: %v", err)
		}
	}
	if !cycleComplete {
		t.Fatal("resumable E3 backfill did not complete")
	}
	assertCount(t, ctx, db, `
		SELECT count(*) FROM r1_product_plan_backfill_state
		WHERE singleton AND cycle_state='idle' AND cursor_product_id IS NULL
		  AND processed_count=5 AND last_completed_at IS NOT NULL`, 1)

	if _, err := db.ExecContext(ctx, `SELECT r1_backfill_product_plans()`); err != nil {
		t.Fatalf("repeat complete E3 backfill cycle: %v", err)
	}
	assertCount(t, ctx, db, `SELECT count(*) FROM schedule_versions`, versionsBefore)
	assertCount(t, ctx, db, `SELECT count(*) FROM dose_slots`, slotsBefore)
	assertCount(t, ctx, db, `SELECT count(*) FROM plan_state_intervals`, statesBefore)
	assertCount(t, ctx, db, `SELECT count(*) FROM migration_quarantines`, quarantinesBefore)
}

func assertLegacyPlanDriftAppendsVersion(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	execSeed(t, ctx, db, `
		UPDATE product_schedules
		SET version=version+1,
		    weekdays=ARRAY[1,3,5]::smallint[],
		    day_cycle_enabled=true,
		    day_cycle_days=7,
		    day_take_days=3,
		    day_anchor_date=CURRENT_DATE + 1,
		    reminder_times=ARRAY['10:15'],
		    updated_at=CURRENT_TIMESTAMP
		WHERE product_id='10000000-0000-4000-8000-000000000101'`)
	execSeed(t, ctx, db, `
		INSERT INTO day_cycle_versions (
			id,user_id,workspace_id,product_id,effective_date,enabled,cycle_days,take_days,anchor_date,created_at
		) VALUES (
			'10000000-0000-4000-8000-000000000599',
			'10000000-0000-4000-8000-000000000001',
			'10000000-0000-4000-8000-000000000011',
			'10000000-0000-4000-8000-000000000101',
			CURRENT_DATE + 1,true,7,3,CURRENT_DATE + 1,CURRENT_TIMESTAMP
		)`)
	if _, err := db.ExecContext(ctx, `SELECT r1_backfill_product_plans()`); err != nil {
		t.Fatalf("catch up N-1 schedule drift: %v", err)
	}

	assertCount(t, ctx, db, `
		SELECT count(*) FROM schedule_versions
		WHERE product_id='10000000-0000-4000-8000-000000000101'`, 2)
	assertCount(t, ctx, db, `
		SELECT count(*)
		FROM product_plans pp
		JOIN schedule_versions sv ON sv.id=pp.current_schedule_version_id
		WHERE pp.product_id='10000000-0000-4000-8000-000000000101'
		  AND pp.aggregate_version=2
		  AND sv.business_version=2
		  AND sv.weekdays=ARRAY[1,3,5]::smallint[]
		  AND sv.day_cycle_enabled
		  AND sv.day_cycle_days=7
		  AND sv.day_take_days=3
		  AND sv.source='legacy_migration'
		  AND sv.effective_to IS NULL`, 1)
	assertCount(t, ctx, db, `
		SELECT count(*)
		FROM product_plans pp
		JOIN dose_slots ds ON ds.schedule_version_id=pp.current_schedule_version_id
		WHERE pp.product_id='10000000-0000-4000-8000-000000000101'
		  AND ds.local_time='10:15'::time
		  AND ds.quantity=1`, 1)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM schedule_versions
		WHERE product_id='10000000-0000-4000-8000-000000000101'
		  AND business_version=1 AND effective_to IS NOT NULL`, 1)

	if _, err := db.ExecContext(ctx, `SELECT r1_backfill_product_plans()`); err != nil {
		t.Fatalf("repeat N-1 schedule catch-up: %v", err)
	}
	assertCount(t, ctx, db, `
		SELECT count(*) FROM schedule_versions
		WHERE product_id='10000000-0000-4000-8000-000000000101'`, 2)
}

func assertPlanConstraints(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	basePlanID, baseScheduleID, baseSlotID := currentPlanRefs(t, ctx, db, fixtureProductID)
	otherPlanID, otherScheduleID, otherSlotID := currentPlanRefs(
		t, ctx, db, "20000000-0000-4000-8000-000000000101",
	)

	expectExecFailure(t, ctx, db, "schedule version body was mutable", `
		UPDATE schedule_versions SET weekdays=ARRAY[2]::smallint[] WHERE id=$1`, baseScheduleID)
	expectExecFailure(t, ctx, db, "activated dose slot was mutable", `
		UPDATE dose_slots SET quantity=2 WHERE id=$1`, baseSlotID)
	expectExecFailure(t, ctx, db, "activated schedule accepted a late dose slot", `
		INSERT INTO dose_slots (
			id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
			slot_key,sort_order,local_time,quantity,meal_relation,label,created_at
		)
		SELECT '30000000-0000-4000-8000-000000000010',user_id,workspace_id,product_id,
		       product_plan_id,id,'late',7,'11:11',1,'unspecified','',CURRENT_TIMESTAMP
		FROM schedule_versions WHERE id=$1`, baseScheduleID)
	expectExecFailure(t, ctx, db, "direct schedule history deletion was accepted", `
		DELETE FROM schedule_versions WHERE id=$1`, baseScheduleID)
	expectExecFailure(t, ctx, db, "overlapping schedule version was accepted", `
		INSERT INTO schedule_versions (
			id,user_id,workspace_id,product_id,product_plan_id,product_profile_version_id,business_version,
			version_state,effective_from,effective_to,timezone_version_id,iana_timezone,
			timezone_ruleset,plan_start_date,weekdays,
			day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,
			long_cycle_enabled,long_take_weeks,long_rest_weeks,long_anchor_date,
			source,history_completeness,legacy_schedule_version,source_updated_at,created_at
		)
		SELECT '30000000-0000-4000-8000-000000000020',user_id,workspace_id,product_id,
		       product_plan_id,product_profile_version_id,99,'active',effective_from,effective_to,timezone_version_id,
		       iana_timezone,timezone_ruleset,plan_start_date,weekdays,
		       day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,
		       long_cycle_enabled,long_take_weeks,long_rest_weeks,long_anchor_date,
		       source,history_completeness,legacy_schedule_version,source_updated_at,CURRENT_TIMESTAMP
		FROM schedule_versions WHERE id=$1`, baseScheduleID)

	expectExecFailure(t, ctx, db, "plan state interval body was mutable", `
		UPDATE plan_state_intervals SET state='active'
		WHERE product_id='10000000-0000-4000-8000-000000000104'`)
	expectExecFailure(t, ctx, db, "direct plan state history deletion was accepted", `
		DELETE FROM plan_state_intervals
		WHERE product_id='10000000-0000-4000-8000-000000000104'`)

	expectExecFailure(t, ctx, db, "product plan accepted another tenant's current schedule", `
		UPDATE product_plans SET current_schedule_version_id=$1 WHERE id=$2`, otherScheduleID, basePlanID)

	var otherTimezoneID string
	if err := db.QueryRowContext(ctx, `
		SELECT timezone_version_id FROM schedule_versions WHERE id=$1`, otherScheduleID,
	).Scan(&otherTimezoneID); err != nil {
		t.Fatal(err)
	}
	expectExecFailure(t, ctx, db, "schedule accepted another tenant's timezone", `
		INSERT INTO schedule_versions (
			id,user_id,workspace_id,product_id,product_plan_id,product_profile_version_id,business_version,
			version_state,effective_from,effective_to,timezone_version_id,iana_timezone,
			timezone_ruleset,plan_start_date,weekdays,
			day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,
			long_cycle_enabled,long_take_weeks,long_rest_weeks,long_anchor_date,
			source,history_completeness,legacy_schedule_version,source_updated_at,created_at
		)
		SELECT '30000000-0000-4000-8000-000000000030',user_id,workspace_id,product_id,
		       product_plan_id,product_profile_version_id,100,'cancelled',CURRENT_DATE + 10,CURRENT_DATE + 10,$1,
		       iana_timezone,timezone_ruleset,plan_start_date,weekdays,
		       day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,
		       long_cycle_enabled,long_take_weeks,long_rest_weeks,long_anchor_date,
		       source,history_completeness,legacy_schedule_version,source_updated_at,CURRENT_TIMESTAMP
		FROM schedule_versions WHERE id=$2`, otherTimezoneID, baseScheduleID)

	insertOccurrence(t, ctx, db, fixtureProductID, 2)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM scheduled_occurrences
		WHERE id=r1_scheduled_occurrence_id(schedule_version_id,local_date,dose_slot_id)
		  AND product_id='10000000-0000-4000-8000-000000000101'
		  AND local_date=CURRENT_DATE + 2`, 1)
	expectExecFailure(t, ctx, db, "occurrence natural key was not deterministic", `
		INSERT INTO scheduled_occurrences (
			id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
			dose_slot_id,timezone_version_id,local_date,local_time,scheduled_at,iana_timezone,
			timezone_ruleset,utc_offset_minutes,time_resolution,planned_quantity,
			unit_snapshot,meal_relation_snapshot,slot_label_snapshot,materialized_at
		)
		SELECT '30000000-0000-4000-8000-000000000102',user_id,workspace_id,product_id,
		       product_plan_id,schedule_version_id,dose_slot_id,timezone_version_id,local_date,local_time,
		       scheduled_at,iana_timezone,timezone_ruleset,utc_offset_minutes,
		       time_resolution,planned_quantity,unit_snapshot,meal_relation_snapshot,
		       slot_label_snapshot,CURRENT_TIMESTAMP
		FROM scheduled_occurrences
		WHERE product_id='10000000-0000-4000-8000-000000000101'
		  AND local_date=CURRENT_DATE + 2`)
	expectExecFailure(t, ctx, db, "occurrence accepted another tenant's dose slot", `
		INSERT INTO scheduled_occurrences (
			id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
			dose_slot_id,timezone_version_id,local_date,local_time,scheduled_at,iana_timezone,
			timezone_ruleset,utc_offset_minutes,time_resolution,planned_quantity,
			unit_snapshot,meal_relation_snapshot,slot_label_snapshot,materialized_at
		)
		SELECT r1_scheduled_occurrence_id(sv.id,CURRENT_DATE + 3,$1),sv.user_id,sv.workspace_id,
		       sv.product_id,sv.product_plan_id,sv.id,$1,sv.timezone_version_id,CURRENT_DATE + 3,
		       '10:15',CURRENT_TIMESTAMP,sv.iana_timezone,sv.timezone_ruleset,
		       0,'exact',1,'capsule','unspecified','',CURRENT_TIMESTAMP
		FROM schedule_versions sv WHERE sv.id=$2`, otherSlotID, baseScheduleID)

	if otherPlanID == basePlanID || otherSlotID == baseSlotID {
		t.Fatal("tenant fixtures unexpectedly share plan identities")
	}
}

func assertPlanParentCascade(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	productID := "10000000-0000-4000-8000-000000000106"
	if _, err := db.ExecContext(ctx, `DELETE FROM products WHERE id=$1`, productID); err != nil {
		t.Fatalf("parent Product cascade was blocked: %v", err)
	}
	assertCount(t, ctx, db, `
		SELECT count(*) FROM product_plans WHERE product_id='10000000-0000-4000-8000-000000000106'`, 0)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM schedule_versions WHERE product_id='10000000-0000-4000-8000-000000000106'`, 0)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM dose_slots WHERE product_id='10000000-0000-4000-8000-000000000106'`, 0)
}

func currentPlanRefs(t *testing.T, ctx context.Context, db *sql.DB, productID string) (string, string, string) {
	t.Helper()
	var planID, scheduleID, slotID string
	if err := db.QueryRowContext(ctx, `
		SELECT pp.id,sv.id,ds.id
		FROM product_plans pp
		JOIN schedule_versions sv ON sv.id=pp.current_schedule_version_id
		JOIN dose_slots ds ON ds.schedule_version_id=sv.id
		WHERE pp.product_id=$1
		ORDER BY ds.local_time
		LIMIT 1`, productID).Scan(&planID, &scheduleID, &slotID); err != nil {
		t.Fatalf("load current plan refs for %s: %v", productID, err)
	}
	return planID, scheduleID, slotID
}

func insertOccurrence(t *testing.T, ctx context.Context, db *sql.DB, productID string, daysAfterToday int) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO scheduled_occurrences (
			id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
			dose_slot_id,timezone_version_id,local_date,local_time,scheduled_at,iana_timezone,
			timezone_ruleset,utc_offset_minutes,time_resolution,planned_quantity,
			unit_snapshot,meal_relation_snapshot,slot_label_snapshot,materialized_at
		)
		SELECT r1_scheduled_occurrence_id(sv.id,CURRENT_DATE + $1::integer,ds.id),
		       sv.user_id,sv.workspace_id,sv.product_id,sv.product_plan_id,sv.id,ds.id,
		       sv.timezone_version_id,CURRENT_DATE + $1::integer,ds.local_time,CURRENT_TIMESTAMP,
		       sv.iana_timezone,sv.timezone_ruleset,0,'exact',ds.quantity,p.unit,
		       ds.meal_relation,ds.label,CURRENT_TIMESTAMP
		FROM product_plans pp
		JOIN schedule_versions sv ON sv.id=pp.current_schedule_version_id
		JOIN dose_slots ds ON ds.schedule_version_id=sv.id
		JOIN products p ON p.id=pp.product_id
		WHERE pp.product_id=$2
		ORDER BY ds.local_time
		LIMIT 1`, daysAfterToday, productID); err != nil {
		t.Fatalf("insert target occurrence: %v", err)
	}
}

func expectExecFailure(t *testing.T, ctx context.Context, db *sql.DB, message, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(ctx, query, args...); err == nil {
		t.Fatal(message)
	}
}

func queryCount(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
