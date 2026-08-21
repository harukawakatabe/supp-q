package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"suppq.local/server/internal/config"
	"suppq.local/server/internal/database"
	"suppq.local/server/internal/identity"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: suppq-admin validate-config [--worker] | status | bootstrap-admin <email>")
		os.Exit(2)
	}

	cfg, err := config.FromEnv()
	if err != nil {
		fail("configuration_invalid", err)
	}
	if os.Args[1] == "validate-config" {
		if len(os.Args) == 3 && os.Args[2] == "--worker" {
			if err := cfg.ValidateRecognitionWorker(); err != nil {
				fail("worker_configuration_invalid", err)
			}
		} else if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: suppq-admin validate-config [--worker]")
			os.Exit(2)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
			"environment": cfg.Environment,
			"status":      "valid",
		})
		return
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fail("database_configuration_invalid", err)
	}
	defer pool.Close()

	if os.Args[1] == "bootstrap-admin" {
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: suppq-admin bootstrap-admin <email>")
			os.Exit(2)
		}
		service := identity.New(pool, nil, identity.Config{Pepper: cfg.TokenPepper, SessionTTL: cfg.SessionTTL, DemoTTL: cfg.DemoTTL, EmailCodeTTL: cfg.EmailCodeTTL})
		if err := service.BootstrapAdmin(ctx, os.Args[2]); err != nil {
			fail("bootstrap_admin_failed", err)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ok", "email": os.Args[2], "role": "admin"})
		return
	}
	if os.Args[1] != "status" || len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: suppq-admin validate-config [--worker] | status | bootstrap-admin <email>")
		os.Exit(2)
	}
	checker := database.Checker{Pool: pool, Timeout: cfg.DatabaseTimeout}
	if err := checker.Ping(ctx); err != nil {
		fail("database_unavailable", err)
	}

	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
		"database": "ready",
		"status":   "ok",
	})
}

func fail(code string, err error) {
	_ = json.NewEncoder(os.Stderr).Encode(map[string]string{
		"code":    code,
		"message": err.Error(),
		"status":  "error",
	})
	os.Exit(1)
}
