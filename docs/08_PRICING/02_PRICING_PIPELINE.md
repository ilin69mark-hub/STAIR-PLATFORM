# STAIR PLATFORM

Document: 02_PRICING_PIPELINE.md

ID: PRC-0003

Status: APPROVED

---

# Purpose

Pricing Pipeline определяет последовательность вычисления стоимости.

---

# Pipeline

Manufacturing Dataset

↓

Validate Inputs

↓

Material Cost

↓

Machine Cost

↓

Labor Cost

↓

Purchased Components

↓

Overhead

↓

Margin

↓

Discounts

↓

Taxes

↓

Currency Conversion

↓

Commercial Price

↓

Reports

---

# Pipeline Stages

1. Validate Dataset
2. Calculate Direct Material Cost
3. Calculate Machine Cost
4. Calculate Labor Cost
5. Calculate Purchased Components
6. Apply Overhead
7. Apply Pricing Rules
8. Apply Margin
9. Apply Discounts
10. Apply Taxes
11. Convert Currency
12. Generate Reports

---

# Recalculation

Допускается пересчет только изменившихся этапов.

---

# Error Handling

При невозможности расчета формируется Pricing Validation Report.

---

# Acceptance Criteria

- детерминированный Pipeline;
- инкрементальный пересчет;
- воспроизводимые результаты.

---

APPROVED