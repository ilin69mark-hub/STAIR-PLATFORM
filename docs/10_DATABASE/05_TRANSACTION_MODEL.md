# STAIR PLATFORM

Document: 05_TRANSACTION_MODEL.md

ID: DB-0005

Status: APPROVED

---

# Purpose

Определяет модель транзакций STAIR Platform.

Transaction Model обеспечивает атомарность, согласованность и воспроизводимость изменений инженерных данных.

Все изменения Domain выполняются только внутри транзакции.

---

# Objectives

- ACID compliance;
- целостность данных;
- защита от гонок;
- поддержка Revision Model;
- поддержка Domain Events.

---

# Transaction Scope

Каждая транзакция изменяет только один Aggregate Root.

Взаимодействие между Aggregate выполняется через:

- Domain Events;
- Application Services;
- Workflow Engine.

---

# Transaction Lifecycle

Transaction Started

↓

Validation

↓

Repository Load

↓

Invariant Validation

↓

Domain Execution

↓

Revision Creation

↓

Domain Event Collection

↓

Persistence

↓

Commit

↓

Event Publishing

---

# Isolation Level

По умолчанию используется:

Repeatable Read

Допускается:

Serializable

Read Committed

только после Architecture Review.

---

# Rules

Transaction не должна обращаться к UI.

Transaction не должна выполнять HTTP-запросы.

Transaction не должна обращаться к AI.

Transaction не должна выполнять длительные вычисления.

---

# Retry Policy

Deadlock Retry

Optimistic Lock Retry

Network Retry

---

# Rollback Rules

Rollback выполняется при:

- нарушении Invariants;
- ошибке Repository;
- конфликте Revision;
- нарушении Constraints.

---

# Acceptance Criteria

Все изменения выполняются внутри транзакций.

Rollback полностью восстанавливает состояние.

---

APPROVED