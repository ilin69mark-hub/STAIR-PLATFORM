# STAIR PLATFORM

**Document:** ADR-0001_Global_Identification_System.md

**ADR ID:** ADR-0001

**Title:** Global Identification System

**Status:** ACCEPTED

**Version:** 1.0.0

**Date:** 2026-08-04

**Decision Makers:**

- Project Architect

---

# 1. Context

По мере роста Stair Platform количество архитектурных артефактов будет постоянно увеличиваться.

Проект включает:

- документацию;
- требования;
- ограничения;
- архитектурные решения;
- инженерные алгоритмы;
- модели данных;
- API;
- UML;
- тесты;
- производственные правила.

Использование локальных имен без единой системы идентификации приведет к:

- неоднозначности;
- дублированию;
- невозможности автоматической трассировки;
- усложнению сопровождения.

Необходим единый стандарт идентификаторов.

---

# 2. Problem

В проекте отсутствует единая система идентификации архитектурных объектов.

Это затрудняет:

- поиск;
- анализ зависимостей;
- автоматическую генерацию документации;
- анализ влияния изменений;
- связь требований с кодом.

---

# 3. Decision

Во всей Stair Platform вводится единая система идентификаторов.

Каждый архитектурный объект обязан иметь уникальный ID.

ID никогда не изменяется.

ID никогда не используется повторно.

---

# 4. Identifier Format

Общий формат:

```
PREFIX-NNNN
```

Примеры

```
FR-0001
ADR-0007
API-0015
DB-0023
TEST-0048
```

---

# 5. Registered Prefixes

## Documentation

```
DOC
```

---

## Architecture

```
ADR
```

---

## Functional Requirement

```
FR
```

---

## Non Functional Requirement

```
NFR
```

---

## Constraint

```
CON
```

---

## Decision

```
DEC
```

---

## Risk

```
RISK
```

---

## Product

```
PRD
```

---

## Domain

```
DOM
```

---

## Entity

```
ENT
```

---

## Value Object

```
VAL
```

---

## Domain Service

```
SRV
```

---

## Domain Event

```
EVT
```

---

## Engineering Algorithm

```
ALG
```

---

## Engineering Rule

```
RULE
```

---

## Formula

```
FORM
```

---

## Constraint Rule

```
CONS
```

---

## API

```
API
```

---

## DTO

```
DTO
```

---

## Database Table

```
DB
```

---

## Database Index

```
IDX
```

---

## UML

```
UML
```

---

## ER Diagram

```
ERD
```

---

## Test

```
TEST
```

---

## Manufacturing

```
MFG
```

---

## CNC

```
CNC
```

---

## Drawing

```
DRW
```

---

## BOM

```
BOM
```

---

# 6. Rules

Каждый ID:

- уникален;
- неизменяем;
- создается один раз;
- используется во всех ссылках;
- участвует в Traceability.

---

# 7. Traceability Example

```
FR-0012
        │
        ▼
ADR-0008
        │
        ▼
DOM-0005
        │
        ▼
ENT-0003
        │
        ▼
ALG-0017
        │
        ▼
API-0008
        │
        ▼
DB-0004
        │
        ▼
TEST-0021
```

---

# 8. Consequences

Преимущества:

- полная трассируемость;
- автоматизация документации;
- анализ влияния изменений;
- автоматическая генерация отчетов;
- поддержка больших проектов.

Недостатки:

- требуется дисциплина при создании новых артефактов;
- необходим централизованный реестр идентификаторов.

---

# 9. Alternatives Considered

## UUID

Отклонено.

Причина:

нечитаемо человеком.

---

## Свободные имена

Отклонено.

Причина:

невозможность автоматизации.

---

## Локальные номера

Отклонено.

Причина:

коллизии между разделами.

---

# 10. Implementation

После утверждения ADR:

- все новые документы используют новую систему;
- существующие документы постепенно приводятся к новому формату;
- автоматические проверки становятся частью CI/CD.

---

# 11. Related Documents

- PROJECT_MANIFEST
- DOCUMENTATION_RULES
- TRACEABILITY_AUDIT
- DECISION_LOG

---

# 12. Status History

| Version | Status | Date |
|----------|--------|------|
|1.0.0|ACCEPTED|2026-08-04|

---

# 13. Approval

Architecture Decision Record принят.

Все новые артефакты Stair Platform обязаны использовать Global Identification System.