# STAIR PLATFORM

Document: 08_COMMIT_CONVENTIONS.md

ID: DEV-0008

Status: APPROVED

---

# Purpose

Определяет правила оформления commit сообщений.

---

# Format

Используется структура:

```text
<type>: <short description>
```

---

# Types

* feat
* fix
* refactor
* perf
* test
* docs
* chore
* ci
* build

---

# Examples

```text
feat: add geometry validation
fix: correct stair width calculation
refactor: simplify constraint solver
test: add pricing regression suite
docs: update API specification
```

---

# Rules

* первая строка краткая
* повелительное наклонение
* без точки в конце
* один commit — одна логическая идея

---

# Breaking Changes

Breaking behavior должен быть явно указан в описании commit и сопровождаться соответствующей документацией.

---

# Squash Policy

Перед merge допускается squash связанных рабочих commit'ов в одну логическую историю.

---

# Traceability

Commit должен быть связан с:

* Task
* Issue
* Requirement
* ADR where applicable

---

# Acceptance Criteria

История репозитория остается читаемой и отражает логические изменения системы.

---

APPROVED
