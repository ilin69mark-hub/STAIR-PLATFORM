# ADR-0013

# Geometry Platform as an Independent Kernel Layer

Status: ACCEPTED

Date: 2026-08-05

---

# Context

Geometry является основой всей инженерной платформы.

Однако геометрия не должна зависеть от предметной области.

Лестница — это всего лишь один из вариантов использования Geometry Platform.

---

# Decision

Geometry выделяется в самостоятельную платформу внутри STAIR-KERNEL.

Она предоставляет универсальные сервисы построения параметрической геометрии.

Geometry ничего не знает о:

- лестницах;
- мебели;
- производстве;
- AI.

Она знает только геометрию.

---

# Responsibilities

Coordinate Systems

Topology

Sketches

Curves

Surfaces

Solids

Boolean Operations

Transformations

Measurements

Geometry Validation

---

# Benefits

Повторное использование.

Высокая тестируемость.

Возможность создания новых инженерных продуктов.

---

# Status

ACCEPTED