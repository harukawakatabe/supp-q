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
	logger.Info("worker started", "environment", cfg.Environment, "version", version)
	checkDatabase(ctx, logger, checker)

	ticker := time.NewTicker(cfg.WorkerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopped")
			return
		case <-ticker.C:
			checkDatabase(ctx, logger, checker)
		}
	}
}

func checkDatabase(ctx context.Context, logger *slog.Logger, checker database.Checker) {
	if err := checker.Ping(ctx); err != nil {
		logger.Error("worker dependency unavailable", "dependency", "postgres", "error", err)
		return
	}
	logger.Info("worker heartbeat", "database", "ready", "jobs", "not_implemented")
}
