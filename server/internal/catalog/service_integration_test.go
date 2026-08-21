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

func TestCatalogInventoryLifecycleAndIsolation(t *testing.T) {
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
	schema := fmt.Sprintf("suppq_catalog_test_%d", time.Now().UnixNano())
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

	now, _ := time.Parse(time.RFC3339, "2026-08-02T09:00:00Z")
	owner := createCatalogOwner(t, ctx, pool, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000011", now)
	other := createCatalogOwner(t, ctx, pool, "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000022", now)
	service := New(pool)
	service.now = func() time.Time { return now }
	product, err := service.CreateProduct(ctx, owner, CreateProductInput{
		Name: "测试镁", Unit: "粒", DoseQuantity: 2, DoseTimesPerDay: 1, IngredientServingQuantity: 2, RestockThresholdDays: 7, ExpiryReminderDays: 30,
		Schedule:     ScheduleInput{StartDate: "2026-08-01", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, ReminderTimes: []string{"22:30"}},
		OpeningBatch: BatchInput{Quantity: 4, ExpiryDate: "2027-12-31", PriceCNY: 40}, Ingredients: []IngredientInput{{Key: "magnesium", Name: "镁", Amount: 200, Unit: "mg"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	product, err = service.AddBatch(ctx, owner, product.ID, BatchInput{Quantity: 2, ExpiryDate: "2026-12-31", PriceCNY: 10})
	if err != nil {
		t.Fatal(err)
	}
	if product.CurrentQuantity != 6 {
		t.Fatalf("expected six units, got %v", product.CurrentQuantity)
	}
	if _, err = service.GetProduct(ctx, other, product.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant read must be hidden, got %v", err)
	}
	product, err = service.UpdateProduct(ctx, owner, product.ID, UpdateProductInput{
		Name: "测试镁（已核对）", Brand: "测试品牌", ProductType: "supplement", Status: "active", Unit: "粒",
		DoseQuantity: 2, DoseTimesPerDay: 1, IngredientServingQuantity: 2, RestockThresholdDays: 10, ExpiryReminderDays: 45,
		Schedule:    ScheduleInput{StartDate: "2026-08-01", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, DayCycle: DayCycleInput{Enabled: true, CycleDays: 28, TakeDays: 21, AnchorDate: "2026-08-03"}, LongCycle: LongCycleInput{Enabled: false, TakeWeeks: 6, RestWeeks: 4, StartDate: "2026-08-01"}, ReminderTimes: []string{"22:00"}},
		Ingredients: []IngredientInput{{Key: "magnesium", Name: "镁", Amount: 200, Unit: "mg"}}, EffectiveDate: "2026-08-03",
	})
	if err != nil {
		t.Fatal(err)
	}
	if product.Name != "测试镁（已核对）" || product.Schedule.Version != 2 || !product.Schedule.DayCycle.Enabled || product.Schedule.ReminderTimes[0] != "22:00" {
		t.Fatalf("product update did not persist: %+v", product)
	}
	if _, err = service.UpdateProduct(ctx, other, product.ID, UpdateProductInput{Name: "越权", ProductType: "supplement", Status: "active", Unit: "粒", DoseQuantity: 1, DoseTimesPerDay: 1, IngredientServingQuantity: 1, RestockThresholdDays: 7, ExpiryReminderDays: 30, Schedule: ScheduleInput{StartDate: "2026-08-02", Weekdays: []int{0}, DayCycle: DayCycleInput{Enabled: false, CycleDays: 28, TakeDays: 28, AnchorDate: "2026-08-02"}, LongCycle: LongCycleInput{Enabled: false, TakeWeeks: 6, RestWeeks: 4, StartDate: "2026-08-02"}, ReminderTimes: []string{"09:00"}}, EffectiveDate: "2026-08-03"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant update must be hidden, got %v", err)
	}

	intake, after, err := service.CreateIntake(ctx, owner, CreateIntakeInput{ProductID: product.ID, Date: "2026-08-02", Time: "22:30", Quantity: 3, Source: "scheduled", Note: "随餐"}, "test-intake-1")
	if err != nil {
		t.Fatal(err)
	}
	if after.CurrentQuantity != 3 {
		t.Fatalf("expected three units left, got %v", after.CurrentQuantity)
	}
	if len(intake.Allocations) != 2 || intake.Allocations[0].Quantity != 2 || intake.Allocations[1].Quantity != 1 {
		t.Fatalf("expected FEFO split 2+1, got %+v", intake.Allocations)
	}
	duplicate, duplicateProduct, err := service.CreateIntake(ctx, owner, CreateIntakeInput{ProductID: product.ID, Date: "2026-08-02", Time: "22:30", Quantity: 3, Source: "scheduled"}, "test-intake-1")
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID != intake.ID || duplicateProduct.CurrentQuantity != 3 {
		t.Fatal("idempotent replay changed inventory")
	}
	if _, _, err = service.CreateIntake(ctx, owner, CreateIntakeInput{ProductID: product.ID, Date: "2026-08-02", Quantity: 4, Source: "ad_hoc"}, "too-large"); !errors.Is(err, ErrInsufficientInventory) {
		t.Fatalf("expected atomic inventory rejection, got %v", err)
	}
	unchanged, err := service.GetProduct(ctx, owner, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.CurrentQuantity != 3 {
		t.Fatal("failed intake mutated inventory")
	}

	revoked, restored, err := service.UndoIntake(ctx, owner, intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Status != "revoked" || restored.CurrentQuantity != 6 {
		t.Fatalf("undo did not restore exact inventory: %+v %v", revoked, restored.CurrentQuantity)
	}
	_, restoredAgain, err := service.UndoIntake(ctx, owner, intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restoredAgain.CurrentQuantity != 6 {
		t.Fatal("repeated undo changed inventory")
	}
	records, err := service.ListIntakes(ctx, owner, "2026-08-01", "2026-08-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Status != "revoked" || records[0].ProductName != "测试镁（已核对）" || records[0].Note != "随餐" {
		t.Fatalf("unexpected intake records: %+v", records)
	}
	otherRecords, err := service.ListIntakes(ctx, other, "2026-08-01", "2026-08-31")
	if err != nil || len(otherRecords) != 0 {
		t.Fatalf("cross-tenant records leaked: %+v err=%v", otherRecords, err)
	}
	if _, _, err = service.UndoIntake(ctx, other, intake.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant undo must be hidden, got %v", err)
	}

	today, err := service.Today(ctx, owner, "2026-08-02")
	if err != nil {
		t.Fatal(err)
	}
	if len(today) != 1 || today[0].Done || today[0].ScheduledQuantity != 2 {
		current, _ := service.GetProduct(ctx, owner, product.ID)
		var history string
		_ = pool.QueryRow(ctx, `SELECT COALESCE(json_agg(json_build_object('effective',effective_date,'enabled',enabled,'anchor',anchor_date) ORDER BY effective_date),'[]')::text FROM day_cycle_versions WHERE product_id=$1`, product.ID).Scan(&history)
		t.Fatalf("unexpected today projection: %+v current=%+v history=%s", today, current, history)
	}
}

func createCatalogOwner(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, workspaceID string, now time.Time) Scope {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,kind,status,role,last_activity_at,created_at) VALUES ($1,'registered','active','member',$2,$2)`, userID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,created_at) VALUES ($1,$2,'registered','test',$3,$3)`, workspaceID, userID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at) VALUES ($1,$2,'owner',$3)`, workspaceID, userID, now); err != nil {
		t.Fatal(err)
	}
	return Scope{UserID: userID, WorkspaceID: workspaceID}
}
