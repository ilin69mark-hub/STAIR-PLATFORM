# STAIR PLATFORM

**Document:** 04_PRODUCT_BOUNDARIES.md

**Document ID:** PROD-0005

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ определяет границы модулей Stair Platform.

Документ используется для:

- разделения ответственности;
- предотвращения сильной связанности;
- определения владельцев данных;
- проектирования API;
- проектирования Domain Model;
- поддержки масштабируемости.

Каждый модуль обязан соблюдать установленные границы.

---

# 2. Boundary Principles

При проектировании модулей используются следующие принципы.

- Single Responsibility
- Explicit Ownership
- Clear Interfaces
- No Shared State
- No Circular Dependencies
- Event Driven Communication
- API First
- Stable Contracts

---

# 3. Bounded Contexts

Каждый модуль является отдельным Bounded Context.

```
Identity

Organization

Workspace

Project

Geometry

Solver

Validation

Pricing

Rendering

Manufacturing

CRM

Orders

Billing

AI

Analytics
```

Каждый контекст имеет собственную модель предметной области.

---

# 4. Data Ownership

Каждая сущность принадлежит только одному модулю.

| Entity | Owner |
|---------|--------|
| User | Identity |
| Organization | Organization |
| Workspace | Organization |
| Project | Project |
| Stair Model | Geometry |
| Calculation | Solver |
| Validation Report | Validation |
| Price Calculation | Pricing |
| Render | Rendering |
| BOM | Manufacturing |
| Order | Orders |
| Invoice | Billing |
| AI Session | AI |

Ни один другой модуль не имеет права напрямую изменять эти данные.

---

# 5. Communication Rules

Взаимодействие между модулями выполняется только посредством:

- Public API;
- Domain Events;
- Application Services.

Запрещается:

- прямой доступ к БД другого модуля;
- импорт внутренних пакетов;
- использование внутренних моделей данных.

---

# 6. Dependency Rules

Допустимые зависимости:

```
Frontend

↓

Application

↓

Domain

↓

Infrastructure
```

Межмодульное взаимодействие:

```
Project

↓

Geometry

↓

Solver

↓

Pricing

↓

Documents
```

Циклические зависимости запрещены.

---

# 7. Shared Kernel

Допускается использование только ограниченного Shared Kernel.

Разрешены:

- Value Objects;
- Primitive Types;
- Common Interfaces;
- Event Contracts;
- Error Codes.

Запрещено:

- Aggregate Root;
- Repository;
- Service;
- Domain Logic.

---

# 8. Integration Contracts

Каждый модуль обязан публиковать:

- API Contract;
- Event Contract;
- Error Contract;
- Version Policy.

Все контракты должны быть обратно совместимыми.

---

# 9. Extension Rules

Новые функции добавляются только посредством:

- новой Capability;
- нового Module;
- нового Domain Context;
- нового API.

Изменение существующих границ допускается только через ADR.

---

# 10. Change Management

Любое изменение границ требует:

- архитектурного анализа;
- обновления документации;
- обновления UML;
- обновления Domain Model;
- создания ADR.

---

# 11. Cross-Cutting Policies

Все контексты обязаны поддерживать:

- RBAC;
- Audit;
- Logging;
- Metrics;
- Tracing;
- Localization;
- Versioning.

---

# 12. Traceability

Каждая граница должна быть связана с:

Capability

↓

Module

↓

Domain Context

↓

API

↓

Database

↓

Tests

↓

ADR

---

# 13. Dependencies

Incoming

- PRODUCT_MODULES

Outgoing

- DOMAIN
- API
- DATABASE
- BACKEND

---

# 14. Acceptance Criteria

Документ считается завершенным, если:

- определены границы всех модулей;
- определены владельцы данных;
- определены правила взаимодействия;
- определены правила расширения;
- определены правила изменения границ.

---

# 15. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 16. Approval

APPROVED