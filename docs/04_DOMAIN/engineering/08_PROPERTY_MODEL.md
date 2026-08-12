# STAIR PLATFORM

Document: 08_PROPERTY_MODEL.md

ID: EDM-0008

Status: APPROVED

---

# Purpose

Определяет модель инженерных свойств (Property Model).

Property описывает измеряемую или вычисляемую характеристику Engineering Object.

Property может использоваться Geometry Engine, Manufacturing Engine, Pricing Engine, AI и API.

---

# Property Structure

Property

├── ID
├── Name
├── Value
├── Data Type
├── Unit
├── Source
├── Validation Rules
├── Visibility
├── Revision
└── Metadata

---

# Property Categories

Geometry

Material

Manufacturing

Pricing

Calculation

Document

Installation

Maintenance

System

Custom

---

# Property Sources

User

Formula

Graph

Geometry

Import

AI

External System

---

# Property Rules

Property имеет уникальный идентификатор.

Property принадлежит одному Engineering Object.

Property может быть вычисляемым.

Property может быть только для чтения.

Property полностью версионируется.

---

# Acceptance Criteria

Все свойства объектов описываются через единую Property Model.