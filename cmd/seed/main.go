package main

import (
	"context"
	"log"

	"NovaQuant/internal/config"
	"NovaQuant/internal/db"
	"NovaQuant/internal/repository"
	"NovaQuant/internal/service"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.PGDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := repository.New(pool)
	if err := service.SeedDemoData(ctx, repo, cfg.DefaultSymbols); err != nil {
		log.Fatal(err)
	}
}
