# STAIR PLATFORM

Document: 11_FACTORIES.md

ID: DOM-0012

Status: APPROVED

---

# Purpose

Factory отвечает за создание сложных Aggregate.

Factory инкапсулирует процесс построения корректной доменной модели.

---

# Principles

Factory:

- возвращает полностью валидный Aggregate;
- не содержит инфраструктуры;
- использует Domain Services;
- проверяет инварианты.

---

# Registry

ProjectFactory

GeometryFactory

ConstraintFactory

ValidationFactory

SolverFactory

OptimizationFactory

ManufacturingFactory

PricingFactory

DocumentFactory

InstallationFactory

MaintenanceFactory

---

# Rules

Factory:

не изменяет существующие Aggregate;

не вызывает Repository напрямую;

не знает SQL.

---

APPROVED