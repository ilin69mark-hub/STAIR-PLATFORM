# STAIR PLATFORM

**Document:** EDR-0003_Validation_Model.md

**ID:** EDR-0003

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-10

**Category:** Engineering

---

# 1. Purpose

Документ фиксирует правила валидации параметрической модели лестницы
и формат результата Validation Engine (`internal/engine/validation`).

---

# 2. Related Artifacts

- ADR-0008 (Engineering Coordinate and Units System)
- BC-003 (Constraints Bounded Context)
- EDR-0002 (Нормативные ограничения геометрии)
- FR-100 (Проверка ограничений)
- DOM-0014 (Domain Rules: расчёт только после Validation)

---

# 3. Validation Flow

```
+----------------+     +-----------------+     +---------------+
| Parameter Model| --> | ValidationEngine| --> | Result (issues)|
+----------------+     +-----------------+     +---------------+
        |                        |                       |
        | применяет             compares                | severity
        +--> active ConstraintSet (BC-003)              v
```

- Вход: активный `ConstraintSet` + параметрическая модель марша;
- Выход: упорядоченный список `ValidationIssue`;
- Валидация выполняется **перед** Solver и генерацией документов (DOM-0014).

---

# 4. ValidationIssue Model

```text
ValidationIssue {
    ID       : string   // ISSUE-<NNNN>, уникален (ADR-0001)
    Code     : RuleCode // ссылка на правило (EDR-0002)
    Severity : Error | Warning
    Element  : string   // id параметра/элемента
    Message  : string   // описание нарушения
    Value    : float64  // текущее значение
    Min      : float64  // граница диапазона (если есть)
    Max      : float64  // граница диапазона (если есть)
    Fix      : string   // suggested fix
}
```

---

# 5. Severity Rules

| Severity | Значение | Переход в Solver |
|----------|----------|------------------|
| Error | нарушение обязательного норматива | запрещён (blocking) |
| Warning | рекомендация / потенциальная проблема | разрешён |

Правила:

- наличие хотя бы одного `Error` ⇒ `Result.Valid = false`, `Blocking = true`;
- только `Warning` ⇒ `Result.Valid = true`, `Blocking = false`;
- пустой список ⇒ `Valid = true`, `Blocking = false`.

---

# 6. Determinism

- Порядок issue детерминирован (сортировка по Code);
- повторная валидация при неизменных входах даёт идентичный результат;
- валидация не изменяет модель (BC-003: Constraints не изменяет модель).

---

# 7. Tests

- каждый EDR-0002 constraint проверяется тестом;
- генерация severity (Error vs Warning);
- блокировка Solver при наличии Error;
- детерминизм вывода.

---

# 8. Acceptance Criteria

- Формат `ValidationIssue` реализован по §4;
- severity-правила по §5;
- интеграция с Constraint Engine (EDR-0002) и Solver (EDR-0001).

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-10 | Первоначальная редакция |

---

APPROVED