# STAIR PLATFORM

Document: 32_DEVELOPMENT_INCIDENT_WORKFLOW.md

ID: DEV-0032

Status: APPROVED

---

# Purpose

Определяет workflow работы с incidents, обнаруженными во время Development или Release.

---

# Incident Lifecycle

```text id="x0p3v7"
Detected
   ↓
Classified
   ↓
Contained
   ↓
Investigated
   ↓
Resolved
   ↓
Validated
   ↓
Closed
```

---

# Detection

Incident может быть обнаружен через:

* CI
* Tests
* Monitoring
* Code Review
* Staging
* Production

---

# Classification

Определяются:

* Severity
* Impact
* Affected Component
* Environment

---

# Containment

Цель containment — остановить дальнейшее распространение проблемы.

---

# Investigation

Определяется:

* Root Cause
* Trigger
* Affected Scope
* Contributing Factors

---

# Resolution

Исправление должно быть:

* Tested
* Reviewed
* Deployable
* Traceable

---

# Validation

После исправления проверяются affected workflows.

---

# Closure

Incident закрывается после подтверждения resolution.

---

# Critical Incidents

Critical production incidents требуют:

* Immediate response
* Root Cause Analysis
* Corrective Action
* Prevention Action

---

# Documentation

Significant incidents должны быть документированы.

---

# Acceptance Criteria

Incidents проходят определенный lifecycle от detection до validated resolution.

---

APPROVED
