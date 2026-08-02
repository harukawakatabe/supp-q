package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"suppq.local/server/internal/catalog"
	"suppq.local/server/internal/config"
	"suppq.local/server/internal/database"
	"suppq.local/server/internal/httpapi"
	"suppq.local/server/internal/identity"
	"suppq.local/server/internal/mailer"
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

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(rootCtx, cfg.DatabaseURL)
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
	if err = objectStore.EnsureBucket(rootCtx, cfg.Environment != "production"); err != nil {
		logger.Error("object storage unavailable", "error", err)
		os.Exit(1)
	}
	catalogService := catalog.New(pool)
	recognitionService := recognition.New(pool, objectStore, catalogService)
	identityService := identity.New(pool, mailer.SMTP{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, From: cfg.SMTPFrom,
		Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
	}, identity.Config{Pepper: cfg.TokenPepper, SessionTTL: cfg.SessionTTL, DemoTTL: cfg.DemoTTL, EmailCodeTTL: cfg.EmailCodeTTL}).WithDemoSeeder(catalogService)

	handler := httpapi.New(httpapi.Dependencies{
		AllowedOrigin: cfg.AllowedOrigin,
		Database:      database.Checker{Pool: pool, Timeout: cfg.DatabaseTimeout},
		Logger:        logger,
		Version:       version,
		Identity:      identityService,
		Catalog:       catalogService,
		Recognition:   recognitionService,
		SessionCookie: cfg.SessionCookie,
		CookieSecure:  cfg.CookieSecure,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddr, "environment", cfg.Environment, "version", version)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("api stopped")
}
