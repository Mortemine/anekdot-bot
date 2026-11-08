SHELL := /bin/bash

GO            ?= go
DATABASE_URL  ?= postgres://anekdot:anekdot@localhost:5432/anekdot?sslmode=disable
DATA_DIR      ?= ../data
WEB_DIR       ?= web
ADMIN_TOKEN   ?=
BIN_DIR       := bin

.PHONY: help build run test lint fmt vet tidy migrate-up migrate-down seed frontend-install frontend-build frontend-dev clean

help:
	@echo "build            compile api, migrate and seed binaries"
	@echo "run              run the API server"
	@echo "test             run Go tests"
	@echo "lint             run golangci-lint"
	@echo "fmt              format Go sources"
	@echo "vet              run go vet"
	@echo "migrate-up       apply all migrations"
	@echo "migrate-down     roll back all migrations"
	@echo "seed             import legacy data (idempotent)"
	@echo "frontend-install install npm dependencies"
	@echo "frontend-build   build the SPA into ./web"
	@echo "frontend-dev     start the Vite dev server"

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/api ./cmd/api
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/migrate ./cmd/migrate
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/seed ./cmd/seed

run:
	env DATABASE_URL="$(DATABASE_URL)" DATA_DIR="$(DATA_DIR)" WEB_DIR="$(WEB_DIR)" ADMIN_TOKEN="$(ADMIN_TOKEN)" $(GO) run ./cmd/api

test:
	$(GO) test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

migrate-up:
	env DATABASE_URL="$(DATABASE_URL)" $(GO) run ./cmd/migrate -direction up

migrate-down:
	env DATABASE_URL="$(DATABASE_URL)" $(GO) run ./cmd/migrate -direction down

seed:
	env DATABASE_URL="$(DATABASE_URL)" DATA_DIR="$(DATA_DIR)" $(GO) run ./cmd/seed

frontend-install:
	cd ../frontend && npm install

frontend-build:
	cd ../frontend && npm run build

frontend-dev:
	cd ../frontend && npm run dev

clean:
	rm -rf $(BIN_DIR) web
