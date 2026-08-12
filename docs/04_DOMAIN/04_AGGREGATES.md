# STAIR PLATFORM

Document: 04_AGGREGATES.md

ID: DOM-0002

Status: APPROVED

---

# Purpose

Документ определяет агрегаты предметной области в соответствии с принципами Domain-Driven Design (DDD).

---

# Aggregate Principles

- Aggregate имеет единственный Aggregate Root.
- Изменения агрегата выполняются только через Root.
- Инварианты гарантируются Aggregate Root.
- Внешний доступ к внутренним объектам агрегата запрещён.

---

# Aggregate Roots

Project

Assembly

Part

Quotation

Manufacturing Plan

Installation

Maintenance

---

# Aggregate Structure

Project
├── Revisions
├── Assemblies
├── Documents
└── Metadata

Assembly
├── Parts
├── Features
└── Constraints

Quotation
├── Items
├── Discounts
└── Taxes

---

# Transaction Boundary

Одна транзакция изменяет только один Aggregate.

Взаимодействие между агрегатами осуществляется через Domain Events или Application Services.

---

# Acceptance Criteria

- определены Aggregate Root;
- определены границы транзакций;
- инварианты агрегатов документированы.

---

APPROVED