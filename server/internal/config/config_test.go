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
	t.Setenv("SUPPQ_SMTP_REQUIRE_TLS", "true")
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
	t.Setenv("SUPPQ_SMTP_REQUIRE_TLS", "true")
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
	t.Setenv("SUPPQ_SMTP_REQUIRE_TLS", "true")
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
	if cfg.RecognitionProvider != "fake" {
		t.Fatalf("development provider must be explicit fake, got %q", cfg.RecognitionProvider)
	}
}

func TestFromEnvRejectsFakeProviderInProduction(t *testing.T) {
	t.Setenv("SUPPQ_ENV", "production")
	t.Setenv("SUPPQ_SMTP_REQUIRE_TLS", "true")
	t.Setenv("SUPPQ_DATABASE_URL", "postgres://user:pass@db.example/suppq?sslmode=require")
	t.Setenv("SUPPQ_TOKEN_PEPPER", "production-secret-that-is-longer-than-thirty-two-characters")
	t.Setenv("SUPPQ_SMTP_HOST", "smtp.example.com")
	t.Setenv("SUPPQ_SMTP_FROM", "no-reply@example.com")
	t.Setenv("SUPPQ_ALLOWED_ORIGIN", "https://app.example.com")
	t.Setenv("SUPPQ_TRUST_PROXY", "true")
	t.Setenv("SUPPQ_OBJECT_ENDPOINT", "s3.example.com")
	t.Setenv("SUPPQ_OBJECT_ACCESS_KEY", "access")
	t.Setenv("SUPPQ_OBJECT_SECRET_KEY", "secret")
	t.Setenv("SUPPQ_OBJECT_BUCKET", "suppq-private")
	t.Setenv("SUPPQ_OBJECT_SECURE", "true")
	t.Setenv("SUPPQ_RECOGNITION_PROVIDER", "fake")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("shared production configuration should be valid: %v", err)
	}
	if err = cfg.ValidateRecognitionWorker(); err == nil {
		t.Fatal("expected fake production provider to fail")
	}
}

func TestRecognitionWorkerRejectsIncompleteLiveProviderInProduction(t *testing.T) {
	cfg := Config{Environment: "production", RecognitionProvider: "openai_vision", RecognitionBaseURL: "https://vision.example.com/v1", RecognitionModel: "vision-model"}
	if err := cfg.ValidateRecognitionWorker(); err == nil {
		t.Fatal("expected missing live provider API key to fail")
	}
}

func TestRecognitionWorkerAcceptsEvidencePipelineAndRejectsMissingStage(t *testing.T) {
	cfg := Config{
		Environment: "production", RecognitionProvider: "evidence_pipeline", RecognitionMode: "ocr_llm",
		OCRBaseURL: "https://ocr.example.com/v1", OCRAPIKey: "ocr-secret", OCRModel: "ocr-model",
		StructureBaseURL: "https://kimi.example.com", StructureAPIKey: "kimi-secret", StructureModel: "kimi-model",
	}
	if err := cfg.ValidateRecognitionWorker(); err != nil {
		t.Fatalf("complete evidence pipeline should pass: %v", err)
	}
	cfg.StructureAPIKey = ""
	if err := cfg.ValidateRecognitionWorker(); err == nil {
		t.Fatal("missing structure key must fail")
	}
}

func TestRecognitionWorkerDualModeRequiresDirectProvider(t *testing.T) {
	cfg := Config{
		Environment: "production", RecognitionProvider: "evidence_pipeline", RecognitionMode: "dual",
		OCRBaseURL: "https://ocr.example.com/v1", OCRAPIKey: "ocr-secret", OCRModel: "ocr-model",
		StructureBaseURL: "https://kimi.example.com", StructureAPIKey: "kimi-secret", StructureModel: "kimi-model",
	}
	if err := cfg.ValidateRecognitionWorker(); err == nil {
		t.Fatal("dual mode without a direct provider must fail")
	}
	cfg.VLBaseURL, cfg.VLAPIKey, cfg.VLModel = "https://vl.example.com/v1", "vl-secret", "vl-model"
	if err := cfg.ValidateRecognitionWorker(); err != nil {
		t.Fatalf("complete dual pipeline should pass: %v", err)
	}
}

