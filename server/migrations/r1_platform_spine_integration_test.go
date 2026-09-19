package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

const launchCleanupVersion int64 = 202608210001

func TestR1PlatformSpineFreshAndCurrentSnapshotUpgrade(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}

	t.Run("fresh", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyAll(t, ctx, db)
			for _, table := range []string{
				"workspace_timezone_versions",
				"client_actions",
				"domain_changes",
				"domain_change_consumer_receipts",
				"projection_revisions",
				"migration_quarantines",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx, `SELECT to_regclass(current_schema() || '.' || $1) IS NOT NULL`, table).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected table %s", table)
				}
			}
		})
	})

	t.Run("current_snapshot", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			configureGoose(t)
			if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatalf("apply launch snapshot: %v", err)
			}
			seedLaunchSnapshot(t, ctx, db)

			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("upgrade current snapshot: %v", err)
			}

			assertCount(t, ctx, db, `SELECT count(*) FROM products`, 3)
			assertCount(t, ctx, db, `SELECT count(*) FROM product_schedules`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM inventory_batches WHERE current_quantity = 8`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM inventory_events`, 2)
			assertCount(t, ctx, db, `SELECT count(*) FROM intake_records WHERE status = 'active'`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM intake_allocations WHERE quantity = 2`, 1)

			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM workspaces w
				JOIN workspace_timezone_versions tz ON tz.id = w.current_timezone_version_id
				WHERE tz.workspace_id = w.id
				  AND tz.user_id = w.owner_user_id
				  AND tz.business_version = 1
				  AND tz.iana_timezone = 'Etc/UTC'
				  AND tz.confirmation_state = 'needs_confirmation'
				  AND tz.source = 'legacy_unspecified'
				  AND tz.effective_to IS NULL`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM migration_quarantines q
				JOIN products p ON p.id = q.source_id
				WHERE q.source_table = 'products'
				  AND q.reason_code = 'unsupported_product_type'
				  AND q.state = 'open'
				  AND p.product_type IN ('otc', 'prescription')`, 2)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM migration_quarantines q
				JOIN products p ON p.id = q.source_id
				WHERE p.product_type = 'supplement'`, 0)
			assertCount(t, ctx, db, `SELECT count(*) FROM client_actions`, 0)
			assertCount(t, ctx, db, `SELECT count(*) FROM domain_changes`, 0)

			assertPlatformConstraints(t, ctx, db)

			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatalf("rollback before target writes: %v", err)
			}
			assertCount(t, ctx, db, `SELECT count(*) FROM products`, 3)
			var pointerExists bool
			if err := db.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_schema = current_schema()
					  AND table_name = 'workspaces'
					  AND column_name = 'current_timezone_version_id'
				)`).Scan(&pointerExists); err != nil {
				t.Fatal(err)
			}
			if pointerExists {
				t.Fatal("rollback left the workspace timezone pointer behind")
			}
		})
	})
}

func withMigrationSchema(t *testing.T, base string, run func(context.Context, *sql.DB)) {
	t.Helper()
	ctx := context.Background()
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	schema := fmt.Sprintf("suppq_migration_test_%d", time.Now().UnixNano())
	if _, err = admin.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
	})

	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	run(ctx, db)
}

func configureGoose(t *testing.T) {
	t.Helper()
	goose.SetBaseFS(files)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
}

func applyAll(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	configureGoose(t)
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("apply all migrations: %v", err)
	}
}

func seedLaunchSnapshot(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	now := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
	userID := "10000000-0000-4000-8000-000000000001"
	workspaceID := "10000000-0000-4000-8000-000000000011"
	execSeed(t, ctx, db, `
		INSERT INTO users (id,kind,status,role,last_activity_at,created_at)
		VALUES ($1,'registered','active','member',$2,$2)`, userID, now)
	execSeed(t, ctx, db, `
		INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,created_at)
		VALUES ($1,$2,'registered','migration fixture',$3,$3)`, workspaceID, userID, now)
	execSeed(t, ctx, db, `
		INSERT INTO workspace_members (workspace_id,user_id,role,created_at)
		VALUES ($1,$2,'owner',$3)`, workspaceID, userID, now)

	products := []struct {
		id          string
		name        string
		productType string
	}{
		{"10000000-0000-4000-8000-000000000101", "fixture supplement", "supplement"},
		{"10000000-0000-4000-8000-000000000102", "fixture otc", "otc"},
		{"10000000-0000-4000-8000-000000000103", "fixture prescription", "prescription"},
	}
	for _, product := range products {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO products (
				id,user_id,workspace_id,name,brand,product_type,status,unit,
				dose_quantity,dose_times_per_day,ingredient_serving_quantity,
				restock_threshold_days,expiry_reminder_days,created_at,updated_at
			) VALUES ($1,$2,$3,$4,'fixture',$5,'active','capsule',1,1,1,7,30,$6,$6)`,
			product.id, userID, workspaceID, product.name, product.productType, now); err != nil {
			t.Fatal(err)
		}
	}

	productID := products[0].id
	batchID := "10000000-0000-4000-8000-000000000201"
	intakeID := "10000000-0000-4000-8000-000000000301"
	execSeed(t, ctx, db, `
		INSERT INTO product_ingredients (
			id,user_id,workspace_id,product_id,ingredient_key,name,amount,unit,created_at
		) VALUES ('10000000-0000-4000-8000-000000000401',$1,$2,$3,'fixture','Fixture',100,'mg',$4)`,
		userID, workspaceID, productID, now)
	execSeed(t, ctx, db, `
		INSERT INTO product_schedules (
			product_id,user_id,workspace_id,version,start_date,weekdays,
			day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,
			long_cycle_enabled,long_take_weeks,long_rest_weeks,long_start_date,
			reminder_times,created_at,updated_at
		) VALUES ($3,$1,$2,1,'2026-09-01',ARRAY[0,1,2,3,4,5,6]::smallint[],
			false,28,28,'2026-09-01',false,6,4,'2026-09-01',ARRAY['09:00'],$4,$4);
		`, userID, workspaceID, productID, now)
	execSeed(t, ctx, db, `
		INSERT INTO day_cycle_versions (
			id,user_id,workspace_id,product_id,effective_date,enabled,cycle_days,take_days,anchor_date,created_at
		) VALUES ('10000000-0000-4000-8000-000000000501',$1,$2,$3,'2026-09-01',false,28,28,'2026-09-01',$4)`,
		userID, workspaceID, productID, now)
	execSeed(t, ctx, db, `
		INSERT INTO inventory_batches (
			id,user_id,workspace_id,product_id,initial_quantity,current_quantity,expiry_date,price_cny,created_at
		) VALUES ($1,$2,$3,$4,10,8,'2027-12-31',100,$5)`,
		batchID, userID, workspaceID, productID, now)
	execSeed(t, ctx, db, `
		INSERT INTO intake_records (
			id,user_id,workspace_id,product_id,intake_date,intake_time,quantity,source,status,idempotency_key,note,created_at
		) VALUES ($1,$2,$3,$4,'2026-09-19','09:00',2,'scheduled','active','fixture-intake','',$5)`,
		intakeID, userID, workspaceID, productID, now)
	execSeed(t, ctx, db, `
		INSERT INTO intake_allocations (intake_id,batch_id,quantity,unit_cost_cny)
		VALUES ($1,$2,2,10)`, intakeID, batchID)
	execSeed(t, ctx, db, `
		INSERT INTO inventory_events (
			id,user_id,workspace_id,product_id,batch_id,intake_id,kind,quantity_delta,created_at
		) VALUES ('10000000-0000-4000-8000-000000000601',$1,$2,$3,$4,NULL,'opening',10,$5)`,
		userID, workspaceID, productID, batchID, now)
	execSeed(t, ctx, db, `
		INSERT INTO inventory_events (
			id,user_id,workspace_id,product_id,batch_id,intake_id,kind,quantity_delta,created_at
		) VALUES ('10000000-0000-4000-8000-000000000602',$1,$2,$3,$4,$5,'intake',-2,$6)`,
		userID, workspaceID, productID, batchID, intakeID, now)
}

