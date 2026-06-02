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
	alerts := service.NewAlerts(repo, registry)

	go serveWorkerHTTP(":"+cfg.AlertsPort, registry)
	ticker := time.NewTicker(cfg.ServiceTick)
	defer ticker.Stop()

	for {
		if err := alerts.RunOnce(ctx); err != nil {
			logger.Error("alerts cycle failed", "error", err)
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
