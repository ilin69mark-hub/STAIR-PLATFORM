# STAIR PLATFORM

**Document:** ADR-0006_Layered_Architecture_Model.md

**ADR ID:** ADR-0006

**Title:** Layered Architecture Model

**Status:** ACCEPTED

**Version:** 1.0.0

**Date:** 2026-08-04

**Decision Makers:**

- Project Architect

---

# 1. Context

Stair Platform является долгоживущей инженерной платформой.

Проект включает множество независимых подсистем:

- Product;
- Domain;
- Engineering;
- Database;
- API;
- Application;
- AI;
- Manufacturing.

Без единой архитектурной модели неизбежно возникают:

- нарушение границ модулей;
- циклические зависимости;
- высокая связанность;
- сложность сопровождения;
- деградация архитектуры.

Необходимо определить единую модель архитектурных слоев.

---

# 2. Problem

Отсутствие формальной модели приводит к:

- случайным зависимостям;
- смешению бизнес-логики и инфраструктуры;
- невозможности автоматической проверки архитектуры;
- росту технического долга.

---

# 3. Decision

В Stair Platform вводится официальная модель архитектурных слоев.

Каждый компонент системы принадлежит только одному слою.

Взаимодействие между слоями допускается исключительно согласно правилам настоящего ADR.

---

# 4. Архитектурные слои

```
L0 Foundation

↓

L1 Product

↓

L2 Domain

↓

L3 Engineering

↓

L4 Application

↓

L5 Interfaces

↓

L6 Infrastructure
```

---

# 5. Назначение слоев

## L0 — Foundation

Определяет архитектурные принципы, стандарты, ADR, документацию и процессы.

---

## L1 — Product

Содержит:

- бизнес-модель;
- пользовательские роли;
- требования;
- сценарии использования;
- бизнес-правила.

---

## L2 — Domain

Содержит:

- Entities;
- Value Objects;
- Aggregates;
- Domain Services;
- Domain Events.

Не содержит инфраструктурного кода.

---

## L3 — Engineering

Содержит инженерное ядро платформы:

- Geometry Engine;
- Solver;
- Constraint Engine;
- Validation Engine;
- Calculation Engine.

Инженерное ядро полностью независимо от UI, HTTP, базы данных и AI.

---

## L4 — Application

Реализует:

- Use Cases;
- Commands;
- Queries;
- Workflow;
- Orchestration.

Не содержит инженерных алгоритмов.

---

## L5 — Interfaces

Содержит:

- REST API;
- Web UI;
- CLI;
- AI Gateway;
- WebSocket.

Не содержит бизнес-логики.

---

## L6 — Infrastructure

Содержит:

- PostgreSQL;
- Redis;
- S3;
- Docker;
- Nginx;
- файловую систему;
- внешние сервисы.

---

# 6. Правила зависимостей

Допускаются зависимости только сверху вниз.

```
Interfaces
      ↓
Application
      ↓
Domain
      ↓
Engineering
      ↓
Infrastructure
```

Обратные зависимости запрещены.

---

# 7. Запрещенные зависимости

Запрещается:

- UI → Database;
- UI → Solver;
- API → Database;
- Domain → HTTP;
- Domain → PostgreSQL;
- Engineering → UI;
- Engineering → REST;
- Engineering → AI;
- AI → Database.

---

# 8. Контроль архитектуры

Во время CI/CD автоматически проверяются:

- принадлежность слоям;
- нарушение границ;
- циклические зависимости;
- запрещенные импорты;
- нарушения настоящего ADR.

---

# 9. Consequences

Преимущества:

- строгие архитектурные границы;
- независимость инженерного ядра;
- простота тестирования;
- возможность масштабирования;
- возможность выделения микросервисов в будущем.

Недостатки:

- требуется дисциплина разработки;
- увеличивается количество интерфейсов между слоями.

---

# 10. Alternatives Considered

## Three-Layer Architecture

Отклонено.

Недостаточно для инженерной платформы.

---

## Microservices с первого дня

Отклонено.

Преждевременное усложнение.

---

## Без формальных слоев

Отклонено.

Высокий риск архитектурной деградации.

---

# 11. Implementation

После принятия ADR:

- каждый модуль относится к одному слою;
- структура каталогов отражает архитектурные слои;
- CI/CD выполняет проверку зависимостей;
- новые компоненты проходят Architecture Review.

---

# 12. Related Documents

- ADR-0001 Global Identification System
- ADR-0002 Traceability Model
- ADR-0005 Document Dependency Graph
- ARCHITECTURAL_PRINCIPLES
- DEPENDENCY_ANALYSIS

---

# 13. Status History

| Version | Status | Date |
|----------|--------|------|
| 1.0.0 | ACCEPTED | 2026-08-04 |

---

# 14. Approval

Architecture Decision Record утвержден.

Layered Architecture Model становится официальной архитектурной моделью Stair Platform.