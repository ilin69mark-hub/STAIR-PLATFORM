# Приоритизация security-бэклога S-104…S-113 (после S-103, 11 confirmed)

> Координатор: @swarm · 2026-09-21 · метод: severity → likelihood → цена/инвазивность → связность файлов.
> CI ⚙️ BLOCKED_INFRA → каждая PR верифицируется локально (go build/vet/test + ради регрессий).

## Кратко

| Prio | Карточки | Зона | Размер | Обоснование |
|---|---|---|---|---|
| **P0** | S-104 (+worker env-нормализация), S-111 | backend | S + S | medium; шриф-дефолты в проде; те же файлы бутстрапа; попутно чинит половину needs_validation (WEBHOOK-MAPPED-LINKLOCAL-BYPASS) |
| **P1** | S-106, S-108, S-109 | backend | S + S + M | low, но дешёвые; S-106 likelihood HIGH (open redirect ломается без каких-либо условий) |
| **P1** | S-107 | backend | S | низкий риск, изолированный случай (cache/no-store) |
| **P2** | S-105 | deploy | M | medium, но неgo-область, статичный helm, нет helm-CI |
| **P2** | S-110 | backend | L | инвазивно: смена auth-flow WS + синк клиента |
| **P3** | S-112 | backend | S–M | informational; чистка мёртвого кода |
| **—** | S-113 (3 needs_validation) | backend,deploy | — | не фикс: runtime-валидация, нужен деплой/человек (план: run-1/NEEDS-VALIDATION.md) |

## P0 — бутстрап-гарды (1 PR: `security/bootstrap-guards`)

### S-104 (medium) — enforce STAIR_SECRETS_KEY в проде (api+worker)
- Факт: парсинг ключа уже есть (cmd/api/main.go:131-135, cmd/worker/main.go:99-104), но фейл-fast в проде отсутствует — ключа нет → legacy-plaintext.
- Фикс: в `isProduction` блоке api — fail-fast как у Stripe (main.go:211-213); worker — тот же guard + **нормализация детекта env** (`env != "production"` exact-match → EqualFold production|prod, main.go:89), это закрывает и «WEBHOOK-MAPPED-LINKLOCAL-BYPASS» (SSRF-политика отключалась неканоничным значением env).
- Плюс: пример ключа в .env.production / deploy-compose (не сам ключ).
- Верификация: go test (новый кейс: prod+без ключа → exit), локальный прогон в dev (не production).

### S-111 (low) — debug-log prod-guard безусловный + redaction
- Факт (internal/transport/http/debug_log.go:22-29): guard пропускает логирование при `STAIR_DEBUG_LOGGING=true` даже в проде → пароли в логах.
- Фикс: в проде — безусловный отказ (независимо от значения); вне прода — redact полей password/secret/token/authorization/cookie в req_body.
- Верификация: unit-тест middleware (prod+true → не логирует тело), go vet.

## P1 — router/auth hardening (1 PR: `security/router-auth`) + P1 (1 PR: `security/cache-no-store`)

### S-106 (low, likelihood HIGH) — SSO open redirect backslash
- internal/transport/http/sso.go `isSafeRedirect`: reject `\` и `%5C` (нормализованный path). Тесты: `/\evil.com`, `/%5Cevil.com` → returnTo отклонён.

### S-108 (low) — CSRF на mutating-роуты
- router: endpoints/quote-send/crm-sync/order-send — `authProtected` → `authMutating`; регресс-тест: «все non-GET роуты под CSRF-мидлварью».

### S-109 (low) — rate-limit SSO-роутов + cap sso_states
- ssoRateLimiter на /auth/sso* (инфра лимитера есть), лимит размера state-карты.

### S-107 (low) — no-store для authenticated cache-префиксов
- router.go:236: убрать /api/v1/projects/ (и auth-префиксы) из browser-cache map ИЛИ Vary: Authorization. Тест: cache-control на проектах без max-age=300/private для авторизованных.

## P2 — по отдельности

### S-105 (medium) — helm secure defaults
- values.yaml: cookieSecure=true, hstsEnabled=true, secretsKey required (fail-fast валидация); templates.
- Отдельный PR; валидация — helm template/unity (нет CI-джобы helm; локально).

### S-110 (low, impact medium) — WS-токен не в query
- Убрать `?token=` fallback (internal/transport/websocket:54-68), ticket/cookie auth. Отдельный PR: меняет клиентский connect — риск регресса WS.

## P3 — чистка
### S-112 (informational) — dead ByIP/XFF-код + WS-ориджины из конфига
- Удалить ByIP/ByEndpoint/ByUser (или оставить RemoteAddr-only) + CheckOrigin брать из STAIR_WS_ORIGINS, не хардкод localhost.

## Отдельный трек — S-113 (валидация, не фикс)
- LOGIN-TIMING-ENUMERATION, WEBHOOK-MAPPED-LINKLOCAL-BYPASS (частично закрыт S-104 env-нормализацией), INTERNAL-ONLY-PROXY-BYPASS.
- Требуют запущенного prod-like окружения / топологии nginx; план проверок — run-1/NEEDS-VALIDATION.md. Нужен деплой или dev-окно (человек).

## Рекомендация
Брать в 5 PR-волн: **W1**: P0 (S-104+S-111), **W2**: S-106+S-108+S-109, **W3**: S-107, **W4**: S-105, **W5**: S-110, **W6**: S-112. Общий объём — ~2-3 часа включая верификацию. S-113 параллельно готовится к запуску (нужен хост).