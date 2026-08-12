# STAIR PLATFORM

**Document:** 02_CAPABILITY_MAP.md

**Document ID:** PROD-0003

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ определяет полную карту возможностей (Capability Map) Stair Platform.

Capability Map является главным каталогом функциональных возможностей платформы.

Документ используется для:

- проектирования архитектуры;
- декомпозиции системы;
- определения границ модулей;
- лицензирования;
- построения API;
- управления Roadmap;
- планирования разработки.

Каждая функциональная возможность (Capability) должна иметь единственного владельца и четко определенную ответственность.

---

# 2. Capability Design Principles

Все возможности платформы проектируются по следующим принципам:

- Single Responsibility;
- High Cohesion;
- Low Coupling;
- API First;
- Independent Evolution;
- Event Driven;
- Configuration Driven;
- Feature Flag Ready.

---

# 3. Capability Hierarchy

Платформа разделяется на четыре уровня.

```
Platform
    │
    ├── Core Capabilities
    ├── Business Capabilities
    ├── Supporting Capabilities
    └── Platform Capabilities
```

---

# 4. Core Capabilities

Основные инженерные возможности платформы.

| ID | Capability | Description |
|----|------------|-------------|
| CAP-001 | Identity | Аутентификация и авторизация |
| CAP-002 | Organizations | Управление организациями |
| CAP-003 | Workspaces | Рабочие пространства |
| CAP-004 | Projects | Управление инженерными проектами |
| CAP-005 | Geometry | Построение геометрии лестниц |
| CAP-006 | Solver | Инженерные расчеты |
| CAP-007 | Validation | Проверка ограничений |
| CAP-008 | Pricing | Расчет стоимости |
| CAP-009 | Rendering | 2D/3D визуализация |
| CAP-010 | Manufacturing | Подготовка производства |
| CAP-011 | Documents | Генерация документации |

---

# 5. Business Capabilities

Бизнес-возможности платформы.

| ID | Capability |
|----|------------|
| CAP-020 | CRM |
| CAP-021 | Orders |
| CAP-022 | Customers |
| CAP-023 | Quotations |
| CAP-024 | Billing |
| CAP-025 | Subscriptions |
| CAP-026 | Licenses |
| CAP-027 | Payments |

---

# 6. Supporting Capabilities

Поддерживающие возможности.

| ID | Capability |
|----|------------|
| CAP-040 | Notifications |
| CAP-041 | Search |
| CAP-042 | Files |
| CAP-043 | Analytics |
| CAP-044 | Reporting |
| CAP-045 | Audit |
| CAP-046 | Logging |
| CAP-047 | Monitoring |

---

# 7. Platform Capabilities

Инфраструктурные возможности.

| ID | Capability |
|----|------------|
| CAP-060 | API Gateway |
| CAP-061 | Integrations |
| CAP-062 | Webhooks |
| CAP-063 | AI Platform |
| CAP-064 | Feature Flags |
| CAP-065 | Configuration |
| CAP-066 | Localization |
| CAP-067 | Administration |

---

# 8. Capability Dependencies

Каждая Capability может зависеть только от нижележащих возможностей.

Пример:

```
Projects
    │
    ├── Geometry
    ├── Solver
    ├── Validation
    ├── Pricing
    └── Documents
```

Запрещается:

- циклическая зависимость;
- взаимное владение данными;
- обращение к внутренним структурам другого модуля.

---

# 9. Capability Lifecycle

Каждая Capability проходит следующие стадии.

```
Idea

↓

Analysis

↓

Specification

↓

Architecture

↓

Development

↓

Testing

↓

Release

↓

Maintenance

↓

Retirement
```

---

# 10. Capability Ownership

Для каждой Capability обязательно определяется:

- Product Owner;
- Technical Owner;
- Domain Owner;
- Documentation Owner.

---

# 11. Capability Metadata

Каждая Capability должна содержать:

- ID;
- Name;
- Description;
- Business Goal;
- Responsibilities;
- Dependencies;
- Domain Objects;
- API;
- Events;
- Permissions;
- Feature Flags;
- Metrics;
- ADR References.

---

# 12. Capability Traceability

Каждая Capability должна быть связана с:

Business Goal

↓

Requirement

↓

Use Case

↓

Domain

↓

Database

↓

API

↓

Frontend

↓

Backend

↓

Tests

↓

Documentation

↓

ADR

---

# 13. Expansion Rules

Добавление новой Capability допускается только при выполнении следующих условий:

- определена бизнес-ценность;
- определены границы ответственности;
- отсутствует дублирование существующих возможностей;
- подготовлена документация;
- принято ADR при изменении архитектуры.

---

# 14. Dependencies

Incoming

- PRODUCT_ARCHITECTURE

Outgoing

- PRODUCT_MODULES
- DOMAIN
- API
- DATABASE
- BACKEND
- FRONTEND
- AI

---

# 15. Acceptance Criteria

Документ считается завершенным, если:

- определен полный каталог возможностей;
- определены уровни Capability;
- описаны зависимости;
- определены правила расширения;
- определены требования к каждой Capability.

---

# 16. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 17. Approval

APPROVED