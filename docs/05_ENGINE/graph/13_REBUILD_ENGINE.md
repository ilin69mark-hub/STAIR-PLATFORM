# STAIR PLATFORM

Document: 09_REBUILD_ENGINE.md

ID: ENG-GEO-0010

Status: APPROVED

---

# Purpose

Rebuild Engine определяет минимальный набор объектов, требующих перестроения после изменения модели.

---

# Responsibilities

Dependency Analysis

Dirty Node Detection

Partial Rebuild

Cache Reuse

Version Tracking

---

# Pipeline

Parameter Changed

↓

Dependency Graph

↓

Dirty Nodes

↓

Rebuild Queue

↓

Geometry

↓

Solid

↓

Mesh

---

# Rules

Полная перестройка запрещена, если возможно частичное обновление.

---

APPROVED