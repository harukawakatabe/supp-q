package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"
)

const r1IntakeInventoryVersion int64 = 202609200004

func TestR1RiskReminderFreshAndCurrentSnapshotUpgrade(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}

	t.Run("fresh_and_conservative_backfill", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			configureGoose(t)
			if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatal(err)
			}
			seedLaunchSnapshot(t, ctx, db)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{
				"inventory_risk_projection_sets", "inventory_risk_projections",
				"reminder_preferences", "reminder_preference_versions",
				"reminder_window_preferences", "product_reminder_overrides",
				"reminder_events", "reminder_targets", "reminder_materialization_cursors",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx,
					`SELECT to_regclass(current_schema() || '.' || $1) IS NOT NULL`, table,
				).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected E6 table %s", table)
				}
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM reminder_preferences
				WHERE configuration_state='needs_confirmation'
				  AND aggregate_version=0 AND current_version_id IS NULL`, 1)
			for _, table := range []string{
				"reminder_preference_versions", "reminder_events", "reminder_targets",
				"inventory_risk_projections", "projection_revisions",
			} {
				assertCount(t, ctx, db, `SELECT count(*) FROM `+table, 0)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM products p
				JOIN reminder_preferences preference ON preference.workspace_id=p.workspace_id
				WHERE p.product_type IN ('otc','prescription')
				  AND (preference.configuration_state<>'needs_confirmation'
				       OR EXISTS (SELECT 1 FROM reminder_events event WHERE event.workspace_id=p.workspace_id))`, 0)

			if _, err := db.ExecContext(ctx, `
				INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,created_at)
				VALUES ('10000000-0000-4000-8000-000000000012',
				'10000000-0000-4000-8000-000000000001','registered','N-1 workspace',
				CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
				t.Fatal(err)
			}
			var inserted int
			if err := db.QueryRowContext(ctx, `SELECT r1_backfill_reminder_preferences()`).Scan(&inserted); err != nil {
				t.Fatal(err)
			}
			if inserted != 1 {
				t.Fatalf("N-1 catch-up inserted %d preferences, want 1", inserted)
			}
			if err := db.QueryRowContext(ctx, `SELECT r1_backfill_reminder_preferences()`).Scan(&inserted); err != nil {
				t.Fatal(err)
			}
			if inserted != 0 {
				t.Fatalf("idempotent catch-up inserted %d preferences", inserted)
			}
		})
	})

	t.Run("complete_revision_activation_and_supersession", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE6Fixture(t, ctx, db)
			seedRiskRevision(t, ctx, db,
				"10000000-0000-4000-8000-000000000701",
				"10000000-0000-4000-8000-000000000711", 1)
			seedRiskProductResult(t, ctx, db,
				"10000000-0000-4000-8000-000000000721",
				"10000000-0000-4000-8000-000000000701")
			if _, err := db.ExecContext(ctx, `
				UPDATE inventory_risk_projection_sets SET completed_at=CURRENT_TIMESTAMP
				WHERE projection_revision_id='10000000-0000-4000-8000-000000000701'`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `
				UPDATE projection_revisions SET projection_state='active',activated_at=CURRENT_TIMESTAMP
				WHERE id='10000000-0000-4000-8000-000000000701'`); err == nil {
				t.Fatal("incomplete inventory risk revision was activated")
			}
			seedRiskBatchResult(t, ctx, db,
				"10000000-0000-4000-8000-000000000722",
				"10000000-0000-4000-8000-000000000701")
			if _, err := db.ExecContext(ctx, `SELECT r1_activate_inventory_risk_revision(
				'10000000-0000-4000-8000-000000000701',CURRENT_TIMESTAMP)`); err != nil {
				t.Fatalf("activate complete revision: %v", err)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM projection_revisions
				WHERE id='10000000-0000-4000-8000-000000000701'
				  AND projection_state='active' AND activated_at IS NOT NULL`, 1)

			seedRiskRevision(t, ctx, db,
				"10000000-0000-4000-8000-000000000702",
				"10000000-0000-4000-8000-000000000712", 0)
			seedRiskProductResult(t, ctx, db,
				"10000000-0000-4000-8000-000000000723",
				"10000000-0000-4000-8000-000000000702")
			if _, err := db.ExecContext(ctx, `SELECT r1_activate_inventory_risk_revision(
				'10000000-0000-4000-8000-000000000702',CURRENT_TIMESTAMP)`); err != nil {
				t.Fatalf("activate replacement revision: %v", err)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM projection_revisions
				WHERE id='10000000-0000-4000-8000-000000000701'
				  AND projection_state='superseded'`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM projection_revisions
				WHERE id='10000000-0000-4000-8000-000000000702'
				  AND projection_state='active'`, 1)
			if _, err := db.ExecContext(ctx, `
				INSERT INTO inventory_risk_projections (
					id,projection_revision_id,user_id,workspace_id,product_id,result_scope,
					stock_state,risk_state,calculation_state,on_hand_quantity,
					auto_allocatable_quantity,expired_quantity,unknown_expiry_quantity,
					covered_occurrence_count,covered_plan_day_count,calculated_at
				) VALUES (
					'10000000-0000-4000-8000-000000000729',
					'10000000-0000-4000-8000-000000000702',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000101','product',
					'in_stock','safe','calculated',8,8,0,0,8,8,CURRENT_TIMESTAMP
				)`); err == nil {
				t.Fatal("active inventory risk revision accepted another result")
			}
		})
	})

	t.Run("preference_versions_event_dedupe_and_cursor_recovery", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE6Fixture(t, ctx, db)
			var preferenceID string
			if err := db.QueryRowContext(ctx, `
				SELECT id FROM reminder_preferences
				WHERE workspace_id='10000000-0000-4000-8000-000000000011'`).Scan(&preferenceID); err != nil {
				t.Fatal(err)
			}
			var version int
			if err := db.QueryRowContext(ctx, `
				SELECT r1_set_reminder_preference(
					$1,'10000000-0000-4000-8000-000000000801',0,true,
					'time_window_digest',false,true,30,true,true,'user_confirmed',
					'10000000-0000-4000-8000-000000000001',
					ARRAY['morning','noon','afternoon','evening'],
					ARRAY[true,true,true,true],
					ARRAY['08:00','12:00','14:00','21:00']::time[]
				)`, preferenceID).Scan(&version); err != nil {
				t.Fatalf("confirm reminder preference: %v", err)
			}
			if version != 1 {
				t.Fatalf("preference version = %d, want 1", version)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM reminder_preferences
				WHERE id='`+preferenceID+`' AND configuration_state='configured'
				  AND aggregate_version=1
				  AND current_version_id='10000000-0000-4000-8000-000000000801'`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM reminder_window_preferences
				WHERE reminder_preference_version_id='10000000-0000-4000-8000-000000000801'`, 4)
			if err := db.QueryRowContext(ctx, `
				SELECT r1_set_reminder_preference(
					$1,'10000000-0000-4000-8000-000000000802',1,true,
					'per_occurrence',true,true,30,true,true,'user_confirmed',
					'10000000-0000-4000-8000-000000000001',
					ARRAY['morning','noon','afternoon','evening'],
					ARRAY[true,true,true,true],
					ARRAY['08:00','12:00','14:00','21:00']::time[]
				)`, preferenceID).Scan(&version); err != nil {
				t.Fatalf("replace reminder preference: %v", err)
			}
			if version != 2 {
				t.Fatalf("replacement preference version = %d, want 2", version)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM reminder_preference_versions
				WHERE reminder_preference_id='`+preferenceID+`'
				  AND ((id='10000000-0000-4000-8000-000000000801' AND effective_to IS NOT NULL)
				    OR (id='10000000-0000-4000-8000-000000000802' AND effective_to IS NULL))`, 2)

			if _, err := db.ExecContext(ctx, `
				INSERT INTO reminder_events (
					id,user_id,workspace_id,reminder_preference_version_id,
					reminder_preference_id,event_type,source_type,source_id,source_revision,
					dedupe_key,scheduled_at,available_at,iana_timezone,channel,event_status,
					title_snapshot,body_snapshot,fact_snapshot,deep_link_path,created_at,updated_at
				) VALUES (
					'10000000-0000-4000-8000-000000000811',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000802',$1,
					'stock_low','inventory_risk_projection',
					'10000000-0000-4000-8000-000000000721',1,
					'risk:stock_low:product:episode-1',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,
					'Etc/UTC','in_app','available','库存待处理','还可覆盖 4 个计划服用日',
					'{"coveredPlanDayCount":4}'::jsonb,'/product/fixture',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
				)`, preferenceID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `
				INSERT INTO reminder_events (
					id,user_id,workspace_id,reminder_preference_version_id,
					reminder_preference_id,event_type,source_type,source_id,source_revision,
					dedupe_key,scheduled_at,available_at,iana_timezone,event_status,
					title_snapshot,created_at,updated_at
				) VALUES (
					'10000000-0000-4000-8000-000000000812',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000802',$1,
					'stock_low','inventory_risk_projection',
					'10000000-0000-4000-8000-000000000721',1,
					'risk:stock_low:product:episode-1',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,
					'Etc/UTC','available','duplicate',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
				)`, preferenceID); err == nil {
				t.Fatal("duplicate reminder dedupe key was accepted")
			}
			if _, err := db.ExecContext(ctx, `
				UPDATE reminder_events SET read_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
				WHERE id='10000000-0000-4000-8000-000000000811'`); err != nil {
				t.Fatalf("mark reminder read: %v", err)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM reminder_events
				WHERE id='10000000-0000-4000-8000-000000000811'
				  AND event_status='available' AND read_at IS NOT NULL`, 1)
			if _, err := db.ExecContext(ctx, `
				UPDATE reminder_events SET event_status='resolved',resolved_at=CURRENT_TIMESTAMP,
				updated_at=CURRENT_TIMESTAMP
				WHERE id='10000000-0000-4000-8000-000000000811'`); err != nil {
				t.Fatalf("resolve reminder: %v", err)
			}
			if _, err := db.ExecContext(ctx, `
				UPDATE reminder_events SET event_status='available',resolved_at=NULL,
				updated_at=CURRENT_TIMESTAMP
				WHERE id='10000000-0000-4000-8000-000000000811'`); err == nil {
				t.Fatal("terminal reminder event reopened")
			}

			if _, err := db.ExecContext(ctx, `
				INSERT INTO reminder_materialization_cursors (
					user_id,workspace_id,policy_partition,source_partition,cursor_version,
					last_processed_at,last_event_id,cursor_state,created_at,updated_at
				) VALUES (
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011','policy-v1','risk',0,
					'2026-09-26T08:00:00Z','10000000-0000-4000-8000-000000000901','idle',
					CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
				)`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `
				UPDATE reminder_materialization_cursors
				SET cursor_version=1,last_processed_at='2026-09-26T09:00:00Z',
				    last_event_id='10000000-0000-4000-8000-000000000902',updated_at=CURRENT_TIMESTAMP
				WHERE workspace_id='10000000-0000-4000-8000-000000000011'
				  AND policy_partition='policy-v1' AND source_partition='risk'`); err != nil {
				t.Fatalf("advance reminder cursor: %v", err)
			}
			if _, err := db.ExecContext(ctx, `
				UPDATE reminder_materialization_cursors
				SET cursor_version=2,last_processed_at='2026-09-26T08:30:00Z',
				    last_event_id='10000000-0000-4000-8000-000000000903',updated_at=CURRENT_TIMESTAMP
				WHERE workspace_id='10000000-0000-4000-8000-000000000011'
				  AND policy_partition='policy-v1' AND source_partition='risk'`); err == nil {
				t.Fatal("reminder cursor moved backwards")
			}
		})
	})

	t.Run("parent_account_cascade", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE6Fixture(t, ctx, db)
			if _, err := db.ExecContext(ctx, `DELETE FROM users
				WHERE id='10000000-0000-4000-8000-000000000001'`); err != nil {
				t.Fatalf("E6 parent account cascade was blocked: %v", err)
			}
			for _, table := range []string{
				"reminder_preferences", "reminder_preference_versions", "reminder_events",
				"reminder_targets", "inventory_risk_projection_sets", "inventory_risk_projections",
			} {
				assertCount(t, ctx, db, `SELECT count(*) FROM `+table, 0)
			}
		})
	})

	t.Run("down_before_and_after_target_writes", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE6Fixture(t, ctx, db)
			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", r1IntakeInventoryVersion); err != nil {
				t.Fatalf("rollback migration-only E6 data: %v", err)
			}
			var exists bool
			if err := db.QueryRowContext(ctx, `SELECT to_regclass(
				current_schema() || '.reminder_preferences') IS NOT NULL`).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists {
				t.Fatal("E6 rollback left reminder tables behind")
			}
		})

		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE6Fixture(t, ctx, db)
			var preferenceID string
			if err := db.QueryRowContext(ctx, `SELECT id FROM reminder_preferences LIMIT 1`).Scan(&preferenceID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `UPDATE reminder_preferences
				SET configuration_state='disabled',aggregate_version=1,
				    current_version_id='10000000-0000-4000-8000-000000000899'
				WHERE id=$1`, preferenceID); err == nil {
				// The FK intentionally rejects a fake current version; use the supported function below.
				t.Fatal("fake reminder preference version was accepted")
			}
			if _, err := db.ExecContext(ctx, `SELECT r1_set_reminder_preference(
				$1,'10000000-0000-4000-8000-000000000899',0,false,
				'time_window_digest',false,true,30,true,true,'user_confirmed',
				'10000000-0000-4000-8000-000000000001',
				ARRAY['morning','noon','afternoon','evening'],ARRAY[true,true,true,true],
				ARRAY['08:00','12:00','14:00','21:00']::time[])`, preferenceID); err != nil {
				t.Fatal(err)
			}
			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", r1IntakeInventoryVersion); err == nil {
				t.Fatal("E6 Down accepted target-owned reminder data")
			}
		})
	})
}

