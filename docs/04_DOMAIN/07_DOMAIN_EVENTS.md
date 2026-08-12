# STAIR PLATFORM

Document: 07_DOMAIN_EVENTS.md

ID: DOM-0008

Status: APPROVED

---

# Purpose

Domain Events описывают произошедшие бизнес-события.

Они являются единственным способом уведомления других контекстов.

---

# Rules

Event:

- описывает факт;
- произошел в прошлом;
- неизменяем;
- содержит минимально необходимый набор данных.

---

# Registry

## Project

ProjectCreated

ProjectArchived

ProjectUpdated

---

## Geometry

GeometryUpdated

GeometryValidated

FlightAdded

StepAdded

---

## Validation

ValidationPassed

ValidationFailed

---

## Solver

AnalysisStarted

AnalysisCompleted

---

## Optimization

BetterSolutionFound

OptimizationFinished

---

## Manufacturing

BOMGenerated

DrawingGenerated

PackageCompleted

---

## Pricing

PriceCalculated

---

## Documents

DocumentGenerated

DocumentPublished

---

## Installation

InstallationCompleted

---

## Maintenance

InspectionCompleted

MaintenanceCompleted

---

## AI

RecommendationGenerated

KnowledgeUpdated

---

# Event Rules

События:

не отменяются;

не изменяются;

имеют версию;

логируются.

---

APPROVED