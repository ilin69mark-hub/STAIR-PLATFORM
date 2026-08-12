# STAIR PLATFORM

Document: 15_GEOMETRY_CACHE.md

ID: ENG-GEO-0016

Status: APPROVED

---

# Purpose

Geometry Cache минимизирует повторные вычисления.

---

# Cached Objects

Topology

Solid

Mesh

Measurements

Bounding Boxes

Preview Geometry

---

# Cache Strategy

Incremental

Lazy Loading

Version Aware

Dependency Based

---

# Invalidation

Изменение параметра инвалидирует только зависимые объекты.

Полная очистка кэша допускается только при смене ревизии проекта.

---

# Performance Targets

- Минимизация полного перестроения.
- Повторное использование геометрии.
- Оптимизация памяти.

---

APPROVED