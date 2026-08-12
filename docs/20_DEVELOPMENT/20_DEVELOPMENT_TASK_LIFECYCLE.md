# STAIR PLATFORM

Document: 20_DEVELOPMENT_TASK_LIFECYCLE.md

ID: DEV-0020

Status: APPROVED

---

# Purpose

Определяет жизненный цикл Development Task.

---

# Lifecycle

```text id="2h6s3d"
BACKLOG
   ↓
READY
   ↓
IN_PROGRESS
   ↓
IN_REVIEW
   ↓
VALIDATION
   ↓
DONE
```

---

# BACKLOG

Task зарегистрирована, но еще не готова к реализации.

---

# READY

Task имеет:

* Defined Scope
* Acceptance Criteria
* Required Dependencies
* Priority

---

# IN_PROGRESS

Разработка выполняется.

---

# IN_REVIEW

Создан Pull Request и выполняется Code Review.

---

# VALIDATION

Изменение прошло review и проверяется через automated и required manual validation.

---

# DONE

Task соответствует Definition of Done и интегрирована в установленную ветку.

---

# BLOCKED

Task переводится в BLOCKED, если выполнение невозможно из-за:

* Dependency
* Technical Issue
* Missing Requirement
* External Constraint

Причина блокировки должна быть зафиксирована.

---

# CANCELLED

Task может быть отменена только при наличии documented reason.

---

# Reopening

Завершенная task может быть повторно открыта при обнаружении:

* Regression
* Incomplete Scope
* Production Defect
* Failed Acceptance Criteria

---

# Acceptance Criteria

Каждая development task имеет однозначный lifecycle state.

---

APPROVED
