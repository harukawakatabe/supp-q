package recognition

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"suppq.local/server/internal/catalog"
	"suppq.local/server/internal/provider"
	"suppq.local/server/internal/storage"
	"suppq.local/server/migrations"
)

type memoryObjects struct {
	mu    sync.Mutex
	items map[string][]byte
}
type failedProvider struct{}

func (failedProvider) Name() string { return "live:test-failure" }
func (failedProvider) Recognize(context.Context, string, string, []byte, provider.EvidenceSink) (provider.Result, error) {
	return provider.Result{}, &provider.Failure{Code: "provider_timeout", Message: "识别超时，图片已保留。"}
}

func (store *memoryObjects) Put(_ context.Context, key, _ string, data []byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.items[key] = append([]byte(nil), data...)
	return nil
}
func (store *memoryObjects) Get(_ context.Context, key string) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	data, ok := store.items[key]
	if !ok {
		return nil, errors.New("missing object")
	}
	return append([]byte(nil), data...), nil
}
func (store *memoryObjects) Delete(_ context.Context, key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.items, key)
	return nil
}
func (store *memoryObjects) List(_ context.Context, prefix, after string, limit int) ([]storage.ObjectInfo, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	items := []storage.ObjectInfo{}
	keys := make([]string, 0, len(store.items))
	for key := range store.items {
		if strings.HasPrefix(key, prefix) && key > after {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		items = append(items, storage.ObjectInfo{Key: key, LastModified: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)})
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func TestRecognitionPersistenceWorkerConfirmationAndIsolation(t *testing.T) {
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
	schema := fmt.Sprintf("suppq_recognition_test_%d", time.Now().UnixNano())
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
	now, _ := time.Parse(time.RFC3339, "2026-08-02T10:00:00Z")
	owner := createRecognitionOwner(t, ctx, pool, "00000000-0000-4000-8000-000000000101", "00000000-0000-4000-8000-000000000111", now)
	other := createRecognitionOwner(t, ctx, pool, "00000000-0000-4000-8000-000000000102", "00000000-0000-4000-8000-000000000122", now)
	objects := &memoryObjects{items: map[string][]byte{}}
	catalogService := catalog.New(pool)
	service := New(pool, objects, catalogService)
	service.now = func() time.Time { return now }
	image := append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 64)...)
	set, err := service.CreateSet(ctx, owner, []Upload{{Role: "front", Name: "front.png", DeclaredMIME: "image/png", Data: image}, {Role: "facts", Name: "facts.png", DeclaredMIME: "image/png", Data: image}, {Role: "expiry", Name: "expiry.png", DeclaredMIME: "image/png", Data: image}})
	if err != nil {
		t.Fatal(err)
	}
	if set.Status != "processing" || len(set.Jobs) != 3 || len(objects.items) != 3 {
		t.Fatalf("unexpected persisted set: %+v objects=%d", set, len(objects.items))
	}
	var productCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE user_id=$1`, owner.UserID).Scan(&productCount); err != nil || productCount != 0 {
		t.Fatal("unconfirmed recognition created a product")
	}
	if _, err = service.GetSet(ctx, other, set.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant set access must be hidden, got %v", err)
	}
	if _, _, err = service.GetFile(ctx, other, set.Files[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant file access must be hidden, got %v", err)
	}
	for range 3 {
		claimed, runErr := service.RunOne(ctx, provider.Fake{})
		if runErr != nil || !claimed {
			t.Fatalf("worker failed: claimed=%v err=%v", claimed, runErr)
		}
	}
	claimed, err := service.RunOne(ctx, provider.Fake{})
	if err != nil || claimed {
		t.Fatalf("queue should be empty: %v %v", claimed, err)
	}
	set, err = service.GetSet(ctx, owner, set.ID)
	if err != nil {
		t.Fatal(err)
	}
	if set.Status != "awaiting_confirmation" {
		t.Fatalf("unexpected status %q", set.Status)
	}
	for _, job := range set.Jobs {
		if job.Status != "partial" || job.Provider != "fake:development" || job.Result == nil || job.OCREvidence == nil || job.OCREvidence.RawText == "" {
			t.Fatalf("unexpected job: %+v", job)
		}
	}
	input := catalog.CreateProductInput{Name: "人工确认 D3", Unit: "粒", DoseQuantity: 1, DoseTimesPerDay: 1, IngredientServingQuantity: 1, RestockThresholdDays: 7, ExpiryReminderDays: 30, Schedule: catalog.ScheduleInput{StartDate: "2026-08-02", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, ReminderTimes: []string{"09:00"}}, OpeningBatch: catalog.BatchInput{Quantity: 60, ExpiryDate: "2027-12-31", PriceCNY: 99}}
	product, err := service.Confirm(ctx, owner, set.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if product.Name != "人工确认 D3" || product.CurrentQuantity != 60 {
		t.Fatalf("unexpected confirmed product: %+v", product)
	}
	same, err := service.Confirm(ctx, owner, set.ID, input)
	if err != nil || same.ID != product.ID {
		t.Fatal("confirmation must be idempotent")
	}
	if err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM products p
		JOIN product_profile_versions ppv ON ppv.id=p.current_product_profile_version_id
		JOIN ingredient_profile_versions ipv ON ipv.id=p.current_ingredient_profile_version_id
		WHERE p.id=$1
		  AND p.source_recognition_set_id=$2
		  AND ppv.source='capture_confirmed'
		  AND ppv.business_version=1
		  AND ipv.business_version=1`, product.ID, set.ID).Scan(&productCount); err != nil || productCount != 1 {
		t.Fatalf("recognition confirmation did not create exactly one target profile set: count=%d err=%v", productCount, err)
	}
	if _, err = service.catalog.GetProduct(ctx, catalog.Scope(other), product.ID); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatal("confirmed product leaked across tenant")
	}

	failedSet, err := service.CreateSet(ctx, owner, []Upload{{Role: "front", Name: "front.png", DeclaredMIME: "image/png", Data: image}, {Role: "facts", Name: "facts.png", DeclaredMIME: "image/png", Data: image}, {Role: "expiry", Name: "expiry.png", DeclaredMIME: "image/png", Data: image}})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		claimed, runErr := service.RunOne(ctx, failedProvider{})
		if runErr != nil || !claimed {
			t.Fatalf("failure worker path failed: %v %v", claimed, runErr)
		}
	}
	failedSet, err = service.GetSet(ctx, owner, failedSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failedSet.Status != "awaiting_confirmation" {
		t.Fatalf("failed set must remain confirmable, got %s", failedSet.Status)
	}
	for _, job := range failedSet.Jobs {
		if job.Status != "failed" || job.ErrorCode != "provider_timeout" {
			t.Fatalf("unexpected retained failure: %+v", job)
		}
	}
	if _, _, err = service.GetFile(ctx, owner, failedSet.Files[0].ID); err != nil {
		t.Fatalf("provider failure lost upload: %v", err)
	}
	retried, err := service.RetryJob(ctx, owner, failedSet.Jobs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.Status != "processing" {
		t.Fatal("retry did not return set to processing")
	}
	claimed, err = service.RunOne(ctx, provider.Fake{})
	if err != nil || !claimed {
		t.Fatalf("retry was not processed: %v %v", claimed, err)
	}

	storageSet, err := service.CreateSet(ctx, owner, []Upload{{Role: "front", Name: "front.png", DeclaredMIME: "image/png", Data: image}, {Role: "facts", Name: "facts.png", DeclaredMIME: "image/png", Data: image}, {Role: "expiry", Name: "expiry.png", DeclaredMIME: "image/png", Data: image}})
	if err != nil {
		t.Fatal(err)
	}
	objects.mu.Lock()
	for key := range objects.items {
		if strings.Contains(key, "/"+storageSet.ID+"/front-") {
			delete(objects.items, key)
		}
	}
	objects.mu.Unlock()
	for range 3 {
		claimed, err = service.RunOne(ctx, provider.Fake{})
		if err != nil || !claimed {
			t.Fatalf("storage failure path was not processed: %v %v", claimed, err)
		}
	}
	storageSet, err = service.GetSet(ctx, owner, storageSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	storageFailureFound := false
	for _, job := range storageSet.Jobs {
		if job.ErrorCode == "storage_unavailable" {
			storageFailureFound = job.Status == "queued" && job.Attempt == 1
		}
	}
	if !storageFailureFound || storageSet.Status != "processing" {
		t.Fatalf("retryable storage failure must stay queued with retained metadata: %+v", storageSet)
	}

	deletionSet, err := service.CreateSet(ctx, other, []Upload{{Role: "front", Name: "front.png", DeclaredMIME: "image/png", Data: image}, {Role: "facts", Name: "facts.png", DeclaredMIME: "image/png", Data: image}, {Role: "expiry", Name: "expiry.png", DeclaredMIME: "image/png", Data: image}})
	if err != nil {
		t.Fatal(err)
	}
	if len(deletionSet.Files) != 3 {
		t.Fatal("expected account cleanup fixture files")
	}
	deletedObjects, err := service.DeleteUserObjects(ctx, other.UserID)
	if err != nil || deletedObjects != 3 {
		t.Fatalf("account object cleanup failed: deleted=%d err=%v", deletedObjects, err)
	}
	objects.mu.Lock()
	objects.items["zz-orphan/old.png"] = append([]byte(nil), image...)
	objectCount := len(objects.items)
	objects.mu.Unlock()
	orphans := 0
	for range objectCount + 1 {
		deleted, reconcileErr := service.ReconcileOrphanObjects(ctx, time.Hour, 1)
		if reconcileErr != nil {
			t.Fatal(reconcileErr)
		}
		orphans += deleted
	}
	if orphans != 1 {
		t.Fatalf("paginated orphan reconciliation failed: deleted=%d", orphans)
	}
}

func createRecognitionOwner(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, workspaceID string, now time.Time) Scope {
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
	timezoneVersionID, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO workspace_timezone_versions (
			id,user_id,workspace_id,business_version,iana_timezone,confirmation_state,source,effective_from,created_at
		) VALUES ($1,$2,$3,1,'Etc/UTC','needs_confirmation','legacy_unspecified',$4,$4)`,
		timezoneVersionID, userID, workspaceID, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
		UPDATE workspaces SET current_timezone_version_id=$1
		WHERE id=$2 AND owner_user_id=$3`, timezoneVersionID, workspaceID, userID); err != nil {
		t.Fatal(err)
	}
	return Scope{UserID: userID, WorkspaceID: workspaceID}
}
