# STAIR PLATFORM

Document: 17_DOMAIN_INVARIANTS.md

ID: DOM-0017

Status: APPROVED

---

# Purpose

Определяет неизменяемые правила (Invariants), которые должны соблюдаться независимо от реализации.

---

# Core Principles

Инварианты являются частью предметной области.

Они не зависят от языка программирования, СУБД или пользовательского интерфейса.

---

# Project Invariants

- Project имеет уникальный идентификатор.
- Project всегда содержит минимум одну Revision.
- Archived Project не может изменяться.

---

# Assembly Invariants

- Assembly принадлежит одному Project.
- Assembly не может содержать саму себя.
- Циклические зависимости запрещены.

---

# Part Invariants

- Part принадлежит одной Assembly.
- Part не существует без Project.
- Part имеет хотя бы одну геометрическую модель.

---

# Geometry Invariants

- Геометрия должна быть валидной.
- Все размеры имеют единицы измерения.
- Площадь и объём не могут быть отрицательными.

---

# Manufacturing Invariants

- BOM содержит только существующие детали.
- Операции выполняются в определённом порядке.

---

# Pricing Invariants

- Стоимость не может быть отрицательной.
- Валюта обязательна для всех денежных значений.

---

# Validation

Инварианты проверяются:

- Domain;
- API;
- Database;
- Import;
- AI.

---

APPROVED# STAIR PLATFORM

Document: 17_DOMAIN_INVARIANTS.md

ID: DOM-0017

Status: APPROVED

---

# Purpose

Определяет неизменяемые правила (Invariants), которые должны соблюдаться независимо от реализации.

---

# Core Principles

Инварианты являются частью предметной области.

Они не зависят от языка программирования, СУБД или пользовательского интерфейса.

---

# Project Invariants

- Project имеет уникальный идентификатор.
- Project всегда содержит минимум одну Revision.
- Archived Project не может изменяться.

---

# Assembly Invariants

- Assembly принадлежит одному Project.
- Assembly не может содержать саму себя.
- Циклические зависимости запрещены.

---

# Part Invariants

- Part принадлежит одной Assembly.
- Part не существует без Project.
- Part имеет хотя бы одну геометрическую модель.

---

# Geometry Invariants

- Геометрия должна быть валидной.
- Все размеры имеют единицы измерения.
- Площадь и объём не могут быть отрицательными.

---

# Manufacturing Invariants

- BOM содержит только существующие детали.
- Операции выполняются в определённом порядке.

---

# Pricing Invariants

- Стоимость не может быть отрицательной.
- Валюта обязательна для всех денежных значений.

---

# Validation

Инварианты проверяются:

- Domain;
- API;
- Database;
- Import;
- AI.

---

APPROVED