# STAIR PLATFORM

Document: 05_REPOSITORIES.md

ID: BE-0005

Status: APPROVED

---

# Purpose

Определяет архитектуру Repository Layer Backend.

Repository предоставляет Application и Domain слоям абстракцию доступа к данным.

---

# Responsibilities

Load Entity

Save Entity

Delete Entity

Find Entity

Query Data

Check Existence

Manage Persistence Boundary

---

# Architecture

```text
Application
     ↓
Repository Interface
     ↓
Repository Implementation
     ↓
Database / Storage
```

---

# Repository Types

Project Repository

User Repository

Geometry Repository

Graph Repository

Manufacturing Repository

Pricing Repository

Document Repository

AI Repository

---

# Rules

Domain и Application не зависят от конкретной Database implementation.

Repository Interface определяется внутренним слоем.

Infrastructure реализует Repository Interface.

---

# Query Rules

Repositories не должны содержать бизнес-правила.

Сложные read-запросы могут использовать специализированные Read Repositories.

---

# Transaction Context

Repository должен поддерживать выполнение операций внутри переданной транзакции.

---

# Acceptance Criteria

Замена persistence implementation не требует изменения Domain Layer.

---

APPROVED