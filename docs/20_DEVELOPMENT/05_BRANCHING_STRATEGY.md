# STAIR PLATFORM

Document: 05_BRANCHING_STRATEGY.md

ID: DEV-0005

Status: APPROVED

---

# Purpose

Определяет стратегию ветвления исходного кода STAIR PLATFORM.

---

# Branch Model

Используется упрощенная trunk-based модель с защищенной основной веткой.

```text
main
 │
 ├── feature/*
 ├── bugfix/*
 ├── hotfix/*
 └── release/*
```

---

# Main Branch

`main` всегда должна содержать потенциально выпускаемую версию продукта.

Прямые коммиты запрещены.

---

# Feature Branch

Используется для новой функциональности.

Пример:

```text
feature/geometry-constraints
feature/pricing-engine
```

---

# Bugfix Branch

Используется для исправления ошибок без изменения архитектуры.

---

# Hotfix Branch

Используется только для критических production-проблем.

После релиза обязательно сливается обратно в `main`.

---

# Release Branch

Создается перед Production Release для стабилизации версии.

Разрешены только:

* bug fixes
* documentation
* release metadata

---

# Branch Lifetime

Feature branches должны быть короткоживущими.

Предпочтительны небольшие Pull Requests.

---

# Naming Rules

Используются только строчные буквы и `kebab-case`.

---

# Merge Policy

Разрешается только Merge после:

* Review
* CI
* Quality Gates

---

# Acceptance Criteria

Каждое изменение проходит через отдельную ветку и не попадает в `main` без review.

---

APPROVED
