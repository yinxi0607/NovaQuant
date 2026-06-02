SHELL := /bin/zsh
NODE_BIN ?= $(HOME)/.nvm/versions/node/v23.11.0/bin
export PATH := $(NODE_BIN):$(PATH)

ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: dev migrate seed check-data mcp-db test test-go test-python test-web test-e2e lint build build-web docker-build up down logs acceptance compose-config

dev:
	docker compose --env-file .env up --build api agent web redis

migrate:
	go run ./cmd/migrate

seed:
	go run ./cmd/seed

check-data:
	go run ./cmd/checkdata

mcp-db:
	go run ./cmd/mcp-db

test:
	$(MAKE) test-go
	$(MAKE) test-web

test-go:
	go test ./...

test-python:
	@echo "Python agent removed by request; running Go agent/service tests instead."
	go test ./internal/service/...

test-web:
	cd web && npm test

test-e2e:
	cd web && npm run test:e2e

lint:
	gofmt -w cmd internal
	cd web && npm run build >/dev/null

build:
	go build ./...
	$(MAKE) build-web

build-web:
	cd web && npm run build

docker-build:
	docker compose build

up:
	docker compose --env-file .env up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

compose-config:
	docker compose --env-file .env config

acceptance:
	$(MAKE) migrate
	$(MAKE) seed
	$(MAKE) test
	$(MAKE) compose-config
