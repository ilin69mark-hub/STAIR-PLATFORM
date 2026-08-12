# STAIR PLATFORM

**Document:** 17_PROJECT_ROADMAP.md

**Document ID:** FOUNDATION-018

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Назначение

Настоящий документ определяет стратегический план разработки Stair Platform.

Roadmap описывает последовательность создания платформы, контрольные точки, зависимости между этапами и критерии перехода.

Roadmap является обязательным для всех участников проекта.

---

# 2. Цель

Документ обеспечивает:

- последовательную разработку;
- управляемость проекта;
- прогнозируемость сроков;
- контроль зависимостей;
- снижение архитектурных рисков.

---

# 3. Принципы Roadmap

Разработка выполняется поэтапно.

Каждый этап начинается только после успешного завершения предыдущего.

Переход между этапами допускается только после прохождения Architecture Review.

Изменение порядка этапов запрещено без утвержденного ADR.

---

# 4. Основные этапы проекта

## Phase 00

FOUNDATION

Формирование архитектурного фундамента проекта.

Статус:

Completed

---

## Phase 01

PRODUCT

Описание продукта.

Результат:

- Product Vision
- User Roles
- User Stories
- Business Rules
- Product Scope

---

## Phase 02

DOMAIN

Проектирование предметной области.

Результат:

- Domain Model
- Bounded Context
- Engineering Dictionary
- Constraint Catalog

---

## Phase 03

ENGINE

Проектирование инженерного ядра.

Результат:

- Geometry Engine
- Solver
- Constraint Engine
- Validation Engine
- Calculation Engine

---

## Phase 04

DATABASE

Проектирование хранения данных.

Результат:

- ERD
- Schema
- Migrations
- Indexes

---

## Phase 05

API

Проектирование контрактов.

Результат:

- REST API
- OpenAPI
- DTO
- Error Model
- Versioning

---

## Phase 06

APPLICATION

Разработка прикладной логики.

Результат:

- Backend
- Frontend
- Authentication
- Authorization
- Project Management

---

## Phase 07

AI

Интеграция интеллектуальных компонентов.

Результат:

- AI Assistant
- Recommendation Engine
- AI Validation
- AI Explanation
- AI Knowledge

---

## Phase 08

MANUFACTURING

Подготовка к производству.

Результат:

- BOM
- CNC Export
- Drawings
- Manufacturing Rules

---

## Phase 09

TESTING

Комплексное тестирование.

Результат:

- Unit Tests
- Integration Tests
- Geometry Tests
- Regression Tests
- Performance Tests

---

## Phase 10

DEPLOYMENT

Развертывание.

Результат:

- CI/CD
- Docker
- Monitoring
- Logging
- Backup

---

## Phase 11

RELEASE

Подготовка промышленной версии.

Результат:

- Release Candidate
- Production Release
- Documentation Review
- Architecture Review

---

# 5. Контрольные точки (Milestones)

## M1

Foundation Approved

---

## M2

Domain Approved

---

## M3

Engineering Approved

---

## M4

Database Approved

---

## M5

API Approved

---

## M6

Application Ready

---

## M7

AI Ready

---

## M8

Manufacturing Ready

---

## M9

Testing Complete

---

## M10

Production Release

---

# 6. Критерии перехода между этапами

Для перехода к следующему этапу должны быть выполнены:

- завершены все документы текущего этапа;
- выполнены Acceptance Criteria;
- отсутствуют блокирующие риски;
- утверждены необходимые ADR;
- успешно пройден Architecture Review.

---

# 7. Зависимости

```
FOUNDATION
      ↓
PRODUCT
      ↓
DOMAIN
      ↓
ENGINE
      ↓
DATABASE
      ↓
API
      ↓
APPLICATION
      ↓
AI
      ↓
MANUFACTURING
      ↓
TESTING
      ↓
DEPLOYMENT
      ↓
RELEASE
```

---

# 8. Управление изменениями

Изменение Roadmap допускается только:

- после анализа влияния;
- после оценки рисков;
- после обновления зависимостей;
- посредством утвержденного ADR.

---

# 9. Управление версиями

Каждый завершенный этап фиксируется отдельной версией документации.

После завершения этапа выполняется Architecture Baseline Review.

---

# 10. Dependencies

## Входящие документы

- 00_PROJECT_MANIFEST.md
- 05_DEVELOPMENT_PROCESS.md
- 06_ADR_PROCESS.md
- 10_SUCCESS_CRITERIA.md
- 12_RISK_REGISTER.md

## Исходящие документы

Все последующие разделы STAIR-DOC.

---

# 11. Acceptance Criteria

Документ считается завершенным, если:

- определены все этапы проекта;
- определены контрольные точки;
- определены критерии перехода;
- определены зависимости;
- определены правила изменения Roadmap.

---

# 12. История изменений

| Версия | Дата | Изменение |
|---------|------|-----------|
| 1.0.0 | 2026-08-04 | Первоначальная редакция |

---

# 13. Утверждение

Статус:

APPROVED

Изменение настоящего документа допускается исключительно посредством Architecture Decision Record (ADR).