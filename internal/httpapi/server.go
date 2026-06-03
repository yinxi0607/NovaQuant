package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/domain"
	"NovaQuant/internal/metrics"
	"NovaQuant/internal/repository"
	"NovaQuant/internal/service"
)

var allowedIntervals = map[string]bool{
	"1m": true, "5m": true, "15m": true, "1h": true, "4h": true, "1d": true,
}

type Server struct {
	cfg        config.Config
	logger     *slog.Logger
	repo       *repository.Repository
	agent      *service.Agent
	auth       *service.AuthService
	backtester *service.Backtester
	metrics    *metrics.Registry
	cache      *memoryCache
}

type memoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	value   []byte
	expires time.Time
}

func NewServer(cfg config.Config, logger *slog.Logger, repo *repository.Repository, agent *service.Agent, auth *service.AuthService, backtester *service.Backtester, registry *metrics.Registry) *Server {
	if auth == nil {
		auth = &service.AuthService{}
	}
	return &Server{
		cfg:        cfg,
		logger:     logger,
		repo:       repo,
		agent:      agent,
		auth:       auth,
		backtester: backtester,
		metrics:    registry,
		cache:      &memoryCache{items: map[string]cacheItem{}},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/auth/config", s.handleAuthConfig)
	mux.HandleFunc("GET /api/v1/auth/public-key", s.handleAuthConfig)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("GET /api/v1/auth/session", s.handleAuthSession)
	mux.HandleFunc("GET /api/v1/symbols", s.handleSymbols)
	mux.HandleFunc("GET /api/v1/market/overview", s.handleOverview)
	mux.HandleFunc("GET /api/v1/market/{symbol}", s.handleMarketSnapshot)
	mux.HandleFunc("GET /api/v1/klines/{symbol}", s.handleKlines)
	mux.HandleFunc("GET /api/v1/analysis/{symbol}", s.handleAnalysis)
	mux.HandleFunc("GET /api/v1/risk/{symbol}", s.handleRisk)
	mux.HandleFunc("GET /api/v1/news", s.handleNews)
	mux.HandleFunc("GET /api/v1/etf", s.handleETF)
	mux.HandleFunc("GET /api/v1/whale", s.handleWhale)
	mux.HandleFunc("GET /api/v1/alerts", s.handleAlerts)
	mux.HandleFunc("POST /api/v1/agent/chat", s.handleAgentChat)
	mux.HandleFunc("POST /api/v1/backtest", s.handleBacktest)
	mux.HandleFunc("GET /metrics", s.metrics.Handler())
	mux.HandleFunc("GET /swagger/index.html", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("OpenAPI is generated into docs/openapi.yaml in this scaffold."))
	})

	handler := http.Handler(mux)
	handler = withAuth(handler, s.auth, map[string]bool{
		"/api/v1/health":          true,
		"/api/v1/auth/config":     true,
		"/api/v1/auth/public-key": true,
		"/api/v1/auth/login":      true,
	})
	handler = withRateLimit(handler, newRateLimiter(120))
	handler = withLogging(handler, s.logger, s.metrics)
	handler = withRecover(handler, s.logger, s.metrics)
	handler = withCORS(handler)
	handler = withRequestID(handler)
	return handler
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	statuses := map[string]string{
		"database": "down",
		"redis":    checkEndpoint(s.cfg.RedisURL),
	}
	overall := "ok"
	if err := s.repo.Ping(ctx); err != nil {
		statuses["database"] = err.Error()
		overall = "degraded"
	} else {
		statuses["database"] = "up"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     overall,
		"services":   statuses,
		"request_id": requestID(r.Context()),
	})
}

func (s *Server) handleAuthConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.auth.Config())
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !s.auth.Enabled() {
		writeError(w, r, http.StatusBadRequest, "AUTH_DISABLED", "authentication is disabled", nil)
		return
	}
	var req domain.AuthLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", nil)
		return
	}
	resp, err := s.auth.Login(r.Context(), req)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "AUTH_FAILED", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthSession(w http.ResponseWriter, r *http.Request) {
	session, ok := authSessionFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing session", nil)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleSymbols(w http.ResponseWriter, r *http.Request) {
	result, err := s.repo.ListSymbols(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"symbols": result, "request_id": requestID(r.Context())})
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	key := "overview"
	if cached, ok := s.cache.get(key); ok {
		writeRawJSON(w, cached)
		return
	}
	result, err := s.repo.GetMarketOverview(r.Context(), nil)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), nil)
		return
	}
	payload := map[string]any{
		"cards":        result,
		"refreshed_at": time.Now().UTC(),
		"request_id":   requestID(r.Context()),
	}
	writeAndCacheJSON(w, s.cache, key, 15*time.Second, payload)
}

