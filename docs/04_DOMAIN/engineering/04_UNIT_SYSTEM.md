# STAIR PLATFORM

Document: 04_UNIT_SYSTEM.md

ID: EDM-0004

Status: APPROVED

---

# Purpose

Определяет единую систему единиц измерения (Engineering Unit System), используемую всеми подсистемами STAIR Platform.

Единая система единиц обеспечивает корректность вычислений, воспроизводимость моделей и совместимость между Geometry, Manufacturing, Pricing, API и AI.

---

# Objectives

- единая система измерений;
- единые правила конвертации;
- исключение неоднозначности;
- контроль точности вычислений.

---

# Base Unit System

По умолчанию используется Международная система единиц (SI).

---

# Supported Dimensions

Length

Area

Volume

Angle

Mass

Time

Force

Pressure

Temperature

Density

Velocity

Acceleration

Currency (отдельная доменная категория)

---

# Default Units

Length → mm

Area → mm²

Volume → mm³

Angle → degree

Mass → kg

Time → second

Temperature → °C

---

# Conversion Rules

Все вычисления выполняются во внутренних единицах платформы.

Конвертация производится только на границе системы:

- UI;
- API;
- Import;
- Export.

---

# Precision

Geometry Precision

1e-6

Calculation Precision

Double Precision

Display Precision

Настраивается пользователем.

---

# Validation

Все значения обязаны иметь единицы измерения.

Сложение различных размерностей запрещено.

Конвертация должна быть обратимой.

---

# Acceptance Criteria

Все инженерные параметры имеют единицы измерения.

Поддерживается безопасная конвертация.

Все вычисления детерминированы.

---

APPROVED