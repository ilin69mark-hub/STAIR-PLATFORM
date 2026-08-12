# STAIR PLATFORM

Document: 01_DOMAIN_MODEL.md

ID: DOM-0001

Status: APPROVED

---

# Purpose

Domain Model определяет концептуальную модель предметной области STAIR Platform.

Документ описывает основные доменные сущности, их взаимосвязи и жизненный цикл без привязки к реализации.

Domain Model является основой для проектирования:

- Architecture;
- Database;
- API;
- AI;
- Manufacturing;
- Pricing.

---

# Objectives

- единая модель предметной области;
- единая терминология;
- единые правила взаимодействия;
- отсутствие противоречий между платформами.

---

# Core Domain

STAIR Platform предназначена для проектирования, расчёта, производства и сопровождения инженерных изделий.

Основными доменными объектами являются:

Project

Assembly

Part

Feature

Sketch

Geometry

Material

Operation

Machine

Manufacturing Plan

Price Calculation

Quotation

Document

Revision

Installation

Maintenance

---

# Domain Hierarchy

Project

↓

Assembly

↓

Part

↓

Feature

↓

Geometry

↓

Manufacturing

↓

Pricing

↓

Documentation

↓

Installation

↓

Maintenance

---

# Domain Principles

- Project является корневой сущностью.
- Каждая сущность имеет владельца.
- Все изменения проходят через Revision.
- Все операции порождают Domain Events.
- Все вычисления воспроизводимы.

---

# Ubiquitous Language

Термины, используемые в документации и коде, должны совпадать с терминами Domain Glossary.

---

# Acceptance Criteria

- определены основные сущности;
- определены отношения между сущностями;
- отсутствуют неоднозначные трактовки.

---

APPROVED