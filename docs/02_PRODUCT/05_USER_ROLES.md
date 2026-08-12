# STAIR PLATFORM

**Document:** 05_USER_ROLES.md

**Document ID:** PROD-0006

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ определяет модель пользователей, ролей и ответственности Stair Platform.

Документ описывает:

- участников системы;
- роли;
- области ответственности;
- уровни доступа;
- взаимодействие ролей.

Роли являются бизнес-абстракциями и не содержат технических разрешений напрямую.

---

# 2. Design Principles

Модель ролей строится на принципах:

- Least Privilege;
- Separation of Duties;
- Policy-Based Access;
- Capability-Based Authorization;
- Multi-Tenant Isolation;
- Extensibility.

---

# 3. User Hierarchy

```
Platform

↓

Organization

↓

Workspace

↓

Project

↓

User
```

Права пользователя определяются контекстом, в котором он работает.

---

# 4. Platform Roles

Роли уровня платформы.

| Role | Responsibility |
|------|----------------|
| Platform Administrator | Полное управление платформой |
| Support Engineer | Поддержка клиентов |
| Sales Manager | Продажи |
| Customer Success | Сопровождение клиентов |
| Finance Manager | Финансовые операции |

---

# 5. Organization Roles

Роли уровня организации.

| Role | Responsibility |
|------|----------------|
| Organization Owner | Владелец организации |
| Organization Administrator | Администратор организации |
| Billing Manager | Управление подпиской и оплатой |
| License Manager | Управление лицензиями |
| Security Administrator | Управление безопасностью |

---

# 6. Workspace Roles

Работа внутри подразделений.

| Role | Responsibility |
|------|----------------|
| Workspace Manager | Управление рабочим пространством |
| Team Lead | Руководитель команды |
| Engineer | Инженер |
| Designer | Проектировщик |
| Sales Engineer | Инженер по продажам |

---

# 7. Project Roles

Работа внутри инженерного проекта.

| Role | Responsibility |
|------|----------------|
| Project Owner | Ответственный за проект |
| Project Manager | Управление проектом |
| Constructor | Конструктор |
| Reviewer | Проверяющий |
| Viewer | Только просмотр |

---

# 8. External Roles

Внешние участники.

| Role | Responsibility |
|------|----------------|
| Customer | Заказчик |
| Dealer | Дилер |
| Manufacturer | Производитель |
| Installer | Монтажная организация |
| Partner | Партнер |

---

# 9. Role Scope

Каждая роль действует только в своем уровне.

```
Platform

↓

Organization

↓

Workspace

↓

Project
```

Роль не должна автоматически наследовать полномочия на других уровнях.

---

# 10. Responsibility Matrix

Каждая роль должна иметь:

- владельца;
- область ответственности;
- ограничения;
- доступные действия;
- связанные политики.

---

# 11. Role Lifecycle

```
Invitation

↓

Activation

↓

Modification

↓

Suspension

↓

Deactivation

↓

Deletion
```

---

# 12. Traceability

Каждая роль должна иметь связь:

Role

↓

Policy

↓

Capability

↓

Permission

↓

API

↓

Frontend

↓

Audit

---

# 13. Dependencies

Incoming

- PRODUCT_BOUNDARIES

Outgoing

- PERMISSION_MODEL
- AUTHORIZATION
- RBAC
- API
- UI

---

# 14. Acceptance Criteria

Документ считается завершенным, если:

- определены роли;
- определены уровни ответственности;
- определены области действия;
- определены правила жизненного цикла;
- определена трассируемость.

---

# 15. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 16. Approval

APPROVED