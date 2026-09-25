# STAIR PLATFORM

Document: 04_AUTHENTICATION.md

ID: API-0005

Status: APPROVED (переписано 2026-09-24 по фактическому коду, forensic-аудит DOC-001)

> **Что изменилось.** Прежняя редакция описывала OAuth 2.1, JWT access/refresh,
> service token, mTLS и «Token Structure» (Token ID / Subject / Issuer / Audience /
> Scopes / Issued At / Expires At / Revision). **Ничего из этого в коде нет**:
> аутентификация — opaque-сессии в БД + вход через OIDC (SSO) + административные
> API-ключи. Пакет `internal/infrastructure/jwt` существует, но не подключён
> нигде (мёртвый код). Документ ниже описывает фактическую реализацию;
> несуществующие механики отмечены явно, чтобы их не искали в коде.

---

# Purpose

Подсистема аутентификации отвечает за проверку подлинности вызывающего и за
выдачу/проверку учётных данных. Права (роль, permission) определяет
Authorization (см. 05_AUTHORIZATION), а не Authentication.

---

# Implemented Methods (факт)

| Метод | Где | Как работает |
|---|---|---|
| Email + пароль | `POST /api/v1/auth/register`, `POST /api/v1/auth/login` | bcrypt; при входе выдаётся **opaque-токен сессии** (crypto/rand, 32 байта) в cookie `session` (httpOnly) / `session_admin` |
| OIDC / SSO | `GET /api/v1/auth/sso`, `/auth/sso/callback`, `/auth/sso/config` | Внешний IdP (issuer/client_id/client_secret в `STAIR_SSO_*`), callback валидирует state/nonce, создаёт локальную сессию |
| API-ключ | `POST /api/v1/admin/api-keys` (право `api_keys.manage`) | Ключ хранится хешем, передаётся как `Authorization: Bearer`; имеет tenant и scopes |

**Не реализовано (осознанно, не «в процессе»):** OAuth 2.1 flows, JWT access/refresh
токены, service-to-service токены, mTLS, device-flow. Если они понадобятся —
это отдельная задача с миграцией схемы сессий.

---

# Session Model (факт)

- Сессия — строка в БД (`sessions`): `token_hash`, `user_id`, `expires_at`,
  `created_at`, `rotated_at`. На сервере хранится **хеш** токена, не сам токен.
- Время жизни: `STAIR_SESSION_TTL` (по умолчанию — см. `cmd/api`).
- Ротация: при успешной аутентификации выдаётся новый токен (прежний аннулируется),
  это защита от replay украденного токена.
- Выход (`POST /api/v1/auth/logout`) удаляет сессию на сервере и чистит cookie.
- Токен из query-параметра **не принимается** (S-110): только Bearer-заголовок или cookie.

---

# CSRF и браузерный контекст

- Сессионные cookie: `session` (store) и `session_admin` (админка), httpOnly.
- CSRF: double-submit — cookie `csrf`/`csrf_admin` + заголовок `X-CSRF-Token`;
  проверяется на всех мутирующих маршрутах (`requireCSRF`), плюс проверка Origin/Referer.
- `Secure` флаг cookie — `STAIR_COOKIE_SECURE` (в Helm-чарте secure по умолчанию).

---

# Throttling и защита от перебора

- `/auth/login` — 10 req/min на IP, `/auth/register` — 5 req/min на IP
  (`STAIR_LOGIN_RATE_LIMIT`, `STAIR_REGISTER_RATE_LIMIT`).
- Per-user лимит для аутентифицированных запросов: 200 req/min.
- Сравнение пароля — bcrypt; для несуществующего email выполняется сравнение с
  dummy-хешем (защита от timing-энумерации, S-113/S-139).

---

# Password Policy (факт)

- Минимальная длина/сложность — `auth.ErrWeakPassword` (см. `internal/application/auth`).
- Хранение: bcrypt-хеш, пароль в логи не попадает (S-111/S-143 redaction).

---

# Что нужно знать интегратору

1. **Refresh-токенов нет.** Сессия истекает — нужен повторный логин (или SSO).
2. **JWT нет.** Клиент не может проверить токен локально; он opaque и проверяется
   сервером по каждому запросу.
3. **Сервисные интеграции** используют admin API-ключи со scopes, а не OAuth client credentials.
4. Верификация email **не реализована** (BACKLOG B-2): аккаунт создаётся сразу после
   регистрации; для чувствительных установок ограничивайте регистрацию на периметре.

---

# Acceptance Criteria (проверяемо)

- [x] Регистрация/вход выдают httpOnly-сессию, токен не хранится в открытом виде.
- [x] Ротация токена при каждом входе; выход аннулирует сессию.
- [x] CSRF-защита на всех мутирующих маршрутах.
- [x] Rate-limit на login/register + per-user лимит.
- [x] Timing-энумерация закрыта dummy-сравнением.
- [ ] Верификация email (BACKLOG B-2, требует SMTP-ключа — человеку).
- [ ] Password reset / восстановление доступа — не реализовано.
