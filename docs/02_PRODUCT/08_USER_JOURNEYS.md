# STAIR PLATFORM

**Document:** 08_USER_JOURNEYS.md

**Document ID:** PROD-0009

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ описывает основные пользовательские сценарии (User Journeys) Stair Platform.

Документ используется для:

- проектирования UX;
- определения бизнес-процессов;
- построения Use Cases;
- проектирования API;
- проверки полноты функциональности.

Каждый сценарий описывает путь пользователя от достижения цели.

---

# 2. Journey Design Principles

Все сценарии должны:

- иметь конкретную бизнес-цель;
- состоять из последовательных шагов;
- учитывать исключительные ситуации;
- быть независимыми от интерфейса;
- завершаться измеримым результатом.

---

# 3. Primary Journey

## Создание инженерного проекта

```
Login
   │
   ▼
Select Organization
   │
   ▼
Select Workspace
   │
   ▼
Create Project
   │
   ▼
Configure Stair Parameters
   │
   ▼
Generate Geometry
   │
   ▼
Run Solver
   │
   ▼
Validate
   │
   ▼
Calculate Price
   │
   ▼
Generate Documents
   │
   ▼
Render Preview
   │
   ▼
Create Quote
   │
   ▼
Create Order
```

---

# 4. Engineering Journey

```
Open Project

↓

Edit Geometry

↓

Validate Constraints

↓

Run Solver

↓

Review Results

↓

Save Revision
```

---

# 5. Manufacturing Journey

```
Approved Project

↓

Generate BOM

↓

Generate Drawings

↓

Generate CNC Files

↓

Export Manufacturing Package
```

---

# 6. Sales Journey

```
Customer Request

↓

Project Creation

↓

Engineering Calculation

↓

Pricing

↓

Commercial Proposal

↓

Customer Approval

↓

Order
```

---

# 7. Customer Journey

```
Invitation

↓

View Proposal

↓

Review Render

↓

Approve Changes

↓

Confirm Order

↓

Track Progress
```

---

# 8. Administration Journey

```
Create Organization

↓

Assign License

↓

Invite Users

↓

Configure Roles

↓

Monitor Activity
```

---

# 9. AI Journey

```
Open Project

↓

Ask AI Assistant

↓

Analyze Geometry

↓

Receive Recommendations

↓

Apply Changes

↓

Recalculate
```

---

# 10. Common Journey States

Каждый сценарий проходит состояния:

```
Started

↓

In Progress

↓

Waiting

↓

Completed
```

Возможные дополнительные состояния:

- Cancelled;
- Failed;
- Suspended;
- Archived.

---

# 11. Success Criteria

Каждый User Journey должен:

- завершаться достижением цели;
- иметь измеримый результат;
- поддерживать восстановление после ошибок;
- фиксироваться в журнале аудита.

---

# 12. Dependencies

Incoming

- USER_ROLES
- AUTHORIZATION_MODEL

Outgoing

- USE_CASES
- UX
- FRONTEND
- API

---

# 13. Acceptance Criteria

Документ считается завершенным, если:

- определены основные сценарии;
- определены последовательности действий;
- определены состояния;
- определены критерии успешности.

---

# 14. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 15. Approval

APPROVED