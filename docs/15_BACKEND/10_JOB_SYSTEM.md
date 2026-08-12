# STAIR PLATFORM

Document: 10_JOB_SYSTEM.md

ID: BE-0010

Status: APPROVED

---

# Purpose

Определяет архитектуру Job System.

Job System используется для операций, которые не должны выполняться непосредственно в рамках пользовательского HTTP-запроса.

---

# Use Cases

Geometry Rebuild

Graph Rebuild

Optimization

CNC Generation

Document Generation

Large Import

Large Export

Price Recalculation

AI Processing

Batch Operations

---

# Job Lifecycle

```text
Created
 ↓
Queued
 ↓
Running
 ↓
Completed
```

Alternative states:

```text
Failed
Cancelled
Retrying
Expired
```

---

# Job Metadata

Job ID

Type

Project ID

User ID

Priority

Status

Created At

Started At

Completed At

Progress

Error

Correlation ID

---

# Rules

Job должен быть идемпотентным либо иметь механизм защиты от повторного выполнения.

HTTP Request не удерживает соединение на протяжении длительной операции.

---

# Acceptance Criteria

Длительные операции выполняются асинхронно и имеют отслеживаемый жизненный цикл.

---

APPROVED