func applyE6Fixture(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	configureGoose(t)
	if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
		t.Fatalf("apply launch snapshot: %v", err)
	}
	seedLaunchSnapshot(t, ctx, db)
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("upgrade launch fixture through E6: %v", err)
	}
}

func seedRiskRevision(t *testing.T, ctx context.Context, db *sql.DB, revisionID, setID string, expectedBatches int) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO projection_revisions (
			id,user_id,workspace_id,projection_type,scope_type,scope_id,revision,
			projection_state,source_revision_vector,created_at
		) VALUES (
			$1,'10000000-0000-4000-8000-000000000001',
			'10000000-0000-4000-8000-000000000011','inventory_risk','product',
			'10000000-0000-4000-8000-000000000101',
			(SELECT COALESCE(max(revision),0)+1 FROM projection_revisions
			 WHERE workspace_id='10000000-0000-4000-8000-000000000011'
			   AND projection_type='inventory_risk'
			   AND scope_id='10000000-0000-4000-8000-000000000101'),
			'building','{"inventoryVersion":1,"scheduleVersion":1,"timezoneVersion":1}'::jsonb,
			CURRENT_TIMESTAMP
		)`, revisionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO inventory_risk_projection_sets (
			projection_revision_id,user_id,workspace_id,product_id,expected_batch_count,created_at
		) VALUES ($1,'10000000-0000-4000-8000-000000000001',
			'10000000-0000-4000-8000-000000000011',
			'10000000-0000-4000-8000-000000000101',$2,CURRENT_TIMESTAMP)`, revisionID, expectedBatches); err != nil {
		t.Fatalf("seed risk set %s: %v", setID, err)
	}
}

