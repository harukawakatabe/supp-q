package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"
)

const r1CaptureEvidenceVersion int64 = 202609200003

func TestR1IntakeInventoryFreshAndCurrentSnapshotUpgrade(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}

	t.Run("fresh", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyAll(t, ctx, db)
			for _, table := range []string{"intake_status_facts", "r1_intake_inventory_backfill_state"} {
				var exists bool
				if err := db.QueryRowContext(ctx,
					`SELECT to_regclass(current_schema() || '.' || $1) IS NOT NULL`, table,
				).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected E5 table %s", table)
				}
			}
			for _, signature := range []string{
				"r1_backfill_intake_inventory_batch(integer)",
				"r1_backfill_intake_inventory()",
				"r1_prepare_inventory_batch()",
				"r1_prepare_intake_record()",
				"r1_prepare_inventory_event()",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx, `SELECT to_regprocedure($1) IS NOT NULL`, signature).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected E5 function %s", signature)
				}
			}
		})
	})

	t.Run("snapshot_backfill_reconciliation_and_constraints", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE5Fixture(t, ctx, db)

			assertCount(t, ctx, db, `
				SELECT count(*) FROM inventory_batches
				WHERE aggregate_version=2 AND lifecycle_state='in_use'
				  AND unit_snapshot='capsule' AND received_quantity=initial_quantity
				  AND received_at=created_at::date AND expiry_raw_value='2027-12-31'
				  AND expiry_precision='day' AND currency='CNY'
				  AND source_kind='legacy_migration' AND updated_at=created_at`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM intake_records
				WHERE aggregate_version=1 AND occurred_at IS NOT NULL
				  AND occurred_time_precision='minute' AND iana_timezone_snapshot='Etc/UTC'
				  AND timezone_version_id IS NOT NULL
				  AND product_profile_version_id IS NOT NULL
				  AND ingredient_profile_version_id IS NOT NULL
				  AND history_completeness='legacy_current_only'
				  AND source_kind='legacy_migration'`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM intake_status_facts
				WHERE status='active' AND business_version=1
				  AND source_kind='legacy_migration'`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM inventory_events
				WHERE ledger_kind IS NOT NULL AND source_type IS NOT NULL AND source_id IS NOT NULL
				  AND posted_at=created_at AND actor_user_id=user_id
				  AND batch_version_before IS NOT NULL AND balance_after IS NOT NULL
				  AND source_kind='legacy_migration'`, 2)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM intake_allocations a
				JOIN inventory_events e ON e.id=a.inventory_event_id
				WHERE a.user_id=e.user_id AND a.workspace_id=e.workspace_id
				  AND a.product_id=e.product_id AND a.allocation_mode='migration_existing'
				  AND a.batch_version_as_allocated=e.batch_version_before
				  AND a.batch_balance_before-a.batch_balance_after=a.quantity
				  AND a.source_kind='legacy_migration'`, 1)

			assertE5Reconciliation(t, ctx, db)
			before := []int{
				queryCount(t, ctx, db, `SELECT count(*) FROM intake_status_facts`),
				queryCount(t, ctx, db, `SELECT count(*) FROM inventory_events`),
				queryCount(t, ctx, db, `SELECT count(*) FROM intake_allocations`),
			}
			if _, err := db.ExecContext(ctx, `SELECT r1_backfill_intake_inventory()`); err != nil {
				t.Fatalf("rerun E5 backfill: %v", err)
			}
			after := []int{
				queryCount(t, ctx, db, `SELECT count(*) FROM intake_status_facts`),
				queryCount(t, ctx, db, `SELECT count(*) FROM inventory_events`),
				queryCount(t, ctx, db, `SELECT count(*) FROM intake_allocations`),
			}
			for index := range before {
				if before[index] != after[index] {
					t.Fatalf("E5 backfill was not idempotent: before=%v after=%v", before, after)
				}
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM r1_intake_inventory_backfill_state
				WHERE singleton AND cycle_state='idle' AND cycle_id IS NULL
				  AND cursor_id IS NULL AND last_completed_at IS NOT NULL`, 1)

			if _, err := db.ExecContext(ctx, `
				UPDATE inventory_events SET note='forbidden mutation'
				WHERE id='10000000-0000-4000-8000-000000000601'`); err == nil {
				t.Fatal("inventory event accepted an in-place update")
			}
			if _, err := db.ExecContext(ctx, `
				UPDATE intake_status_facts SET status='revoked'
				WHERE intake_id='10000000-0000-4000-8000-000000000301'`); err == nil {
				t.Fatal("intake status fact accepted an in-place update")
			}
			if _, err := db.ExecContext(ctx, `
				UPDATE intake_allocations SET quantity=1
				WHERE intake_id='10000000-0000-4000-8000-000000000301'`); err == nil {
				t.Fatal("intake allocation accepted a core-fact update")
			}

			execSeed(t, ctx, db, `
				INSERT INTO inventory_batches (
					id,user_id,workspace_id,product_id,initial_quantity,current_quantity,
					expiry_date,price_cny,ingredient_profile_version_id,created_at
				) VALUES (
					'10000000-0000-4000-8000-000000000299',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000102',1,1,NULL,0,NULL,
					'2026-09-20T10:00:00Z'
				)`)
			if _, err := db.ExecContext(ctx, `
				INSERT INTO intake_allocations (intake_id,batch_id,quantity,unit_cost_cny)
				VALUES (
					'10000000-0000-4000-8000-000000000301',
					'10000000-0000-4000-8000-000000000299',1,0
				)`); err == nil {
				t.Fatal("cross-product allocation was accepted")
			}

			if _, err := db.ExecContext(ctx, `
				INSERT INTO inventory_events (
					id,user_id,workspace_id,product_id,batch_id,kind,ledger_kind,
					source_type,source_id,quantity_delta,created_at
				) VALUES (
					'10000000-0000-4000-8000-000000000699',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000101',
					'10000000-0000-4000-8000-000000000201','opening','opening','batch',
					'10000000-0000-4000-8000-000000000201',0,'2026-09-20T10:00:00Z'
				)`); err == nil {
				t.Fatal("duplicate inventory source identity was accepted")
			}
		})
	})

	t.Run("down_before_target_writes", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE5Fixture(t, ctx, db)
			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", r1CaptureEvidenceVersion); err != nil {
				t.Fatalf("rollback migration-only E5 data: %v", err)
			}
			var exists bool
			if err := db.QueryRowContext(ctx,
				`SELECT to_regclass(current_schema() || '.intake_status_facts') IS NOT NULL`,
			).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists {
				t.Fatal("E5 rollback left target tables behind")
			}
			assertCount(t, ctx, db, `SELECT count(*) FROM intake_records`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM inventory_events`, 2)
		})
	})

	t.Run("n_minus_one_catch_up", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE5Fixture(t, ctx, db)
			execSeed(t, ctx, db, `
				UPDATE inventory_batches SET current_quantity=7
				WHERE id='10000000-0000-4000-8000-000000000201'`)
			execSeed(t, ctx, db, `
				INSERT INTO intake_records (
					id,user_id,workspace_id,product_id,intake_date,intake_time,quantity,
					source,status,idempotency_key,note,created_at
				) VALUES (
					'10000000-0000-4000-8000-000000000302',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000101',
					'2026-09-20','09:00',1,'ad_hoc','active','n-minus-one','','2026-09-20T09:00:00Z'
				)`)
			execSeed(t, ctx, db, `
				INSERT INTO intake_allocations (intake_id,batch_id,quantity,unit_cost_cny)
				VALUES (
					'10000000-0000-4000-8000-000000000302',
					'10000000-0000-4000-8000-000000000201',1,10
				)`)
			execSeed(t, ctx, db, `
				INSERT INTO inventory_events (
					id,user_id,workspace_id,product_id,batch_id,intake_id,kind,quantity_delta,created_at
				) VALUES (
					'10000000-0000-4000-8000-000000000603',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000101',
					'10000000-0000-4000-8000-000000000201',
					'10000000-0000-4000-8000-000000000302','intake',-1,'2026-09-20T09:00:00Z'
				)`)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM intake_allocations
				WHERE intake_id='10000000-0000-4000-8000-000000000302'
				  AND inventory_event_id IS NULL`, 1)

			complete := false
			for attempt := 0; attempt < 20 && !complete; attempt++ {
				if err := db.QueryRowContext(ctx, `SELECT r1_backfill_intake_inventory_batch(1)`).Scan(&complete); err != nil {
					t.Fatalf("resume E5 N-1 catch-up: %v", err)
				}
			}
			if !complete {
				t.Fatal("E5 N-1 catch-up did not complete")
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM intake_allocations allocation
				JOIN inventory_events event ON event.id=allocation.inventory_event_id
				WHERE allocation.intake_id='10000000-0000-4000-8000-000000000302'
				  AND allocation.source_kind='legacy_n1'
				  AND allocation.batch_version_as_allocated=event.batch_version_before
				  AND allocation.batch_balance_after=event.balance_after`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM r1_intake_inventory_backfill_state
				WHERE singleton AND cycle_state='idle' AND attempt_count=2
				  AND source_count=8 AND source_snapshot_hash IS NOT NULL
				  AND last_progress_at IS NOT NULL`, 1)
			assertE5Reconciliation(t, ctx, db)
		})
	})

	t.Run("parent_account_cascade", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE5Fixture(t, ctx, db)
			if _, err := db.ExecContext(ctx, `
				DELETE FROM users
				WHERE id='10000000-0000-4000-8000-000000000001'`); err != nil {
				t.Fatalf("E5 parent account cascade was blocked: %v", err)
			}
			for _, table := range []string{
				"intake_status_facts", "intake_allocations", "inventory_events",
				"intake_records", "inventory_batches",
			} {
				assertCount(t, ctx, db, `SELECT count(*) FROM `+table, 0)
			}
		})
	})

	t.Run("down_refuses_target_writes", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyE5Fixture(t, ctx, db)
			if _, err := db.ExecContext(ctx, `
				UPDATE inventory_batches SET source_kind='application_current'
				WHERE id='10000000-0000-4000-8000-000000000201'`); err != nil {
				t.Fatal(err)
			}
			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", r1CaptureEvidenceVersion); err == nil {
				t.Fatal("E5 Down accepted target-owned inventory data")
			}
		})
	})
}

func applyE5Fixture(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	configureGoose(t)
	if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
		t.Fatalf("apply launch snapshot: %v", err)
	}
	seedLaunchSnapshot(t, ctx, db)
	if err := goose.UpToContext(ctx, db, ".", r1CaptureEvidenceVersion); err != nil {
		t.Fatalf("upgrade fixture through E4: %v", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("upgrade E4 fixture through E5: %v", err)
	}
}

func assertE5Reconciliation(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	assertCount(t, ctx, db, `
		WITH replay AS (
			SELECT b.id,b.current_quantity,COALESCE(sum(e.quantity_delta),0) replay_quantity
			FROM inventory_batches b LEFT JOIN inventory_events e ON e.batch_id=b.id
			GROUP BY b.id,b.current_quantity
		)
		SELECT count(*) FROM replay WHERE current_quantity<>replay_quantity`, 0)
	assertCount(t, ctx, db, `
		WITH allocation_totals AS (
			SELECT i.id,i.quantity,COALESCE(sum(a.quantity),0) allocated
			FROM intake_records i LEFT JOIN intake_allocations a ON a.intake_id=i.id
			GROUP BY i.id,i.quantity
		)
		SELECT count(*) FROM allocation_totals WHERE quantity<>allocated`, 0)
	assertCount(t, ctx, db, `
		SELECT count(*) FROM intake_allocations a
		JOIN intake_records i ON i.id=a.intake_id
		JOIN inventory_batches b ON b.id=a.batch_id
		WHERE a.user_id<>i.user_id OR a.workspace_id<>i.workspace_id OR a.product_id<>i.product_id
		   OR a.user_id<>b.user_id OR a.workspace_id<>b.workspace_id OR a.product_id<>b.product_id`, 0)
}
