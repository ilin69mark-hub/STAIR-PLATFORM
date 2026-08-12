# STAIR PLATFORM

Document: 20_DOMAIN_COMMANDS.md

ID: DOM-0020

Status: APPROVED

---

# Purpose

Domain Commands определяют все операции, изменяющие состояние Engineering Domain.

Команда представляет собой намерение изменить предметную область и является единственной допустимой точкой входа для изменения Aggregate.

Commands являются основой:

- API;
- Application Services;
- Event Bus;
- AI Workers;
- Scheduler;
- CLI;
- SDK.

---

# Scope

Команды применяются ко всем Engineering Objects.

---

# Command Principles

Каждая команда:

- изменяет состояние;
- имеет одного инициатора;
- выполняется атомарно;
- проверяет Domain Invariants;
- создает Domain Events;
- журналируется.

---

# Command Lifecycle

Command Created

↓

Validation

↓

Authorization

↓

Invariant Check

↓

Execution

↓

Aggregate Updated

↓

Revision Created

↓

Domain Event Published

↓

Audit Record Stored

---

# Command Categories

## Project

CreateProject

RenameProject

ArchiveProject

RestoreProject

DeleteProject

---

## Assembly

CreateAssembly

RenameAssembly

MoveAssembly

DeleteAssembly

DuplicateAssembly

---

## Part

CreatePart

DeletePart

DuplicatePart

MovePart

ReplacePart

---

## Geometry

CreateSketch

UpdateSketch

CreateFeature

UpdateFeature

DeleteFeature

RegenerateGeometry

RebuildModel

---

## Manufacturing

GenerateBOM

OptimizeCutting

CreateCNCTask

GenerateToolpath

ApproveManufacturing

---

## Pricing

CalculatePrice

ApplyDiscount

ApproveQuotation

PublishQuotation

---

## Documents

GenerateDrawing

GeneratePDF

GenerateDXF

GenerateSTEP

ExportProject

---

## Installation

CreateInstallationPlan

ApproveInstallation

CompleteInstallation

---

## Maintenance

CreateMaintenanceTask

CloseMaintenanceTask

---

# Command Rules

Команда не читает данные напрямую из Database.

Команда работает только через Aggregate.

Команда не обращается к другому Aggregate напрямую.

Команда не вызывает Query.

---

# Idempotency

Все внешние команды должны поддерживать Idempotency Key.

Повторная отправка команды не должна приводить к повторному изменению состояния.

---

# Acceptance Criteria

- все изменения выполняются через команды;
- отсутствуют прямые изменения Aggregate;
- каждая команда документирована.

---

APPROVED