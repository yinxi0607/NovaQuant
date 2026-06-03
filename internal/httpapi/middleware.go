package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"NovaQuant/internal/domain"
	"NovaQuant/internal/metrics"
	"NovaQuant/internal/service"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const authSessionKey contextKey = "auth_session"

type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]*rateWindow
	limit   int
}

type rateWindow struct {
	start time.Time
	count int
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{
		windows: map[string]*rateWindow{},
		limit:   limit,
	}
}

func (r *rateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	window := r.windows[key]
	if window == nil || now.Sub(window.start) >= time.Minute {
		r.windows[key] = &rateWindow{start: now, count: 1}
		return true
	}
	if window.count >= r.limit {
		return false
	}
	window.count++
	return true
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := randomID()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func withRecover(next http.Handler, logger *slog.Logger, registry *metrics.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if registry != nil {
					registry.Inc("http_errors_total")
				}
				logger.Error("panic recovered", "panic", rec, "path", r.URL.Path)
				writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler, logger *slog.Logger, registry *metrics.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if registry != nil {
			registry.Inc("http_requests_total")
		}
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "latency", time.Since(start).String())
	})
}

func withRateLimit(next http.Handler, limiter *rateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		if !limiter.Allow(host) {
			writeError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withAuth(next http.Handler, auth *service.AuthService, publicPaths map[string]bool) http.Handler {
	if auth == nil || !auth.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token", nil)
			return
		}
		session, err := auth.ValidateToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), nil)
			return
		}
		ctx := context.WithValue(r.Context(), authSessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestID(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDKey).(string); ok {
		return value
	}
	return ""
}

func authSessionFromContext(ctx context.Context) (domain.AuthSession, bool) {
	value, ok := ctx.Value(authSessionKey).(domain.AuthSession)
	return value, ok
}

func randomID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405")
	}
	return "req_" + hex.EncodeToString(buf)
}
