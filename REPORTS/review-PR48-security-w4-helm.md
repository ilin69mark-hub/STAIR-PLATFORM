# Review PR #48 — security W4: S-105 helm chart secure defaults

- **PR:** https://github.com/ilin69mark-hub/STAIR-PLATFORM/pull/48
- **Branch:** dev/swarm/security-w4-helm → main (commit 045427a, 5 файлов, +14/−12)
- **Reviewer:** @swarm (self-review; Task-dispatch ⚙️ BLOCKED_INFRA в этой сессии — кеш model, фикс вступит со следующей)
- **Дата:** 2026-09-21

## Scope (S-105, fingerprint HELM-DEFAULTS-INSECURE)

| Файл | Изменение |
|---|---|
| `deploy/helm/stair-platform/values.yaml` | `cookieSecure: true`, `hstsEnabled: true` (secure defaults); комментарий secretsKey → ОБЯЗАТЕЛЕН |
| `templates/secret.yaml` | `secrets-key` всегда рендерится через `required` → fail-fast при пустом secretsKey |
| `templates/deployment.yaml` | `STAIR_SECRETS_KEY` env безусловно (убран `{{- if }}`) |
| `templates/worker-deployment.yaml` | то же для worker |
| `.github/workflows/cd.yml` | helm upgrade += `--set secretsKey="${{ secrets.STAIR_SECRETS_KEY }}"` |

## Проверки

1. **Helm-синтаксис**: `required "msg" .Values.secretsKey | b64enc | quote` — корректный приоритет пайплайна (required → b64enc → quote). ✓
2. **Fail-fast**: `helm template` без secretsKey → `Error: execution error at (secret.yaml:11:18): secretsKey is required (S-105)...` ✓
3. **Позитивный рендер**: с secretsKey → 6 документов (PDB, Secret, ConfigMap, Service, Deployment api, Deployment worker), YAML валиден (python yaml.safe_load_all). STAIR_SECRETS_KEY присутствует в api И worker (2 вхождения). ✓
4. **Env-имена совпадают с кодом**: `STAIR_COOKIE_SECURE` (cmd/api/main.go:268), `STAIR_HSTS_ENABLED` (cmd/api/main.go:298), `STAIR_SECRETS_KEY` (cmd/api/main.go:137-151, S-104 fail-fast). ✓
5. **CD не сломан**: cd.yml передаёт ключ; без него CD упал бы на required — это и есть целевой fail-fast. ✓
6. **helm lint**: 1 chart, 0 failed (WARN про required values — ожидаемо). ✓

## Замечания

- **Неблокирующее (деплой-пререквизит)**: для работы CD в GitHub Actions должен быть настроен secret `STAIR_SECRETS_KEY` (64 hex). Пока его нет — CD будет падать на fail-fast. Это by design (лучше упасть, чем задеплоить без ключа), но человеку нужно добавить secret в репозиторий.
- **Неблокирующее**: `cookieSecure: true` требует HTTPS на ingress (иначе cookie не будут слаться). Отражено в комментарии values.yaml; для dev-окружений без TLS — осознанный override на false.

## Вердикт

**APPROVED** — изменения минимальны, соответствуют таргету S-105, валидированы helm lint + template (fail-fast и позитивный рендер). Гейты: Go-код не затронут (helm/cd.yml only).