# STAIR PLATFORM

**Document:** ADR-0005_Document_Dependency_Graph.md

**ADR ID:** ADR-0005

**Title:** Document Dependency Graph

**Status:** ACCEPTED

**Version:** 1.0.0

**Date:** 2026-08-04

**Decision Makers:**

- Project Architect

---

# 1. Context

Stair Platform включает сотни взаимосвязанных документов.

К ним относятся:

- Foundation;
- Review;
- Product;
- Domain;
- Engine;
- Database;
- API;
- AI;
- Manufacturing;
- ADR;
- EDR;
- EKB;
- UML;
- ERD;
- Requirements;
- Tests.

По мере роста проекта ручной контроль связей становится невозможным.

Необходима единая модель зависимостей документации.

---

# 2. Problem

При отсутствии графа зависимостей возникают:

- потеря связей между документами;
- устаревшая документация;
- ошибки при изменениях;
- невозможность анализа влияния;
- сложность навигации.

---

# 3. Decision

Во всей Stair Platform документация рассматривается как ориентированный граф.

Каждый документ является вершиной графа.

Каждая зависимость является направленным ребром.

Все зависимости должны быть явно описаны.

---

# 4. Graph Model

```
Document
     │
     ▼
Dependencies
     │
     ▼
Related Documents
     │
     ▼
Requirements
     │
     ▼
Implementation
```

Каждый документ обязан содержать раздел:

```
Dependencies
```

---

# 5. Типы зависимостей

## Incoming

Документы, от которых зависит текущий документ.

---

## Outgoing

Документы, которые используют текущий документ.

---

## Reference

Логические ссылки.

---

## Traceability

Связи требований.

---

## Architecture

Архитектурные зависимости.

---

## Engineering

Инженерные зависимости.

---

# 6. Правила

Каждый документ обязан:

- иметь уникальный идентификатор;
- иметь список входящих зависимостей;
- иметь список исходящих зависимостей;
- не иметь циклических зависимостей;
- иметь владельца.

---

# 7. Dependency Levels

Определяются уровни зависимостей.

```
L0

Foundation

↓

L1

Review

↓

L2

ADR

↓

L3

Product

↓

L4

Domain

↓

L5

Engineering

↓

L6

Database

↓

L7

API

↓

L8

Application

↓

L9

AI

↓

L10

Manufacturing

↓

L11

Testing

↓

L12

Deployment
```

Зависимости допускаются только сверху вниз.

---

# 8. Dependency Validation

Во время проверки автоматически анализируются:

- отсутствующие ссылки;
- циклические зависимости;
- недостижимые документы;
- устаревшие ссылки;
- разорванные цепочки.

---

# 9. Graph Usage

Dependency Graph используется для:

- Architecture Review;
- Traceability Audit;
- Impact Analysis;
- генерации документации;
- построения Roadmap;
- оценки изменений;
- AI-навигации по проекту.

---

# 10. Automation

Граф должен автоматически строиться средствами CI/CD.

После каждого Pull Request выполняется:

- построение графа;
- поиск циклов;
- поиск отсутствующих ссылок;
- проверка целостности.

При обнаружении критических ошибок сборка завершается с ошибкой.

---

# 11. Consequences

Преимущества:

- единая карта документации;
- автоматическая навигация;
- анализ влияния изменений;
- поддержка AI;
- контроль архитектурной целостности.

Недостатки:

- увеличение количества метаданных;
- необходимость поддержки графа.

---

# 12. Alternatives Considered

## Ручная навигация

Отклонено.

Причина:

не масштабируется.

---

## Wiki

Отклонено.

Причина:

не поддерживает формальный граф зависимостей.

---

## Только Markdown-ссылки

Отклонено.

Причина:

не позволяют выполнять автоматический анализ.

---

# 13. Implementation

После принятия ADR:

- каждый документ содержит раздел Dependencies;
- создается Graph Builder;
- создается Dependency Validator;
- граф становится частью CI/CD;
- результаты анализа сохраняются в Architecture Report.

---

# 14. Related Documents

- ADR-0001 Global Identification System
- ADR-0002 Traceability Model
- ADR-0003 Engineering Decision Records
- ADR-0004 Engineering Knowledge Base
- TRACEABILITY_AUDIT
- DEPENDENCY_ANALYSIS

---

# 15. Status History

| Version | Status | Date |
|----------|--------|------|
| 1.0.0 | ACCEPTED | 2026-08-04 |

---

# 16. Approval

Architecture Decision Record утвержден.

Document Dependency Graph становится обязательной архитектурной моделью Stair Platform.