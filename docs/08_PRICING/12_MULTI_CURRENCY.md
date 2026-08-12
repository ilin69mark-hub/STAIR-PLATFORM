# STAIR PLATFORM

Document: 12_MULTI_CURRENCY.md

ID: PRC-0013

Status: APPROVED

---

# Purpose

Multi Currency Engine обеспечивает работу Pricing Platform с несколькими валютами без изменения базовой модели расчета стоимости.

Все внутренние вычисления выполняются в базовой валюте проекта.

Конвертация производится только при формировании коммерческих документов и отчетов.

---

# Objectives

- единая базовая валюта проекта;
- поддержка нескольких валют отображения;
- воспроизводимость расчета;
- минимизация ошибок округления.

---

# Supported Currencies

Project Base Currency

Display Currency

Customer Currency

Supplier Currency

Accounting Currency

Custom Currency

---

# Currency Data

Currency Code

ISO Code

Exchange Rate

Effective Date

Source

Precision

Rounding Rules

Revision

---

# Exchange Rate Sources

Manual

ERP

Central Bank

Commercial Provider

Custom Provider

---

# Conversion Rules

Все вычисления выполняются в базовой валюте.

Конвертация выполняется после завершения расчета стоимости.

История использованных курсов сохраняется вместе с Revision проекта.

---

# Output

Converted Price

Currency Breakdown

Exchange Rate Report

---

# Acceptance Criteria

- воспроизводимые расчеты;
- поддержка нескольких валют;
- сохранение истории курсов.

---

APPROVED