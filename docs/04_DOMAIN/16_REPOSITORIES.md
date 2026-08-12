# STAIR PLATFORM

Document: 16_REPOSITORIES.md

ID: DOM-0016

Status: APPROVED

---

# Purpose

Определяет архитектурную модель репозиториев предметной области.

Repository является единственной точкой доступа к постоянному хранению Aggregate Root.

Repository скрывает детали хранения данных и предоставляет доменной модели единый интерфейс работы с агрегатами.

---

# Scope

Документ распространяется на:

- Engine
- Geometry
- Manufacturing
- Pricing
- Documents
- Installation
- Maintenance

---

# Repository Principles

- Один Repository обслуживает один Aggregate Root.
- Repository не содержит бизнес-логики.
- Repository не возвращает внутренние модели хранения.
- Repository работает только с Domain Model.

---

# Repository Catalog

ProjectRepository

AssemblyRepository

PartRepository

GeometryRepository

MaterialRepository

ManufacturingRepository

PricingRepository

QuotationRepository

DocumentRepository

InstallationRepository

MaintenanceRepository

RevisionRepository

---

# Responsibilities

Repository отвечает за:

- получение Aggregate;
- сохранение Aggregate;
- поиск Aggregate;
- управление Revision;
- оптимистичную блокировку;
- контроль целостности.

---

# Rules

Запрещается:

- обращаться напрямую к Database из Domain;
- реализовывать бизнес-правила в Repository;
- объединять несколько Aggregate в одном Repository.

---

# Acceptance Criteria

- каждый Aggregate имеет собственный Repository;
- отсутствует обход Repository;
- Repository не содержит бизнес-логики.

---

APPROVED