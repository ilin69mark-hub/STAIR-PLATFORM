# STAIR PLATFORM

Document: 21_ALERTING.md

ID: INFRA-0021

Status: APPROVED

---

# Purpose

Определяет Alerting Architecture STAIR PLATFORM.

---

# Principle

Alert должен сигнализировать о проблеме, требующей действия.

Не каждый telemetry event является alert.

---

# Alert Sources

Metrics

Logs

Traces

Security Events

Infrastructure Events

Queue Events

Database Events

External Provider Events

---

# Severity

Critical

High

Warning

Info

---

# Critical Alerts

Примеры:

Service Unavailable

Database Unavailable

Data Loss Risk

Queue Failure

Security Incident

Certificate Expiration Risk

Storage Failure

---

# Alert Structure

Каждый Alert содержит:

Alert ID

Severity

Service

Environment

Condition

Timestamp

Description

Impact

Runbook Reference

---

# Alert Lifecycle

```text
Detected
   ↓
Alert
   ↓
Acknowledged
   ↓
Investigated
   ↓
Resolved
   ↓
Closed

---

# Alert Rules

Актуальные правила — `deployments/observability/prometheus/alerts.yml`
(EDR-0021): StairApiDown, StairApiHighErrorRate/ElevatedErrorRate,
StairApiHighLatencyP95/P99, StairCalculateSlow/OptimizeSlow,
StairDbPoolSaturated/WaitPressure, StairCircuitBreakerOpen,
StairMemoryHigh, StairGoroutineGrowth, StairRateLimitSpike.

---

# Delivery (S5-1)

Доставка уведомлений — через **Alertmanager webhook** (без blackhole).
Конфиг — `deployments/observability/alertmanager/alertmanager.yml`:
дефолтный роут `notify` → `webhook_configs.url`.

- **Dev/локально**: URL указывает на `alert-sink`
  (`docker-compose.observability.yml`, контейнер stair-platform-alert-sink,
  :8099). Смотреть полученные алерты: `docker logs stair-platform-alert-sink`
  (доп. файл `/var/log/alert-sink.log` внутри контейнера).
- **Прод**: заменить `url` в `receivers.notify.webhook_configs` на реальный
  канал — Slack/Telegram/Grafana contact point или вебхук тикет-системы;
  `send_resolved: true` оставить. Секреты (API-ключи/токены) — через
  webhook URL или basic_auth/headers, НЕ через аннотации алертов.
- **SMTP** (email) можно добавить отдельным receiver`ом `email` и вручную
  включить в route, если прод хочет e-mail как канал.

## Smoke-тест доставки

CI-джоба `test-alerting` (ci.yml) гонит end-to-end:
`alertmanager.test.yml` (короткие таймеры) → POST синтетического алерта в
`/api/v2/alerts` → проверка, что webhook пришёл в `alert-sink`.

Локально:

```bash
docker compose -f deployments/observability/docker-compose.alerting-test.yml up -d
curl -sf -X POST http://localhost:19093/api/v2/alerts -H 'Content-Type: application/json' \
  -d '[{"labels":{"alertname":"StairAlertDeliveryProbe","severity":"critical","service":"stair-api"}}]'
docker exec stair-alert-sink-test sh -c 'grep StairAlertDeliveryProbe /var/log/alert-sink.log'
```