func (s *Server) handleMarketSnapshot(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.PathValue("symbol"))
	key := "snapshot:" + symbol
	if cached, ok := s.cache.get(key); ok {
		writeRawJSON(w, cached)
		return
	}
	item, err := s.repo.GetMarketSnapshot(r.Context(), symbol)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "symbol snapshot not found", nil)
		return
	}
	writeAndCacheJSON(w, s.cache, key, 15*time.Second, item)
}

func (s *Server) handleKlines(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.PathValue("symbol"))
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}
	if !allowedIntervals[interval] {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid interval", map[string]any{"allowed": allowedIntervalList()})
		return
	}
	limit := parseQueryInt(r, "limit", 200, 1, 500)
	result, err := s.repo.ListKlines(r.Context(), symbol, interval, limit)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"symbol": symbol, "interval": interval, "ohlcv": result, "request_id": requestID(r.Context())})
}

func (s *Server) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.PathValue("symbol"))
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}
	if !allowedIntervals[interval] {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid interval", map[string]any{"allowed": allowedIntervalList()})
		return
	}
	key := "analysis:" + symbol + ":" + interval
	if cached, ok := s.cache.get(key); ok {
		writeRawJSON(w, cached)
		return
	}
	item, err := s.repo.GetLatestAnalysis(r.Context(), symbol, interval)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "analysis not found", nil)
		return
	}
	writeAndCacheJSON(w, s.cache, key, 20*time.Second, item)
}

func (s *Server) handleRisk(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.PathValue("symbol"))
	item, err := s.repo.GetLatestRisk(r.Context(), symbol)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "risk not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleNews(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	pageSize := parseQueryInt(r, "page_size", 20, 1, 200)
	items, err := s.repo.ListNews(r.Context(), symbol, pageSize)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"news": items})
}

func (s *Server) handleETF(w http.ResponseWriter, r *http.Request) {
	asset := strings.ToUpper(r.URL.Query().Get("asset"))
	pageSize := parseQueryInt(r, "page_size", 20, 1, 200)
	items, err := s.repo.ListETFFlows(r.Context(), asset, pageSize)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"flows": items})
}

func (s *Server) handleWhale(w http.ResponseWriter, r *http.Request) {
	asset := strings.ToUpper(r.URL.Query().Get("asset"))
	pageSize := parseQueryInt(r, "page_size", 20, 1, 200)
	items, err := s.repo.ListWhaleTransactions(r.Context(), asset, pageSize)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": items})
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(r.URL.Query().Get("symbol"))
	pageSize := parseQueryInt(r, "page_size", 20, 1, 200)
	items, err := s.repo.ListAlerts(r.Context(), symbol, pageSize)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "QUERY_ERROR", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": items})
}

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	var req domain.AgentChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", nil)
		return
	}
	if strings.TrimSpace(req.Question) == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "question is required", nil)
		return
	}
	resp, err := s.agent.Chat(r.Context(), req)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "AGENT_ERROR", err.Error(), nil)
		return
	}
	if s.metrics != nil {
		s.metrics.Inc("agent_requests_total")
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleBacktest(w http.ResponseWriter, r *http.Request) {
	var req domain.BacktestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", nil)
		return
	}
	if req.Symbol == "" || req.Interval == "" || req.StartTime.IsZero() || req.EndTime.IsZero() {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "symbol, interval, start_time and end_time are required", nil)
		return
	}
	resp, err := s.backtester.Run(r.Context(), req)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "BACKTEST_ERROR", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseQueryInt(r *http.Request, key string, fallback, minValue, maxValue int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	raw, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func writeRawJSON(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func writeAndCacheJSON(w http.ResponseWriter, cache *memoryCache, key string, ttl time.Duration, payload any) {
	raw, _ := json.Marshal(payload)
	cache.set(key, raw, ttl)
	writeRawJSON(w, raw)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	writeJSON(w, status, map[string]any{
		"code":       code,
		"message":    message,
		"details":    details,
		"request_id": requestID(r.Context()),
	})
}

func allowedIntervalList() []string {
	return []string{"1m", "5m", "15m", "1h", "4h", "1d"}
}

func (c *memoryCache) get(key string) ([]byte, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(item.expires) {
		if ok {
			c.mu.Lock()
			delete(c.items, key)
			c.mu.Unlock()
		}
		return nil, false
	}
	return item.value, true
}

func (c *memoryCache) set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cacheItem{value: value, expires: time.Now().Add(ttl)}
}

func checkEndpoint(raw string) string {
	if raw == "" {
		return "not_configured"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "invalid_url"
	}
	address := u.Host
	if !strings.Contains(address, ":") {
		switch u.Scheme {
		case "redis":
			address += ":6379"
		}
	}
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return "down"
	}
	_ = conn.Close()
	return "up"
}

func Serve(addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
