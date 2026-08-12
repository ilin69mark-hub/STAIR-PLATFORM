# STAIR PLATFORM

Document: 06_VALUE_OBJECTS.md

ID: DOM-0007

Status: APPROVED

---

# Purpose

Value Object представляет неизменяемое значение предметной области.

Value Object не имеет собственной идентичности.

---

# Principles

Value Object:

- immutable;
- сравнивается по значению;
- не имеет ID;
- потокобезопасен;
- переиспользуем.

---

# Registry

## Geometry

Length

Width

Height

Angle

Radius

Coordinate3D

Rotation

Direction

Thickness

Tolerance

MaterialReference

SectionProfile

---

## Solver

Force

Moment

Stress

Strain

Deflection

Displacement

SafetyFactor

---

## Manufacturing

PartNumber

MaterialCode

MachineType

SurfaceFinish

---

## Pricing

Money

Currency

ExchangeRate

Markup

VAT

DiscountRate

---

## Documents

DocumentVersion

Language

Format

Signature

---

## AI

ConfidenceScore

PromptTemplate

SourceReference

---

# Rules

Все Value Objects:

- неизменяемы;
- валидируются при создании;
- не содержат побочных эффектов.

---

APPROVED