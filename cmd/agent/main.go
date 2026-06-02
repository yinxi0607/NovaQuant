package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"NovaQuant/internal/config"
	"NovaQuant/internal/db"
	"NovaQuant/internal/httpapi"
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
	agent := service.NewAgent(repo)
	backtester := service.NewBacktester(repo)
	server := httpapi.NewServer(cfg, logger, repo, agent, backtester, registry)

	logger.Info("agent starting", "port", cfg.AgentPort)
	if err := httpapi.Serve(":"+cfg.AgentPort, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
