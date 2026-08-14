# STAIR PLATFORM

**Document:** EDR-0009_Project_Comments.md

**ID:** EDR-0009

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-14

**Category:** Collaboration

---

# 1. Purpose

Документ фиксирует модель комментариев к проекту (Phase C, Comments):
обсуждение в рамках участников проекта. Комментарий — короткий текст,
привязанный к проекту и автору (члену проекта); удаление доступно автору
или владельцу проекта.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase C — Collaboration)
- SEC-0005 (Tenant Isolation)
- BC-001 (Projects Bounded Context)
- EDR-0008 (Multi-user Project — членство/роли)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 Сущность Comment

| Field | Описание |
|-------|----------|
| ID | UUID |
| ProjectID | проект (FK, ON DELETE CASCADE) |
| AuthorID | автор (FK users, ON DELETE CASCADE) |
| Body | текст, непустой |
| CreatedAt | время создания |

Комментарии скоупятся по tenant через проект: доступ проверяется по
членству (EDR-0008).

## 3.2 Права

| Операция | Требование |
|----------|------------|
| AddComment | член проекта (owner/editor/viewer) |
| ListComments | член проекта |
| DeleteComment | автор комментария ИЛИ владелец проекта (owner) |

Комментарий не изменяется после создания (иммутабелен): правка в скоупе
вне C3.

---

# 4. Invariants

```
1. Комментировать может только член проекта (иначе 404/для не-члена).
2. Тело комментария непустое.
3. Удалить может автор или владелец; чужой член — 403 (Forbidden).
4. Комментарии не видны вне tenant (SEC-0005 через проекты).
5. Удалённый повторно комментарий → 404.
```

---

# 5. Schema

```sql
CREATE TABLE project_comments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    author_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       TEXT NOT NULL CHECK (length(body) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX project_comments_project_idx ON project_comments (project_id, created_at);
CREATE INDEX project_comments_author_idx ON project_comments (author_id);
```

---

# 6. API

| Method | Path | Body | Result |
|--------|------|------|--------|
| GET | `/api/v1/projects/{id}/comments` | - | 200 list, 403, 404 |
| POST | `/api/v1/projects/{id}/comments` | `{"body"}` | 201, 400, 403, 404 |
| DELETE | `/api/v1/projects/{id}/comments/{commentID}` | - | 204, 403, 404 |

`DELETE` маршрутизируется по SQL: удаление только для
`author_id = actor` или когда actor имеет роль owner (subquery по
project_members). Остальные случаи → 404 (не найден/не удалён).

---

# 7. Tests

- Service unit: член добавляет/читает; не-член получает ErrNotFound;
  пустое тело — ошибка; удаление автором/владельцем — ок; чужой член —
  ErrForbidden; повторное удаление — ErrNotFound.
- Infra repository: CRUD комментариев (сортировка по времени),
  удаление автором/владельцем, чужой — ErrNotFound.
- Transport: DTO и коды (201/204/403/404).
- Security: не-член не видит комментарии.

---

# 8. Acceptance Criteria

- Модель реализована в `internal/application/project` (Comment),
  репозиторий — `migration 000007` + `internal/infrastructure/database`.
- Права фактически проверяются (членство для чтения/записи; автор/owner
  для удаления).
- Миграция 000007 применяется на всех средах.
- Фронтенд: панель «Обсуждение» на странице проекта.
- E2E: участник добавляет комментарий, видит его; владелец удаляет.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-14 | Первоначальная редакция |

---

APPROVED