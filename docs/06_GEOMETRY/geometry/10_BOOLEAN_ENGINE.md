# STAIR PLATFORM

Document: 10_BOOLEAN_ENGINE.md

ID: ENG-GEO-0011

Status: APPROVED

---

# Purpose

Boolean Engine выполняет операции над твердотельной геометрией.

---

# Supported Operations

Union

Intersection

Difference

Split

Slice

Merge

Imprint

---

# Input

Solid A

Solid B

↓

Operation

---

# Output

Resulting Solid

---

# Validation

Перед выполнением операции проверяются:

- корректность топологии;
- замкнутость тел;
- отсутствие самопересечений;
- совместимость допусков.

---

# Error Handling

При невозможности выполнения операции:

- создается диагностический отчет;
- исходные тела остаются неизменными;
- публикуется событие BooleanOperationFailed.

---

# Performance Goals

- Incremental processing.
- Reuse existing topology where possible.
- Parallel preparation of operands.

---

APPROVED