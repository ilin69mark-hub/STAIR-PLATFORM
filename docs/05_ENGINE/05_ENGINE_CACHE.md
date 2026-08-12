# STAIR PLATFORM

Document: 05_ENGINE_CACHE.md

ID: ENG-0006

Status: APPROVED

---

# Purpose

Engine Cache определяет единую стратегию кэширования результатов вычислений во всех инженерных движках STAIR-KERNEL.

Целью кэширования является минимизация повторных вычислений при сохранении детерминированности результатов.

---

# Objectives

- уменьшение времени перестроения модели;
- повторное использование вычислений;
- минимизация использования CPU;
- снижение нагрузки на память;
- поддержка инкрементального пересчета.

---

# Cache Levels

## L1 — Runtime Cache

Живет во время выполнения вычислений.

Используется для промежуточных результатов.

---

## L2 — Engine Cache

Кэш конкретного Engine.

Пример:

- Geometry Cache
- Solver Cache
- Mesh Cache
- Manufacturing Cache

---

## L3 — Graph Cache

Кэш зависимостей Graph Platform.

Используется всеми движками.

---

## L4 — Persistent Cache

Опциональный долговременный кэш.

Используется между сессиями.

---

# Cache Objects

Допускается кэширование:

- Topology
- Solid
- Mesh
- Measurements
- Validation Results
- Solver Results
- Optimization Results
- BOM
- Drawings
- Preview Geometry

---

# Cache Keys

Каждый Cache Entry определяется:

- Project Revision
- Graph Version
- Engine Version
- Input Parameters Hash
- Dependency Hash

---

# Invalidation Rules

Инвалидация производится только при изменении зависимостей.

Полная очистка запрещена, кроме случаев:

- смена Revision;
- изменение версии Engine;
- повреждение Cache;
- изменение формата хранения.

---

# Requirements

Engine Cache обязан:

- поддерживать lazy loading;
- поддерживать incremental invalidation;
- быть потокобезопасным;
- поддерживать диагностику использования.

---

# Acceptance Criteria

- отсутствуют повторные вычисления при неизменных входных данных;
- поддерживается частичная инвалидация;
- обеспечивается повторное использование результатов между Engine.

---

APPROVED