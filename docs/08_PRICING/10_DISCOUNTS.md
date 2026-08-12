# STAIR PLATFORM

Document: 10_DISCOUNTS.md

ID: PRC-0011

Status: APPROVED

---

# Purpose

Discount Engine управляет системой скидок.

---

# Objectives

- единый механизм скидок;
- поддержка комбинаций правил;
- предотвращение конфликтов.

---

# Discount Types

Customer Discount

Dealer Discount

Partner Discount

Volume Discount

Seasonal Discount

Campaign Discount

Project Discount

Tender Discount

Manual Discount

Custom Discount

---

# Discount Rules

Каждая скидка содержит:

Discount ID

Priority

Calculation Method

Maximum Value

Effective Period

Revision

---

# Conflict Resolution

По умолчанию применяется правило с наивысшим приоритетом.

Возможность суммирования скидок определяется политикой компании.

---

# Output

Applied Discounts

Discount Breakdown

Final Discount

---

# Acceptance Criteria

- поддержка нескольких скидок;
- отсутствие неоднозначностей;
- воспроизводимый расчет.

---

APPROVED