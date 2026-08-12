# STAIR PLATFORM

Document: 21_DOMAIN_DEPENDENCIES.md

ID: DOM-0021

Status: APPROVED

---

# Purpose

Документ определяет допустимые зависимости между доменными областями.

---

# Objectives

- исключить циклические зависимости;
- определить направление зависимостей;
- обеспечить независимость доменных моделей.

---

# Dependency Graph

Project
↓
Assembly
↓
Part
↓
Geometry
↓
Manufacturing
↓
Pricing
↓
Documents
↓
Installation
↓
Maintenance

---

# Allowed Dependencies

Assembly → Project

Part → Assembly

Geometry → Part

Manufacturing → Geometry

Pricing → Manufacturing

Documents → Project

Installation → Manufacturing

Maintenance → Installation

---

# Forbidden Dependencies

Pricing → Geometry Database

Geometry → Pricing

Manufacturing → Project Repository

Maintenance → Pricing

Part → Documents

---

# Cross Context Communication

Взаимодействие между Bounded Context осуществляется только через:

- Domain Events;
- Application Services;
- Public Contracts.

---

# Dependency Validation

Каждая новая зависимость проходит:

- Architecture Review;
- ADR Review;
- Dependency Analysis.

---

# Acceptance Criteria

- отсутствуют циклические зависимости;
- соблюдены границы Bounded Context;
- все зависимости документированы.

---

APPROVED