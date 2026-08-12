# STAIR PLATFORM

**Bounded Context:** Documents

**ID:** BC-009

**Type:** Supporting Domain

**Status:** APPROVED

---

# 1. Purpose

Documents отвечает за формирование, хранение, версионирование и публикацию всей документации проекта.

Контекст не рассчитывает инженерные параметры и не изменяет модель проекта.

Он преобразует результаты других контекстов в документы, пригодные для использования людьми и внешними системами.

---

# 2. Responsibilities

Контекст отвечает за:

- генерацию PDF;
- генерацию спецификаций;
- генерацию коммерческих предложений;
- генерацию паспортов изделий;
- генерацию монтажной документации;
- генерацию производственной документации;
- управление шаблонами;
- управление версиями документов.

---

# 3. Aggregate Root

DocumentPackage

---

# 4. Main Entities

- DocumentPackage
- Document
- Template
- Revision
- Attachment

---

# 5. Value Objects

- DocumentNumber
- DocumentVersion
- TemplateVersion
- Language
- Format
- Signature

---

# 6. Document Types

- Commercial Proposal
- Specification
- BOM
- Manufacturing Package
- Assembly Guide
- Installation Manual
- Product Passport
- Calculation Report
- Quality Certificate

---

# 7. Domain Services

- DocumentGenerator
- TemplateRenderer
- ExportService
- SignatureService

---

# 8. Domain Events

- DocumentGenerated
- DocumentPublished
- TemplateUpdated
- RevisionCreated

---

# 9. Invariants

- документ всегда относится к одной ревизии проекта;
- документ неизменяем после публикации;
- шаблон имеет собственную версию;
- все документы трассируются до исходных данных.

---

# 10. Dependencies

Incoming

- Manufacturing
- Pricing
- Solver
- Project

Outgoing

- Orders
- Customer Portal
- Archive

---

# 11. Public Interfaces

Documents предоставляет:

- Generate()
- Publish()
- Export()
- Archive()

---

# 12. Approval

APPROVED