package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"suppq.local/server/internal/catalog"
)

type fakeDatabase struct{ err error }

func (database fakeDatabase) Ping(context.Context) error { return database.err }

func newTestHandler(database fakeDatabase) http.Handler {
	return New(Dependencies{
		AllowedOrigin: "http://127.0.0.1:5173",
		Database:      database,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:       "test",
	})
}

func TestLivenessDoesNotClaimDatabaseReadiness(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	newTestHandler(fakeDatabase{err: errors.New("down")}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), `"database":"ready"`) {
		t.Fatal("liveness must not claim database readiness")
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing server-generated request ID")
	}
}

func TestReadinessFailsWhenDatabaseFails(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil)
	newTestHandler(fakeDatabase{err: errors.New("down")}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"database":"unavailable"`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestReadinessPassesWhenDatabasePings(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil)
	newTestHandler(fakeDatabase{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"database":"ready"`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestCORSAllowsOnlyConfiguredOrigin(t *testing.T) {
	handler := newTestHandler(fakeDatabase{})
	for _, testCase := range []struct {
		origin  string
		allowed bool
	}{
		{origin: "http://127.0.0.1:5173", allowed: true},
		{origin: "https://attacker.invalid", allowed: false},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
		request.Header.Set("Origin", testCase.origin)
		handler.ServeHTTP(recorder, request)
		got := recorder.Header().Get("Access-Control-Allow-Origin")
		if testCase.allowed && got != testCase.origin {
			t.Fatalf("expected origin %q, got %q", testCase.origin, got)
		}
		if !testCase.allowed && got != "" {
			t.Fatalf("unexpected allowed origin %q", got)
		}
	}
}

func TestOperationalMetricsAndSecurityHeaders(t *testing.T) {
	handler := newTestHandler(fakeDatabase{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Permissions-Policy") == "" {
		t.Fatalf("missing API security headers: %+v", recorder.Header())
	}

	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Body.String(), "suppq_http_requests_total") || !strings.Contains(metrics.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("unexpected metrics response: %d %s", metrics.Code, metrics.Body.String())
	}
}

func TestAuthenticationEndpointsHaveIPRateLimit(t *testing.T) {
	handler := newTestHandler(fakeDatabase{})
	var last *httptest.ResponseRecorder
	for range 31 {
		last = httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/login", strings.NewReader(`{}`))
		request.RemoteAddr = "192.0.2.10:12345"
		handler.ServeHTTP(last, request)
	}
	if last == nil || last.Code != http.StatusTooManyRequests || last.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected auth IP throttling, got %+v", last)
	}
}

func TestIdempotencyConflictIsHTTPConflict(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/intakes", nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	writeApplicationError(recorder, request, logger,
		catalog.NewError("idempotency_conflict", "同一个幂等键不能用于不同的服用请求。"))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"idempotency_conflict"`) {
		t.Fatalf("unexpected error body: %s", recorder.Body.String())
	}
}
