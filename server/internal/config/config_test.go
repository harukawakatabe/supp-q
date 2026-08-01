package config

import "testing"

func TestFromEnvRequiresDatabaseURL(t *testing.T) {
	t.Setenv("SUPPQ_DATABASE_URL", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected missing database URL to fail")
	}
}

func TestFromEnvRejectsDisabledTLSInProduction(t *testing.T) {
	t.Setenv("SUPPQ_ENV", "production")
	t.Setenv("SUPPQ_TOKEN_PEPPER", "production-secret-that-is-longer-than-thirty-two-characters")
	t.Setenv("SUPPQ_SMTP_HOST", "smtp.example.com")
	t.Setenv("SUPPQ_SMTP_FROM", "no-reply@example.com")
	t.Setenv("SUPPQ_DATABASE_URL", "postgres://user:pass@db.example/suppq?sslmode=disable")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected disabled production TLS to fail")
	}
}

func TestFromEnvRejectsDisabledTLSForPostgresqlScheme(t *testing.T) {
	t.Setenv("SUPPQ_ENV", "production")
	t.Setenv("SUPPQ_TOKEN_PEPPER", "production-secret-that-is-longer-than-thirty-two-characters")
	t.Setenv("SUPPQ_SMTP_HOST", "smtp.example.com")
	t.Setenv("SUPPQ_SMTP_FROM", "no-reply@example.com")
	t.Setenv("SUPPQ_DATABASE_URL", "postgresql://user:pass@db.example/suppq?sslmode=disable")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected disabled production TLS to fail for postgresql scheme")
	}
}

func TestFromEnvRejectsDevelopmentPepperInProduction(t *testing.T) {
	t.Setenv("SUPPQ_ENV", "production")
	t.Setenv("SUPPQ_DATABASE_URL", "postgres://user:pass@db.example/suppq?sslmode=require")
	t.Setenv("SUPPQ_SMTP_HOST", "smtp.example.com")
	t.Setenv("SUPPQ_SMTP_FROM", "no-reply@example.com")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected missing production token pepper to fail")
	}
}

func TestFromEnvAcceptsLocalDefaults(t *testing.T) {
	t.Setenv("SUPPQ_ENV", "development")
	t.Setenv("SUPPQ_DATABASE_URL", "postgres://suppq:local@127.0.0.1/suppq?sslmode=disable")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("unexpected address %q", cfg.HTTPAddr)
	}
}
