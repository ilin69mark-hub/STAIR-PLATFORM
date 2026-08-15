# STAIR PLATFORM

**Document:** EDR-0017_SSO.md

**ID:** EDR-0017

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Enterprise

---

# 1. Purpose

Документ фиксирует SSO (Phase G, SSO) — вход через внешний identity
provider (IdP) по протоколу OAuth 2.1 / OpenID Connect Authorization Code
flow c PKCE (API-0005, SEC-0003). Скоуп G5:

1. **OIDC Discovery** — конфигурация провайдера через
   `/.well-known/openid-configuration` (без хардкода endpoints).
2. **Authorization Code flow + PKCE** — начало входа, колбэк, обмен кода
   на токены, верификация id_token (signature, iss, aud, exp, nonce).
3. **Provisioning по email** — первый вход: если пользователь с email из
   claims существует — привязка; иначе — автосоздание (default tenant,
   роль user, активен).
4. **Единый вход** — после успешного SSO выдаётся обычная session-сессия
   (SEC-0003, EDR-0014): SSO-пользователи неотличимы от парольных на
   уровне сессий/полномочий.

Реализация — на стандартной библиотеке Go (net/http, crypto/rsa,
crypto/x509, crypto/rand, crypto/sha256) без новых внешних зависимостей
(офлайн-сборка, DEV-0009).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase G — Enterprise, SSO)
- SEC-0003 (Identity and Authentication)
- SEC-0004 (Authorization Model)
- SEC-0005 (Tenant Isolation)
- SEC-0011 (Token Security)
- SEC-0013 (Audit Security)
- API-0005 (API Architecture: OAuth 2.1, OpenID Connect)
- EDR-0013 (Audit Log)
- EDR-0014 (Advanced Security)
- EDR-0016 (Enterprise Controls)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 OAuthAccount

| Field | Описание |
|-------|----------|
| ID | UUID |
| Provider | имя IdP (конфигурация, например `google`/`generic`) |
| Subject | `sub` claim от IdP (уникальный в рамках провайдера) |
| UserID | привязанная учётная запись (FK users) |
| CreatedAt | дата привязки |

Уникальность — `UNIQUE (provider, subject)`: один внешний identity может
быть привязан только к одной учётной записи.

Пользователь может иметь несколько OAuthAccount (разные провайдеры), но
каждый `(provider, subject)` — один раз.

## 3.2 SsoProvider (конфигурация)

Один IdP на инстанс (env). OIDC Discovery отдаёт authorization/token/keys
endpoints и issuer:

```yaml
sso:
  provider:   "example.com"        # STAIR_SSO_PROVIDER (имя)
  issuer:     "https://issuer"     # STAIR_SSO_ISSUER
  client_id:  "..."                # STAIR_SSO_CLIENT_ID
  client_secret: "..."             # STAIR_SSO_CLIENT_SECRET
  redirect_url: "https://app/callback" # STAIR_SSO_REDIRECT_URL
```

Disabled, если `STAIR_SSO_ISSUER` не задан (публичный ключ). SSO-кнопка на
фронте отображается только когда SSO включён.

## 3.3 Поток (Authorization Code + PKCE)

1. `GET /api/v1/auth/sso` → начало: генерируются `state` (32 байта hex,
   proves Authz Code) и PKCE `verifier`/`challenge` (S256); `state` и
   `verifier` сохраняются в `sso_states` (hash state, TTL 10 мин) и
   выдаётся session-нейтральный cookie с `state`. Роутер делает redirect
   302 на authorization endpoint IdP.
2. IdP аутентифицирует пользователя и возвращает `code` + `state` на
   redirect_url.
3. Колбэк `GET /api/v1/auth/sso/callback`:
   - проверка `state` (сверка с `sso_states`; одноразовое использование —
     строка удаляется);
   - обмен `code` + PKCE `verifier` на токены (token endpoint);
   - верификация `id_token`: signature (JWKS), iss, aud, exp, nonce;
   - подбор пользователя по email из claims (`email` claim обязателен —
     enterprise email domain, SEC-0003):
     - найден → подтверждаем привязку (upserto OAuthAccount);
     - не найден → создаём (default tenant, role user, active);
   - создание session (как Login), выдаётся session+csrf cookie, редирект
     на фронтенд.

