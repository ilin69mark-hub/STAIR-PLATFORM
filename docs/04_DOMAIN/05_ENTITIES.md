# STAIR PLATFORM

Document: 05_ENTITIES.md

ID: DOM-0006

Status: APPROVED

---

# Purpose

Документ определяет все Entity предметной области.

Entity обладает собственной идентичностью и жизненным циклом.

Entity всегда принадлежит Aggregate.

---

# Entity Principles

Каждая Entity:

- имеет уникальный идентификатор;
- принадлежит одному Aggregate;
- изменяется только через Aggregate Root;
- не существует вне своего Aggregate.

---

# Entity Registry

## Project

- ProjectRevision
- ProjectMember
- ProjectSettings

## Geometry

- StairModel
- Flight
- Landing
- Step
- Stringer
- Beam
- Support
- Platform
- Opening
- Connection
- Railing
- Baluster
- Handrail

## Constraints

- Constraint
- Rule
- RuleGroup

## Validation

- ValidationIssue
- ValidationResult

## Solver

- AnalysisSession
- LoadCase
- Result

## Optimization

- CandidateSolution
- Scenario

## Manufacturing

- Part
- Assembly
- CNCProgram
- BOM
- Drawing

## Pricing

- CostItem
- Margin
- Discount

## Documents

- Template
- Document
- Revision

## AI

- Recommendation
- Prompt
- ContextSnapshot

---

# Entity Rules

Entity:

- может изменяться;
- имеет состояние;
- может публиковать Domain Events через Aggregate;
- не должна содержать инфраструктурную логику.

---

# Dependencies

Incoming

AGGREGATES

Outgoing

VALUE_OBJECTS

---

APPROVED