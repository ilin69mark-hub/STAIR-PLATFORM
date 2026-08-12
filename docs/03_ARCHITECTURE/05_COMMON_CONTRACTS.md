# STAIR PLATFORM

Document: 05_COMMON_CONTRACTS.md

ID: ARCH-0006

Status: APPROVED

---

# Purpose

Документ определяет единые типы данных, используемые всеми платформами STAIR Platform.

Common Contracts являются фундаментом Canonical Data Model.

---

# Objectives

- единая терминология;
- единые типы;
- отсутствие дублирования;
- совместимость модулей.

---

# Global Identifiers

ProjectID

AssemblyID

PartID

SketchID

FeatureID

MaterialID

OperationID

MachineID

DocumentID

OrganizationID

UserID

RevisionID

EventID

JobID

AIRequestID

NotificationID

---

# Measurement Types

Length

Width

Height

Area

Volume

Mass

Weight

Angle

Radius

Diameter

Thickness

Tolerance

Coordinate

Vector2

Vector3

Matrix4

BoundingBox

---

# Financial Types

Money

Currency

Tax

Discount

Margin

Price

ExchangeRate

---

# Temporal Types

Timestamp

Duration

Date

Timezone

Schedule

---

# Metadata Types

Revision

Version

Labels

Tags

Attributes

Checksum

Hash

Signature

---

# Rules

Все платформы используют только Common Contracts.

Создание собственных аналогов запрещено.

Любое изменение требует ADR.

---

# Acceptance Criteria

- отсутствуют дублирующие типы;
- единая система идентификаторов;
- единая система измерений.

---

APPROVED