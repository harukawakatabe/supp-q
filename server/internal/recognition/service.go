package recognition

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"suppq.local/server/internal/catalog"
	"suppq.local/server/internal/provider"
	"suppq.local/server/internal/storage"
)

type Error struct{ Code, Message string }

func (value *Error) Error() string             { return value.Code }
func newError(code, message string) *Error     { return &Error{Code: code, Message: message} }
func NewHTTPError(code, message string) *Error { return newError(code, message) }

var ErrNotFound = newError("resource_not_found", "记录不存在。")

type Scope struct{ UserID, WorkspaceID string }
type Upload struct {
	Role, Name, DeclaredMIME string
	Data                     []byte
}
type File struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	MIMEType  string    `json:"mimeType"`
	ByteSize  int64     `json:"byteSize"`
	CreatedAt time.Time `json:"createdAt"`
}
type Job struct {
	ID           string              `json:"id"`
	Role         string              `json:"role"`
	Status       string              `json:"status"`
	Provider     string              `json:"provider"`
	ErrorCode    string              `json:"errorCode,omitempty"`
	ErrorMessage string              `json:"errorMessage,omitempty"`
	Attempt      int                 `json:"attempt"`
	MaxAttempts  int                 `json:"maxAttempts"`
	Confidence   float64             `json:"confidence"`
	Result       *provider.Candidate `json:"result,omitempty"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}
type Set struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ProductID *string   `json:"productId,omitempty"`
	Files     []File    `json:"files"`
	Jobs      []Job     `json:"jobs"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Service struct {
	pool    *pgxpool.Pool
	objects storage.ObjectStore
	catalog *catalog.Service
	now     func() time.Time
}
type storedObject struct{ fileID, key string }

func New(pool *pgxpool.Pool, objects storage.ObjectStore, catalogService *catalog.Service) *Service {
	return &Service{pool: pool, objects: objects, catalog: catalogService, now: func() time.Time { return time.Now().UTC() }}
}

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(value[:4]), hex.EncodeToString(value[4:6]), hex.EncodeToString(value[6:8]), hex.EncodeToString(value[8:10]), hex.EncodeToString(value[10:])), nil
}

func validateUpload(upload Upload) (Upload, string, error) {
	if upload.Role != "front" && upload.Role != "facts" && upload.Role != "expiry" {
		return upload, "", newError("invalid_upload", "图片角色无效。")
	}
	if len(upload.Data) < 16 || len(upload.Data) > 10<<20 {
		return upload, "", newError("invalid_upload", "每张图片必须在 16 字节到 10 MB 之间。")
	}
	detected := http.DetectContentType(upload.Data)
	if detected != "image/jpeg" && detected != "image/png" && detected != "image/webp" {
		return upload, "", newError("invalid_upload", "只支持 JPEG、PNG 或 WebP 图片。")
	}
	if upload.DeclaredMIME != "" && strings.TrimSpace(strings.Split(upload.DeclaredMIME, ";")[0]) != detected {
		return upload, "", newError("invalid_upload", "图片内容与声明类型不一致。")
	}
	upload.DeclaredMIME = detected
	upload.Name = filepath.Base(strings.TrimSpace(upload.Name))
	if len([]rune(upload.Name)) > 180 {
		upload.Name = string([]rune(upload.Name)[:180])
	}
	ext := "jpg"
	if detected == "image/png" {
		ext = "png"
	} else if detected == "image/webp" {
		ext = "webp"
	}
	return upload, ext, nil
}

