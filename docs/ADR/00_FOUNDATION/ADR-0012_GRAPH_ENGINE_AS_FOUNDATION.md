# ADR-0012

# Graph Engine as the Foundation of the Engineering Kernel

Status: ACCEPTED

Date: 2026-08-05

Authors:
- Mark Ilyin
- ChatGPT

---

# Context

STAIR Platform проектируется как промышлененная Digital Engineering Platform.

В процессе проектирования стало очевидно, что практически каждый вычислительный модуль использует графовую модель данных.

Примеры:

- Geometry использует Dependency Graph;
- Constraints использует Constraint Graph;
- Solver использует Calculation Graph;
- Optimization использует Search Graph;
- Manufacturing использует Assembly Graph;
- Documents использует Document Dependency Graph;
- AI использует Knowledge Graph;
- Workflow использует Execution Graph.

Использование отдельных реализаций графов приводит к:

- дублированию алгоритмов;
- различным правилам обхода;
- различным механизмам кэширования;
- различным моделям событий;
- высокой стоимости сопровождения.

---

# Decision

Создать единый Graph Engine как часть STAIR-KERNEL.

Все остальные движки используют Graph Engine как инфраструктурную библиотеку.

Graph Engine не содержит инженерной логики.

Он предоставляет универсальные механизмы:

- Graph Storage
- Graph Traversal
- Dependency Resolution
- Incremental Updates
- Dirty Node Detection
- Graph Cache
- Event Propagation
- Execution Scheduling
- Versioning
- Snapshotting

---

# Architecture

Parameter Graph

↓

Constraint Graph

↓

Dependency Graph

↓

Semantic Graph

↓

Execution Graph

↓

Assembly Graph

↓

Knowledge Graph

↓

Workflow Graph

---

# Responsibilities

Graph Engine отвечает исключительно за работу с графами.

Он не знает ничего о:

- лестницах;
- CAD;
- материалах;
- расчетах;
- AI.

---

# Benefits

Единая инфраструктура.

Повторное использование алгоритмов.

Инкрементальное обновление.

Быстрый rebuild.

Быстрый solver.

Минимизация вычислений.

Поддержка AI.

Поддержка Digital Twin.

---

# Consequences

Появляется дополнительный уровень абстракции.

Зато все движки становятся значительно проще.

Graph Engine становится ядром STAIR-KERNEL.

---

# Status

ACCEPTED