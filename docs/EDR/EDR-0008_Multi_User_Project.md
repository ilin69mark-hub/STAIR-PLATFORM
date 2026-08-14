# STAIR PLATFORM

**Document:** EDR-0008_Multi_User_Project.md

**ID:** EDR-0008

**Status:** DRAFT

**Author:** Project Team

**Date:** 2026-08-14

**Category:** Collaboration / Access Control

---

# 1. Purpose

Документ фиксирует модель многопользовательских проектов (Phase C,
Multi-user Projects): владение проектом и соучастники с ролями.
Определяет права участников, инварианты и защиту от межтенантного
доступа (SEC-0005).

Совместная работа внутри одного tenant (ECR): проект принадлежит
пользователю (owner), другие пользователи того же tenant добавляются
как участники с ролью editor или viewer.

---

# 2. Related Artifacts

- SEC-0005 (Tenant Isolation)
- ROADMAP-0011 (Phase C — Collaboration)
- BC-001 (Projects Bounded Context)
- ADR-0006 (Layered Architecture)
- FR-060 (Проекты) — создание/список/расчёт (MVP)

---

# 3. Model

## 3.1 Сущности

| Entity | Описание |
|--------|----------|
| Project | проект; имеет `owner_id` — владельца из `users` |
| ProjectMember | соучастник проекта: `(project_id, user_id, role)` |

## 3.2 Roles

| Role | Права |
|------|-------|
| owner | чтение, изменение, расчёт, управление участниками (добавление, смена роли, удаление) |
| editor | чтение, изменение, расчёт |
| viewer | только чтение |

- `CanEdit(role)` = owner ∨ editor.
- `CanManage(role)` = owner.

## 3.3 Жизненный цикл

- Проект создаётся от имени пользователя, который становится owner
  (owner автоматически добавляется в `project_members` с ролью owner).
- Участники добавляются/удаляются/меняют роль только владельцем.
- Удаление участника снимает доступ к проекту (редиректы на 404/403).
- Роль владельца не может быть изменена или удалена.

---

# 4. Invariants

```
1. Проект имеет ровно одного владельца (unique partial index
   project_members_one_owner по роли 'owner').
2. Участник относится к тому же tenant, что и проект
   (проверка по таблице users при добавлении).
3. Изменение роли владельца запрещено.
4. Удаление владельца запрещено.
5. Все операции скоупированы по tenant: caller передаёт
   (tenantID, userID), Repository проверяет членство через
   LEFT JOIN project_members.
6. Не-член проекта не видит его (404), член с недостаточной
   ролью для операции получает 403 (forbidden).
```

---

# 5. Authorization

| Operation | Требование |
|-----------|------------|
| CreateProject | любой аутентифицированный пользователь tenant (становится owner) |
| GetProject / ListProjects | членство (owner/editor/viewer) |
| ListMembers | членство |
| AddMember / UpdateMemberRole / RemoveMember | роль owner (иначе 403) |
| Calculate / GetResult / GetLatestConfig | owner/editor для Calculate; любое членство для чтения |

---

# 6. Tenancy (SEC-0005)

- Каждая операция передаёт `tenantID` и `userID` вызывающего.
- Права проверяются по строке `project_members` с учётом ownership.
- `CreateProject` связывает владельца с его tenant и добавляет
  запись о включении в проект внутри той же транзакции.
- `AddMember` подтверждает существование приглашённого в том же
  tenant (запрет межтенантных ссылок).

---

# 7. Schema

```sql
ALTER TABLE projects ADD COLUMN owner_id UUID REFERENCES users(id);

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id),
    role       text  NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

-- единственный владелец на проект
CREATE UNIQUE INDEX project_members_one_owner
    ON project_members (project_id) WHERE role = 'owner';

CREATE INDEX project_members_user_idx ON project_members (user_id);
CREATE INDEX projects_owner_idx ON projects (owner_id);
```

---

# 8. API

| Method | Path | Body | Result |
|--------|------|------|--------|
| GET | `/api/v1/projects/{id}/members` | - | 200 list, 404 нет проекта |
| POST | `/api/v1/projects/{id}/members` | `{"user_id", "role"}` | 201, 403 нет прав, 422 невалидная роль |
| PATCH | `/api/v1/projects/{id}/members/{userID}` | `{"role"}` | 200, 403, 404 |
| DELETE | `/api/v1/projects/{id}/members/{userID}` | - | 204, 403, 404 |

Ошибка авторизации — 403 (forbidden), несуществующий проект/участник —
404 (not found).

---

# 9. Tests

- Service unit: owner добавляет/меняет/удаляет участников; editor не
  может управлять участниками; viewer не может рассчитывать; не-член
  получает ErrNotFound; смена роли viewer→editor открывает расчёт.
- Infra repository: членство (CRUD), защита владельца
  (UpdateMemberRole/RemoveMember на owner отклоняется), автовложение
  owner при создании.
- Transport: DTO и коды ответов (201/204/403/404/422).
- Security: проект tenant A не виден из tenant B; добавление
  пользователя другого tenant отклоняется.

---

# 10. Acceptance Criteria

- Модель реализована в `internal/application/project` (member.go,
  service.go), репозиторий — `internal/infrastructure/database`
  (migration 000006).
- Права: фактически проверяются Role.CanEdit / Role.CanManage.
- Миграция 000006 применяется на всех средах (в т.ч. тестовых).
- Фронтенд: управление участниками на странице проекта.
- E2E: owner создаёт проект; второй пользователь не видит проект до
  добавления; viewer видит, но не редактирует; editor редактирует.

---

# 11. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-14 | Первоначальная редакция (DRAFT) |

---

DRAFT