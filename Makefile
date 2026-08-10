# STAIR PLATFORM — Makefile
# DEV-0009: единый набор команд: setup, run, test, migrate, seed, lint, build

GO        ?= go
GOLANGCI  ?= golangci-lint
COMPOSE   ?= docker compose

.PHONY: setup run test migrate seed lint build fmt vet env-up env-down clean

## env-bootstrap
setup: env-up
	@echo "Environment up. Create configs/local.yaml from configs/local.yaml.example if needed."

## run all dev components
run: env-up
	$(GO) run ./cmd/api

## tests
test: 
	$(GO) test ./...

## database migrations
migrate:
	$(GO) run ./cmd/migrate -dir migrations -database "$${STAIR_DATABASE_URL:?set STAIR_DATABASE_URL}"

## seed demo data
seed:
	$(GO) run ./cmd/migrate -dir migrations/seeds -database "$${STAIR_DATABASE_URL:?set STAIR_DATABASE_URL}"

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

## local infra (PostgreSQL + Redis)
env-up:
	$(COMPOSE) -f deployments/docker-compose.yml up -d

env-down:
	$(COMPOSE) -f deployments/docker-compose.yml down

clean:
	rm -f coverage.out