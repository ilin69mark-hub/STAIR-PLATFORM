# STAIR PLATFORM

Document: 10_BOM_ENGINE.md

ID: MFG-0011

Status: APPROVED

---

# Purpose

BOM Engine автоматически формирует спецификацию изделия на основе инженерной модели и производственных данных.

---

# Objectives

- единый источник спецификации;
- автоматическая генерация;
- поддержка ревизий;
- экспорт в ERP/MES.

---

# BOM Structure

Assembly

↓

Subassembly

↓

Part

↓

Fastener

↓

Purchased Component

---

# BOM Item

Item Number

Part Number

Name

Revision

Material

Quantity

Unit

Mass

Supplier

Manufacturer

Notes

---

# Generation Rules

Каждая позиция должна иметь:

- уникальный идентификатор;
- ссылку на геометрию;
- материал;
- количество;
- ревизию.

---

# Export Formats

CSV

Excel

JSON

XML

ERP Integration

---

# Output

Bill of Materials

Weight Summary

Material Summary

Purchased Components

---

# Acceptance Criteria

- автоматическая генерация;
- отсутствие дублирования;
- трассируемость до геометрической модели.

---

APPROVED