func execSeed(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func assertPlatformConstraints(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO client_actions (
			workspace_id,client_action_id,user_id,action_type,request_hash,action_state,
			expires_at,created_at,updated_at
		) VALUES (
			'10000000-0000-4000-8000-000000000011',
			'10000000-0000-4000-8000-000000000701',
			'10000000-0000-4000-8000-000000000001',
			'fixture.invalid_hash','not-a-sha256','received',
			'2026-12-01T00:00:00Z','2026-09-19T00:00:00Z','2026-09-19T00:00:00Z'
		)`)
	if err == nil {
		t.Fatal("client_actions accepted an invalid request hash")
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO projection_revisions (
			id,user_id,workspace_id,projection_type,scope_type,scope_id,revision,
			projection_state,source_revision_vector,created_at
		) VALUES (
			'10000000-0000-4000-8000-000000000801',
			'10000000-0000-4000-8000-000000000001',
			'10000000-0000-4000-8000-000000000011',
			'fixture','product','10000000-0000-4000-8000-000000000101',1,
			'active','{}'::jsonb,'2026-09-19T00:00:00Z'
		)`)
	if err == nil {
		t.Fatal("projection_revisions accepted active state without activated_at")
	}
}

func assertCount(t *testing.T, ctx context.Context, db *sql.DB, query string, expected int) {
	t.Helper()
	var actual int
	if err := db.QueryRowContext(ctx, query).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("count mismatch: got %d want %d for %s", actual, expected, query)
	}
}