`nonce` — случайное значение, генерируется при начале входа и передаётся
в authorization request, проверяется в колбэке (replay-защита id_token).

## 3.4 Безопасность

- PKCE S256 (защита от interception на публичных клиентах).
- id_token проверяется: RS256 подпись по JWKS IdP, `iss` == issuer
  discovery, `aud` == client_id, `exp` в будущем, `nonce` == выданный.
- `state` одноразовый + TTL 10 минут (anti-CSRF логина).
- В `sso_states` хранится SHA-256 от state (как token в sessions).
- SSO-пользователи: пароль не требуется. Для совместимости с текущей
  схемой users.password_hash NOT NULL сущности SSO хранят случайный
  bcrypt-хеш (заглушка) — вход по паролю блокируется на уровне service
  (нет установленного пароля).

---

# 4. Invariants

```
1. Один (provider, subject) — одна учётная запись (UNIQUE).
2. SSO доступен только если STAIR_SSO_ISSUER задан; иначе 404/404 на
   SSO-маршрутах и кнопка не показывается.
3. Привязка по email: создание/поиск происходит в default tenant.
4. state используется один раз и истекает через 10 минут.
5. id_token обязан пройти: signature, iss, aud, exp, nonce.
6. Все SSO-события фиксируются в аудите (EDR-0013): sso.login,
   sso.login_denied, sso.linked; ошибки IdP — sso.login_denied.
7. Вход через SSO создаёт ту же session-сессию, что Login (EDR-0014),
   со всеми применёнными контролями (ротация, TTL, статус active).
```

---

# 5. Schema

## 5.1 Миграция 000013_sso

```sql
CREATE TABLE oauth_accounts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider   TEXT NOT NULL,
    subject    TEXT NOT NULL,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

CREATE INDEX oauth_accounts_user_idx ON oauth_accounts (user_id);

CREATE TABLE sso_states (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    state_hash TEXT NOT NULL UNIQUE,
    nonce      TEXT NOT NULL,
    pkce_verifier TEXT NOT NULL,
    redirect   TEXT NOT NULL DEFAULT '/',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
```

---

# 6. API

| Method | Path | Auth | Result |
|--------|------|------|--------|
| GET | `/api/v1/auth/sso` | public | 302 → IdP (state+PKCE cookie) |
| GET | `/api/v1/auth/sso/callback?code&state` | public | 302 → frontend (session cookie), 400/422/401 |
| GET | `/api/v1/auth/sso/config` | public | 200 `{"enabled":bool,"provider":str}` |

Ошибки колбэка: обработка IdP-отказа → `?error=access_denied`, возврат на
фронтенд с параметром ошибки (SSO-страница выводит сообщение). Никогда не
раскрываются raw-детали токенов в URL.

---

# 7. Tests

- Service (mock IdP httptest): begin (state/PKCE созданы, redirect),
  callback (валидация state, обмен, verify id_token, provisioning по email
  новый/существующий, nonce mismatch, iss/aud/exp неправильные — denied),
  аудит sso.login/sso.login_denied.
- Repo DB: oauth_accounts (create/get-by-provider-subject/upsert), sso_states
  (create/consume/expired).
- Transport: GET /sso 302 + cookies set; /callback 302 → / , 400 bad state,
  denied; /config 200 enabled/disabled.
- Frontend: AuthPage (кнопка SSO показывается при enabled, редирект на
  /api/v1/auth/sso, обработка error в URL) — vitest.

---

# 8. Acceptance Criteria

- Кнопка «Войти через SSO» на AuthPage; клик → редирект на IdP.
- Колбэк создаёт session гармонично с парольным входом (профиль/полномочия).
- Первый вход SSO-пользователя с новым email создаёт пользователя (default
  tenant, role user); с существующим — привязывает OAuthAccount.
- Валидация id_token отклоняет подменённые/истёкшие токены.
- Аудит фиксирует SSO-входы и отказы.
- EDR-0017 помечен APPROVED; ROADMAP Phase G SSO CLOSED (Phase G — целиком
  CLOSED).

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализовано end-to-end (G5); APPROVED |

---

APPROVED