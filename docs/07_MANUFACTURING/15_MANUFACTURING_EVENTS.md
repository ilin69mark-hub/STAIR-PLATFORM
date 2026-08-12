# STAIR PLATFORM

Document: 15_MANUFACTURING_EVENTS.md

ID: MFG-0016

Status: APPROVED

---

# Purpose

Документ определяет события Manufacturing Platform.

---

# Event Categories

Production Events

Validation Events

Export Events

Assembly Events

Operation Events

---

# Events

ManufacturingStarted

ManufacturingCompleted

PartCreated

PartUpdated

MaterialAssigned

AssemblyGenerated

BOMGenerated

OperationCalculated

NestingCompleted

CNCExportCompleted

QCValidationPassed

QCValidationFailed

CostDatasetGenerated

ManufacturingFailed

---

# Event Structure

Event ID

Timestamp

Revision

Project ID

Source Engine

Correlation ID

Payload

---

# Rules

Все события:

- immutable;
- versioned;
- traceable;
- replayable.

---

# Acceptance Criteria

- публикация событий после завершения операций;
- поддержка Event Bus;
- совместимость с Graph Events.

---

APPROVED