# STAIR PLATFORM

**Bounded Context:** Geometry

**ID:** BC-002

**Type:** Core Engineering Domain

**Status:** APPROVED

---

# 1. Purpose

Geometry отвечает за параметрическое описание лестницы и построение её инженерной геометрической модели.

Контекст определяет состав лестницы, взаимное расположение элементов и правила их формирования.

Geometry не выполняет расчёты прочности, не рассчитывает стоимость и не отвечает за производство.

---

# 2. Responsibilities

Контекст отвечает за:

- создание геометрической модели;
- изменение параметров модели;
- построение пространственной структуры;
- проверку геометрической корректности;
- экспорт геометрии;
- управление ревизиями модели.

---

# 3. Aggregate Root

```
StairModel
```

Любое изменение геометрии проходит через Aggregate Root.

---

# 4. Main Entities

- StairModel
- Flight
- Landing
- Step
- Stringer
- Support
- Beam
- Railing
- Baluster
- Handrail
- Connection
- Opening

---

# 5. Value Objects

- Length
- Width
- Height
- Angle
- Elevation
- Coordinate3D
- Direction
- MaterialReference
- SectionProfile
- Thickness
- Rotation
- Tolerance

---

# 6. Domain Services

- GeometryBuilder
- GeometryValidator
- GeometryTransformer
- GeometryExporter
- GeometryImportService

---

# 7. Domain Events

- GeometryCreated
- GeometryUpdated
- GeometryValidated
- GeometryImported
- GeometryExported
- FlightAdded
- StepAdded

---

# 8. Invariants

Всегда должны соблюдаться:

- существует одна активная модель;
- все элементы принадлежат одной StairModel;
- отсутствуют "висячие" элементы;
- все соединения валидны;
- координатная система едина;
- размеры имеют допустимые значения;
- отрицательные размеры запрещены.

---

# 9. Dependencies

Incoming

- Project

Outgoing

- Solver
- Validation
- Rendering
- Manufacturing

---

# 10. Public Interfaces

Geometry публикует:

- GeometryCreated
- GeometryUpdated
- GeometryValidated
- GeometryChanged

Geometry принимает:

- CreateGeometry
- UpdateGeometry
- DeleteGeometry
- CloneGeometry

---

# 11. Policies

- Parametric Update Policy
- Geometry Integrity Policy
- Coordinate Policy
- Precision Policy

---

# 12. Domain Rules

Основные правила:

- параметры являются единственным источником истины;
- геометрия всегда может быть полностью перестроена;
- изменение параметра приводит к пересборке модели;
- все элементы имеют уникальный идентификатор;
- модель должна быть детерминированной.

---

# 13. Future Extensions

Контекст предусматривает поддержку:

- прямых лестниц;
- Г-образных лестниц;
- П-образных лестниц;
- винтовых лестниц;
- лестниц произвольной формы;
- комбинированных конструкций;
- модульных систем.

---

# 14. Approval

APPROVED