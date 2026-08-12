# STAIR PLATFORM

Document: 04_PRICE_RULES.md

ID: PRC-0005

Status: APPROVED

---

# Purpose

Price Rules Engine определяет правила формирования коммерческой стоимости изделий.

Правила являются декларативными и не требуют изменения исходного кода платформы.

---

# Objectives

- централизованное управление правилами;
- поддержка различных бизнес-моделей;
- поддержка региональных настроек;
- возможность расширения без изменения ядра.

---

# Rule Categories

Material Rules

Machine Rules

Labor Rules

Margin Rules

Discount Rules

Tax Rules

Regional Rules

Customer Rules

Project Rules

Custom Rules

---

# Rule Structure

Rule ID

Name

Priority

Category

Condition

Action

Effective Date

Expiration Date

Revision

Status

---

# Evaluation Order

Validation

↓

Material Rules

↓

Labor Rules

↓

Machine Rules

↓

Commercial Rules

↓

Taxes

↓

Currency

---

# Rule Resolution

При конфликте применяется правило с более высоким приоритетом.

При одинаковом приоритете используется более специализированное правило.

---

# Acceptance Criteria

- декларативное описание правил;
- поддержка версионирования;
- детерминированный результат.

---

APPROVED