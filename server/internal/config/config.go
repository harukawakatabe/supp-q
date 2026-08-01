package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddr        string
	AllowedOrigin   string
	DatabaseURL     string
	DatabaseTimeout time.Duration
	ShutdownTimeout time.Duration
	WorkerInterval  time.Duration
}

func FromEnv() (Config, error) {
	cfg := Config{
		Environment:   envOr("SUPPQ_ENV", "development"),
		HTTPAddr:      envOr("SUPPQ_HTTP_ADDR", "127.0.0.1:8080"),
		AllowedOrigin: envOr("SUPPQ_ALLOWED_ORIGIN", "http://127.0.0.1:5173"),
		DatabaseURL:   os.Getenv("SUPPQ_DATABASE_URL"),
	}

	var err error
	if cfg.DatabaseTimeout, err = durationOr("SUPPQ_DATABASE_TIMEOUT", 3*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationOr("SUPPQ_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WorkerInterval, err = durationOr("SUPPQ_WORKER_INTERVAL", 30*time.Second); err != nil {
		return Config{}, err
	}

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, errors.New("SUPPQ_DATABASE_URL is required")
	}
	if cfg.Environment == "production" {
		parsed, parseErr := url.Parse(cfg.DatabaseURL)
		if parseErr != nil {
			return Config{}, fmt.Errorf("SUPPQ_DATABASE_URL is invalid: %w", parseErr)
		}
		if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
			return Config{}, errors.New("SUPPQ_DATABASE_URL must use postgres or postgresql scheme")
		}
		if parsed.Query().Get("sslmode") == "disable" {
			return Config{}, errors.New("SUPPQ_DATABASE_URL cannot disable TLS in production")
		}
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func durationOr(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return duration, nil
}
