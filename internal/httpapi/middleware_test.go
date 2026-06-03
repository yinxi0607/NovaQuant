package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/service"
)

func TestCORSPreflightBypassesAuth(t *testing.T) {
	auth, err := service.NewAuthService(config.Config{
		AuthEnabled:     true,
		AuthUsername:    "admin",
		AuthPassword:    "secret",
		AuthTokenSecret: "test-secret",
		AuthTokenTTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/protected", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := http.Handler(mux)
	handler = withAuth(handler, auth, map[string]bool{})
	handler = withRateLimit(handler, newRateLimiter(120))
	handler = withLogging(handler, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	handler = withRecover(handler, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	handler = withCORS(handler)
	handler = withRequestID(handler)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/protected", nil)
	req.Header.Set("Origin", "http://localhost:51740")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("expected Access-Control-Allow-Headers to be present")
	}
}

func TestUnauthorizedResponseStillIncludesCORSHeaders(t *testing.T) {
	auth, err := service.NewAuthService(config.Config{
		AuthEnabled:     true,
		AuthUsername:    "admin",
		AuthPassword:    "secret",
		AuthTokenSecret: "test-secret",
		AuthTokenTTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/protected", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := http.Handler(mux)
	handler = withAuth(handler, auth, map[string]bool{})
	handler = withRateLimit(handler, newRateLimiter(120))
	handler = withLogging(handler, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	handler = withRecover(handler, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	handler = withCORS(handler)
	handler = withRequestID(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Origin", "http://localhost:51740")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", got)
	}
}
