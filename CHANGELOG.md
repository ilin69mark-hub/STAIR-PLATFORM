# Changelog

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
проект следует [Semantic Versioning](https://semver.org/lang/ru/).

## [Unreleased]

### Added

- **Observability-стек** (P3, EDR-0021): Prometheus + Alertmanager + Grafana +
  Jaeger через оверрайд `deployments/observability/docker-compose.observability.yml`;
  13 alert-правил по реальным метрикам `/metrics` (availability 99.9%,
  latency p95/p99, engine calc/optimize, БД-пул, circuit breaker, memory,
  rate-limit); дашборд Grafana «STAIR — API Golden Signals» с автопровижном
  datasource; `make obs-up / obs-tracing-up / obs-down / obs-config`.
- **Release versioning**: пакет `internal/version` (Version/Commit/BuildTime)
  заполняется через ldflags во всех артефактах (Makefile `build-api`,
  Dockerfile API, CI build-push-action); версия логируется на старте API/worker;
  `make version` печатает текущую версию.
- **Perf baseline** (P2): `make bench` — бенчмарки Calculate/Generate/Optimize,
  базовая линия в `benchmarks/baseline.txt`.
- **Backup/recovery PostgreSQL** (P2): `make backup / restore / backup-check`
  — `pg_dump -Fc` + sha256 + метаданные, восстановление в отдельную БД со
  сверкой checksum, round-trip проверка обратимости.
- **Secret scan** (P3): Gitleaks в CI (`security-scan`, вся история, v8.30.1);
  конфиг `.gitleaks.toml` с allowlist'ом для публичного AWS SigV4 test-vector
  и Stripe test-ключей в unit-тестах. Проверка 117 коммитов — 0 утечек.

### Changed

- CI: сборка API-образа передаёт VERSION/COMMIT/BUILD_TIME в образ.

## [0.1.0] — 2026-09-17

Начальная зафиксированная версия. Функционально закрыты Phases 1-9:
engine (straight/L/U/spiral), geometry, pricing, manufacturing, API
(REST + GraphQL + WebSocket), auth/SSO/audit, admin, AI, payments, storage,
orders/testimonials. Coverage-gates: Go total 87.4%, admin frontend 86.7%.