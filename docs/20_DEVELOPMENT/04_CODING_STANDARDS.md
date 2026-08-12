# STAIR PLATFORM

Document: 04_CODING_STANDARDS.md

ID: DEV-0004

Status: APPROVED

---

# Purpose

Определяет базовые coding standards STAIR PLATFORM.

---

# General Principles

Code должен быть:

* Readable
* Explicit
* Testable
* Maintainable
* Consistent

---

# Naming

Names должны отражать responsibility и intent.

Не допускаются meaningless names в production code, кроме общепринятых short-lived variables.

---

# Functions

Functions должны иметь одну основную responsibility.

Слишком большие functions должны быть разделены.

---

# Error Handling

Ошибки должны:

* проверяться
* сохранять необходимый context
* корректно передаваться выше
* не игнорироваться без documented reason

---

# Logging

Production code не должен использовать ad-hoc output для operational logging.

Используется установленный logging mechanism.

---

# Configuration

Configuration не должна быть hardcoded, если она зависит от environment или deployment.

---

# Secrets

Secrets запрещено хранить:

* в source code
* в configuration files
* в tests
* в logs
* в repository history

---

# Comments

Комментарии должны объяснять:

* Why
* Constraint
* Non-obvious Decision

Комментарии не должны просто повторять код.

---

# Tests

Изменение business-critical behavior должно сопровождаться соответствующими tests.

---

# Dependencies

Новые dependencies должны иметь:

* Justification
* Version
* Security Assessment where required
* Maintenance Consideration

---

# Formatting

Автоматические formatters должны использоваться там, где они определены для соответствующего языка.

---

# Static Analysis

Code должен проходить configured static analysis tools.

---

# Acceptance Criteria

Production code соответствует установленным coding standards и проходит automated formatting и static analysis.

---

APPROVED
