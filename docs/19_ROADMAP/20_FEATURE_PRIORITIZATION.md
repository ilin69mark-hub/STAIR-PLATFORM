# STAIR PLATFORM

Document: 20_FEATURE_PRIORITIZATION.md

ID: ROADMAP-0020

Status: APPROVED

---

# Purpose

Определяет правила приоритизации Product Features.

---

# Priority Factors

Feature оценивается по:

* Business Value
* Customer Value
* Technical Risk
* Dependency Impact
* Implementation Cost
* Security Impact
* Operational Impact

---

# Priority Classes

## P0 — Critical

Feature необходима для работоспособности системы или устранения critical risk.

---

## P1 — Core

Feature необходима для основного Product Workflow.

---

## P2 — Important

Feature существенно улучшает продукт, но не блокирует основной workflow.

---

## P3 — Optional

Feature может быть отложена без существенного ущерба для продукта.

---

# Prioritization Rule

Высокий priority получают features с комбинацией:

```text
High Value
+
High Risk
+
Strong Dependency
```

Такие features должны быть validated early.

---

# MVP Rule

MVP включает только features, необходимые для:

* Core User Journey
* Core Business Value
* Technical Validation

---

# Feature Expansion

Новые features не должны автоматически попадать в MVP.

Они проходят отдельную prioritization assessment.

---

# Reassessment

Priority может изменяться на основании:

* Customer Feedback
* Usage Data
* Production Errors
* Business Metrics
* Technical Findings

---

# Acceptance Criteria

Каждая значимая feature имеет explicit priority и обоснование.

---

APPROVED
