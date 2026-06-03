package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"NovaQuant/internal/config"
	"NovaQuant/internal/db"
	"NovaQuant/internal/metrics"
	"NovaQuant/internal/repository"
	"NovaQuant/internal/service"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.PGDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := repository.New(pool)
	registry := metrics.New()
	if !cfg.CollectorEnabled {
		logger.Info("collector disabled by configuration", "collector_enabled", cfg.CollectorEnabled)
		go serveWorkerHTTP(":"+cfg.CollectorPort, registry)
		select {}
	}
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	providers, err := service.NewMarketProviders(cfg, httpClient)
	if err != nil {
		log.Fatal(err)
	}
	providerNames := make([]string, 0, len(providers))
	for _, provider := range providers {
		providerNames = append(providerNames, provider.Name())
	}
	logger.Info("collector providers configured", "providers", providerNames)
	etfCollector := service.NewETFCollector(cfg, httpClient)
	if etfCollector == nil {
		logger.Info("etf collector disabled", "reason", "SOSO_ETF_API_KEY not configured")
	} else {
		logger.Info("etf collector configured", "provider", "soso", "country_code", cfg.ETFCountryCode)
	}
	newsCollector := service.NewNewsCollector(cfg, httpClient)
	if newsCollector == nil {
		logger.Info("news collector disabled", "reason", "SOSO_ETF_API_KEY not configured")
	} else {
		logger.Info("news collector configured", "provider", "soso")
	}
	collector := service.NewCollector(repo, providers, etfCollector, newsCollector, registry)

	go serveWorkerHTTP(":"+cfg.CollectorPort, registry)
	ticker := time.NewTicker(cfg.ServiceTick)
	defer ticker.Stop()

	for {
		symbols, err := repo.ListEnabledSymbols(ctx)
		if err != nil {
			logger.Error("load symbols", "error", err)
		} else if err := collector.RunOnce(ctx, symbols, cfg.CollectionIntervals); err != nil {
			if service.IsPartialCollectionError(err) {
				logger.Warn("collector cycle completed with partial failures", "error", err)
			} else {
				logger.Error("collector cycle failed", "error", err)
			}
		}
		<-ticker.C
	}
}

func serveWorkerHTTP(addr string, registry *metrics.Registry) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("GET /metrics", registry.Handler())
	_ = http.ListenAndServe(addr, mux)
}
