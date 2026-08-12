# STAIR PLATFORM

Document: 03_SCHEMA_DESIGN.md

ID: DB-0003

Status: APPROVED

---

# Purpose

Определяет правила проектирования схем базы данных.

---

# Design Principles

Нормализация до 3NF по умолчанию.

Денормализация допускается только после анализа производительности.

Все внешние ключи обязательны.

Все UUID являются первичными идентификаторами.

---

# Naming Convention

snake_case

UUID Primary Key

created_at

updated_at

revision_id

deleted_at

---

# Constraints

Primary Key

Foreign Key

Unique

Check

Not Null

---

# Index Rules

Все Foreign Key индексируются.

Все Search Fields индексируются.

Revision индексируется.

Graph Node индексируется.

---

APPROVED