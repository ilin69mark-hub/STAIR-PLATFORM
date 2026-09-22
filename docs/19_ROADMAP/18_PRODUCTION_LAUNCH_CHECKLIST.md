# STAIR PLATFORM

Document: 18_PRODUCTION_LAUNCH_CHECKLIST.md

ID: ROADMAP-0018

Status: APPROVED

---

# Purpose

Определяет checklist перед Production Launch.

---

# Architecture

* [x] Architecture reviewed (S-130: security-аудиты S-103 + full audit 8.5/10, prod-readiness ~75%)
* [x] Critical ADRs approved (EDR-0014/0017/0018/0021, ADR-папка)
* [x] Dependencies validated (S-130: Dependabot-бэклог вычищен, go 1.26/npm workspaces актуальны)

---

# Application

* [x] Critical workflows completed (S-130: e2e admin/store 8/8 + unit 404/2100 зеленые)
* [x] API contracts stable (S-130: swagger_sync_test + docs/openapi)
* [x] Error handling verified (S-130: 14_ERROR_HANDLING + recovery/errorlog тесты)

---

# Database

* [x] Production schema validated (S-130: миграции + DB-тесты в 58/58)
* [x] Migrations tested (S-130: up/down-пары, 49 файлов, migrate в CI)
* [ ] Indexes verified (нет прямых доказательств — кандидат на проверку EXPLAIN)
* [x] Backup verified (S-128: roundtrip check OK, 23 таблицы)
* [x] Recovery tested (S-128: restore в отдельную БД + сверка строк)

---

# Security

* [x] Authentication verified (S-104…S-112, S-115/116/119; тесты auth/WS)
* [x] Authorization verified (S-108 CSRF, S-119 role-гейт /ws/admin)
* [x] Tenant isolation verified (S-130: *Isolation-тесты репозиториев)
* [x] Secrets protected (S-104/S-105: fail-fast + required, без дефолтов)
* [x] Security tests passed (S-130: CI Security Scan/gosec зеленые)

---

# Testing

* [x] Unit tests passed (S-130: Go 58/58 race+БД, покрытие 87.1% ≥ 85%)
* [x] Integration tests passed (S-130: DB/Redis-интеграционные в сьюте)
* [x] API tests passed (S-130: http-пакеты + e2e API)
* [x] Regression tests passed (S-130: S-120 доверенные прокси, S-112 и др.)
* [x] E2E tests passed (S-130: Playwright admin/store, k6 load-smoke в CI)
* [x] Critical performance tests passed (S-130: S5-2 k6 smoke; тяжелое S-134 — отдельно, не блокер)

---

# Infrastructure

* [ ] Production environment configured (нужен деплой — человек; пререквизиты: S-118 секреты, S-124 EKS)
* [ ] Deployment pipeline verified (CD ждет секретов S-118; cd.yml починен S-114)
* [x] Health checks active (S-130: /health//ready + k8s probes + compose healthchecks)
* [x] Monitoring active (S-126: compose observability + ServiceMonitor для Helm)
* [x] Alerts active (S-126: 14 правил, promtool valid, CI alerting-test)

---

# Observability

* [x] Logs available (S-130: structured logging + debug-guard S-111)
* [x] Metrics available (S-130: /metrics, canonical path-лейблы)
* [ ] Tracing available (код есть, по умолчанию выключено — включить при деплое)
* [ ] Error tracking available (S-127: выбор вендора за человеком)

---

# Operations

* [x] Incident procedure documented (S-129: 17_INFRASTRUCTURE/26_INCIDENT_RESPONSE.md)
* [x] Rollback/recovery procedure documented (S-129: rollback runbook там же)
* [x] On-call responsibility defined (S-129: роли + handoff; имена/контакты — человек при запуске)
* [ ] Critical contacts available

---

# Release

* [ ] Release version assigned
* [ ] Release artifact created
* [ ] Changelog created
* [ ] Release approved

---

# Launch

* [ ] Deployment completed
* [ ] Smoke tests passed
* [ ] Health checks passed
* [ ] Production monitoring verified

---

# Acceptance Criteria

Production Launch не выполняется при наличии незакрытого Critical Blocker.

---

APPROVED
