# STAIR PLATFORM

**Bounded Context:** Manufacturing

**ID:** BC-007

**Type:** Core Engineering Domain

**Status:** APPROVED

---

# 1. Purpose

Manufacturing отвечает за подготовку инженерной модели к производству.

Контекст преобразует валидную и рассчитанную конструкцию в комплект производственных данных.

Manufacturing не изменяет конструкцию.

Он лишь определяет, как её изготовить.

---

# 2. Responsibilities

Контекст отвечает за:

- спецификации (BOM);
- карты раскроя;
- производственные чертежи;
- CNC-программы;
- маршрут производства;
- план сборки;
- план упаковки;
- комплект производственной документации.

---

# 3. Aggregate Root

ManufacturingPackage

---

# 4. Main Entities

- ManufacturingPackage
- Part
- Assembly
- BOM
- Drawing
- CNCProgram
- MaterialSheet
- ProductionRoute
- PackagePlan

---

# 5. Value Objects

- MaterialCode
- PartNumber
- Quantity
- Thickness
- SurfaceFinish
- ManufacturingTolerance
- MachineType

---

# 6. Manufacturing Modules

- BOM Generator
- Drawing Generator
- CNC Generator
- Cutting Optimizer
- Nesting Engine
- Assembly Planner
- Packaging Planner
- Production Validator

---

# 7. Domain Services

- ManufacturingPlanner
- BOMBuilder
- DrawingBuilder
- CNCBuilder
- NestingPlanner

---

# 8. Domain Events

- BOMGenerated
- DrawingsGenerated
- CNCGenerated
- ProductionValidated
- PackageCompleted

---

# 9. Invariants

- производство выполняется только по валидной ревизии проекта;
- каждая деталь имеет уникальный Part Number;
- все позиции BOM трассируются до геометрии;
- производственные файлы соответствуют версии проекта;
- спецификация и чертежи синхронизированы.

---

# 10. Dependencies

Incoming

- Geometry
- Validation
- Solver
- Optimization

Outgoing

- Pricing
- Documents
- ERP Integration

---

# 11. Public Interfaces

Manufacturing предоставляет:

- GenerateBOM()
- GenerateDrawings()
- GenerateCNC()
- GenerateAssemblyPlan()
- GenerateProductionPackage()

---

# 12. Policies

- Manufacturing Policy
- Material Policy
- Drawing Standard Policy
- CNC Policy
- Revision Policy

---

# 13. Future Extensions

- автоматическое планирование производства;
- интеграция с ERP/MES;
- роботизированные производственные линии;
- цифровой двойник производства;
- контроль качества.

---

# 14. Approval

APPROVED