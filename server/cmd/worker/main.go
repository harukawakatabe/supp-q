package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"suppq.local/server/internal/config"
	"suppq.local/server/internal/database"
	"suppq.local/server/internal/identity"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
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

	checker := database.Checker{Pool: pool, Timeout: cfg.DatabaseTimeout}
	identityService := identity.New(pool, nil, identity.Config{Pepper: cfg.TokenPepper, SessionTTL: cfg.SessionTTL, DemoTTL: cfg.DemoTTL, EmailCodeTTL: cfg.EmailCodeTTL})
	logger.Info("worker started", "environment", cfg.Environment, "version", version)
	runCycle(ctx, logger, checker, identityService)

	ticker := time.NewTicker(cfg.WorkerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopped")
			return
		case <-ticker.C:
			runCycle(ctx, logger, checker, identityService)
		}
	}
}

func runCycle(ctx context.Context, logger *slog.Logger, checker database.Checker, identities *identity.Service) {
	if !checkDatabase(ctx, logger, checker) {
		return
	}
	deleted, err := identities.CleanupExpiredDemos(ctx, 100)
	if err != nil {
		logger.Error("demo cleanup failed", "error", err)
		return
	}
	logger.Info("worker cycle complete", "database", "ready", "demo_users_deleted", deleted, "domain_jobs", "not_implemented")
}

func checkDatabase(ctx context.Context, logger *slog.Logger, checker database.Checker) bool {
	if err := checker.Ping(ctx); err != nil {
		logger.Error("worker dependency unavailable", "dependency", "postgres", "error", err)
		return false
	}
	return true
}
