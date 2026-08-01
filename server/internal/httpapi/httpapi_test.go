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
