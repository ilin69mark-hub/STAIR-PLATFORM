
---

# `20_TESTING/12_PRICING_TESTING.md`

```markdown
# STAIR PLATFORM

Document: 12_PRICING_TESTING.md

ID: TEST-0012

Status: APPROVED

---

# Purpose

Определяет Testing Strategy Pricing Layer.

---

# Scope

Cost Model

Material Cost

Labor Cost

Machine Cost

Overhead

Margin

Discounts

Taxes

Currency

Price Validation

Price History

Reports

---

# Calculation Testing

Каждый pricing component тестируется отдельно.

---

# Cost Model

Проверяются:

Material Cost

Labor Cost

Machine Cost

Overhead

Additional Costs

---

# Price Rules

Проверяются:

Minimum Price

Maximum Discount

Margin Constraints

Tax Rules

Currency Rules

---

# Boundary Cases

Тестируются:

Zero Cost

Zero Margin

Maximum Discount

Minimum Margin

Large Quantities

Decimal Values

Currency Conversion

---

# Rounding

Все monetary calculations должны соответствовать установленной rounding policy.

---

# Historical Testing

Изменение Price Rule не должно изменять исторический finalized price.

---

# Regression

Критические pricing scenarios сохраняются как regression tests.

---

# Acceptance Criteria

Для одинакового input и одинаковых pricing rules система возвращает deterministic price.

---

APPROVED