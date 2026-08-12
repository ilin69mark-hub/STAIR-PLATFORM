# STAIR PLATFORM

Document: 05_CLASSIFICATION.md

ID: EDM-0005

Status: APPROVED

---

# Purpose

Определяет единую систему классификации инженерных объектов.

---

# Engineering Classes

Project

Assembly

SubAssembly

Part

Sketch

Feature

Geometry

Material

Machine

Operation

Tool

BOM

Quotation

Drawing

Revision

Installation

Maintenance

---

# Object Categories

Engineering

Commercial

Manufacturing

Administrative

System

AI

---

# Object Hierarchy

Engineering Object

↓

Physical Object

↓

Assembly

↓

Part

↓

Geometry

↓

Feature

---

# Metadata

Каждый объект имеет:

Class

Category

Subtype

Owner

Lifecycle

Revision

---

# Extensibility

Новые классы допускаются только через ADR.

---

# Acceptance Criteria

Все инженерные объекты классифицированы.

Классификация используется API, Search, Database и AI.

---

APPROVED