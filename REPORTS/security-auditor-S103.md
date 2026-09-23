# S-103 — Полный security-аудит stair-platform (CF security-audit skill)

- **Дата**: 2026-09-21 · **Source ref**: `edab11a` (main) · **Профиль**: deep, бюджет безлимит
- **Метод**: скилл `security-audit` (Cloudflare), полный режим, `sandboxed-source-and-local-only` (без запуска таргета)
- **Артефакты**: `~/security-audit-skill/stair-platform/run-1/` — `architecture.md`, `coverage-ledger.json` (27 units, **PASS**), `findings.json` (14 записей, **PASS**), `REPORT.md`, `FINDINGS-DETAIL.md`, `NEEDS-VALIDATION.md`

**Вердикт:** аудит завершён — **11 confirmed** (3 medium, 6 low, 2 informational), **3 needs_validation**, **0 rejected**. Оба валидатора PASS. Критических уязвимостей не найдено; главная системная проблема — **шифрование webhook-секретов at rest не enforced в проде** (3 medium-находки: ключ опционален в api и worker, helm-дефолты небезопасны). Регрессий по `06_AUDIT_FIX_PLAN_S1_S5` нет по S2–S5; по S1-1/S1-2 — частичные (см. ниже). Покрытие честно частичное: application-сервисы {assistant,audit,order,storage,jobs} и CLIENT-SIDE-класс вне scope; supply-chain проверен локально (CI billing ⚙️ BLOCKED_INFRA).

## Confirmed findings

| Sev | Fingerprint | Суть | Ключевые строки |
|---|---|---|---|
| medium | PROD-SECRETS-KEY-NOT-ENFORCED | `STAIR_SECRETS_KEY` опционален; `.env.production`/compose его не задают → webhook-секреты plaintext at rest в stock-проде | cmd/api/main.go:132-145, 211-213 (контраст Stripe); .env.production |
| medium | WORKER-SECRETS-KEY-NOT-ENFORCED | Тот же паттерн в worker; env-детект exact `"production"` (API — EqualFold) | cmd/worker/main.go:89, 101-114 |
| medium | HELM-DEFAULTS-INSECURE | chart-дефолты: cookieSecure=false, hstsEnabled=false, secretsKey="" | deploy/helm/stair-platform/values.yaml:115,128-129 |
| low | SSO-OPEN-REDIRECT-BACKSLASH | `isSafeRedirect("/\\evil.com")`=true → open redirect (браузер нормализует `\`→`/`) | internal/transport/http/sso.go:70-73,87,91 |
| low | PROJECTS-BROWSER-CACHE-LEAK | `/api/v1/projects/` → `max-age=300, private` на auth-ответах; утечка между пользователями на общем браузере | internal/transport/http/router.go:236; cache.go:25,72 |
| low | MUTATING-ROUTES-NO-CSRF | DELETE endpoints/{id}, POST quote-send/crm-sync/order-send без requireCSRF (митигировано JSON-only + CORS) | internal/transport/http/router.go:193-196,100-103 |
| low | SSO-ROUTES-NO-RATE-LIMIT | SSO-роуты без лимитера; рост sso_states, OIDC-амплификация | internal/transport/http/router.go:110-112 |
| low | WS-TOKEN-IN-QUERY | session-токен принимается через `?token=` → утечка в логи/history/referrer | internal/transport/http/websocket_handler.go:56-66 |
| low | DEBUG-LOGGING-PROD-BODIES | `STAIR_DEBUG_LOGGING=true` в проде логирует тела запросов (пароли) | internal/transport/http/debug_log.go:26,64 |
| informational | RATELIMIT-DEAD-XFF-CODE | Мёртвый `ByIP` доверяет X-Forwarded-For (латентный байпас; live-путь RemoteAddr-only) | internal/infrastructure/security/ratelimit.go:131-158 |
| informational | WS-ORIGIN-HARDCODED-LOCALHOST | CheckOrigin только localhost → WS сломан в проде (не CSWSH) | internal/transport/websocket/websocket.go:20-38 |

## Needs validation (3) — требуют runtime/деплой-проверки

| Fingerprint | Claimed root cause | План проверки |
|---|---|---|
| LOGIN-TIMING-ENUMERATION | unknown email → мгновенный ответ; known → bcrypt (~50-100ms) | замер таймингов (bench/staging) |
| WEBHOOK-MAPPED-LINKLOCAL-BYPASS | worker `env != "production"` exact; `env=prod` отключает SSRF-политику loopback | worker с env=prod + link-local цель |
| INTERNAL-ONLY-PROXY-BYPASS | InternalOnlyMiddleware только RemoteAddr; экспозиция зависит от топологии | проверка портов/nginx на проде |

## Регрессии vs 06_AUDIT_FIX_PLAN (S1–S5)

- **S1-1 (SSRF webhook)**: реализован (classifyIP + allow-hosts). ⚠️ Регрессия-риск: env-детект worker exact-match → `STAIR_ENVIRONMENT=prod` отключает политику (WEBHOOK-MAPPED-LINKLOCAL-BYPASS).
- **S1-2 (шифрование секретов)**: реализован, но **не enforced** — 3 medium-находки (PROD/WORKER-SECRETS-KEY-NOT-ENFORCED, HELM-DEFAULTS-INSECURE).
- **S2 (сессии)**: ротация на half-TTL ок; нота — используется конструкторский TTL, не tenant-политика (service.go:248).
- **S3 (rate-limit)**: live-путь RemoteAddr-only ок; мёртвый XFF-код (informational); SSO без лимита (low).
- **S4 (input)**: body 1MiB, pagination 100, storage-валидация — ок.
- **S5 (headers/CORS)**: реализованы; helm-дефолты выключены (medium); `.env.production` корректен (COOKIE_SECURE=1, HSTS=1).

## Подтверждённый hardening (не находки)

crypto/rand токены; tenant-scoped project repo; storage без traversal; Redis TLS 1.2+; Stripe HMAC constant-time + 5min + SETNX dedup; OIDC RS256/iss/aud/exp/nonce + JWKS cache; GraphQL не в роутере; helm pod securityContext (non-root 10001, seccomp, drop ALL); CI gitleaks/gosec/trivy + pinned версии; CD OIDC least-privilege.

## Покрытие и ограничения

- 27 coverage units (PASS); 14 fingerprints с финальными вердиктами.
- **Частичное покрытие**: application/{assistant,audit,order,storage,jobs} internals и CLIENT-SIDE — out_of_scope; supply-chain — локально (Actions billing мёртв).
- **Инфраструктура**: Task-dispatch недоступен (provider error) → self-execute (⚙️ BLOCKED_INFRA на борде).
- **Гигиена репо**: в корне лежит untracked-файл `" + (.merged|tostring)"` — вне scope аудита, требует удаления.

## Что дальше (ждёт решения человека)

1. Взять в работу Backlog-карточки **S-104…S-112** (фиксы confirmed) — приоритет: S-104/S-105 (medium, secrets/helm), затем S-106 (open redirect).
2. Прогнать **S-113** (3 needs_validation) на dev/staging-окружении.
3. Починить CI billing (⚙️ BLOCKED_INFRA) для гейтов/мержей.