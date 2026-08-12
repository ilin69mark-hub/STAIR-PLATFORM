# ADR-0015

Title: Engineering Domain Model (EDM)

Status: ACCEPTED

Date: 2026-08-06

---

# Context

Классическая Domain-Driven Design (DDD) хорошо описывает бизнес-домены, однако недостаточно полно отражает особенности инженерных CAD/CAM/CAE платформ.

STAIR Platform проектируется как инженерная вычислительная система, где ключевыми объектами являются параметрические модели, геометрия, ограничения, граф зависимостей, производственные данные и инженерные ревизии.

Использование только классических Entity, Aggregate и Value Object приводит к размытию инженерной модели.

---

# Decision

Ввести Engineering Domain Model (EDM) как расширение классического DDD.

DDD остается базовой методологией.

EDM добавляет инженерные концепции.

---

# Engineering Concepts

Platform вводит следующие первичные понятия:

Engineering Object

Engineering Revision

Parameter

Constraint

Dependency

Calculation

Geometry

Manufacturing Object

Engineering State

Engineering Workflow

Engineering Metadata

Engineering History

Engineering Snapshot

Engineering Graph

Engineering Event

---

# Principles

Каждый инженерный объект имеет:

- UUID;
- Revision;
- Lifecycle;
- Parameters;
- Constraints;
- Metadata;
- Events;
- History.

Каждый инженерный объект воспроизводим.

Все вычисления являются детерминированными.

Все изменения фиксируются.

---

# Consequences

Расширяются документы Domain.

Расширяется Database.

Расширяется Storage.

Расширяется Graph.

Расширяется AI.

Расширяется Manufacturing.

---

# Related Documents

03_DOMAIN

04_ENGINE

05_GEOMETRY

09_DATABASE

10_STORAGE

17_AI

---

Accepted