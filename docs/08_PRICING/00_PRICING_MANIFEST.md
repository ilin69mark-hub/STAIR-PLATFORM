# STAIR PLATFORM

Document: 00_PRICING_MANIFEST.md

ID: PRC-0001

Status: APPROVED

---

# Purpose

Pricing Platform определяет единый механизм расчета стоимости изделий, проектов и коммерческих предложений.

Pricing Platform использует исключительно производственные данные, подготовленные Manufacturing Platform, и не выполняет инженерных расчетов.

---

# Scope

Pricing Platform включает:

- Cost Model
- Pricing Rules
- Material Cost
- Labor Cost
- Machine Cost
- Overhead
- Margin
- Discounts
- Tax Engine
- Multi Currency
- Price Validation
- Reporting

---

# Inputs

Manufacturing Platform

Material Library

Supplier Catalogs

Company Pricing Rules

Currency Rates

Tax Rules

---

# Outputs

Production Cost

Sales Price

Commercial Offer

Price Breakdown

Quotation

Pricing Reports

---

# Dependencies

04_ENGINE

05_GEOMETRY

06_MANUFACTURING

---

# Non-Goals

Pricing Platform не выполняет:

- построение геометрии;
- производственные расчеты;
- планирование производства;
- управление складом.

---

# Design Principles

Single Source of Truth

Deterministic Calculations

Version Aware

Rule Based

Auditable

Reproducible

---

# Acceptance Criteria

- единый механизм расчета стоимости;
- воспроизводимость результатов;
- поддержка различных моделей ценообразования.

---

APPROVED