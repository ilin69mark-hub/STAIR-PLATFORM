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
- **Worker в dev-стеке** (P3, EDR-0020/0035/0018): сервис `worker` в
  `deployments/docker-compose.yml` (тот же образ, `/bin/worker`) потребляет
  общую Redis-очередь — async-расчёты `calc.calculate`, доставка webhook,
  очистка истёкших сессий/sso_states и аудит-ретенция; env
  `STAIR_CLEANUP_INTERVAL`/`STAIR_AUDIT_RETENTION_DAYS`/`STAIR_WORKER_SHUTDOWN_TIMEOUT`.
  API в compose получил `STAIR_REDIS_ADDR=redis:6379` — постановка заданий в
  общую очередь (иначе API enqueue-ил в in-memory и воркер не видел джобы).
- **Трейсинг worker** (P3): `cmd/worker` инициализирует OpenTelemetry (OTLP,
  Jaeger-сервис `stair-platform-worker`); каждый обработанный job — span
  `job.process` с атрибутами job.id/type/attempts и error при провале,
  включается в обсервабилити-оверрайде через STAIR_TRACING_ENABLED.
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
- `report-400.spec.ts`: комментарий о валидном радиусе спирали приведён к
  документированному окну [900,1200] (`boundary.spec.ts`); узкое окно ~960 —
  частный случай (явный шаг 165–172, H 2680–2720).

## [0.1.0] — 2026-09-17

Начальная зафиксированная версия. Функционально закрыты Phases 1-9:
engine (straight/L/U/spiral), geometry, pricing, manufacturing, API
(REST + GraphQL + WebSocket), auth/SSO/audit, admin, AI, payments, storage,
orders/testimonials. Coverage-gates: Go total 87.4%, admin frontend 86.7%.