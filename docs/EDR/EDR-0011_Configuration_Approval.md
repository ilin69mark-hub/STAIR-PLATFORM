# STAIR PLATFORM

**Document:** EDR-0011_Configuration_Approval.md

**ID:** EDR-0011

**Status:** DRAFT

**Author:** Project Team

**Date:** 2026-08-14

**Category:** Collaboration

---

# 1. Purpose

Документ фиксирует модель утверждения итоговой конфигурации проекта
(Phase C, Approval): владелец проекта (owner) явно утверждает ревизию
конфигурации лестницы для дальнейшего производства. В отличие от C4
(Review — статус проекта), C5 утверждает конкретную конфигурацию
(configuration_id), включая ссылку на ревизию. Закладывает основу для C6
(Versioning).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase C — Collaboration)
- SEC-0005 (Tenant Isolation)
- BC-001 (Projects Bounded Context)
- BC-002 (Stair Configuration)
- EDR-0008 (Multi-user Project — членство/роли)
- EDR-0010 (Project Review)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 Сущность ConfigurationApproval

| Field | Описание |
|-------|----------|
| ID | UUID |
| ProjectID | проект (FK, ON DELETE CASCADE) |
| ConfigurationID | утверждаемая ревизия (FK, ON DELETE CASCADE) |
| ApprovedByID | владелец, утвердивший (FK users) |
| Comment | комментарий, может быть пустым |
| CreatedAt | время утверждения |

Одна ревизия утверждается не более одного раза (`UNIQUE (configuration_id)`);
повторное утверждение той же ревизии — ErrConflict (422). Утверждения
скоупятся по tenant через конфигурацию проекта (SEC-0005).

## 3.2 Права

| Операция | Требование |
|----------|------------|
| ApproveConfiguration | роль owner |
| GetConfigurationApproval | член проекта |
| ListApprovals | член проекта |

---

# 4. Invariants

```
1. Все операции доступны только члену проекта (иначе 404).
2. ApproveConfiguration: только owner (иначе 403).
3. Утверждаемая конфигурация принадлежит проекту внутри tenant (иначе 404).
4. Повторное утверждение той же ревизии → ErrConflict (422).
5. Утверждения не видны вне tenant (SEC-0005 через проекты).
6. Утверждение не меняет статус проекта (C4 отвечает за переходы);
   C5 только фиксирует финальное решение владельца по ревизии.
```

---

# 5. Schema

```sql
CREATE TABLE configuration_approvals (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    configuration_id UUID NOT NULL REFERENCES stair_configurations(id) ON DELETE CASCADE,
    approved_by      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment          TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (configuration_id)
);

CREATE INDEX configuration_approvals_project_idx ON configuration_approvals (project_id, created_at);
```

---

# 6. API

| Method | Path | Body | Result |
|--------|------|------|--------|
| POST | `/api/v1/projects/{id}/configurations/{configID}/approve` | `{"comment"}` | 201 approval, 403, 404, 422 |
| GET | `/api/v1/projects/{id}/configurations/{configID}/approval` | - | 200 approval, 403, 404 |
| GET | `/api/v1/projects/{id}/approvals` | - | 200 list, 403, 404 |

`approval` GET по конкретной ревизии: 404 — ревизия не существует или не
утверждена (не найдено). Список `approvals` — история утверждений проекта
по возрастанию времени.

---

# 7. Tests

- Service unit: owner утверждает ревизию; editor — ErrForbidden; не-член —
  ErrNotFound; повторное утверждение — ErrConflict; чтение членом — ок.
- Infra repository: вставка + уникальность ревизии, чтение по ревизии,
  список, чужой tenant — ErrNotFound.
- Transport: DTO и коды (201/200/403/404/422).
- Security: не-член не видит утверждения.

---

# 8. Acceptance Criteria

- Модель реализована в `internal/application/project` (ConfigurationApproval),
  репозиторий — `migration 000009` + `internal/infrastructure/database`.
- Права фактически проверяются (Approve — owner; чтение — членство).
- Миграция 000009 применяется на всех средах.
- Фронтенд: панель «Утверждение» на странице проекта (при владельце —
  кнопка «Утвердить итоговую конфигурацию», история утверждений).
- E2E: владелец утверждает итоговую конфигурацию; editor — 403.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-14 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализация end-to-end (backend+API+frontend, тесты); status APPROVED |

---

APPROVED