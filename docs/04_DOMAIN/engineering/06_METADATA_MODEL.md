# STAIR PLATFORM

Document: 06_METADATA_MODEL.md

ID: EDM-0006

Status: APPROVED

---

# Purpose

Определяет единую модель инженерных метаданных.

---

# Metadata Structure

Metadata

├── Author

├── Organization

├── CreatedAt

├── UpdatedAt

├── Revision

├── Status

├── Labels

├── Tags

├── Attributes

├── Classification

└── Custom Properties

---

# Principles

Metadata отделены от Geometry.

Metadata не влияют на вычисления.

Metadata полностью версионируются.

---

# Attribute Types

String

Integer

Float

Boolean

Date

Enumeration

Reference

List

---

# Rules

Все инженерные объекты имеют Metadata.

Все изменения Metadata создают новую Revision.

---

# Acceptance Criteria

Metadata единообразны во всех подсистемах.

---

APPROVED