package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestR1ProductProfilesFreshAndCurrentSnapshotUpgrade(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}

	t.Run("fresh", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyAll(t, ctx, db)
			for _, table := range []string{
				"product_profile_versions",
				"ingredient_profile_versions",
				"ingredient_profile_items",
				"product_media_links",
				"product_deletion_jobs",
				"r1_product_profile_backfill_state",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx, `SELECT to_regclass(current_schema() || '.' || $1) IS NOT NULL`, table).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected E2 table %s", table)
				}
			}
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM information_schema.columns
				WHERE table_schema=current_schema()
				  AND ((table_name='products' AND column_name IN (
				      'catalog_state','aggregate_version','current_product_profile_version_id','current_ingredient_profile_version_id'
				  )) OR (table_name='inventory_batches' AND column_name='ingredient_profile_version_id'))`, 5)
		})
	})

	t.Run("current_snapshot", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			configureGoose(t)
			if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatalf("apply launch snapshot: %v", err)
			}
			seedLaunchSnapshot(t, ctx, db)
			seedAdditionalSupportedProducts(t, ctx, db)

			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("upgrade current snapshot: %v", err)
			}

			assertCount(t, ctx, db, `SELECT count(*) FROM products`, 5)
			assertCount(t, ctx, db, `SELECT count(*) FROM product_profile_versions`, 3)
			assertCount(t, ctx, db, `SELECT count(*) FROM ingredient_profile_versions`, 3)
			assertCount(t, ctx, db, `SELECT count(*) FROM ingredient_profile_items`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM ingredient_profile_versions WHERE profile_status='confirmed_empty'`, 0)
			assertCount(t, ctx, db, `SELECT count(*) FROM ingredient_profile_versions WHERE profile_status='partial' AND change_kind='legacy_import' AND history_completeness='legacy_current_only'`, 3)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM products p
				JOIN product_profile_versions ppv ON ppv.id=p.current_product_profile_version_id
				JOIN ingredient_profile_versions ipv ON ipv.id=p.current_ingredient_profile_version_id
				WHERE p.product_type='supplement'
				  AND ppv.product_id=p.id AND ppv.user_id=p.user_id AND ppv.workspace_id=p.workspace_id
				  AND ipv.product_id=p.id AND ipv.user_id=p.user_id AND ipv.workspace_id=p.workspace_id
				  AND ppv.business_version=1 AND ipv.business_version=1
				  AND ppv.source='legacy_current'`, 3)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM products
				WHERE product_type='supplement'
				  AND catalog_state=CASE WHEN status='archived' THEN 'archived' ELSE 'in_cabinet' END
				  AND aggregate_version=1`, 3)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM products
				WHERE product_type IN ('otc','prescription')
				  AND catalog_state IS NULL
				  AND aggregate_version IS NULL
				  AND current_product_profile_version_id IS NULL
				  AND current_ingredient_profile_version_id IS NULL`, 2)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM ingredient_profile_items target
				JOIN product_ingredients source ON source.id=target.legacy_ingredient_id
				WHERE target.original_key=source.ingredient_key
				  AND target.original_name=source.name
				  AND target.label_amount=source.amount
				  AND target.label_unit=source.unit
				  AND target.mapping_status='unmapped'
				  AND target.ingredient_definition_id IS NULL`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM inventory_batches b
				JOIN products p ON p.id=b.product_id AND p.user_id=b.user_id AND p.workspace_id=b.workspace_id
				JOIN ingredient_profile_versions ipv
				  ON ipv.id=b.ingredient_profile_version_id
				 AND ipv.product_id=b.product_id
				 AND ipv.user_id=b.user_id
				 AND ipv.workspace_id=b.workspace_id
				WHERE p.product_type='supplement'`, 1)

			var profilesBefore, itemsBefore int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM ingredient_profile_versions`).Scan(&profilesBefore); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM ingredient_profile_items`).Scan(&itemsBefore); err != nil {
				t.Fatal(err)
			}
			var cycleComplete bool
			if err := db.QueryRowContext(ctx, `SELECT r1_backfill_product_profiles_batch(1)`).Scan(&cycleComplete); err != nil {
				t.Fatalf("start resumable E2 backfill: %v", err)
			}
			if cycleComplete {
				t.Fatal("one-row batch unexpectedly completed a three-product cycle")
			}
			assertCount(t, ctx, db, `
					SELECT count(*) FROM r1_product_profile_backfill_state
					WHERE singleton AND cycle_state='running'
					  AND cursor_product_id IS NOT NULL AND processed_count=1`, 1)
			for attempts := 0; attempts < 10 && !cycleComplete; attempts++ {
				if err := db.QueryRowContext(ctx, `SELECT r1_backfill_product_profiles_batch(1)`).Scan(&cycleComplete); err != nil {
					t.Fatalf("resume E2 backfill: %v", err)
				}
			}
			if !cycleComplete {
				t.Fatal("resumable E2 backfill did not complete")
			}
			assertCount(t, ctx, db, `SELECT count(*) FROM ingredient_profile_versions`, profilesBefore)
			assertCount(t, ctx, db, `SELECT count(*) FROM ingredient_profile_items`, itemsBefore)
			assertCount(t, ctx, db, `
					SELECT count(*) FROM r1_product_profile_backfill_state
					WHERE singleton AND cycle_state='idle' AND cursor_product_id IS NULL
					  AND processed_count=3 AND last_completed_at IS NOT NULL`, 1)

			if _, err := db.ExecContext(ctx, `
					UPDATE products SET catalog_state='deleting'
					WHERE id='10000000-0000-4000-8000-000000000101';
					SELECT r1_backfill_product_profiles();`); err != nil {
				t.Fatalf("rerun E2 backfill with an explicit target catalog state: %v", err)
			}
			assertCount(t, ctx, db, `
					SELECT count(*) FROM products
					WHERE id='10000000-0000-4000-8000-000000000101'
					  AND catalog_state='deleting'`, 1)
			if _, err := db.ExecContext(ctx, `
					UPDATE products SET catalog_state='in_cabinet'
					WHERE id='10000000-0000-4000-8000-000000000101'`); err != nil {
				t.Fatal(err)
			}

			assertE2Constraints(t, ctx, db)

			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatalf("rollback before target application cutover: %v", err)
			}
			assertCount(t, ctx, db, `SELECT count(*) FROM products`, 5)
			assertCount(t, ctx, db, `SELECT count(*) FROM product_ingredients`, 1)
			var pointerExists bool
			if err := db.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_schema=current_schema()
					  AND table_name='products'
					  AND column_name='current_product_profile_version_id'
				)`).Scan(&pointerExists); err != nil {
				t.Fatal(err)
			}
			if pointerExists {
				t.Fatal("rollback left E2 product profile pointer behind")
			}
		})
	})

	t.Run("legacy_drift_creates_next_snapshot", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			configureGoose(t)
			if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatalf("apply launch snapshot: %v", err)
			}
			seedLaunchSnapshot(t, ctx, db)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("upgrade current snapshot: %v", err)
			}

			if _, err := db.ExecContext(ctx, `
				UPDATE products
				SET name='fixture supplement drifted', brand='drifted brand',
				    updated_at='2026-09-20T09:00:00Z'
				WHERE id='10000000-0000-4000-8000-000000000101';
				DELETE FROM product_ingredients
				WHERE product_id='10000000-0000-4000-8000-000000000101';
				INSERT INTO product_ingredients (
					id,user_id,workspace_id,product_id,ingredient_key,name,amount,unit,created_at
				) VALUES (
					'10000000-0000-4000-8000-000000000402',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000101','fixture','Fixture',100,'mg','2026-09-20T09:00:00Z'
				)`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `SELECT r1_backfill_product_profiles()`); err != nil {
				t.Fatalf("catch up same-value legacy provenance drift: %v", err)
			}

			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM products p
				JOIN product_profile_versions ppv ON ppv.id=p.current_product_profile_version_id
				JOIN ingredient_profile_versions ipv ON ipv.id=p.current_ingredient_profile_version_id
				WHERE p.id='10000000-0000-4000-8000-000000000101'
				  AND p.aggregate_version=2
				  AND ppv.business_version=2
				  AND ppv.name='fixture supplement drifted'
				  AND ppv.brand='drifted brand'
				  AND ppv.management_unit='capsule'
				  AND ppv.source='legacy_current'
				  AND ipv.business_version=2
				  AND ipv.serving_quantity=1
				  AND ipv.serving_unit='capsule'
				  AND ipv.change_kind='legacy_import'`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM ingredient_profile_items item
				JOIN products p
				  ON p.current_ingredient_profile_version_id=item.ingredient_profile_version_id
				WHERE p.id='10000000-0000-4000-8000-000000000101'
				  AND item.legacy_ingredient_id='10000000-0000-4000-8000-000000000402'
				  AND item.original_key='fixture'
				  AND item.original_name='Fixture'
				  AND item.label_amount=100
				  AND item.label_unit='mg'`, 1)

			if _, err := db.ExecContext(ctx, `
				UPDATE products
				SET unit='tablet', ingredient_serving_quantity=2,
				    updated_at='2026-09-20T10:00:00Z'
				WHERE id='10000000-0000-4000-8000-000000000101';
				DELETE FROM product_ingredients
				WHERE product_id='10000000-0000-4000-8000-000000000101';
				INSERT INTO product_ingredients (
					id,user_id,workspace_id,product_id,ingredient_key,name,amount,unit,created_at
				) VALUES
					('10000000-0000-4000-8000-000000000403',
					 '10000000-0000-4000-8000-000000000001',
					 '10000000-0000-4000-8000-000000000011',
					 '10000000-0000-4000-8000-000000000101','vitamin-c','Vitamin C',500,'mg','2026-09-20T10:00:00Z'),
					('10000000-0000-4000-8000-000000000404',
					 '10000000-0000-4000-8000-000000000001',
					 '10000000-0000-4000-8000-000000000011',
					 '10000000-0000-4000-8000-000000000101','zinc','Zinc',10,'mg','2026-09-20T10:00:01Z');
				SELECT r1_backfill_product_profiles()`); err != nil {
				t.Fatalf("catch up legacy value drift: %v", err)
			}

			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM products p
				JOIN product_profile_versions ppv ON ppv.id=p.current_product_profile_version_id
				JOIN ingredient_profile_versions ipv ON ipv.id=p.current_ingredient_profile_version_id
				WHERE p.id='10000000-0000-4000-8000-000000000101'
				  AND p.aggregate_version=3
				  AND ppv.business_version=3
				  AND ppv.management_unit='tablet'
				  AND ipv.business_version=3
				  AND ipv.serving_quantity=2
				  AND ipv.serving_unit='tablet'`, 1)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM ingredient_profile_items item
				JOIN products p ON p.current_ingredient_profile_version_id=item.ingredient_profile_version_id
				WHERE p.id='10000000-0000-4000-8000-000000000101'
				  AND item.legacy_ingredient_id IN (
				      '10000000-0000-4000-8000-000000000403',
				      '10000000-0000-4000-8000-000000000404'
				  )`, 2)
			assertCount(t, ctx, db, `
				SELECT count(*)
				FROM inventory_batches b
				JOIN ingredient_profile_versions ipv ON ipv.id=b.ingredient_profile_version_id
				WHERE b.product_id='10000000-0000-4000-8000-000000000101'
				  AND ipv.business_version=1`, 1)

			if _, err := db.ExecContext(ctx, `SELECT r1_backfill_product_profiles()`); err != nil {
				t.Fatalf("repeat drift catch-up: %v", err)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM product_profile_versions
				WHERE product_id='10000000-0000-4000-8000-000000000101'`, 3)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM ingredient_profile_versions
				WHERE product_id='10000000-0000-4000-8000-000000000101'`, 3)
		})
	})

	t.Run("parent_cascade_is_allowed", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			configureGoose(t)
			if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatalf("apply launch snapshot: %v", err)
			}
			seedLaunchSnapshot(t, ctx, db)
			seedAdditionalSupportedProducts(t, ctx, db)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("upgrade current snapshot: %v", err)
			}

			if _, err := db.ExecContext(ctx, `
				DELETE FROM products
				WHERE id='10000000-0000-4000-8000-000000000104'`); err != nil {
				t.Fatalf("parent Product cascade was blocked: %v", err)
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM product_profile_versions
				WHERE product_id='10000000-0000-4000-8000-000000000104'`, 0)
			assertCount(t, ctx, db, `
				SELECT count(*) FROM ingredient_profile_items
				WHERE product_id='10000000-0000-4000-8000-000000000104'`, 0)

			// The launch schema's batch FK predates this migration and is not
			// cascading. Remove that unrelated leaf before exercising User ->
			// Workspace -> Product -> immutable profile cascades.
			if _, err := db.ExecContext(ctx, `
				DELETE FROM intake_allocations
				WHERE intake_id IN (
					SELECT id FROM intake_records
					WHERE user_id='10000000-0000-4000-8000-000000000001'
				)`); err != nil {
				t.Fatalf("remove launch allocation fixture: %v", err)
			}

			if _, err := db.ExecContext(ctx, `
				DELETE FROM users
				WHERE id='10000000-0000-4000-8000-000000000001'`); err != nil {
				t.Fatalf("parent User/Workspace cascade was blocked: %v", err)
			}
			assertCount(t, ctx, db, `SELECT count(*) FROM users`, 0)
			assertCount(t, ctx, db, `SELECT count(*) FROM product_profile_versions`, 0)
			assertCount(t, ctx, db, `SELECT count(*) FROM ingredient_profile_versions`, 0)
		})
	})

	t.Run("down_refuses_target_writes", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			configureGoose(t)
			if err := goose.UpToContext(ctx, db, ".", launchCleanupVersion); err != nil {
				t.Fatalf("apply launch snapshot: %v", err)
			}
			seedLaunchSnapshot(t, ctx, db)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("upgrade current snapshot: %v", err)
			}

			if _, err := db.ExecContext(ctx, `
				UPDATE product_profile_versions
				SET effective_to='2026-09-20T10:00:00Z'
				WHERE product_id='10000000-0000-4000-8000-000000000101'
				  AND effective_to IS NULL;
				INSERT INTO product_profile_versions (
					id,user_id,workspace_id,product_id,business_version,name,brand,product_form,
					management_unit,source,history_completeness,source_updated_at,effective_from,created_at
				) VALUES (
					'10000000-0000-4000-8000-000000000991',
					'10000000-0000-4000-8000-000000000001',
					'10000000-0000-4000-8000-000000000011',
					'10000000-0000-4000-8000-000000000101',2,
					'manual target write','fixture','','capsule','manual','complete',
					'2026-09-20T10:00:00Z','2026-09-20T10:00:00Z','2026-09-20T10:00:00Z'
				);
				UPDATE products
				SET current_product_profile_version_id='10000000-0000-4000-8000-000000000991',
				    aggregate_version=2
				WHERE id='10000000-0000-4000-8000-000000000101'`); err != nil {
				t.Fatal(err)
			}

			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", launchCleanupVersion); err == nil {
				t.Fatal("E2 Down accepted non-migration target writes")
			}
			assertCount(t, ctx, db, `
				SELECT count(*) FROM product_profile_versions
				WHERE id='10000000-0000-4000-8000-000000000991'`, 1)
		})
	})
}

func seedAdditionalSupportedProducts(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	now := time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC)
	for _, item := range []struct {
		id     string
		name   string
		status string
	}{
		{"10000000-0000-4000-8000-000000000104", "fixture empty archived supplement", "archived"},
		{"10000000-0000-4000-8000-000000000105", "fixture depleted supplement", "depleted"},
	} {
		execSeed(t, ctx, db, `
			INSERT INTO products (
				id,user_id,workspace_id,name,brand,product_type,status,unit,
				dose_quantity,dose_times_per_day,ingredient_serving_quantity,
				restock_threshold_days,expiry_reminder_days,created_at,updated_at
			) VALUES ($1,'10000000-0000-4000-8000-000000000001',
				'10000000-0000-4000-8000-000000000011',$2,'fixture','supplement',$3,
				'capsule',1,1,1,7,30,$4,$4)`, item.id, item.name, item.status, now)
	}
}

func assertE2Constraints(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	if _, err := db.ExecContext(ctx, `
		DELETE FROM product_profile_versions
		WHERE product_id='10000000-0000-4000-8000-000000000101'`); err == nil {
		t.Fatal("direct product profile deletion was accepted")
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE product_profile_versions
		SET name='mutated'
		WHERE product_id='10000000-0000-4000-8000-000000000101'`); err == nil {
		t.Fatal("product profile body was mutable")
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE ingredient_profile_items
		SET original_name='mutated'
		WHERE legacy_ingredient_id='10000000-0000-4000-8000-000000000401'`); err == nil {
		t.Fatal("ingredient profile item was mutable")
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ingredient_profile_items (
			id,user_id,workspace_id,product_id,ingredient_profile_version_id,
			item_order,original_key,original_name,label_amount,label_unit,mapping_status,created_at
		)
		SELECT
			'10000000-0000-4000-8000-000000000998',user_id,workspace_id,product_id,id,
			99,'late','Late mutation',1,'mg','exact','2026-09-20T00:00:00Z'
		FROM ingredient_profile_versions
		WHERE product_id='10000000-0000-4000-8000-000000000101'
		  AND effective_to IS NULL`); err == nil {
		t.Fatal("activated ingredient profile accepted a late item")
	}
	assertCount(t, ctx, db, `
		SELECT count(*)
		FROM pg_constraint
		WHERE conname='ingredient_profile_items_mapping_check'
		  AND conrelid='ingredient_profile_items'::regclass
		  AND pg_get_constraintdef(oid) LIKE '%exact%'
		  AND pg_get_constraintdef(oid) LIKE '%alias_matched%'
		  AND pg_get_constraintdef(oid) LIKE '%user_confirmed%'
		  AND pg_get_constraintdef(oid) LIKE '%ambiguous%'
		  AND pg_get_constraintdef(oid) LIKE '%unmapped%'`, 1)
	if _, err := db.ExecContext(ctx, `
		UPDATE products
		SET current_product_profile_version_id=(
			SELECT id FROM product_profile_versions
			WHERE product_id='10000000-0000-4000-8000-000000000101'
		)
		WHERE id='10000000-0000-4000-8000-000000000104'`); err == nil {
		t.Fatal("product accepted another product's current profile pointer")
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ingredient_profile_versions (
			id,user_id,workspace_id,product_id,business_version,serving_quantity,
			serving_unit,serving_relation_state,profile_status,change_kind,
			history_completeness,source_updated_at,effective_from,created_at
		) VALUES (
			'10000000-0000-4000-8000-000000000999',
			'10000000-0000-4000-8000-000000000001',
			'10000000-0000-4000-8000-000000000011',
			'10000000-0000-4000-8000-000000000101',2,1,'capsule',
			'legacy_assumed_same_unit','partial','correction','legacy_current_only',
			'2026-09-20T00:00:00Z','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z'
		)`); err == nil {
		t.Fatal("overlapping ingredient profile interval was accepted")
	}
}
