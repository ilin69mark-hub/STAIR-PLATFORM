# STAIR PLATFORM

**Document:** 02_CONTEXT_MAP.md

**Document ID:** DOM-0003

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ определяет карту ограниченных контекстов (Context Map) Stair Platform.

Context Map определяет:

- границы предметной области;
- взаимодействие доменов;
- зависимости;
- интеграционные паттерны;
- правила обмена данными.

Данный документ является основой всей архитектуры DDD.

---

# 2. Context Map Principles

Все контексты должны:

- иметь собственную модель данных;
- иметь собственный язык;
- иметь собственные бизнес-правила;
- взаимодействовать только через опубликованные контракты;
- не разделять внутренние модели.

---

# 3. Domain Classification

Предметная область разделяется на четыре уровня.

```
Core Domain

Supporting Domain

Generic Domain

Infrastructure Domain
```

---

# 4. Core Domain

Core Domain содержит ключевую интеллектуальную собственность платформы.

```
Geometry

Structural Solver

Validation

Manufacturing

Pricing

Rendering
```

Эти контексты являются конкурентным преимуществом платформы.

---

# 5. Supporting Domains

```
Projects

Documents

AI

Analytics

Search
```

Поддерживают Core Domain, но не содержат уникальных алгоритмов.

---

# 6. Generic Domains

```
Identity

Organization

Licensing

Billing

Notifications

Files

Audit
```

Типовые сервисы SaaS-платформы.

---

# 7. Infrastructure Domains

```
Storage

Messaging

Monitoring

Logging

Caching

Configuration
```

Используются только инфраструктурным слоем.

---

# 8. Context Relationships

```
Organization
        │
        ▼
Workspace
        │
        ▼
Project
        │
        ├────────────┬──────────────┬──────────────┐
        ▼            ▼              ▼              ▼
 Geometry       Validation      Solver       Rendering
        │            │              │              │
        └────────────┴──────────────┴──────────────┘
                       │
                       ▼
                 Manufacturing
                       │
                       ▼
                    Pricing
                       │
                       ▼
                   Documents
                       │
                       ▼
                     Orders
```

---

# 9. Integration Patterns

Разрешены следующие паттерны DDD:

- Customer / Supplier;
- Conformist;
- Anti-Corruption Layer;
- Shared Kernel;
- Published Language;
- Open Host Service.

Использование других паттернов требует отдельного ADR.

---

# 10. Shared Kernel

Допускается совместное использование только:

- Value Objects;
- Domain Events;
- Public Contracts;
- Enumerations;
- Primitive Types.

Запрещается совместное использование:

- Aggregate;
- Repository;
- Domain Service;
- Entity.

---

# 11. Anti-Corruption Layer

Все внешние системы интегрируются через ACL.

Примеры:

```
ERP

↓

ACL

↓

Orders
```

```
External CAD

↓

ACL

↓

Geometry
```

---

# 12. Published Language

Каждый контекст публикует:

- API Contract;
- Event Contract;
- Error Contract;
- Version Policy.

---

# 13. Context Independence

Каждый контекст должен:

- независимо тестироваться;
- независимо развиваться;
- иметь собственную документацию;
- иметь собственные ADR;
- иметь собственную модель.

---

# 14. Traceability

Каждый Context связан с:

Business Goal

↓

Capability

↓

Module

↓

Context

↓

Aggregate

↓

Entity

↓

Repository

↓

API

↓

Tests

↓

ADR

---

# 15. Dependencies

Incoming

- UBIQUITOUS_LANGUAGE

Outgoing

- BOUNDED_CONTEXTS
- AGGREGATES
- DOMAIN_SERVICES

---

# 16. Acceptance Criteria

Документ считается завершенным, если:

- определены все Bounded Context;
- определены связи;
- определены паттерны интеграции;
- определены правила взаимодействия;
- определены границы ответственности.

---

# 17. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 18. Approval

APPROVED