# STAIR PLATFORM

**Document:** 06_AUTHORIZATION_MODEL.md

**Document ID:** PROD-0007

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ определяет архитектуру авторизации Stair Platform.

Документ описывает:

- модель контроля доступа;
- уровни авторизации;
- политики безопасности;
- модель лицензирования;
- связь ролей, разрешений и возможностей платформы.

---

# 2. Authorization Principles

Архитектура авторизации строится на принципах:

- Default Deny;
- Least Privilege;
- Explicit Allow;
- Policy Based;
- Context Aware;
- Multi-Tenant Isolation;
- Auditable;
- Extensible.

---

# 3. Authorization Architecture

```
License
      │
      ▼
Capabilities
      │
      ▼
Policies
      │
      ▼
Permissions
      │
      ▼
Roles
      │
      ▼
Assignments
      │
      ▼
Users
```

---

# 4. Authorization Levels

Доступ проверяется последовательно.

```
Platform

↓

Organization

↓

Workspace

↓

Project

↓

Resource
```

Переход к следующему уровню выполняется только после успешной проверки предыдущего.

---

# 5. Authorization Context

Каждая проверка выполняется в контексте.

Контекст включает:

- Platform ID;
- Organization ID;
- Workspace ID;
- Project ID;
- User ID;
- Session ID;
- License ID.

---

# 6. Policy Engine

Policy Engine отвечает за принятие решения.

Поддерживаются политики:

- Allow;
- Deny;
- Conditional;
- Time-Based;
- Ownership-Based;
- Organization-Based;
- License-Based.

---

# 7. License Validation

Перед выполнением действия проверяется:

- активность лицензии;
- доступность Capability;
- лимиты;
- срок действия;
- ограничения тарифа.

---

# 8. Capability Validation

После проверки лицензии определяется доступность Capability.

Пример:

```
Geometry

↓

Available

↓

Continue
```

или

```
Geometry

↓

Disabled

↓

Access Denied
```

---

# 9. Permission Validation

После проверки Capability определяется наличие разрешения.

Например:

```
project.update
```

```
solver.execute
```

```
geometry.create
```

---

# 10. Ownership Rules

Дополнительно проверяется:

- владелец ресурса;
- организация;
- рабочее пространство;
- проект;
- дополнительные политики.

---

# 11. Authorization Decision

Результатом проверки является:

```
ALLOW
```

или

```
DENY
```

Все решения фиксируются в журнале аудита.

---

# 12. Audit Requirements

Каждая проверка должна сохранять:

- пользователя;
- ресурс;
- действие;
- время;
- результат;
- причину отказа.

---

# 13. Dependencies

Incoming

- USER_ROLES

Outgoing

- PERMISSION_MODEL
- IDENTITY
- API_GATEWAY
- SECURITY

---

# 14. Acceptance Criteria

Документ считается завершенным, если:

- определена архитектура авторизации;
- описаны уровни проверки;
- определены политики;
- определены правила лицензирования;
- определены правила аудита.

---

# 15. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 16. Approval

APPROVED