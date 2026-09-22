# S-126: Мониторинг — login-SLO алерт + ServiceMonitor для Helm

**Вердикт:** DONE — дельта к готовому заделу (S5-1 + observability compose) закрыта, promtool + helm зеленые, запушено в `dev/swarm/ws-s115-s116`.

## Что было уже (задел, не трогал)
- Prometheus scrape `api:8080/metrics` + `alertmanager:9093`, 13 алертов (ApiDown, 5xx, p95/p99, calculate/optimize SLO, пул БД, CB, память, горутины, rate-limit spike), Grafana golden-signals дашборд + provisioning, alerting-test в CI.

## Что сделано (дельта S-126)
- `deployments/observability/prometheus/alerts.yml` — новый `StairLoginSlow`: p95 `http_request_duration_seconds_bucket{path="/api/v1/auth/login"}` > 0.3 (10м, warning, runbook 22_SLO_SLA). Путь-лейбл безопасен: `canonicalPath` режет UUID в `{id}`, login — фиксированный путь.
- `deploy/helm/stair-platform/templates/servicemonitor.yaml` (новый) + `values.monitoring.serviceMonitor` (enabled:false, interval 30s, scrapeTimeout 10s, labels-коммент) — скрейп для kube-prometheus-stack; рендерится только при включении.

## Гейты (observability/deploy-зона, Go-код не тронут)
- `promtool check rules` — SUCCESS, 14 rules; `promtool check config` — prometheus.yml valid (образ prom/prometheus:v2.55.0 — тот же, что в compose).
- `helm lint` — 0 failed; ServiceMonitor: enabled → корректный CRD-манифест, disabled → пусто.

## Счётчик CI
Задача 14/20 после PR #51 (следующий CI-прогон после 20-й).
