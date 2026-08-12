# STAIR PLATFORM

**Bounded Context:** Optimization

**ID:** BC-005

**Type:** Core Engineering Domain

**Status:** APPROVED

---

# 1. Purpose

Optimization отвечает за поиск оптимальных инженерных решений на основе результатов расчётов, ограничений, производственных требований и бизнес-целей.

Контекст не строит геометрию и не выполняет расчёты прочности.

Optimization анализирует готовые варианты и предлагает лучшие.

---

# 2. Responsibilities

Контекст отвечает за:

- многокритериальную оптимизацию;
- генерацию альтернативных решений;
- оптимизацию материалов;
- оптимизацию стоимости;
- оптимизацию массы;
- оптимизацию производства;
- оптимизацию монтажа;
- ранжирование вариантов.

---

# 3. Aggregate Root

OptimizationSession

---

# 4. Main Entities

- OptimizationSession
- OptimizationGoal
- OptimizationScenario
- CandidateSolution
- OptimizationResult

---

# 5. Value Objects

- Weight
- Cost
- ManufacturabilityScore
- SafetyMargin
- InstallationComplexity
- MaterialEfficiency
- OptimizationScore

---

# 6. Optimization Goals

## Cost

Минимизировать стоимость.

---

## Weight

Минимизировать массу.

---

## Manufacturing

Минимизировать сложность производства.

---

## Installation

Минимизировать сложность монтажа.

---

## Material

Минимизировать отходы материала.

---

## Safety

Максимизировать запас прочности.

---

## Aesthetics

Максимизировать визуальную привлекательность.

---

# 7. Domain Services

- Optimizer
- ScenarioGenerator
- CandidateEvaluator
- RankingEngine
- TradeOffAnalyzer

---

# 8. Domain Events

- OptimizationStarted
- CandidateGenerated
- CandidateRejected
- OptimizationFinished
- BetterSolutionFound

---

# 9. Invariants

- используется только валидная модель;
- используется только рассчитанная модель;
- все решения сравнимы по единым критериям;
- результаты детерминированы при одинаковых входных данных.

---

# 10. Dependencies

Incoming

- Validation
- Solver
- Constraints

Outgoing

- Manufacturing
- Pricing
- AI

---

# 11. Public Interfaces

Optimization предоставляет:

- Optimize()
- Compare()
- Rank()
- ExplainDecision()
- GetAlternatives()

---

# 12. Policies

- Multi Objective Policy
- Trade-Off Policy
- Material Policy
- Manufacturing Policy

---

# 13. Future Extensions

- генетические алгоритмы;
- эволюционная оптимизация;
- симуляция отжига;
- NSGA-II;
- Pareto Front;
- обучение на исторических проектах;
- AI-оптимизация.

---

# 14. Approval

APPROVED