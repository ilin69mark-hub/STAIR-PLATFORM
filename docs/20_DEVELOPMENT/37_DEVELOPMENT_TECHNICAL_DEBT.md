# STAIR PLATFORM

Document: 37_DEVELOPMENT_TECHNICAL_DEBT.md

ID: DEV-0037

Status: APPROVED

---

# Purpose

Определяет правила управления Technical Debt.

---

# Definition

Technical Debt — осознанное или накопившееся техническое ограничение, которое увеличивает будущую стоимость изменения системы.

---

# Sources

Technical Debt может возникнуть из-за:

* Temporary Solutions
* Legacy Code
* Architectural Constraints
* Performance Limitations
* Missing Tests
* Outdated Dependencies
* Incomplete Refactoring

---

# Registration

Значимая Technical Debt должна быть зарегистрирована как отдельная item.

---

# Required Information

Каждая item должна содержать:

* Description
* Reason
* Impact
* Risk
* Priority
* Proposed Resolution

---

# Priority

Приоритет определяется по:

```text id="d9s3e2"
Impact × Risk × Cost
```

---

# Critical Debt

Critical Technical Debt должна иметь planned resolution.

---

# Development Rule

Новая Technical Debt не должна создаваться случайно.

Если временное решение необходимо, оно должно быть documented.

---

# Debt Reduction

Technical Debt должна учитываться при planning новых phases и releases.

---

# Acceptance Criteria

Значимая Technical Debt видима, классифицирована и имеет понятный план управления.

---

APPROVED
