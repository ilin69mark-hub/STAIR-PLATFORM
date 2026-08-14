# STAIR PLATFORM

**Document:** EDR-0012_Configuration_Versioning.md

**ID:** EDR-0012

**Status:** DRAFT

**Author:** Project Team

**Date:** 2026-08-14

**Category:** Collaboration

---

# 1. Purpose

Документ фиксирует модель версионирования конфигурации лестницы
(Phase C, Versioning): каждая посчитанная конфигурация становится
иммутабельной ревизией проекта с монотонным номером; история ревизий
доступна членам проекта; владелец/редактор может восстановить любую
прежнюю ревизию (сделать её текущей). Реализует DB-0006 (Versioning
Model): Revision Immutable, Recovery любой Revision.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase C — Collaboration, C6)
- DB-0006 (Versioning Model)
- SEC-0005 (Tenant Isolation)
- BC-001 (Projects Bounded Context)
- BC-002 (Stair Configuration)
- EDR-0008 (Multi-user Project — членство/роли)
- EDR-0011 (Configuration Approval — фиксация утверждённой ревизии)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 Ревизии конфигурации

Каждый `Calculate` и любое сохранение конфигурации создаёт новую
строку `stair_configurations` — она никогда не изменяется и не
удаляется (DB-0006). Версионирование добавляет:

| Field | Описание |
|-------|----------|
| `revision` | монотонный номер ревизии внутри проекта (1, 2, 3, …) |
| `project.current_configuration_id` | текущая (активная) ревизия проекта |

`UNIQUE (project_id, revision)` — номер ревизии уникален в проекте.

## 3.2 Текущая ревизия

Проект хранит указатель на текущую ревизию. При новом расчёте текущей
становится новая конфигурация. `RestoreRevision` переводит указатель на
выбранную прежнюю ревизию — восстановление состояния (DB-0006 Recovery).

## 3.3 Права

| Операция | Требование |
|----------|------------|
| ListRevisions | член проекта (чтение) |
| GetRevision | член проекта (чтение) |
| RestoreRevision | роль owner или editor (право на изменение) |

---

# 4. Invariants

```
1. Все операции доступны только члену проекта (иначе 404).
2. RestoreRevision: только owner/editor (иначе 403).
3. Восстанавливаемая ревизия принадлежит проекту внутри tenant (иначе 404).
4. Номер ревизии уникален внутри проекта; ревизии иммутабельны.
5. При новом расчёте текущей становится новая ревизия.
6. История ревизий не теряется ни при каком переходе статуса.
```

---

# 5. Schema

```sql
ALTER TABLE stair_configurations
    ADD COLUMN revision INTEGER NOT NULL DEFAULT 1;

CREATE UNIQUE INDEX stair_configurations_project_revision_idx
    ON stair_configurations (project_id, revision);

ALTER TABLE projects
    ADD COLUMN current_configuration_id UUID
        REFERENCES stair_configurations(id) ON DELETE SET NULL;
```

---

# 6. API

| Method | Path | Body | Result |
|--------|------|------|--------|
| GET | `/api/v1/projects/{id}/configurations` | - | 200 revisions, 403, 404 |
| GET | `/api/v1/projects/{id}/configurations/{configID}` | - | 200 revision, 403, 404 |
| POST | `/api/v1/projects/{id}/configurations/{configID}/restore` | - | 200 revision, 403, 404 |

`current` — флаг текущей ревизии в списке и при чтении.

---

# 7. Tests

- Service unit: владелец перечисляет ревизии; editor восстанавливает
  ревизию и она становится текущей; viewer — ErrForbidden; не-член —
  ErrNotFound; чужая ревизия — ErrNotFound.
- Infra repository: номер ревизии монотонен; уникальность; restore.
- Transport: DTO и коды (200/201/403/404).
- Security: не-член не видит историю ревизий.

---

# 8. Acceptance Criteria

- Модель реализована в `internal/application/project` (revision,
  current_configuration_id), репозиторий — `migration 000010` +
  `internal/infrastructure/database`.
- Права фактически проверяются (List/Get — членство; Restore — owner/editor).
- Миграция 000010 применяется на всех средах.
- Фронтенд: панель «Версии» на странице проекта (список ревизий с номером,
  временем и флагом текущей; кнопка «Восстановить» у owner/editor).
- E2E: владелец восстанавливает прежнюю ревизию; viewer — 403.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-14 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализация end-to-end (backend+API+frontend, тесты); status APPROVED |

---

APPROVED