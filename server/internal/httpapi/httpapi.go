package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"suppq.local/server/internal/catalog"
	"suppq.local/server/internal/identity"
	"suppq.local/server/internal/recognition"
)

type DatabaseChecker interface {
	Ping(context.Context) error
}

type Dependencies struct {
	AllowedOrigin string
	Database      DatabaseChecker
	Logger        *slog.Logger
	Version       string
	Identity      *identity.Service
	Catalog       *catalog.Service
	Recognition   *recognition.Service
	SessionCookie string
	CookieSecure  bool
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
	if deps.Identity != nil {
		registerIdentityRoutes(mux, deps)
	}
	if deps.Identity != nil && deps.Catalog != nil {
		registerCatalogRoutes(mux, deps)
	}
	if deps.Identity != nil && deps.Recognition != nil {
		registerRecognitionRoutes(mux, deps)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response := errorResponse{}
		response.Error.Code = "route_not_found"
		response.Error.Message = "The requested route does not exist."
		response.Error.RequestID = requestID(r.Context())
		writeJSON(w, http.StatusNotFound, response)
	})

	return requestIDMiddleware(loggingMiddleware(deps.Logger, corsMiddleware(deps.AllowedOrigin, mux)))
}

func registerCatalogRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		items, err := deps.Catalog.ListProducts(r.Context(), catalog.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID})
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		var body catalog.CreateProductInput
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		created, err := deps.Catalog.CreateProduct(r.Context(), catalog.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, body)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})
	mux.HandleFunc("GET /api/v1/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		item, err := deps.Catalog.GetProduct(r.Context(), catalog.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, r.PathValue("id"))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("POST /api/v1/products/{id}/batches", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		var body catalog.BatchInput
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		item, err := deps.Catalog.AddBatch(r.Context(), catalog.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, r.PathValue("id"), body)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})
	mux.HandleFunc("GET /api/v1/today", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		scope := catalog.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}
		if actor.Kind == "demo_ephemeral" {
			if err := deps.Catalog.EnsureDemo(r.Context(), scope); err != nil {
				writeApplicationError(w, r, deps.Logger, err)
				return
			}
		}
		date := strings.TrimSpace(r.URL.Query().Get("date"))
		if date == "" {
			date = time.Now().UTC().Format("2006-01-02")
		}
		items, err := deps.Catalog.Today(r.Context(), scope, date)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"date": date, "items": items})
	})
	mux.HandleFunc("POST /api/v1/intakes", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		var body catalog.CreateIntakeInput
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		intake, product, err := deps.Catalog.CreateIntake(r.Context(), catalog.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, body, strings.TrimSpace(r.Header.Get("Idempotency-Key")))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"intake": intake, "product": product})
	})
	mux.HandleFunc("DELETE /api/v1/intakes/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		intake, product, err := deps.Catalog.UndoIntake(r.Context(), catalog.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, r.PathValue("id"))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"intake": intake, "product": product})
	})
}

func registerRecognitionRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("POST /api/v1/recognition/sets", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 31<<20)
		if err := r.ParseMultipartForm(31 << 20); err != nil {
			writeApplicationError(w, r, deps.Logger, recognition.NewHTTPError("invalid_upload", "上传内容无效或超过 30 MB。"))
			return
		}
		defer r.MultipartForm.RemoveAll()
		if len(r.MultipartForm.File) != 3 || len(r.MultipartForm.File["front"]) != 1 || len(r.MultipartForm.File["facts"]) != 1 || len(r.MultipartForm.File["expiry"]) != 1 {
			writeApplicationError(w, r, deps.Logger, recognition.NewHTTPError("invalid_upload", "必须且只能提供正面、成分表和有效期三张图片。"))
			return
		}
		uploads := make([]recognition.Upload, 0, 3)
		for _, role := range []string{"front", "facts", "expiry"} {
			file, header, err := r.FormFile(role)
			if err != nil {
				writeApplicationError(w, r, deps.Logger, recognition.NewHTTPError("invalid_upload", "必须同时提供正面、成分表和有效期三张图片。"))
				return
			}
			data, readErr := io.ReadAll(io.LimitReader(file, (10<<20)+1))
			_ = file.Close()
			if readErr != nil {
				writeApplicationError(w, r, deps.Logger, recognition.NewHTTPError("invalid_upload", "图片读取失败。"))
				return
			}
			uploads = append(uploads, recognition.Upload{Role: role, Name: header.Filename, DeclaredMIME: header.Header.Get("Content-Type"), Data: data})
		}
		created, err := deps.Recognition.CreateSet(r.Context(), recognition.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, uploads)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusAccepted, created)
	})
	mux.HandleFunc("GET /api/v1/recognition/sets/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		item, err := deps.Recognition.GetSet(r.Context(), recognition.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, r.PathValue("id"))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("POST /api/v1/recognition/jobs/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		item, err := deps.Recognition.RetryJob(r.Context(), recognition.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, r.PathValue("id"))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusAccepted, item)
	})
	mux.HandleFunc("POST /api/v1/recognition/sets/{id}/confirm", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		var body catalog.CreateProductInput
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		product, err := deps.Recognition.Confirm(r.Context(), recognition.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, r.PathValue("id"), body)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, product)
	})
	mux.HandleFunc("GET /api/v1/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := catalogActor(w, r, deps)
		if !ok {
			return
		}
		mimeType, data, err := deps.Recognition.GetFile(r.Context(), recognition.Scope{UserID: actor.UserID, WorkspaceID: actor.WorkspaceID}, r.PathValue("id"))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func catalogActor(w http.ResponseWriter, r *http.Request, deps Dependencies) (identity.Actor, bool) {
	actor, err := deps.Identity.ActorForToken(r.Context(), sessionToken(r, deps.SessionCookie), true)
	if err != nil {
		writeApplicationError(w, r, deps.Logger, err)
		return identity.Actor{}, false
	}
	return actor, true
}

func registerIdentityRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/session", func(w http.ResponseWriter, r *http.Request) {
		result, err := deps.Identity.EnsureSession(r.Context(), sessionToken(r, deps.SessionCookie))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		if result.Fresh {
			setSessionCookie(w, deps, result.Token)
		}
		writeJSON(w, http.StatusOK, map[string]any{"actor": result.Actor})
	})
	mux.HandleFunc("POST /api/v1/auth/email-code/request", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email      string `json:"email"`
			Invitation string `json:"invitation"`
		}
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		if err := deps.Identity.RequestLoginCode(r.Context(), body.Email, body.Invitation); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	})
	mux.HandleFunc("POST /api/v1/auth/email-code/verify", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email      string `json:"email"`
			Code       string `json:"code"`
			Invitation string `json:"invitation"`
			Password   string `json:"password"`
		}
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		result, err := deps.Identity.VerifyLoginCode(r.Context(), body.Email, body.Code, body.Invitation, body.Password, sessionToken(r, deps.SessionCookie))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		setSessionCookie(w, deps, result.Token)
		writeJSON(w, http.StatusOK, map[string]any{"actor": result.Actor})
	})
	mux.HandleFunc("POST /api/v1/auth/password/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		result, err := deps.Identity.PasswordLogin(r.Context(), body.Email, body.Password, sessionToken(r, deps.SessionCookie))
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		setSessionCookie(w, deps, result.Token)
		writeJSON(w, http.StatusOK, map[string]any{"actor": result.Actor})
	})
	mux.HandleFunc("POST /api/v1/auth/password-reset/request", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		if err := deps.Identity.RequestPasswordReset(r.Context(), body.Email); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	})
	mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		if err := deps.Identity.ConfirmPasswordReset(r.Context(), body.Email, body.Code, body.Password); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		clearSessionCookie(w, deps)
		writeJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
	})
	mux.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Identity.Logout(r.Context(), sessionToken(r, deps.SessionCookie)); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		clearSessionCookie(w, deps)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/admin/invitations", func(w http.ResponseWriter, r *http.Request) {
		actor, err := deps.Identity.ActorForToken(r.Context(), sessionToken(r, deps.SessionCookie), true)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		items, err := deps.Identity.ListInvitations(r.Context(), actor)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/v1/admin/invitations", func(w http.ResponseWriter, r *http.Request) {
		actor, err := deps.Identity.ActorForToken(r.Context(), sessionToken(r, deps.SessionCookie), true)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		var body identity.CreateInvitationInput
		if err = readJSON(r, &body); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		created, err := deps.Identity.CreateInvitation(r.Context(), actor, body)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})
	mux.HandleFunc("DELETE /api/v1/admin/invitations/{id}", func(w http.ResponseWriter, r *http.Request) {
		actor, err := deps.Identity.ActorForToken(r.Context(), sessionToken(r, deps.SessionCookie), true)
		if err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		if err = deps.Identity.RevokeInvitation(r.Context(), actor, r.PathValue("id")); err != nil {
			writeApplicationError(w, r, deps.Logger, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func readJSON(r *http.Request, target any) error {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return identity.NewError("invalid_content_type", "请求必须使用 application/json。")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return identity.NewError("invalid_json", "请求内容格式无效。")
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return identity.NewError("invalid_json", "请求只能包含一个 JSON 对象。")
	}
	return nil
}

func sessionToken(r *http.Request, name string) string {
	if name == "" {
		name = "suppq_session"
	}
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, deps Dependencies, token string) {
	name := deps.SessionCookie
	if name == "" {
		name = "suppq_session"
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: token, Path: "/", HttpOnly: true, Secure: deps.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 60 * 60})
}
func clearSessionCookie(w http.ResponseWriter, deps Dependencies) {
	name := deps.SessionCookie
	if name == "" {
		name = "suppq_session"
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: deps.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func writeApplicationError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "服务暂时不可用。"
	var appErr *identity.Error
	if errors.As(err, &appErr) {
		code = appErr.Code
		message = appErr.Message
		switch code {
		case "unauthorized":
			status = http.StatusUnauthorized
		case "forbidden":
			status = http.StatusForbidden
		case "rate_limited":
			status = http.StatusTooManyRequests
		case "invitation_required", "invitation_invalid", "invalid_credentials", "invalid_email", "invalid_password", "invalid_json", "invalid_content_type", "invalid_invitation_kind", "invalid_expiration", "invalid_max_uses":
			status = http.StatusBadRequest
		case "invitation_not_found":
			status = http.StatusNotFound
		case "email_delivery_failed":
			status = http.StatusBadGateway
		}
	}
	var catalogErr *catalog.Error
	if errors.As(err, &catalogErr) {
		code, message = catalogErr.Code, catalogErr.Message
		switch code {
		case "resource_not_found":
			status = http.StatusNotFound
		case "insufficient_inventory", "intake_already_undone":
			status = http.StatusConflict
		case "invalid_product", "invalid_schedule", "invalid_batch", "invalid_ingredient", "invalid_intake", "invalid_date":
			status = http.StatusBadRequest
		}
	}
	var recognitionErr *recognition.Error
	if errors.As(err, &recognitionErr) {
		code, message = recognitionErr.Code, recognitionErr.Message
		switch code {
		case "resource_not_found":
			status = http.StatusNotFound
		case "recognition_processing":
			status = http.StatusConflict
		case "recognition_cancelled":
			status = http.StatusGone
		case "invalid_upload":
			status = http.StatusBadRequest
		case "storage_unavailable":
			status = http.StatusServiceUnavailable
		}
	}
	if status >= 500 {
		logger.Error("application request failed", "request_id", requestID(r.Context()), "error", err)
	}
	response := errorResponse{}
	response.Error.Code = code
	response.Error.Message = message
	response.Error.RequestID = requestID(r.Context())
	writeJSON(w, status, response)
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
