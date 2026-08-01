package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type DatabaseChecker interface {
	Ping(context.Context) error
}

type Dependencies struct {
	AllowedOrigin string
	Database      DatabaseChecker
	Logger        *slog.Logger
	Version       string
}

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	RequestID string `json:"requestId"`
	Timestamp string `json:"timestamp"`
	Database  string `json:"database,omitempty"`
}

type errorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	} `json:"error"`
}

type contextKey string

const requestIDKey contextKey = "request-id"

func New(deps Dependencies) http.Handler {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Status: "ok", Service: "suppq-api", Version: deps.Version,
			RequestID: requestID(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("GET /api/v1/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Database.Ping(r.Context()); err != nil {
			deps.Logger.Warn("readiness check failed", "request_id", requestID(r.Context()), "dependency", "postgres", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{
				Status: "degraded", Service: "suppq-api", Version: deps.Version,
				RequestID: requestID(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339), Database: "unavailable",
			})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{
			Status: "ok", Service: "suppq-api", Version: deps.Version,
			RequestID: requestID(r.Context()), Timestamp: time.Now().UTC().Format(time.RFC3339), Database: "ready",
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response := errorResponse{}
		response.Error.Code = "route_not_found"
		response.Error.Message = "The requested route does not exist."
		response.Error.RequestID = requestID(r.Context())
		writeJSON(w, http.StatusNotFound, response)
	})

	return requestIDMiddleware(loggingMiddleware(deps.Logger, corsMiddleware(deps.AllowedOrigin, mux)))
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "request_id", requestID(r.Context()), "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func corsMiddleware(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if origin != allowedOrigin {
				writeJSON(w, http.StatusForbidden, map[string]string{"status": "forbidden"})
				return
			}
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func newRequestID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buffer)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
