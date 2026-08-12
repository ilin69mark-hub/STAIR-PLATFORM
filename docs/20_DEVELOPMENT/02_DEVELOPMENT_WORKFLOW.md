# STAIR PLATFORM

Document: 02_DEVELOPMENT_WORKFLOW.md

ID: DEV-0002

Status: APPROVED

---

# Purpose

Определяет стандартный workflow разработки.

---

# Workflow

```text
Requirement
    ↓
Task
    ↓
Analysis
    ↓
Implementation
    ↓
Local Validation
    ↓
Tests
    ↓
Pull Request
    ↓
Code Review
    ↓
CI
    ↓
Merge
```

---

# Step 1 — Requirement

Developer должен понимать:

* Что необходимо изменить
* Почему изменение необходимо
* Какой результат ожидается

---

# Step 2 — Task

Создается конкретная development task.

Task должна иметь:

* Scope
* Acceptance Criteria
* Dependencies

---

# Step 3 — Analysis

Перед implementation определяется:

* Existing Behavior
* Affected Components
* Dependencies
* Risks
* Required Tests

---

# Step 4 — Implementation

Изменение реализуется в пределах определенного scope.

---

# Step 5 — Local Validation

До создания Pull Request выполняются необходимые локальные проверки.

---

# Step 6 — Tests

Добавляются или обновляются tests, соответствующие изменению.

---

# Step 7 — Pull Request

Изменение отправляется на review.

---

# Step 8 — Code Review

Проверяются:

* Correctness
* Architecture
* Security
* Tests
* Maintainability

---

# Step 9 — CI

CI выполняет автоматические quality checks.

---

# Step 10 — Merge

Изменение интегрируется только после прохождения обязательных gates.

---

# Acceptance Criteria

Каждое значимое изменение проходит установленный development workflow.

---

APPROVED
