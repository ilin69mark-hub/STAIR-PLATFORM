# STAIR PLATFORM

**Document:** 03_BOUNDED_CONTEXTS.md

**Document ID:** DOM-0004

**Version:** 1.0.0

**Status:** APPROVED

---

# 1. Purpose

Настоящий документ содержит перечень всех Bounded Context платформы Stair Platform.

Каждый контекст является самостоятельной моделью предметной области.

Полная спецификация каждого контекста находится в отдельном документе каталога `bounded-contexts`.

---

# 2. Context Classification

## Core Engineering Contexts

| ID | Context |
|----|---------|
| BC-001 | Project |
| BC-002 | Geometry |
| BC-003 | Solver |
| BC-004 | Validation |
| BC-005 | Manufacturing |
| BC-006 | Pricing |
| BC-007 | Rendering |

---

## Business Contexts

| ID | Context |
|----|---------|
| BC-020 | Organization |
| BC-021 | Workspace |
| BC-022 | Orders |
| BC-023 | Billing |
| BC-024 | Licensing |

---

## Platform Contexts

| ID | Context |
|----|---------|
| BC-040 | Identity |
| BC-041 | Notifications |
| BC-042 | Files |
| BC-043 | Analytics |
| BC-044 | Search |

---

## AI Contexts

| ID | Context |
|----|---------|
| BC-060 | AI Assistant |
| BC-061 | Knowledge Base |
| BC-062 | Recommendations |

---

# 3. Context Documentation

Каждый Bounded Context обязан содержать:

- Purpose;
- Responsibilities;
- Aggregates;
- Entities;
- Value Objects;
- Domain Services;
- Domain Events;
- Policies;
- Invariants;
- Public Interfaces;
- Dependencies.

---

# 4. Dependencies

Incoming

- CONTEXT_MAP

Outgoing

- AGGREGATES
- ENTITIES
- VALUE_OBJECTS
- DOMAIN_SERVICES

---

# 5. Approval

APPROVED