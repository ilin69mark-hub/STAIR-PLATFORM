# STAIR PLATFORM

**Document:** EDR-0016_Enterprise_Controls.md

**ID:** EDR-0016

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Enterprise

---

# 1. Purpose

Документ фиксирует Enterprise Controls (Phase G, Enterprise Controls):
административные контроли корпоративного уровня поверх SEC-0004/0005 и
EDR-0013/0014/0015. Скоуп G4:

1. **Tenant-администрирование** — блокировка/разблокировка пользователей,
   смена роли, обзорная статистика tenant.
2. **Политики безопасности** — настраиваемые per-tenant требования к
   паролю, TTL сессий и лимиты входа (JSONB в tenant_settings).
3. **Экспорт данных** — выгрузка данных tenant (пользователи, проекты,
   аудит) в JSON/CSV.
4. **API-ключи (service tokens)** — долгоживущие токены для интеграций
   (Partner API, API-0012) со scope-правами.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase G — Enterprise, Enterprise Controls)
- SEC-0003 (Identity and Authentication)
- SEC-0004 (Authorization Model)
- SEC-0005 (Tenant Isolation)
- SEC-0013 (Audit Security)
- EDR-0013 (Audit Log)
- EDR-0014 (Advanced Security)
- EDR-0015 (Advanced Permissions)
- API-0012 (Partner API)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 Права (дополнение матрицы EDR-0015 §3.2)

| Permission | user | admin |
|------------|------|-------|
| users.manage | - | ✓ |
| settings.read | - | ✓ |
| settings.write | - | ✓ |
| data.export | - | ✓ |
| api_keys.manage | - | ✓ |

`users.manage` расширяет `users.update_role`: смена роли **и** статуса
(active/disabled). `users.list` остаётся для чтения списка и overview.

## 3.2 TenantSettings и Policy

`tenant_settings` — одна строка на tenant (JSONB-поле `policy`):

```json
{
  "min_password_length": 8,
  "require_number": false,
  "require_upper": false,
  "session_ttl_seconds": 86400,
  "login_rate_limit_per_min": 10
}
```

Дефолтная политика (при отсутствии строки) совпадает с текущим
поведением: min 8 символов, TTL 24ч, лимит входа 10/мин.

## 3.3 ApiKey

| Field | Описание |
|-------|----------|
| ID | UUID |
| TenantID | tenant ключа (SEC-0005) |
| Name | человекочитаемое имя |
| TokenHash | SHA-256 от opaque-токена (открытый токен отдаётся один раз) |
| Scopes | права, предоставленные ключу (TEXT[]) |
| CreatedBy | кто создал (FK users) |
| CreatedAt | дата создания |
| RevokedAt | NULL — активен; дата — отозван (soft delete) |
| LastUsedAt | последнее использование (аудит активности) |

Токен — 32 случайных байта (hex, 64 символа), как session-токен
(EDR-0014). В БД хранится только SHA-256 хеш.

---

# 4. Invariants

```
1. Все admin-операции — tenant-скоуп (SEC-0005): список, смена роли/статуса,
   политика, экспорт и API-ключи ограничены tenant вызывающего.
2. Проверка прав — только через Role.HasPermission (EDR-0015); нет
   inline-сравнений ролей.
3. Admin не может изменить роль/статус самому себе (инвариант админа).
4. Блокировка пользователя немедленно ревокает его активные сессии.
5. Политика применяется ко всем новым операциям: регистрация (длина и
   сложность пароля), создание сессий (TTL), вход (лимит/мин).
6. Открытый API-токен отдаётся один раз при создании; хранится только
   SHA-256 хеш. Отзыв — мягкий (revoked_at), мгновенно инвалидирует ключ.
7. Все admin-действия фиксируются в аудите (EDR-0013): user.status_changed,
   settings.updated, data.exported, api_key.created/revoked; отказы — denied.
```

---

# 5. Schema

## 5.1 Миграция 000012_enterprise

```sql
CREATE TABLE tenant_settings (
    tenant_id  UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    policy     JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    scopes      TEXT[] NOT NULL DEFAULT '{}',
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ
);

CREATE INDEX api_keys_tenant_idx ON api_keys (tenant_id);
```

---

# 6. API

| Method | Path | Permission | Body | Result |
|--------|------|------------|------|--------|
| GET | `/api/v1/admin/overview` | users.list | - | 200 stats, 403 |
| PATCH | `/api/v1/admin/users/{id}` | users.manage | `{"role"?,"status"?}` | 200, 403, 404, 422 |
| GET | `/api/v1/admin/settings` | settings.read | - | 200 policy |
| PUT | `/api/v1/admin/settings` | settings.write | policy | 200, 422 |
| GET | `/api/v1/admin/export` | data.export | `scope=users\|projects\|audit&format=json\|csv` | 200 file, 403, 422 |
| GET | `/api/v1/admin/api-keys` | api_keys.manage | - | 200 list |
| POST | `/api/v1/admin/api-keys` | api_keys.manage | `{"name","scopes"}` | 201 (token once), 422 |
| DELETE | `/api/v1/admin/api-keys/{id}` | api_keys.manage | - | 200, 404 |

`PATCH users/{id}`: `role` и `status` — опциональны (хотя бы один); роль
только user|admin; статус только active|disabled. Изменение собственной
роли/статуса — 403. `update role` в одиночку совместим с EDR-0015.

`GET export`: `format` по умолчанию `json`; `scope` обязателен. CSV —
заголовки + строки; JSON — массив объектов.

---

# 7. Security

- API-ключи принимаются в `Authorization: Bearer <token>`, хеш сверяется
  с `api_keys.token_hash`; ключ с `revoked_at` невалиден. Scopes ключа
  действуют как права служебного субъекта (EDR-0015).
- `requireAuth` (middleware_auth.go) принимает либо session-cookie, либо
  Bearer-ключ; для key-запросов CSRF не требуется (не-браузерный клиент,
  EDR-0014 §3.3).
- Политика пароля применяется в `Register`; нарушение — `ErrWeakPassword`
  (422 при регистрации). TTL сессий — из политики tenant (дефолт 24ч).

---

# 8. Tests

- Permission: расширение матрицы admin (users.manage, settings.*, data.export,
  api_keys.manage).
- Auth service: UpdateUser (role|status, self запрещён, 422), блокировка
  ревокает сессии, политика (min length/сложность/TTL/лимит), API-key CRUD
  (токен один раз, отзыв, хеш).
- Repo DB: tenant_settings (upsert), api_keys (create/list/revoke/touch).
- Transport: 403 не-admin; PATCH users 200/403/404/422; settings 200/422;
  export 200/422 (json+csv); api-keys 201/200/404/422; Bearer auth.
- Frontend: AdminPanel (пользователи, настройки, экспорт, API-ключи) —
  vitest.

---

# 9. Acceptance Criteria

- Admin-панель: пользователи (роль/статус), настройки политики, экспорт,
  API-ключи — end-to-end (Go + React).
- PATCH users/{id} поддерживает role и status; self-запрет соблюдается.
- Политика применяется к регистрации и сессиям; дефолты не ломают MVP.
- Экспорт отдаёт данные tenant в JSON/CSV.
- API-ключи: создание (токен один раз), Bearer-доступ по scopes, отзыв.
- EDR-0016 помечен APPROVED; ROADMAP Phase G Enterprise Controls CLOSED.

---

# 10. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализовано end-to-end (G4); APPROVED |

---

APPROVED
