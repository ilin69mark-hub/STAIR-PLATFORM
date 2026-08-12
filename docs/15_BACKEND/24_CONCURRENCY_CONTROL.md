# STAIR PLATFORM

Document: 24_CONCURRENCY_CONTROL.md

ID: BE-0024

Status: APPROVED

---

# Purpose

Определяет управление конкурентными изменениями.

---

# Problems

Concurrent Updates

Lost Updates

Race Conditions

Conflicting Jobs

Graph Rebuild Conflicts

Geometry Update Conflicts

---

# Strategies

Optimistic Locking

Pessimistic Locking

Version Checking

Distributed Locks

Queue Serialization

---

# Default Strategy

Optimistic Concurrency Control.

Каждый изменяемый Aggregate имеет Version.

---

# Flow

```text
Read Version N
      ↓
Modify
      ↓
Save if Version == N
      ↓
Version N+1
```

При несовпадении:

```text
Conflict
```

---

# Long Running Jobs

Job проверяет актуальность Project Version перед применением результата.

---

# Rules

Lock используется только при необходимости.

Distributed Lock не заменяет Domain Consistency.

---

# Acceptance Criteria

Конкурирующие операции не приводят к незаметной потере изменений.

---

APPROVED