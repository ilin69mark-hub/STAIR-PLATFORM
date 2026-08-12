# STAIR PLATFORM

Document: 19_DOMAIN_QUERIES.md

ID: DOM-0019

Status: APPROVED

---

# Purpose

Определяет модель чтения (Query Model) предметной области.

---

# Objectives

- разделение чтения и записи;
- поддержка CQRS;
- оптимизация запросов;
- поддержка аналитики.

---

# Query Categories

Project Queries

Assembly Queries

Part Queries

Geometry Queries

Manufacturing Queries

Pricing Queries

Document Queries

Revision Queries

Audit Queries

---

# Principles

Запросы не изменяют состояние системы.

Запросы используют специализированные модели чтения.

Запросы могут использовать кэш и поисковые индексы.

---

# Query Examples

- GetProjectByID
- GetAssemblyTree
- GetPartGeometry
- GetBOM
- GetQuotation
- GetRevisionHistory
- SearchDocuments

---

# Rules

- Query не публикует события.
- Query не изменяет агрегаты.
- Query не создаёт новые Revision.

---

# Acceptance Criteria

- операции чтения отделены от записи;
- Query Model документирована;
- поддерживается масштабирование чтения.

---

APPROVED