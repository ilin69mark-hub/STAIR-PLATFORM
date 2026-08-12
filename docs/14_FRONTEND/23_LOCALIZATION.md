# STAIR PLATFORM

Document: 23_LOCALIZATION.md

ID: FE-0023

Status: APPROVED

---

# Purpose

Определяет архитектуру локализации Frontend.

---

# Supported Languages

Russian

English

German

---

# Localization Scope

UI

Messages

Errors

Units

Dates

Numbers

Documentation

---

# Locale Resolution

User Preference

↓

Project Preference

↓

Browser Locale

↓

Default Locale

---

# Rules

Тексты интерфейса не хранятся непосредственно в компонентах.

Все строки используют Localization Layer.

---

# Engineering Data

Числовые значения отображаются с учетом локали.

Единицы измерения управляются Unit System.

---

# Acceptance Criteria

Frontend поддерживает переключение языка без изменения архитектуры приложения.

---

APPROVED