package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"suppq.local/server/internal/catalog"
	"suppq.local/server/internal/config"
	"suppq.local/server/internal/database"
	"suppq.local/server/internal/identity"
	"suppq.local/server/internal/provider"
	"suppq.local/server/internal/recognition"
	"suppq.local/server/internal/storage"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
	if err = cfg.ValidateRecognitionWorker(); err != nil {
		logger.Error("recognition worker configuration invalid", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database configuration invalid", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	objectStore, err := storage.NewS3(storage.S3Config{Endpoint: cfg.ObjectEndpoint, AccessKey: cfg.ObjectAccessKey, SecretKey: cfg.ObjectSecretKey, Bucket: cfg.ObjectBucket, Region: cfg.ObjectRegion, Secure: cfg.ObjectSecure})
	if err != nil {
		logger.Error("object storage configuration invalid", "error", err)
		os.Exit(1)
	}
	if err = objectStore.EnsureBucket(ctx, cfg.Environment != "production"); err != nil {
		logger.Error("object storage unavailable", "error", err)
		os.Exit(1)
	}
	var recognizer provider.Recognition = provider.Fake{}
	if cfg.RecognitionProvider == "openai_vision" {
		recognizer = provider.NewOpenAIVision(provider.VisionConfig{BaseURL: cfg.RecognitionBaseURL, APIKey: cfg.RecognitionAPIKey, Model: cfg.RecognitionModel, Timeout: cfg.RecognitionTimeout})
	} else if cfg.RecognitionProvider == "evidence_pipeline" {
		recognizer = provider.NewEvidencePipeline(provider.PipelineConfig{
			Mode:      cfg.RecognitionMode,
			OCR:       provider.TextOCRConfig{BaseURL: cfg.OCRBaseURL, APIKey: cfg.OCRAPIKey, Model: cfg.OCRModel, Timeout: cfg.RecognitionTimeout},
			Structure: provider.AnthropicConfig{BaseURL: cfg.StructureBaseURL, APIKey: cfg.StructureAPIKey, Model: cfg.StructureModel, AuthMode: cfg.StructureAuthMode, DisableThinking: cfg.StructureThinkingOff, Timeout: cfg.RecognitionTimeout},
			Direct:    provider.VisionConfig{BaseURL: cfg.VLBaseURL, APIKey: cfg.VLAPIKey, Model: cfg.VLModel, Timeout: cfg.RecognitionTimeout},
		})
	}
	recognitionService := recognition.New(pool, objectStore, catalog.New(pool))

	checker := database.Checker{Pool: pool, Timeout: cfg.DatabaseTimeout}
	identityService := identity.New(pool, nil, identity.Config{Pepper: cfg.TokenPepper, SessionTTL: cfg.SessionTTL, DemoTTL: cfg.DemoTTL, EmailCodeTTL: cfg.EmailCodeTTL})
	logger.Info("worker started", "environment", cfg.Environment, "version", version)
	runCycle(ctx, logger, pool, checker, identityService, recognitionService, recognizer, version)

	ticker := time.NewTicker(cfg.WorkerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopped")
			return
		case <-ticker.C:
			runCycle(ctx, logger, pool, checker, identityService, recognitionService, recognizer, version)
		}
	}
}

func runCycle(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, checker database.Checker, identities *identity.Service, recognitions *recognition.Service, recognizer provider.Recognition, workerVersion string) {
	if !checkDatabase(ctx, logger, checker) {
		return
	}
	processed := 0
	for processed < 20 {
		claimed, jobErr := recognitions.RunOne(ctx, recognizer)
		if jobErr != nil {
			logger.Error("recognition job failed", "error", jobErr)
			break
		}
		if !claimed {
			break
		}
		processed++
	}
	accountUsersDeleted, accountObjectsDeleted := 0, 0
	for accountUsersDeleted < 20 {
		userID, claimed, cleanupErr := identities.ClaimAccountCleanup(ctx)
		if cleanupErr != nil {
			logger.Error("account cleanup claim failed", "error", cleanupErr)
			break
		}
		if !claimed {
			break
		}
		objectCount, cleanupErr := recognitions.DeleteUserObjects(ctx, userID)
		accountObjectsDeleted += objectCount
		if cleanupErr == nil {
			cleanupErr = identities.CompleteAccountCleanup(ctx, userID)
		}
		if cleanupErr != nil {
			logger.Error("account cleanup failed", "error", cleanupErr)
			if failErr := identities.FailAccountCleanup(ctx, userID, cleanupErr); failErr != nil {
				logger.Error("account cleanup retry scheduling failed", "error", failErr)
			}
			continue
		}
		accountUsersDeleted++
	}
	objectsDeleted, err := recognitions.CleanupExpiredDemoObjects(ctx, 1000)
	if err != nil {
		logger.Error("demo object cleanup failed", "error", err)
		return
	}
	deleted, err := identities.CleanupExpiredDemos(ctx, 100)
	if err != nil {
		logger.Error("demo cleanup failed", "error", err)
		return
	}
	orphansDeleted, err := recognitions.ReconcileOrphanObjects(ctx, time.Hour, 1000)
	if err != nil {
		logger.Error("orphan object reconciliation failed", "error", err)
	}
	details, _ := json.Marshal(map[string]int{"recognitionJobsProcessed": processed, "demoUsersDeleted": deleted, "demoObjectsDeleted": objectsDeleted, "accountUsersDeleted": accountUsersDeleted, "accountObjectsDeleted": accountObjectsDeleted, "orphanObjectsDeleted": orphansDeleted})
	if _, heartbeatErr := pool.Exec(ctx, `INSERT INTO worker_heartbeats (worker_name,provider,version,details,last_seen_at) VALUES ('recognition-cleanup',$1,$2,$3,$4) ON CONFLICT (worker_name) DO UPDATE SET provider=EXCLUDED.provider,version=EXCLUDED.version,details=EXCLUDED.details,last_seen_at=EXCLUDED.last_seen_at`, recognizer.Name(), workerVersion, details, time.Now().UTC()); heartbeatErr != nil {
		logger.Error("worker heartbeat persistence failed", "error", heartbeatErr)
	}
	logger.Info("worker cycle complete", "database", "ready", "demo_users_deleted", deleted, "demo_objects_deleted", objectsDeleted, "account_users_deleted", accountUsersDeleted, "account_objects_deleted", accountObjectsDeleted, "orphan_objects_deleted", orphansDeleted, "recognition_jobs_processed", processed, "recognition_provider", recognizer.Name())
}

func checkDatabase(ctx context.Context, logger *slog.Logger, checker database.Checker) bool {
	if err := checker.Ping(ctx); err != nil {
		logger.Error("worker dependency unavailable", "dependency", "postgres", "error", err)
		return false
	}
	return true
}
