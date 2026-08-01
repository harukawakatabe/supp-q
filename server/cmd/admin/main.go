package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"suppq.local/server/internal/config"
	"suppq.local/server/internal/database"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "status" {
		fmt.Fprintln(os.Stderr, "usage: suppq-admin status")
		os.Exit(2)
	}

	cfg, err := config.FromEnv()
	if err != nil {
		fail("configuration_invalid", err)
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fail("database_configuration_invalid", err)
	}
	defer pool.Close()

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
