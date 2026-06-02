package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"NovaQuant/internal/config"
	"NovaQuant/internal/db"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.PGDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	files, err := os.ReadDir("migrations")
	if err != nil {
		log.Fatal(err)
	}
	var names []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			names = append(names, file.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join("migrations", name)
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			log.Fatalf("migration %s failed: %v", name, err)
		}
		fmt.Println("applied", name)
	}
}
