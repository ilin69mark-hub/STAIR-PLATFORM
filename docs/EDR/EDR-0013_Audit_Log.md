# STAIR PLATFORM

**Document:** EDR-0013_Audit_Log.md

**ID:** EDR-0013

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Enterprise

---

# 1. Purpose

Документ фиксирует систему аудита (Phase G, Audit): персистентный
журнал событий безопасности (SEC-0013) с синхронной записью из
прикладных сервисов, чтением через API и панелью на странице проекта.
Аудит — фундамент Phase G: Security, Permissions, Enterprise Controls
полагаются на единый журнал действий.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase G — Enterprise, Audit)
- SEC-0013 (Audit Security)
- SEC-0003 (Identity and Authentication)
- SEC-0004 (Authorization Model)
- SEC-0005 (Tenant Isolation)
- ADR-0006 (Layered Architecture)
- BC-001 (Projects Bounded Context)

---

# 3. Model

## 3.1 Сущность AuditEvent

| Field | Описание |
|-------|----------|
| ID | UUID |
| ActorID | инициатор (FK users; NULL для системных событий) |
| TenantID | tenant события (FK tenants) |
| ProjectID | проект (FK projects, nullable — глобальные события) |
| Action | вид действия (см. каталог 3.3) |
| ResourceType | тип ресурса (project/configuration/...) |
| ResourceID | id ресурса |
| Result | ok / denied / failed |
| Detail | детализация (роль, ошибка, ...) |
| RequestID | request id запроса (наблюдаемость) |
| IP | IP клиента (SEC-0013, где оправдано) |
| CreatedAt | время события |

Событие скоупится по tenant (SEC-0005): tenant берётся из
аутентифицированного контекста (не из тела запроса).

## 3.2 Права

| Операция | Требование |
|----------|------------|
| Record (внутр.) | только прикладные сервисы (auth, project) |
| ListProjectAudit | член проекта |
| ListAudit (глобальный) | роль admin (SEC-0004) |

## 3.3 Каталог действий (Action)

| Action | Описание |
|--------|----------|
| auth.register | регистрация пользователя |
| auth.login | вход (успешный) |
| auth.logout | выход |
| auth.login_denied | неудачный вход (неверные учётные данные) |
| authz.denied | отказ авторизации (недостаточно прав) |
| project.created | создание проекта |
| project.modified | изменение проекта (конфигурация/расчёт) |
| member.added | добавление участника |
| member.role_changed | смена роли участника |
| member.removed | удаление участника |
| config.approved | утверждение конфигурации |
| config.restored | восстановление ревизии |
| review.requested | запрос ревью |
| review.signed | подпись ревью |
| review.changes | запрос изменений |

---

# 4. Invariants

```
1. Запись события — синхронная, best-effort: сбой записи не ломает
   бизнес-операцию (логируется, операция продолжается).
2. ActorID/TenantID берутся из контекста запроса, никогда из тела.
3. Чтение аудита проекта — только член проекта (иначе 404).
4. Глобальный аудит (все tenant) — только роль admin (иначе 403).
5. Аудит скоуплен по tenant: ListProjectAudit видит события проекта
   внутри tenant вызывающего (SEC-0005).
6. Каждое событие неизменяемо (append-only): нет update/delete API.
7. События хранятся бессрочно; retention — задача отдельного EDR.
```

---

# 5. Schema

```sql
CREATE TABLE audit_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    result        TEXT NOT NULL DEFAULT 'ok',
    detail        TEXT NOT NULL DEFAULT '',
    request_id    TEXT NOT NULL DEFAULT '',
    ip            TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_tenant_idx ON audit_events (tenant_id, created_at);
CREATE INDEX audit_events_project_idx ON audit_events (project_id, created_at);
```

---

# 6. API

| Method | Path | Body | Result |
|--------|------|------|--------|
| GET | `/api/v1/projects/{id}/audit` | - | 200 list, 403, 404 |
| GET | `/api/v1/audit` | - | 200 list (admin), 403 |

`project audit` — события проекта по убыванию времени (новые сверху).
`global audit` — события tenant (все проекты + глобальные).

---

# 7. Tests

- Service unit: Record пишет событие (fake repo); ListProjectAudit — член/не-член.
- Infra repository: вставка + чтение по проекту, скоуп по tenant (чужой
  tenant — пусто/не найдено).
- Transport: DTO и коды (200/403/404); глобальный аудит только admin.
- Integration: после создания проекта в БД есть событие project.created.

---

# 8. Acceptance Criteria

- Модель реализована в `internal/application/audit` (AuditEvent,
  каталог Action, порт Repository, Service.Record).
- Запись подключена в auth (register/login/logout/login_denied) и project
  (create/calculate/member/approve/restore/review) — синхронно, best-effort.
- Миграция 000011 применяется на всех средах.
- API чтения: `GET /projects/{id}/audit` (член) и `GET /audit` (admin).
- Фронтенд: панель «Аудит» на странице проекта (история событий).
- E2E: создание проекта → в аудите появляется запись project.created.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализация end-to-end; status APPROVED |

---

APPROVED
