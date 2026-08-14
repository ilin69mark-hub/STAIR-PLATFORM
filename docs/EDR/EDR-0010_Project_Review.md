# STAIR PLATFORM

**Document:** EDR-0010_Project_Review.md

**ID:** EDR-0010

**Status:** DRAFT

**Author:** Project Team

**Date:** 2026-08-14

**Category:** Collaboration

---

# 1. Purpose

Документ фиксирует модель ревью проекта (Phase C, Review): цикл
обсуждения и приёмки конфигурации в рамках участников проекта. Ревью —
переходы статуса проекта (request / sign-off / request changes) с
записью истории переходов (project_reviews). Закладывает основу для C5
(Approval) и C6 (Versioning).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase C — Collaboration)
- SEC-0005 (Tenant Isolation)
- BC-001 (Projects Bounded Context)
- EDR-0008 (Multi-user Project — членство/роли)
- EDR-0009 (Project Comments)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 Статусы проекта (projects.status)

| Status | Описание |
|--------|----------|
| draft | черновик, расчёт разрешён (по умолчанию) |
| in_review | запрошено ревью; конфигурация «заморожена» от изменений |
| approved | подписано владельцем; терминальное (до нового draft) |
| changes_requested | владелец вернул на доработку; открыто для изменений |

Статус хранится в существующей колонке `projects.status`; допустимые
значения фиксируются CHECK-констрейнтом (миграция 000008).

## 3.2 Сущность ProjectReview

| Field | Описание |
|-------|----------|
| ID | UUID |
| ProjectID | проект (FK, ON DELETE CASCADE) |
| RequesterID | автор запроса ревью (FK users) |
| ReviewerID | подписант/вернувший (FK users); NULL до решения |
| Decision | `requested` \| `approved` \| `changes_requested` |
| Comment | комментарий, может быть пустым |
| CreatedAt | время запроса |
| DecidedAt | время решения; NULL до решения |

Каждый переход статуса создаёт новую строку `project_reviews` (аудит).
Ревью скоупятся по tenant через проект; доступ проверяется по членству
(EDR-0008).

## 3.3 Права

| Операция | Требование |
|----------|------------|
| RequestReview | член проекта с ролью owner/editor (`CanEdit`) |
| SignOffReview | роль owner |
| RequestChanges | роль owner |
| ListReviews | член проекта |

Запрет self-approve: подписант не может быть автором запроса. Возврат на
доработку (changes_requested) также недоступен автору запроса.

---

# 4. Invariants

```
1. Все операции доступны только члену проекта (иначе 404).
2. RequestReview: только owner/editor; переход draft|changes_requested → in_review.
3. SignOffReview: только owner; переход in_review → approved.
4. RequestChanges: только owner; переход in_review → changes_requested.
5. Ревью не может подписать/вернуть автор запроса (403).
6. Переходы из недопустимых статусов → ErrConflict (422).
7. Проект в статусе in_review не принимает расчет (Calculate недоступен
   до решения: draft|changes_requested|approved).
8. Ревью не видны вне tenant (SEC-0005 через проекты).
```

---

# 5. Schema

```sql
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_status_check;
ALTER TABLE projects ADD CONSTRAINT projects_status_check
    CHECK (status IN ('draft', 'in_review', 'approved', 'changes_requested'));

CREATE TABLE project_reviews (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    requester_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reviewer_id  UUID REFERENCES users(id) ON DELETE CASCADE,
    decision     TEXT NOT NULL CHECK (decision IN ('requested', 'approved', 'changes_requested')),
    comment      TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at   TIMESTAMPTZ
);

CREATE INDEX project_reviews_project_idx ON project_reviews (project_id, created_at);
```

---

# 6. API

| Method | Path | Body | Result |
|--------|------|------|--------|
| POST | `/api/v1/projects/{id}/review` | `{"comment"}` | 201 review, 403, 404, 422 |
| POST | `/api/v1/projects/{id}/reviews/{reviewID}/sign-off` | `{"comment"}` | 200, 403, 404, 422 |
| POST | `/api/v1/projects/{id}/reviews/{reviewID}/changes` | `{"comment"}` | 200, 403, 404, 422 |
| GET | `/api/v1/projects/{id}/reviews` | - | 200 list, 403, 404 |

Переходы атомарны: обновление `projects.status` + строка `project_reviews`
в одной транзакции (BE-0006). Неверный переход → 422 (invalid_status);
не-член → 404; недостаточная роль → 403; self-approve → 403.

---

# 7. Tests

- Service unit: owner/editor запрашивает ревью; viewer — ErrForbidden;
  не-член — ErrNotFound; sign-off только owner; автор запроса не может
  подписать (ErrForbidden); подпись из draft — ErrConflict; возврат на
  доработку валюты только owner из in_review; повторный request из
  in_review — ErrConflict; Calculate в in_review — ErrConflict.
- Infra repository: запись перехода + смена статуса атомарно, история
  по возрастанию времени, чужой tenant — ErrNotFound.
- Transport: DTO и коды (201/200/403/404/422).
- Security: не-член не видит историю ревью.

---

# 8. Acceptance Criteria

- Модель реализована в `internal/application/project` (ProjectReview),
  репозиторий — `migration 000008` + `internal/infrastructure/database`.
- Права фактически проверяются (CanEdit для запроса; owner для решения;
  запрет self-approve).
- Миграция 000008 применяется на всех средах.
- Фронтенд: панель «Ревью» на странице проекта (бейдж статуса, кнопки,
  история).
- E2E: участник запрашивает ревью; владелец подписывает.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-14 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-14 | Реализация end-to-end (backend+API+frontend, тесты); status APPROVED |

---

APPROVED