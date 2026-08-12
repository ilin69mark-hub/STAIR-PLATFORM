# STAIR PLATFORM

Document: 26_ROADMAP_CHANGE_CONTROL.md

ID: ROADMAP-0026

Status: APPROVED

---

# Purpose

Определяет Change Control для Roadmap.

---

# Change Types

## Minor Change

Не меняет:

* Architecture
* Critical Dependencies
* MVP Objective

Может быть выполнен без ADR.

---

## Major Change

Изменяет:

* Phase
* Milestone
* MVP Scope
* Critical Dependency
* Release Strategy

Требует documented impact analysis.

---

## Architectural Change

Изменяет архитектурное решение.

Требует ADR.

---

# Change Process

```text
Change Request
      ↓
Classification
      ↓
Impact Analysis
      ↓
Decision
      ↓
Documentation
      ↓
Roadmap Update
```

---

# Impact Analysis

Оцениваются:

* Scope
* Schedule
* Dependencies
* Architecture
* Security
* Testing
* Infrastructure
* Product Value

---

# Rejected Changes

Rejected changes должны иметь documented reason, если они были значимыми.

---

# Versioning

После major change версия Roadmap должна быть обновлена.

---

# Acceptance Criteria

Ни одно существенное изменение Roadmap не выполняется без documented impact assessment.

---

APPROVED
