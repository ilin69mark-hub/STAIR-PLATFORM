# STAIR PLATFORM

**Document:** EDR-0008_Multi_User_Project.md

**ID:** EDR-0008

**Status:** APPROVED

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

Подсистема совместного доступа (Phase C, Project Sharing) использует ту же
модель: приглашение участника по email (C2), список «доступные мне»
(проекты, где вызывающий — не владелец).

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
| POST | `/api/v1/projects/{id}/members` | `{"email", "role"}` или `{"user_id", "role"}` | 201, 403 нет прав, 404 пользователь не найден, 422 невалидная роль |
| PATCH | `/api/v1/projects/{id}/members/{userID}` | `{"role"}` | 200, 403, 404 |
| DELETE | `/api/v1/projects/{id}/members/{userID}` | - | 204, 403, 404 |

Добавление участника (share) принимает **email** или **user_id**:
`email` резолвится в пользователя того же tenant (SEC-0005) —
C2 Project Sharing. Если передан `email`, используется
`AddMemberByEmail`; иначе — `AddMember` по `user_id`.

Список проектов `GET /api/v1/projects` уже включает поле `owner_id`
(EDR-0008); фронтенд делит его на «мои проекты» (`owner_id == текущий
пользователь`) и «доступные мне» (`owner_id != текущий пользователь`).

Ошибка авторизации — 403 (forbidden), несуществующий проект/участник —
404 (not found).

---

# 9. Tests

- Service unit: owner добавляет/меняет/удаляет участников; editor не
  может управлять участниками; viewer не может рассчитывать; не-член
  получает ErrNotFound; смена роли viewer→editor открывает расчёт;
  приглашение по email (C2) резолвит в пользователя tenant и запрещено
  не-owner'у; неизвестный/пустой email → ошибка.
- Infra repository: членство (CRUD), защита владельца
  (UpdateMemberRole/RemoveMember на owner отклоняется), автовложение
  owner при создании; AddMemberByEmail (C2) по email с проверкой tenant.
- Transport: DTO и коды ответов (201/204/403/404/422); приём email или
  user_id при добавлении.
- Security: проект tenant A не виден из tenant B; добавление
  пользователя другого tenant отклоняется (не резолвится по email).

---

# 10. Acceptance Criteria

- Модель реализована в `internal/application/project` (member.go,
  service.go), репозиторий — `internal/infrastructure/database`
  (migration 000006).
- Права: фактически проверяются Role.CanEdit / Role.CanManage.
- Приглашение по email (C2) работает на уровне service/repo/transport.
- Миграция 000006 применяется на всех средах (в т.ч. тестовых).
- Фронтенд: управление участниками на странице проекта; список делится
  на «мои проекты» и «доступные мне».
- E2E: owner создаёт проект; второй пользователь не видит проект до
  добавления (виджет по email); viewer видит, но не редактирует;
  editor редактирует.

---

# 11. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-14 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-14 | C2 Project Sharing: приглашение участника по email (AddMemberByEmail), список «доступные мне» на фронтенде; status APPROVED |

---

APPROVED

## §7. Передача владения проектом

Проект принадлежит конкретному человеку, а не компании. Пока единственный способ
это изменить был — удалить сотрудника, и тогда все его проекты уезжают
получателю (EDR-0041 §7). Для живой ситуации («менеджер уходит в отпуск, проект
нужно вести дальше») нужна отдельная операция: `POST /api/v1/projects/{id}/transfer`
с `{"to_user_id": "..."}`.

**Владение описано в двух местах, и меняются оба одной транзакцией:**
`project_members.role` (уникальный индекс «один owner на проект») и
`projects.owner_id` (денормализация, по которой считаются выборки). Обновить одно
из двух — значит оставить проект без владельца или с двумя. Порядок внутри
транзакции задан индексом: прежний владелец понижается до `editor`, и только
потом новый повышается до `owner`.

**Правила:**

| Правило | Поведение | Почему |
|---|---|---|
| кто передаёт | владелец проекта; либо любой участник, если у него есть платформенное право `staff.project_transfer` | владелец компании должен уметь забрать проект у человека, который исчез |
| кому | только участник этого проекта | передача «на улице» означала бы, что человек появится в проекте без приглашения |
| прежний владелец | становится `editor`, не удаляется | он работал с проектом; уход из проекта — его решение, как и при удалении сотрудника |
| себе | отказ `409 conflict` | владелец уже этот человек — действие пустое |
| несуществующий проект | `404` | «передача» не должна рапортовать об успехе вслепую |

Проверка платформенного права живёт в транспорте (`hasCapability`) и
передаётся в сервис флагом `byCapability`: в прикладном уровне метрики
неизвестны, а решение «владелец проекта ИЛИ право» — доменное.

Из прежней роли `owner` нельзя было выйти обычным путём
(`UpdateMemberRole` специально не трогает строки `owner`), поэтому передача —
отдельная операция, а не побочный эффект смены роли.

Список участников отдаёт имя и почту (`memberDTO.email/name`, джойн с `users` в
`ListMembers`): без них выбор «кому передать» невозможен. Данные отдаются только
после проверки членства, поэтому чужие адреса наружу не уходят.
