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
	Environment         string
	HTTPAddr            string
	AllowedOrigin       string
	DatabaseURL         string
	DatabaseTimeout     time.Duration
	ShutdownTimeout     time.Duration
	WorkerInterval      time.Duration
	TokenPepper         string
	SessionCookie       string
	SessionTTL          time.Duration
	DemoTTL             time.Duration
	EmailCodeTTL        time.Duration
	CookieSecure        bool
	SMTPHost            string
	SMTPPort            string
	SMTPFrom            string
	SMTPUsername        string
	SMTPPassword        string
	ObjectEndpoint      string
	ObjectAccessKey     string
	ObjectSecretKey     string
	ObjectBucket        string
	ObjectRegion        string
	ObjectSecure        bool
	RecognitionProvider string
	RecognitionBaseURL  string
	RecognitionAPIKey   string
	RecognitionModel    string
	RecognitionTimeout  time.Duration
}

func FromEnv() (Config, error) {
	cfg := Config{
		Environment:         envOr("SUPPQ_ENV", "development"),
		HTTPAddr:            envOr("SUPPQ_HTTP_ADDR", "127.0.0.1:8080"),
		AllowedOrigin:       envOr("SUPPQ_ALLOWED_ORIGIN", "http://127.0.0.1:5173"),
		DatabaseURL:         os.Getenv("SUPPQ_DATABASE_URL"),
		TokenPepper:         envOr("SUPPQ_TOKEN_PEPPER", "suppq-development-pepper-not-for-production"),
		SessionCookie:       envOr("SUPPQ_SESSION_COOKIE", "suppq_session"),
		SMTPHost:            envOr("SUPPQ_SMTP_HOST", "127.0.0.1"),
		SMTPPort:            envOr("SUPPQ_SMTP_PORT", "1025"),
		SMTPFrom:            envOr("SUPPQ_SMTP_FROM", "小补Q <no-reply@suppq.local>"),
		SMTPUsername:        strings.TrimSpace(os.Getenv("SUPPQ_SMTP_USERNAME")),
		SMTPPassword:        os.Getenv("SUPPQ_SMTP_PASSWORD"),
		ObjectEndpoint:      envOr("SUPPQ_OBJECT_ENDPOINT", "127.0.0.1:8333"),
		ObjectAccessKey:     envOr("SUPPQ_OBJECT_ACCESS_KEY", "suppq_local"),
		ObjectSecretKey:     envOr("SUPPQ_OBJECT_SECRET_KEY", "suppq_local_only"),
		ObjectBucket:        envOr("SUPPQ_OBJECT_BUCKET", "suppq-uploads"),
		ObjectRegion:        envOr("SUPPQ_OBJECT_REGION", "us-east-1"),
		RecognitionProvider: envOr("SUPPQ_RECOGNITION_PROVIDER", "fake"),
		RecognitionBaseURL:  strings.TrimSpace(os.Getenv("SUPPQ_RECOGNITION_BASE_URL")),
		RecognitionAPIKey:   os.Getenv("SUPPQ_RECOGNITION_API_KEY"),
		RecognitionModel:    strings.TrimSpace(os.Getenv("SUPPQ_RECOGNITION_MODEL")),
	}
	cfg.ObjectSecure = strings.EqualFold(strings.TrimSpace(os.Getenv("SUPPQ_OBJECT_SECURE")), "true")

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
	if cfg.SessionTTL, err = durationOr("SUPPQ_SESSION_TTL", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.DemoTTL, err = durationOr("SUPPQ_DEMO_TTL", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.EmailCodeTTL, err = durationOr("SUPPQ_EMAIL_CODE_TTL", 10*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.RecognitionTimeout, err = durationOr("SUPPQ_RECOGNITION_TIMEOUT", 45*time.Second); err != nil {
		return Config{}, err
	}

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, errors.New("SUPPQ_DATABASE_URL is required")
	}
	if cfg.Environment == "production" {
		cfg.CookieSecure = true
		if len(cfg.TokenPepper) < 32 || cfg.TokenPepper == "suppq-development-pepper-not-for-production" {
			return Config{}, errors.New("SUPPQ_TOKEN_PEPPER must be a production secret of at least 32 characters")
		}
		if strings.TrimSpace(os.Getenv("SUPPQ_SMTP_HOST")) == "" || strings.TrimSpace(os.Getenv("SUPPQ_SMTP_FROM")) == "" {
			return Config{}, errors.New("SUPPQ_SMTP_HOST and SUPPQ_SMTP_FROM are required in production")
		}
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
		if strings.TrimSpace(os.Getenv("SUPPQ_OBJECT_ENDPOINT")) == "" || strings.TrimSpace(os.Getenv("SUPPQ_OBJECT_ACCESS_KEY")) == "" || strings.TrimSpace(os.Getenv("SUPPQ_OBJECT_SECRET_KEY")) == "" || strings.TrimSpace(os.Getenv("SUPPQ_OBJECT_BUCKET")) == "" {
			return Config{}, errors.New("production object storage configuration is required")
		}
		if !cfg.ObjectSecure {
			return Config{}, errors.New("SUPPQ_OBJECT_SECURE must be true in production")
		}
	}
	if cfg.RecognitionProvider != "fake" && cfg.RecognitionProvider != "openai_vision" {
		return Config{}, errors.New("SUPPQ_RECOGNITION_PROVIDER must be fake or openai_vision")
	}
	return cfg, nil
}

func (cfg Config) ValidateRecognitionWorker() error {
	if cfg.Environment != "production" {
		return nil
	}
	if cfg.RecognitionProvider == "fake" {
		return errors.New("fake recognition provider is forbidden in production")
	}
	if cfg.RecognitionProvider != "openai_vision" || cfg.RecognitionBaseURL == "" || cfg.RecognitionAPIKey == "" || cfg.RecognitionModel == "" {
		return errors.New("production recognition provider configuration is incomplete")
	}
	return nil
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
