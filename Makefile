# STAIR PLATFORM — Makefile
# DEV-0009: единый набор команд: setup, run, test, migrate, seed, lint, build
# DEV-0010: docker-based dev stack: make up / make stop / make fe

GO        ?= go
GOLANGCI  ?= golangci-lint
COMPOSE   ?= docker compose
NPM       ?= npm

# Host-side DB URL (used by migrate/seed/run). Default matches docker-compose.
STAIR_DATABASE_URL ?= postgres://stair:stair@localhost:5432/stair_platform?sslmode=disable

.PHONY: setup up stop run test coverage coverage-check migrate seed lint build fmt vet env-up env-down clean frontend-install frontend-test frontend-build fe

## start everything except frontend (PostgreSQL + Redis + API in Docker) and apply migrations
up: env-up
	$(GO) run ./cmd/migrate -dir migrations -database "$(STAIR_DATABASE_URL)"
	@echo "Stack up: API on http://localhost:8080. Frontend: make fe"

## stop everything (Docker stack)
stop: env-down
	@echo "Stack stopped."

## start frontend dev server (http://localhost:5173, /api proxied to :8080)
fe:
	@test -d frontend/node_modules || $(NPM) --prefix frontend install
	$(NPM) --prefix frontend run dev

## env-bootstrap
setup: up

## run API locally (go run, outside Docker)
run: env-up
	$(GO) run ./cmd/api

## tests
test:
	$(GO) test ./...

## coverage report (per package)
coverage:
	$(GO) test ./... -coverprofile=coverage.out

## coverage quality gate (default threshold 85%; override with COVERAGE_THRESHOLD)
coverage-check:
	./scripts/coverage-check.sh

## database migrations
migrate:
	$(GO) run ./cmd/migrate -dir migrations -database "$(STAIR_DATABASE_URL)"

## seed demo data
seed:
	$(GO) run ./cmd/migrate -dir migrations/seeds -database "$(STAIR_DATABASE_URL)"

## lint
lint:
	$(GOLANGCI) run ./...

## static analysis + vet
vet:
	$(GO) vet ./...

## format
fmt:
	$(GO) fmt ./...

## build
build:
	$(GO) build ./...

## frontend: install dependencies
frontend-install:
	$(NPM) --prefix frontend install

## frontend: tests
frontend-test:
	$(NPM) --prefix frontend run test

## frontend: production build
frontend-build:
	$(NPM) --prefix frontend run build

## build & start the Docker stack (PostgreSQL + Redis + API)
env-up:
	$(COMPOSE) -f deployments/docker-compose.yml up -d --build

env-down:
	$(COMPOSE) -f deployments/docker-compose.yml down

clean:
	rm -f coverage.out coverage.html
	rm -rf frontend/dist
