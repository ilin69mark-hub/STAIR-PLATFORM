# STAIR PLATFORM — Makefile
# DEV-0009: единый набор команд: setup, run, test, migrate, seed, lint, build
# DEV-0010: docker-based dev stack: make up / make stop / make fe

GO        ?= go
GOLANGCI  ?= golangci-lint
COMPOSE   ?= docker compose
NPM       ?= npm

# Host-side DB URL (used by migrate/seed/run). Default matches docker-compose.
STAIR_DATABASE_URL ?= postgres://stair:stair@127.0.0.1:5432/stair_platform?sslmode=disable

.PHONY: setup up stop run test coverage coverage-check migrate seed lint build fmt vet env-up env-down clean frontend-install frontend-test frontend-build fe store admin frontends store-logs admin-logs

## start the whole stack (PostgreSQL + Redis + API + store + admin frontends),
## rebuild images, apply migrations. Frontends: store :3000, admin :5174.
up: env-up
	$(GO) run ./cmd/migrate -dir migrations -database "$(STAIR_DATABASE_URL)"
	@echo "Stack up: API :8080, store :3000, admin :5174. Frontend dev server: make fe"

## rebuild & restart the store frontend container (part of the stack)
store:
	@docker build --pull=false --network=host -f deployments/store.Dockerfile -t stair-platform-store . 2>&1 | tail -3 || true
	$(COMPOSE) -f deployments/docker-compose.yml up -d store

## rebuild & restart the admin frontend container (part of the stack)
admin:
	@docker build --pull=false --network=host -f deployments/admin.Dockerfile -t stair-platform-admin . 2>&1 | tail -3 || true
	$(COMPOSE) -f deployments/docker-compose.yml up -d admin

## rebuild & restart both frontend containers
frontends: store admin

## tail logs of the store frontend
store-logs:
	$(COMPOSE) -f deployments/docker-compose.yml logs -f store

## tail logs of the admin frontend
admin-logs:
	$(COMPOSE) -f deployments/docker-compose.yml logs -f admin

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

## seed demo data (own version table: schema_migrations_seeds)
seed:
	$(GO) run ./cmd/migrate -dir migrations/seeds -table schema_migrations_seeds -database "$(STAIR_DATABASE_URL)"

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
## Always rebuilds all images (offline-safe, --pull=false, no registry fetch).
env-up:
	@echo "Rebuilding all images (offline-safe, --pull=false)..."
	@docker build --pull=false --network=host -f deployments/Dockerfile -t stair-platform-api . 2>&1 | tail -5 || true
	@docker build --pull=false --network=host -f deployments/admin.Dockerfile -t stair-platform-admin . 2>&1 | tail -5 || true
	@docker build --pull=false --network=host -f deployments/store.Dockerfile -t stair-platform-store . 2>&1 | tail -5 || true
	$(COMPOSE) -f deployments/docker-compose.yml up -d

## alias for env-up (always rebuild)
rebuild: env-up

## force rebuild with pull (requires registry access)
rebuild-pull:
	$(COMPOSE) -f deployments/docker-compose.yml up -d --build

## alias for env-up
up-fast: env-up

env-down:
	$(COMPOSE) -f deployments/docker-compose.yml down

clean:
	rm -f coverage.out coverage.html
	rm -rf frontend/dist
