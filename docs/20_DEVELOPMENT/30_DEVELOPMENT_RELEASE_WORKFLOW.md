# STAIR PLATFORM

Document: 30_DEVELOPMENT_RELEASE_WORKFLOW.md

ID: DEV-0030

Status: APPROVED

---

# Purpose

Определяет workflow подготовки Development Release.

---

# Release Flow

```text id="l8dy6n"
Feature Complete
      ↓
Code Freeze
      ↓
Regression
      ↓
Build
      ↓
Release Candidate
      ↓
Validation
      ↓
Approval
      ↓
Release
```

---

# Feature Complete

Все features, включенные в release, должны соответствовать Definition of Done.

---

# Code Freeze

После Code Freeze допускаются только изменения, необходимые для стабилизации release.

---

# Regression

Выполняется соответствующий regression suite.

---

# Build

Создается versioned build artifact.

---

# Release Candidate

Release Candidate должен быть deployable без дополнительных code changes.

---

# Validation

Проверяются:

* Functional Behavior
* API
* Database
* Security
* Performance where applicable
* Infrastructure

---

# Approval

Release получает approval после прохождения обязательных Quality Gates.

---

# Release

Release должен быть traceable к:

* Version
* Commit
* Build Artifact
* Database Migration
* Documentation

---

# Post-Release

После release выполняются:

* Health Check
* Smoke Test
* Monitoring
* Error Review

---

# Acceptance Criteria

Каждый Development Release проходит воспроизводимый release workflow и имеет traceable artifact.

---

APPROVED
