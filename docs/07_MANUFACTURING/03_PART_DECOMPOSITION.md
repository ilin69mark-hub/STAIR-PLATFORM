# STAIR PLATFORM

Document: 03_PART_DECOMPOSITION.md

ID: MFG-0004

Status: APPROVED

---

# Purpose

Part Decomposition преобразует инженерную модель в набор производственных деталей.

---

# Objectives

- выделение отдельных деталей;
- формирование производственных единиц;
- сохранение связей с исходной моделью;
- подготовка данных для BOM.

---

# Part Types

Structural Parts

Panels

Profiles

Fasteners

Hardware

Purchased Components

Custom Parts

---

# Part Structure

Part ID

Part Name

Revision

Geometry Reference

Material

Dimensions

Mass

Manufacturing Metadata

---

# Decomposition Rules

Каждая производственная деталь должна:

- иметь уникальный идентификатор;
- ссылаться на геометрию;
- содержать материал;
- содержать технологические параметры;
- поддерживать отслеживание ревизий.

---

# Output

Part List

Part Metadata

Manufacturing References

Assembly Links

---

# Acceptance Criteria

- корректное разбиение изделия на детали;
- отсутствие дублирования;
- полная трассируемость между геометрией и производственной деталью.

---

APPROVED