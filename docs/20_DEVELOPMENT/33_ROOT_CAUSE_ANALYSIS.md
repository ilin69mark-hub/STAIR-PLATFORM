# STAIR PLATFORM

Document: 33_ROOT_CAUSE_ANALYSIS.md

ID: DEV-0033

Status: APPROVED

---

# Purpose

Определяет процесс Root Cause Analysis для значимых технических проблем.

---

# When Required

RCA требуется для:

* Critical Production Incidents
* Repeated Failures
* Data Integrity Issues
* Security Incidents
* Major Performance Regressions

---

# RCA Structure

```text id="8e4mzc"
Problem
   ↓
Impact
   ↓
Timeline
   ↓
Root Cause
   ↓
Contributing Factors
   ↓
Corrective Action
   ↓
Preventive Action
```

---

# Problem

Фиксируется точная проблема.

---

# Impact

Определяется влияние на:

* Users
* Data
* Availability
* Performance
* Security
* Business Operations

---

# Timeline

Фиксируются ключевые события.

---

# Root Cause

Определяется underlying cause, а не только непосредственный symptom.

---

# Contributing Factors

Указываются дополнительные условия, которые способствовали проблеме.

---

# Corrective Action

Определяет действия для устранения существующей проблемы.

---

# Preventive Action

Определяет действия для предотвращения повторения.

---

# Verification

После выполнения actions должна быть проверена их эффективность.

---

# Blameless Principle

RCA анализирует систему и процесс, а не ищет персональную вину.

---

# Acceptance Criteria

Для значимых incidents определена root cause и documented corrective/preventive actions.

---

APPROVED