func TestFromEnvRequiresExplicitSecureObjectStorageInProduction(t *testing.T) {
	t.Setenv("SUPPQ_ENV", "production")
	t.Setenv("SUPPQ_SMTP_REQUIRE_TLS", "true")
	t.Setenv("SUPPQ_DATABASE_URL", "postgres://user:pass@db.example/suppq?sslmode=require")
	t.Setenv("SUPPQ_TOKEN_PEPPER", "production-secret-that-is-longer-than-thirty-two-characters")
	t.Setenv("SUPPQ_SMTP_HOST", "smtp.example.com")
	t.Setenv("SUPPQ_SMTP_FROM", "no-reply@example.com")
	t.Setenv("SUPPQ_ALLOWED_ORIGIN", "https://app.example.com")
	t.Setenv("SUPPQ_TRUST_PROXY", "true")
	t.Setenv("SUPPQ_RECOGNITION_PROVIDER", "openai_vision")
	t.Setenv("SUPPQ_RECOGNITION_BASE_URL", "https://vision.example.com/v1")
	t.Setenv("SUPPQ_RECOGNITION_API_KEY", "test-secret")
	t.Setenv("SUPPQ_RECOGNITION_MODEL", "vision-model")
	t.Setenv("SUPPQ_OBJECT_ENDPOINT", "")
	t.Setenv("SUPPQ_OBJECT_ACCESS_KEY", "")
	t.Setenv("SUPPQ_OBJECT_SECRET_KEY", "")
	t.Setenv("SUPPQ_OBJECT_BUCKET", "")
	t.Setenv("SUPPQ_OBJECT_SECURE", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected implicit development object defaults to fail in production")
	}

	t.Setenv("SUPPQ_OBJECT_ENDPOINT", "s3.example.com")
	t.Setenv("SUPPQ_OBJECT_ACCESS_KEY", "access")
	t.Setenv("SUPPQ_OBJECT_SECRET_KEY", "secret")
	t.Setenv("SUPPQ_OBJECT_BUCKET", "suppq-private")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected insecure production object storage to fail")
	}
	t.Setenv("SUPPQ_OBJECT_SECURE", "true")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("expected complete production configuration to pass: %v", err)
	}
	if err = cfg.ValidateRecognitionWorker(); err != nil {
		t.Fatalf("expected complete recognition worker configuration to pass: %v", err)
	}
}

func TestFromEnvRequiresProductionSMTPTLSAndHTTPSOrigin(t *testing.T) {
	t.Setenv("SUPPQ_ENV", "production")
	t.Setenv("SUPPQ_DATABASE_URL", "postgres://user:pass@db.example/suppq?sslmode=require")
	t.Setenv("SUPPQ_TOKEN_PEPPER", "production-secret-that-is-longer-than-thirty-two-characters")
	t.Setenv("SUPPQ_SMTP_HOST", "smtp.example.com")
	t.Setenv("SUPPQ_SMTP_FROM", "no-reply@example.com")
	t.Setenv("SUPPQ_OBJECT_ENDPOINT", "s3.example.com")
	t.Setenv("SUPPQ_OBJECT_ACCESS_KEY", "access")
	t.Setenv("SUPPQ_OBJECT_SECRET_KEY", "secret")
	t.Setenv("SUPPQ_OBJECT_BUCKET", "suppq-private")
	t.Setenv("SUPPQ_OBJECT_SECURE", "true")
	t.Setenv("SUPPQ_ALLOWED_ORIGIN", "http://app.example.com")
	t.Setenv("SUPPQ_TRUST_PROXY", "true")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected production SMTP without required TLS to fail")
	}
	t.Setenv("SUPPQ_SMTP_REQUIRE_TLS", "true")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected insecure production origin to fail")
	}
	t.Setenv("SUPPQ_ALLOWED_ORIGIN", "https://app.example.com")
	if _, err := FromEnv(); err != nil {
		t.Fatalf("expected hardened production transport configuration to pass: %v", err)
	}
}