func seedRiskProductResult(t *testing.T, ctx context.Context, db *sql.DB, resultID, revisionID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO inventory_risk_projections (
			id,projection_revision_id,user_id,workspace_id,product_id,result_scope,
			stock_state,risk_state,calculation_state,on_hand_quantity,
			auto_allocatable_quantity,expired_quantity,unknown_expiry_quantity,
			covered_occurrence_count,covered_plan_day_count,calculated_at
		) VALUES ($1,$2,
			'10000000-0000-4000-8000-000000000001',
			'10000000-0000-4000-8000-000000000011',
			'10000000-0000-4000-8000-000000000101','product',
			'low','near_expiry','calculated',8,8,0,0,4,4,CURRENT_TIMESTAMP
		)`, resultID, revisionID); err != nil {
		t.Fatalf("seed product risk result: %v", err)
	}
}

func seedRiskBatchResult(t *testing.T, ctx context.Context, db *sql.DB, resultID, revisionID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO inventory_risk_projections (
			id,projection_revision_id,user_id,workspace_id,product_id,result_scope,batch_id,
			risk_state,expiry_precision,expiry_period_start,expiry_period_end,
			precision_limited,projected_remaining_at_expiry,calculated_at
		) VALUES ($1,$2,
			'10000000-0000-4000-8000-000000000001',
			'10000000-0000-4000-8000-000000000011',
			'10000000-0000-4000-8000-000000000101','batch',
			'10000000-0000-4000-8000-000000000201','near_expiry','day',
			'2027-12-31','2027-12-31',false,0,CURRENT_TIMESTAMP
		)`, resultID, revisionID); err != nil {
		t.Fatalf("seed batch risk result: %v", err)
	}
}
