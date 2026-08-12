# STAIR PLATFORM

Document: 24_DEVELOPMENT_DOCUMENTATION_WORKFLOW.md

ID: DEV-0024

Status: APPROVED

---

# Purpose

Определяет процесс обновления технической документации в ходе разработки.

---

# Principle

Documentation является частью development lifecycle.

---

# Documentation Triggers

Documentation должна обновляться при изменении:

* Architecture
* API
* Database
* Security
* Development Process
* Deployment
* Public Behavior

---

# Workflow

```text id="1vl0hf"
Change
  ↓
Impact Analysis
  ↓
Documentation Update
  ↓
Review
  ↓
Integration
```

---

# Architecture

Architectural changes требуют соответствующих ADR и обновления affected documentation.

---

# API

API changes должны отражаться в API documentation.

---

# Database

Schema changes должны отражаться в database documentation, если они изменяют documented behavior или structure.

---

# Development Process

Изменения development process должны отражаться в соответствующих `22_DEVELOPMENT` documents.

---

# Documentation Review

Documentation проверяется вместе с code change.

---

# Outdated Documentation

Устаревшая documentation должна рассматриваться как defect, если она может привести к неправильной реализации или эксплуатации системы.

---

# Acceptance Criteria

Значимые изменения системы сопровождаются актуализацией соответствующей документации.

---

APPROVED
