# STAIR PLATFORM

Document: 09_DEPENDENCY_RULES.md

ID: ARCH-0010

Status: APPROVED

---

# Purpose

Документ определяет допустимые зависимости между платформами.

---

# Objectives

- исключить циклические зависимости;
- обеспечить независимость модулей;
- определить направление зависимостей.

---

# Dependency Direction

Foundation

↓

Product

↓

Domain

↓

Architecture

↓

Engine

↓

Business Platforms

↓

API

↓

Infrastructure

---

# Allowed Dependencies

Engine

→ Domain

→ Architecture

Geometry

→ Engine

→ Domain

Manufacturing

→ Geometry

→ Engine

Pricing

→ Manufacturing

→ Geometry

API

→ все платформы через публичные контракты

---

# Forbidden Dependencies

Foundation ← Engine

Geometry ← Pricing

Geometry ← API

Manufacturing ← Database

Pricing ← Geometry Database

API ← Database

AI ← Database

Frontend ← Database

Plugin ← Internal Engine

---

# Dependency Validation

Каждая новая зависимость проверяется:

- ADR Review;
- Architecture Review;
- Dependency Analysis.

---

# Fitness Functions

Архитектура должна автоматически проверять:

- отсутствие циклов;
- отсутствие запрещённых зависимостей;
- отсутствие обращения к внутренним моделям.

---

# Acceptance Criteria

- граф зависимостей ацикличен;
- направление зависимостей соответствует Layered Architecture;
- новые зависимости проходят архитектурную проверку.

---

APPROVED