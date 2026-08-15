# STAIR PLATFORM

**Document:** EDR-0015_Advanced_Permissions.md

**ID:** EDR-0015

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Enterprise

---

# 1. Purpose

Документ фиксирует Advanced Permissions (Phase G, Advanced Permissions):
централизованную permission-модель (RBAC) поверх SEC-0004. Вводится
единый механизм проверки прав `HasPermission(permission)` вместо
разрозненных inline-сравнений ролей и хелперов `CanEdit`/`CanManage`.
Скоуп G3: матрица прав для системных ролей (user/admin) и проектных
(owner/editor/viewer), перевод существующих проверок на единый механизм
и admin-эндпоинты управления пользователями.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase G — Enterprise, Advanced Permissions)
- SEC-0004 (Authorization Model)
- SEC-0003 (Identity and Authentication)
- SEC-0005 (Tenant Isolation)
- EDR-0008 (Multi-User Project — проектные роли)
- EDR-0013 (Audit Log — admin-доступ к глобальному журналу)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 Permission

`Permission` — именованное право на операцию (строка). Размещаются в
контексте владельца: системные — `auth` package, проектные — `project`
package (инверсия зависимостей: каждый Bounded Context определяет свои
права).

## 3.2 Системная матрица (auth.Role)

| Permission | user | admin |
|------------|------|-------|
| audit.read_all | - | ✓ |
| users.list | - | ✓ |
| users.update_role | - | ✓ |

Роль предоставляет набор прав: `Role.Permissions() []Permission`;
проверка — `Role.HasPermission(p) bool`.

## 3.3 Проектная матрица (project.ProjectRole)

| Permission | viewer | editor | owner |
|------------|--------|--------|-------|
| project.read | ✓ | ✓ | ✓ |
| project.edit | - | ✓ | ✓ |
| project.manage | - | - | ✓ |

`ProjectRole.Permissions()` / `ProjectRole.HasPermission(p)` заменяют
`CanEdit` (read+edit) и `CanManage` (owner).

## 3.4 Admin-эндпоинты управления пользователями

Admin (permission `users.list`, `users.update_role`) управляет
пользователями своего tenant (SEC-0005):

| Method | Path | Permission | Описание |
|--------|------|------------|----------|
| GET | `/api/v1/admin/users` | users.list | список пользователей tenant |
| PATCH | `/api/v1/admin/users/{id}` | users.update_role | смена роли user↔admin |

Запрет: admin не может сменить роль самому себе (защита от удаления
последнего администратора через API); роль меняется только на
user|admin.

---

# 4. Invariants

```
1. Проверка прав — только через Role.HasPermission / ProjectRole.HasPermission
   (без inline-сравнений ролей в сервисах и транспорте).
2. Матрица задаётся декларативно: Permissions() по каждой роли.
3. Admin-эндпоинты скоуплены по tenant (SEC-0005): список и смена роли
   только в пределах tenant вызывающего.
4. Роль admin (привилегия) создаётся только через БД/seed (SEC-0004);
   через API роль можно назначить только существующему пользователю tenant.
5. Admin не может изменить собственную роль (инвариант наличия админа).
6. Отказы фиксируются в аудите (EDR-0013): users.list/users.update_role
   (denied).
```

---

# 5. Schema

Изменений схемы нет: матрица прав декларативна (в коде), роли уже
хранятся в `users.role` (`CHECK role IN ('user','admin')`, миграция
000003) и `project_members.role` (owner/editor/viewer, миграция 000006).

---

# 6. API

| Method | Path | Body | Result |
|--------|------|------|--------|
| GET | `/api/v1/admin/users` | - | 200 list, 403 |
| PATCH | `/api/v1/admin/users/{id}` | `{"role":"admin"}` | 200 user, 403, 404, 422 |

`list users` — пользователи tenant (id, email, name, role, status).
`update role` — смена роли; 422 — невалидная роль; 403 — не-admin или
смена собственной роли; 404 — пользователь вне tenant.

---

# 7. Tests

- Permission: матрицы (Permissions()/HasPermission) для всех ролей.
- Auth service: ListUsers/UpdateUserRole (только admin, tenant-скоуп,
  нельзя самому себе, невалидная роль).
- Repo DB: ListUsers/UpdateUserRole реальной БД.
- Transport: 403 не-admin; 200 список; смена роли; 422; 404.
- Integration: admin меняет роль пользователю, тот получает admin-права.

---

# 8. Acceptance Criteria

- Permission-модель реализована в auth и project; проверки переведены
  на HasPermission (без inline-сравнений ролей).
- Admin-эндпоинты GET /admin/users и PATCH /admin/users/{id} работают
  с tenant-скоупом и проверкой прав.
- Нельзя сменить собственную роль; роль меняется только на user|admin.
- EDR-0015 помечен APPROVED; ROADMAP Phase G Advanced Permissions CLOSED.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализация end-to-end; статус APPROVED |

---

APPROVED
