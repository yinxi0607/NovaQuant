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
	analyzer := service.NewAnalyzer(repo, registry)

	go serveWorkerHTTP(":"+cfg.AnalyzerPort, registry)
	ticker := time.NewTicker(cfg.ServiceTick)
	defer ticker.Stop()

	for {
		symbols, err := repo.ListEnabledSymbols(ctx)
		if err != nil {
			logger.Error("load symbols", "error", err)
		} else {
			for _, interval := range []string{"1h", "4h", "1d"} {
				if err := analyzer.RunOnce(ctx, symbols, interval); err != nil {
					logger.Error("analyzer cycle failed", "interval", interval, "error", err)
				}
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