func (service *Service) CreateSet(ctx context.Context, scope Scope, uploads []Upload) (Set, error) {
	if len(uploads) != 3 {
		return Set{}, newError("invalid_upload", "必须同时提供正面、成分表和有效期三张图片。")
	}
	seen := map[string]bool{}
	normalized := make([]Upload, 3)
	extensions := make([]string, 3)
	for index, item := range uploads {
		value, ext, err := validateUpload(item)
		if err != nil {
			return Set{}, err
		}
		if seen[value.Role] {
			return Set{}, newError("invalid_upload", "每种图片角色只能上传一次。")
		}
		seen[value.Role] = true
		normalized[index], extensions[index] = value, ext
	}
	setID, err := newUUID()
	if err != nil {
		return Set{}, err
	}
	now := service.now()
	storedObjects := []storedObject{}
	for index, item := range normalized {
		fileID, createErr := newUUID()
		if createErr != nil {
			service.deleteStored(ctx, storedObjects)
			return Set{}, createErr
		}
		key := fmt.Sprintf("%s/recognition/%s/%s-%s.%s", scope.WorkspaceID, setID, item.Role, fileID, extensions[index])
		if createErr = service.objects.Put(ctx, key, item.DeclaredMIME, item.Data); createErr != nil {
			service.deleteStored(ctx, storedObjects)
			return Set{}, newError("storage_unavailable", "图片保存失败，请稍后重试。")
		}
		storedObjects = append(storedObjects, storedObject{fileID: fileID, key: key})
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		service.deleteStored(ctx, storedObjects)
		return Set{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO recognition_sets (id,user_id,workspace_id,status,created_at,updated_at) VALUES ($1,$2,$3,'processing',$4,$4)`, setID, scope.UserID, scope.WorkspaceID, now); err != nil {
		service.deleteStored(ctx, storedObjects)
		return Set{}, err
	}
	for index, item := range normalized {
		storedItem := storedObjects[index]
		digest := sha256.Sum256(item.Data)
		if _, err = tx.Exec(ctx, `INSERT INTO files (id,user_id,workspace_id,object_key,original_name,mime_type,byte_size,sha256_hex,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, storedItem.fileID, scope.UserID, scope.WorkspaceID, storedItem.key, item.Name, item.DeclaredMIME, len(item.Data), hex.EncodeToString(digest[:]), now); err != nil {
			service.deleteStored(ctx, storedObjects)
			return Set{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO recognition_files (recognition_set_id,file_id,role) VALUES ($1,$2,$3)`, setID, storedItem.fileID, item.Role); err != nil {
			service.deleteStored(ctx, storedObjects)
			return Set{}, err
		}
		jobID, createErr := newUUID()
		if createErr != nil {
			service.deleteStored(ctx, storedObjects)
			return Set{}, createErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO recognition_jobs (id,user_id,workspace_id,recognition_set_id,file_id,role,status,run_after,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,'queued',$7,$7,$7)`, jobID, scope.UserID, scope.WorkspaceID, setID, storedItem.fileID, item.Role, now); err != nil {
			service.deleteStored(ctx, storedObjects)
			return Set{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		service.deleteStored(ctx, storedObjects)
		return Set{}, err
	}
	return service.GetSet(ctx, scope, setID)
}

func (service *Service) deleteStored(ctx context.Context, items []storedObject) {
	for _, item := range items {
		_ = service.objects.Delete(ctx, item.key)
	}
}

func (service *Service) GetSet(ctx context.Context, scope Scope, id string) (Set, error) {
	var result Set
	err := service.pool.QueryRow(ctx, `SELECT id,status,product_id,created_at,updated_at FROM recognition_sets WHERE id=$1 AND user_id=$2 AND workspace_id=$3`, id, scope.UserID, scope.WorkspaceID).Scan(&result.ID, &result.Status, &result.ProductID, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Set{}, ErrNotFound
	}
	if err != nil {
		return Set{}, err
	}
	rows, err := service.pool.Query(ctx, `SELECT f.id,rf.role,f.mime_type,f.byte_size,f.created_at FROM recognition_files rf JOIN files f ON f.id=rf.file_id WHERE rf.recognition_set_id=$1 AND f.user_id=$2 AND f.workspace_id=$3 ORDER BY CASE rf.role WHEN 'front' THEN 1 WHEN 'facts' THEN 2 ELSE 3 END`, id, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return Set{}, err
	}
	result.Files = []File{}
	for rows.Next() {
		var item File
		if err = rows.Scan(&item.ID, &item.Role, &item.MIMEType, &item.ByteSize, &item.CreatedAt); err != nil {
			rows.Close()
			return Set{}, err
		}
		result.Files = append(result.Files, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Set{}, err
	}
	jobRows, err := service.pool.Query(ctx, `SELECT id,role,status,provider,attempt,max_attempts,confidence::float8,result,error_code,error_message,created_at,updated_at FROM recognition_jobs WHERE recognition_set_id=$1 AND user_id=$2 AND workspace_id=$3 ORDER BY CASE role WHEN 'front' THEN 1 WHEN 'facts' THEN 2 ELSE 3 END`, id, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return Set{}, err
	}
	result.Jobs = []Job{}
	for jobRows.Next() {
		var item Job
		var raw []byte
		if err = jobRows.Scan(&item.ID, &item.Role, &item.Status, &item.Provider, &item.Attempt, &item.MaxAttempts, &item.Confidence, &raw, &item.ErrorCode, &item.ErrorMessage, &item.CreatedAt, &item.UpdatedAt); err != nil {
			jobRows.Close()
			return Set{}, err
		}
		if len(raw) > 0 {
			var candidate provider.Candidate
			if err = json.Unmarshal(raw, &candidate); err != nil {
				jobRows.Close()
				return Set{}, err
			}
			item.Result = &candidate
		}
		result.Jobs = append(result.Jobs, item)
	}
	err = jobRows.Err()
	jobRows.Close()
	return result, err
}

func (service *Service) GetFile(ctx context.Context, scope Scope, id string) (string, []byte, error) {
	var key, mime string
	err := service.pool.QueryRow(ctx, `SELECT object_key,mime_type FROM files WHERE id=$1 AND user_id=$2 AND workspace_id=$3 AND status='active'`, id, scope.UserID, scope.WorkspaceID).Scan(&key, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	data, err := service.objects.Get(ctx, key)
	return mime, data, err
}

func (service *Service) RetryJob(ctx context.Context, scope Scope, id string) (Set, error) {
	now := service.now()
	var setID string
	result, err := service.pool.Exec(ctx, `UPDATE recognition_jobs SET status='queued',run_after=$1,lease_until=NULL,error_code='',error_message='',completed_at=NULL,updated_at=$1 WHERE id=$2 AND user_id=$3 AND workspace_id=$4 AND status IN ('failed','partial','succeeded') AND EXISTS (SELECT 1 FROM recognition_sets rs WHERE rs.id=recognition_set_id AND rs.status<>'confirmed')`, now, id, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return Set{}, err
	}
	if result.RowsAffected() != 1 {
		return Set{}, ErrNotFound
	}
	err = service.pool.QueryRow(ctx, `SELECT recognition_set_id FROM recognition_jobs WHERE id=$1 AND user_id=$2 AND workspace_id=$3`, id, scope.UserID, scope.WorkspaceID).Scan(&setID)
	if err != nil {
		return Set{}, err
	}
	_, _ = service.pool.Exec(ctx, `UPDATE recognition_sets SET status='processing',updated_at=$1 WHERE id=$2 AND user_id=$3 AND workspace_id=$4`, now, setID, scope.UserID, scope.WorkspaceID)
	return service.GetSet(ctx, scope, setID)
}

type claimedJob struct {
	ID, SetID, FileID, Role, ObjectKey, MIME string
	Attempt, MaxAttempts                     int
}

func (service *Service) RunOne(ctx context.Context, recognizer provider.Recognition) (bool, error) {
	now := service.now()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var job claimedJob
	err = tx.QueryRow(ctx, `SELECT j.id,j.recognition_set_id,j.file_id,j.role,f.object_key,f.mime_type,j.attempt,j.max_attempts FROM recognition_jobs j JOIN files f ON f.id=j.file_id WHERE ((j.status='queued' AND j.run_after<=$1) OR (j.status='running' AND j.lease_until<$1)) ORDER BY j.run_after,j.created_at FOR UPDATE OF j SKIP LOCKED LIMIT 1`, now).Scan(&job.ID, &job.SetID, &job.FileID, &job.Role, &job.ObjectKey, &job.MIME, &job.Attempt, &job.MaxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	job.Attempt++
	if _, err = tx.Exec(ctx, `UPDATE recognition_jobs SET status='running',provider=$1,attempt=$2,started_at=COALESCE(started_at,$3),lease_until=$4,updated_at=$3 WHERE id=$5`, recognizer.Name(), job.Attempt, now, now.Add(2*time.Minute), job.ID); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	jobCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	data, runErr := service.objects.Get(jobCtx, job.ObjectKey)
	if runErr != nil {
		runErr = &provider.Failure{Code: "storage_unavailable", Message: "读取识别图片失败，任务将自动重试。", Retryable: true}
	}
	var candidate provider.Candidate
	if runErr == nil {
		candidate, runErr = recognizer.Recognize(jobCtx, job.Role, job.MIME, data)
	}
	if runErr != nil {
		return true, service.finishFailure(ctx, job, recognizer.Name(), runErr)
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		return true, service.finishFailure(ctx, job, recognizer.Name(), err)
	}
	status := "partial"
	if candidate.Status == "recognized" {
		status = "succeeded"
	}
	completed := service.now()
	if _, err = service.pool.Exec(ctx, `UPDATE recognition_jobs SET status=$1,provider=$2,confidence=$3,result=$4,error_code='',error_message='',lease_until=NULL,completed_at=$5,updated_at=$5 WHERE id=$6`, status, recognizer.Name(), candidate.Confidence, encoded, completed, job.ID); err != nil {
		return true, err
	}
	return true, service.refreshSetStatus(ctx, job.SetID, completed)
}

func (service *Service) finishFailure(ctx context.Context, job claimedJob, providerName string, runErr error) error {
	failure := &provider.Failure{Code: "recognition_failed", Message: "识别失败，图片已保留，可重试或手工填写。"}
	var typed *provider.Failure
	if errors.As(runErr, &typed) {
		failure = typed
	}
	now := service.now()
	if failure.Retryable && job.Attempt < job.MaxAttempts {
		delay := time.Duration(1<<(job.Attempt-1)) * 5 * time.Second
		_, err := service.pool.Exec(ctx, `UPDATE recognition_jobs SET status='queued',provider=$1,error_code=$2,error_message=$3,run_after=$4,lease_until=NULL,updated_at=$5 WHERE id=$6`, providerName, failure.Code, failure.Message, now.Add(delay), now, job.ID)
		return err
	}
	_, err := service.pool.Exec(ctx, `UPDATE recognition_jobs SET status='failed',provider=$1,error_code=$2,error_message=$3,lease_until=NULL,completed_at=$4,updated_at=$4 WHERE id=$5`, providerName, failure.Code, failure.Message, now, job.ID)
	if err != nil {
		return err
	}
	return service.refreshSetStatus(ctx, job.SetID, now)
}
func (service *Service) refreshSetStatus(ctx context.Context, setID string, now time.Time) error {
	_, err := service.pool.Exec(ctx, `UPDATE recognition_sets SET status=CASE WHEN EXISTS (SELECT 1 FROM recognition_jobs WHERE recognition_set_id=$1 AND status IN ('queued','running')) THEN 'processing' ELSE 'awaiting_confirmation' END,updated_at=$2 WHERE id=$1 AND status NOT IN ('confirmed','cancelled')`, setID, now)
	return err
}

func (service *Service) Confirm(ctx context.Context, scope Scope, setID string, input catalog.CreateProductInput) (catalog.Product, error) {
	var status string
	var existing *string
	err := service.pool.QueryRow(ctx, `SELECT status,product_id FROM recognition_sets WHERE id=$1 AND user_id=$2 AND workspace_id=$3`, setID, scope.UserID, scope.WorkspaceID).Scan(&status, &existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.Product{}, ErrNotFound
	}
	if err != nil {
		return catalog.Product{}, err
	}
	if existing != nil {
		return service.catalog.GetProduct(ctx, catalog.Scope(scope), *existing)
	}
	if status == "processing" {
		return catalog.Product{}, newError("recognition_processing", "识别任务仍在处理中。")
	}
	if status == "cancelled" {
		return catalog.Product{}, newError("recognition_cancelled", "识别任务已取消。")
	}
	input.SourceRecognitionSetID = setID
	product, err := service.catalog.CreateProduct(ctx, catalog.Scope(scope), input)
	if err != nil {
		return catalog.Product{}, err
	}
	payload, _ := json.Marshal(input)
	now := service.now()
	_, err = service.pool.Exec(ctx, `UPDATE recognition_sets SET status='confirmed',product_id=$1,confirmed_payload=$2,confirmed_at=$3,updated_at=$3 WHERE id=$4 AND user_id=$5 AND workspace_id=$6`, product.ID, payload, now, setID, scope.UserID, scope.WorkspaceID)
	return product, err
}

func (service *Service) CleanupExpiredDemoObjects(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := service.pool.Query(ctx, `SELECT f.id,f.object_key FROM files f JOIN workspaces w ON w.id=f.workspace_id WHERE w.kind='demo' AND w.expires_at<=now() AND f.status='active' ORDER BY w.expires_at LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type item struct{ id, key string }
	items := []item{}
	for rows.Next() {
		var value item
		if err = rows.Scan(&value.id, &value.key); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, value := range items {
		if err = service.objects.Delete(ctx, value.key); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}
