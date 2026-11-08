SHELL := /bin/bash

GO            ?= go
DATABASE_URL  ?= postgres://anekdot:anekdot@localhost:5432/anekdot?sslmode=disable
DATA_DIR      ?= data
WEB_DIR       ?= backend/web
BIN_DIR       := bin
DB_USER       ?= anekdot
DB_PASSWORD   ?= anekdot
DB_NAME       ?= anekdot
PG_SERVICE    ?= postgresql
PG_ADMIN      ?= sudo -n -u postgres
API_URL       ?= http://127.0.0.1:8080
PSQL          := env PGPASSWORD=$(DB_PASSWORD) psql -h 127.0.0.1 -U $(DB_USER) -d $(DB_NAME) -P pager=off

.PHONY: help db-start db-init db-reset db-setup dev check smoke build run test lint fmt vet tidy migrate-up migrate-down migrate-status seed frontend-install frontend-build frontend-dev clean

help:
	@echo "db-start         start the local PostgreSQL cluster"
	@echo "db-init          create the local role and database if missing"
	@echo "db-reset         drop and recreate the local database"
	@echo "db-setup         db-start + db-init + migrate-up + seed"
	@echo "db-check         count rows in the local database"
	@echo "dev              db-setup, then run the API server"
	@echo "check            fmt, vet, test, lint and the frontend build"
	@echo "smoke            curl the running API"
	@echo "build            compile api, migrate and seed binaries"
	@echo "run              run the API server"
	@echo "test             run Go tests"
	@echo "lint             run golangci-lint"
	@echo "fmt              format Go sources"
	@echo "vet              run go vet"
	@echo "migrate-up       apply all migrations"
	@echo "migrate-down     roll back all migrations"
	@echo "migrate-status   show the current migration version"
	@echo "seed             import legacy data (idempotent)"
	@echo "frontend-install install npm dependencies"
	@echo "frontend-build   build the SPA into ./backend/web"
	@echo "frontend-dev     start the Vite dev server"

db-start:
	@pg_isready -q -h 127.0.0.1 -p 5432 || sudo -n systemctl start $(PG_SERVICE)
	@for i in $$(seq 1 30); do pg_isready -q -h 127.0.0.1 -p 5432 && break; sleep 1; done
	@pg_isready -h 127.0.0.1 -p 5432

db-init: db-start
	@$(PG_ADMIN) psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_roles WHERE rolname='$(DB_USER)'" | grep -q 1 \
		|| $(PG_ADMIN) psql -v ON_ERROR_STOP=1 -c "CREATE ROLE $(DB_USER) LOGIN PASSWORD '$(DB_PASSWORD)'"
	@$(PG_ADMIN) psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM pg_database WHERE datname='$(DB_NAME)'" | grep -q 1 \
		|| $(PG_ADMIN) createdb -O $(DB_USER) $(DB_NAME)
	@PGPASSWORD=$(DB_PASSWORD) psql -h 127.0.0.1 -U $(DB_USER) -d $(DB_NAME) -P pager=off -tAc 'SELECT 1' > /dev/null
	@echo "local database $(DB_NAME) is ready"

db-reset: db-start
	@$(PG_ADMIN) psql -v ON_ERROR_STOP=1 -tAc "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$(DB_NAME)' AND pid <> pg_backend_pid()" > /dev/null
	@$(PG_ADMIN) dropdb --if-exists $(DB_NAME)
	@$(PG_ADMIN) createdb -O $(DB_USER) $(DB_NAME)
	@echo "local database $(DB_NAME) recreated"

db-setup: db-init migrate-up seed

db-check:
	@$(PSQL) -tAc "SELECT 'categories=' || (SELECT count(*) FROM categories) || ' anekdots=' || (SELECT count(*) FROM anekdots) || ' users=' || (SELECT count(*) FROM users)"

dev: db-setup run

check: fmt vet test lint frontend-build

smoke:
	@curl -fsS $(API_URL)/api/v1/health | grep -q '"status":"ok"'
	@curl -fsS $(API_URL)/api/v1/categories | grep -q '"categories"'
	@curl -fsS $(API_URL)/api/v1/anekdots/random | grep -q '"text"'
	@curl -fsS $(API_URL)/api/v1/anekdots/external | grep -q '"text"'
	@curl -fsS -o /dev/null -w 'spa %{http_code}\n' $(API_URL)/
	@echo "api smoke ok"

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/api ./backend/cmd/api
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/migrate ./backend/cmd/migrate
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/seed ./backend/cmd/seed

run:
	env DATABASE_URL="$(DATABASE_URL)" DATA_DIR="$(DATA_DIR)" WEB_DIR="$(WEB_DIR)" $(GO) run ./backend/cmd/api

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
	env DATABASE_URL="$(DATABASE_URL)" $(GO) run ./backend/cmd/migrate -direction up

migrate-down:
	env DATABASE_URL="$(DATABASE_URL)" $(GO) run ./backend/cmd/migrate -direction down

migrate-status:
	env DATABASE_URL="$(DATABASE_URL)" $(GO) run ./backend/cmd/migrate -direction version

seed:
	env DATABASE_URL="$(DATABASE_URL)" DATA_DIR="$(DATA_DIR)" $(GO) run ./backend/cmd/seed

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build

frontend-dev:
	cd frontend && npm run dev

clean:
	rm -rf $(BIN_DIR) $(WEB_DIR)
