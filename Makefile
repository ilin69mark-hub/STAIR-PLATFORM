# STAIR PLATFORM — Makefile
# DEV-0009: единый набор команд: setup, run, test, migrate, seed, lint, build
# DEV-0010: docker-based dev stack: make up / make stop / make fe

GO        ?= go
GOLANGCI  ?= golangci-lint
COMPOSE   ?= docker compose
NPM       ?= npm

# Host-side DB URL (used by migrate/seed/run). Default matches docker-compose.
# Для локальной разработки: POSTGRES_PASSWORD задаётся в deployments/.env.
STAIR_DATABASE_URL ?= postgres://stair:changeme@127.0.0.1:5432/stair_platform?sslmode=disable

# Release versioning (P3): подставляется в бинарь через ldflags →
# internal/version. VERSION берётся из git tag/describe.
GIT_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X stairplatform/internal/version.Version=$(GIT_VERSION) \
	-X stairplatform/internal/version.Commit=$(GIT_COMMIT) \
	-X stairplatform/internal/version.BuildTime=$(BUILD_TIME)

.PHONY: setup up stop run test coverage coverage-check migrate seed lint build build-api fmt vet env-up env-down clean frontend-install frontend-test frontend-build fe store admin frontends store-logs admin-logs bench backup restore backup-check obs-up obs-down obs-config obs-tracing-up version

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

## observability stack (Prometheus :9090, Alertmanager :9093, Grafana :3030)
## поверх dev-стека; трейсинг Jaeger — make obs-tracing-up
OBS_COMPOSE = -f deployments/docker-compose.yml -f deployments/observability/docker-compose.observability.yml
obs-up:
	$(COMPOSE) $(OBS_COMPOSE) up -d

## stop the observability stack (dev-стек остаётся)
obs-down:
	$(COMPOSE) $(OBS_COMPOSE) down

## validate merged compose + observability config
obs-config:
	$(COMPOSE) $(OBS_COMPOSE) config

## observability stack + Jaeger (OTLP :4318, UI :16686); включает трейсинг API
obs-tracing-up:
	STAIR_TRACING_ENABLED=true $(COMPOSE) $(OBS_COMPOSE) --profile tracing up -d

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

## performance baseline: Go benchmarks (calc/optimize/generate, memory stats)
## save to file: make bench BENCH_OUT=benchmarks/baseline.txt
BENCH_OUT ?=
bench:
	$(GO) test -run '^$$' -bench . -benchmem -count=1 ./internal/... $(if $(BENCH_OUT),| tee $(BENCH_OUT),)

## database backup (custom format + checksum + metadata) -> BACKUP_DIR (default ./backups)
backup:
	./scripts/db-backup.sh

## database restore: make restore FILE=backups/<dump> [TARGET_DB=...] [FORCE=1]
restore:
	@test -n "$(FILE)" || (echo "usage: make restore FILE=backups/<dump> [TARGET_DB=name] [FORCE=1]" && exit 2)
	./scripts/db-restore.sh "$(FILE)"

## verify backup/restore reversibility (backup -> restore into temp DB -> compare)
backup-check:
	./scripts/db-backup-restore-check.sh

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

## build API binary with release ldflags -> bin/stair-api
build-api:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/stair-api ./cmd/api

## print current release version info
version:
	@echo "VERSION=$(GIT_VERSION) COMMIT=$(GIT_COMMIT) BUILD_TIME=$(BUILD_TIME)"

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
	@docker build --pull=false --network=host --build-arg VERSION="$(GIT_VERSION)" --build-arg COMMIT="$(GIT_COMMIT)" --build-arg BUILD_TIME="$(BUILD_TIME)" -f deployments/Dockerfile -t stair-platform-api . 2>&1 | tail -5 || true
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
