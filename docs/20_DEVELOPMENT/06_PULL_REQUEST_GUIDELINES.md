# STAIR PLATFORM

Document: 06_PULL_REQUEST_GUIDELINES.md

ID: DEV-0006

Status: APPROVED

---

# Purpose

Определяет правила подготовки Pull Request.

---

# Objective

Pull Request должен описывать изменение, а не только содержать код.

---

# PR Structure

Каждый PR содержит:

* Summary
* Motivation
* Scope
* Affected Components
* Testing
* Risks
* Related Task

---

# Summary

Краткое описание изменения.

---

# Motivation

Почему изменение необходимо.

---

# Scope

Что входит и что не входит в данный PR.

---

# Testing

Указываются выполненные проверки:

* Unit
* Integration
* API
* E2E where applicable

---

# Risks

Описание потенциальных рисков изменения.

---

# Size

Предпочтительны небольшие PR.

Очень большие изменения разбиваются на несколько независимых PR.

---

# Review Readiness

PR считается готовым только если:

* проходит локальную сборку
* проходит тесты
* не содержит незавершенного debug-кода
* обновлена документация при необходимости

---

# Draft PR

Draft используется для раннего обсуждения архитектуры.

---

# Acceptance Criteria

Каждый PR предоставляет достаточный контекст для независимого review.

---

APPROVED
