# S-125: Helm ingress-шаблон

**Вердикт:** DONE — шаблон создан, values оживлены, lint + рендеры (tls/class/disabled) зеленые, запушено в `dev/swarm/ws-s115-s116`.

## Что сделано
- `deploy/helm/stair-platform/templates/ingress.yaml` (новый) — `networking.k8s.io/v1` Ingress за флагом `ingress.enabled`; backend = release-fullname:`.Values.service.port`; className/annotations/hosts/paths/tls из values.
- `values.yaml` — дефолтные пути `/` → `/api/` + `/ws` (Prefix; `/ws` покрывает и `/ws/admin`); закомментированные примеры: cert-manager cluster-issuer, nginx proxy timeouts 86400 для WS, TLS-блок. `/metrics,/swagger,/debug/pprof` остаются ClusterIP-only (S-113).

## Гейты (deploy-зона, Go-код не тронут)
- `helm lint` — 1 chart linted, 0 failed.
- `helm template --show-only templates/ingress.yaml`: enabled → 2 пути на 8080; TLS-вариант (class+annotations+tls) → корректный spec; disabled (дефолт) → пусто (ожидаемо).

## Счётчик CI
Задача 13/20 после PR #51 (следующий CI-прогон после 20-й).
