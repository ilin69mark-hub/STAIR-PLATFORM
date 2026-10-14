# STAIR PLATFORM

Document: 06_SOLID_BUILDER.md

ID: ENG-GEO-0007

Status: APPROVED (состояние ответственностей уточнено 2026-10-12)

---

# Purpose

Solid Builder строит твердотельную модель.

---

# Responsibilities

Состояние проверено по коду `internal/geometry` 2026-10-12. Прежняя
редакция перечисляла восемь ответственностей без указания, какие из них
реально есть; две из них отсутствовали полностью.

| Ответственность | Состояние | Где в коде |
|---|---|---|
| B-Rep | РЕАЛИЗОВАНА | `topology.go` — Vertex→Edge→Wire→Face→Shell→Solid→Compound |
| Extrude | РЕАЛИЗОВАНА | `extrude.go` — `Extrude`, `ExtrudeWithHoles` |
| Boolean Operations | **НЕ РЕАЛИЗОВАНА** | см. `10_BOOLEAN_ENGINE.md` (ENG-GEO-0011) |
| Fillet | ЧАСТИЧНО | `curves.go` — `RoundCorner` скругляет угол КОНТУРА. Скругления ребра твёрдого тела нет |
| Chamfer | ЧАСТИЧНО | тип `Chamfer` и `Tessellate` есть, но в билдерах лестниц не используется |
| Sweep | **НЕ РЕАЛИЗОВАНА** | — |
| Loft | **НЕ РЕАЛИЗОВАНА** | — |
| Revolve | **НЕ РЕАЛИЗОВАНА** | — |

---

# Input

Topology

↓

Semantic Model

↓

Parameters

---

# Output

Solid Model

---

# Rules

Solid Builder никогда не работает напрямую с UI.

Все операции детерминированы.

---

APPROVED