# STAIR PLATFORM

Document: 11_TESTING_WORKFLOW.md

ID: DEV-0011

Status: APPROVED

---

# Purpose

Определяет testing workflow в процессе разработки.

---

# Principle

Testing является частью implementation workflow, а не отдельным этапом после разработки.

---

# Workflow

```text
Implementation
    ↓
Unit Tests
    ↓
Integration Tests
    ↓
API Tests
    ↓
E2E Tests
    ↓
Regression
```

---

# Unit Tests

Используются для проверки:

* Domain Logic
* Calculation Logic
* Validation
* Pure Functions
* Isolated Components

---

# Integration Tests

Используются для проверки взаимодействия:

* Database
* Repositories
* Services
* External Dependencies

---

# API Tests

Проверяются:

* Request Validation
* Authentication
* Authorization
* Response Contract
* Error Handling

---

# E2E Tests

Проверяются критические пользовательские workflows.

---

# Regression Tests

После изменения существующего behavior должны выполняться соответствующие regression tests.

---

# Test Ownership

Автор изменения отвечает за добавление или обновление необходимых tests.

---

# Test Failure

Failed mandatory tests блокируют integration.

---

# Flaky Tests

Нестабильные tests должны быть:

* исправлены
* изолированы
* временно отключены только с documented reason

---

# Acceptance Criteria

Каждое значимое изменение имеет соответствующий testing scope, а mandatory failures блокируют integration.

---

APPROVED
