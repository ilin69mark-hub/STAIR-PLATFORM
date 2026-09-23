
---

# `19_INFRASTRUCTURE/17_OBSERVABILITY_INFRASTRUCTURE.md`

```markdown
# STAIR PLATFORM

Document: 17_OBSERVABILITY_INFRASTRUCTURE.md

ID: INFRA-0017

Status: APPROVED

---

# Purpose

Определяет Infrastructure для Observability.

---

# Sentry (Error Tracking)

S-127 (решение человека: **Sentry cloud**) → S-140. Go-бэкенд (api + worker)
шлёт ошибки и паники в Sentry; фронтенды — отдельно (React ErrorBoundary).

## Включение (выключено по умолчанию)

Интеграция **выключена по умолчанию**: без `STAIR_SENTRY_DSN` SDK не
инициализируется — ноль поведения, ноль запросов наружу. Код корректно
работает с пустым DSN (реальный DSN вносится позже, S-118 отложен).

```text
GitHub Actions Secrets: STAIR_SENTRY_DSN
        │
        ▼  cd.yml: helm upgrade --set sentry.dsn="${{ secrets.STAIR_SENTRY_DSN }}"
deploy/helm/stair-platform: Secret → env STAIR_SENTRY_DSN (api + worker)
        │
        ▼
sentry.Init (cmd/api/main.go, cmd/worker/main.go) → BeforeSend-скраб → Sentry
```

План чарта (deploy-зона, провести вместе с внесением DSN): в
`templates/secret.yaml` — ключ `sentry-dsn` (b64) по аналогии с
`stripe-secret-key`; в `deployment.yaml`/`worker-deployment.yaml` —
`envFrom`/`secretKeyRef` → `STAIR_SENTRY_DSN`.

## Env-переменные

| Переменная | Дефолт | Назначение |
|---|---|---|
| `STAIR_SENTRY_DSN` | пусто = выключено | DSN Sentry-проекта (Secret, не ConfigMap) |
| `STAIR_SENTRY_TRACES_SAMPLE_RATE` | `0` | доля транзакций; `0` = только ошибки (tracing — OpenTelemetry, см. 20_DISTRIBUTED_TRACING.md) |
| `STAIR_ENVIRONMENT` | `development` (api) | → sentry environment (development/staging/production) |
| ldflags `version.Version` | `dev` | → sentry release (см. Makefile) |

## PII-скраб (BeforeSend, `internal/infrastructure/sentry/scrub.go`)

- **email-адреса** (user, extra, contexts, tags — где бы ни встретились) —
  SHA-256-хеш, первые 12 hex (группировка по пользователю без раскрытия);
- **IP** (user.ip_address, REMOTE_ADDR/REMOTE_PORT, X-Forwarded-For и proxy-заголовки) — дроп;
- **username/name** — дроп (остаётся только user.id — UUID, не PII);
- **Authorization/Cookie/X-Api-Key/proxy-заголовки** — дроп целиком;
- **password/passwd/secret/token/cookie/api_key/client_secret** — маска `***`
  (переиспользуется список `redactSensitive`, S-111 → `internal/infrastructure/redaction`).

Паники (recovery.go) идут в Sentry с контекстом запроса (method/path) и стеком;
в лог — как и раньше.

---

# Observability Signals

Logs

Metrics

Traces

Events

Profiles where required

---

# Architecture

```text
Application
   │
   ├── Logs
   ├── Metrics
   ├── Traces
   └── Events
        │
        ▼
Telemetry Pipeline
        │
        ▼
Observability Backend
        │
        ├── Dashboards
        ├── Alerts
        └── Investigation