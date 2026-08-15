# STAIR PLATFORM

**Document:** EDR-0037_AI_Engineering_Assistant.md

**ID:** EDR-0037

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** AI

---

# 1. Purpose

Документ фиксирует Phase D, элемент D2 — **AI Engineering Assistant**:
инженерно-конструкторский анализ конкретной конфигурации лестницы на
соответствие нормативам STANDARD (EDR-0002) с формированием замечаний,
подсказок и общей оценки применимости.

Вход — `AnalysisRequest{Config, Options}` (тип марша и геометрические
параметры). Единственный вычислительный шаг — полный конвейер
`stair.Service.Calculate` через тул; анализатор не считает ничего сам
(AI-0000). Результат — структурный `Response` + комментарий (локальный
или LLM с фолбэком, каркас D1/EDR-0036).

Ассистент применяет **только** нормативы из существующего профиля
`constraint.StandardProfile` (STANDARD): высота ступени 150–200 мм,
проступь 260–320 мм, угол наклона 30–45°, просвет ≥ 2000 мм, толщина
косоура ≥ 30 мм, ограждение ≥ 900 мм, плюс шаг комфорта 2h+b = 600–640 мм
(EDR-0001, доменный норматив). Значения норм высчитываются из данных
конвейера (solver-результат), а не дублируются из кода движка.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase D — AI: AI Engineering Assistant, D2)
- EDR-0036 (каркас AI Layer: Service/ModelRouter/Tools/audit — переиспользуется)
- EDR-0002 (нормативный профиль STANDARD — источник норм)
- EDR-0001 (шаг комфорта 2h+b)
- ADR-0003 (детерминизм), AI-0000 (истина в конвейере)
- `internal/application/assistant/engineering.go`, `design.go` (тулы и
  вспомогательные функции stepHeightOf/treadOf/comfortStepOf/angleDegOf),
  `internal/transport/http/assistant.go`

---

# 3. Model

## 3.1 Expert `engineeringExpert`

Алгоритм (детерминированный):

1. `ValidateConfig` — невалидный вход → `ErrInvalid` (422).
2. `Calculate` полного конвейера — ошибка конвейера → запрос провален.
3. **Blocking-валидация**: рекомендация «конфигурация не проходит
   блокирующую валидацию», Rating=0, замечания из
   `Validation.Issues`+`GeometryIssues`, подсказки по исправлению.
4. **Неблокирующая экспертиза**: замечания конвейера + нормативные
   проверки из данных solver-результата (h, b, S, α) и входной
   конфигурации (просвет/косоур/ограждение, только для заданных ненулевых
   значений).
5. `Rating` — мера применимости (0..1): доля прошедших нормативов;
   1 — полностью в норме.
6. Отдельная подсказка по комфорту (при отклонении S) с отсылкой к
   design-ассистенту.

## 3.2 Нормативные проверки (источник — STANDARD)

| элемент | норматив | откуда значение |
|---|---|---|
| высота ступени h | 150–200 мм | solver-результат |
| проступь b | 260–320 мм | solver-результат |
| шаг комфорта S=2h+b | 600–640 мм | solver-результат |
| угол наклона α | 30–45° | solver-результат |
| просвет | ≥ 2000 мм | вход (если задан) |
| косоур | ≥ 30 мм | вход (если задан) |
| ограждение | ≥ 900 мм | вход (если задан) |

Каждое отклонение — `Finding{Severity:warning}` с конкретным числом и
нормативом; подсказки `Suggestion` ссылаются на правила STANDARD.

## 3.3 Transport

`POST /api/v1/assistant/engineering` (auth) — тело как у `stairs:calculate`
(включая опциональные rates). Ответ как у design: `{kind, response, commentary}`.
Обработчик собирает `AnalysisRequest` из DTO (включая `toOptions`).

## 3.4 Audit / metrics

Записи аудита `ai.assist.engineering` (ok/failed) и гистограмма
`stair_assistant_duration_seconds{kind="engineering",valid}` — штатно через
каркас D1 (EDR-0036 §3.1).

---

# 4. Invariants

1. **Истина в конвейере** (AI-0000): все числа — из `Calculate`;
   нормы — только из существующего `StandardProfile`.
2. **Детерминизм** (ADR-0003): фиксированный порядок проверок.
3. **Blocking ⇒ Rating 0**, рекомендация о блокирующей валидации.
4. **Нулевые входные параметры не проверяются** (просвет/косоур/ограждение
   с 0 мм пропускаются — означает «не задано»).
5. **Комментарий не влияет на структуру** (каркас D1).

---

# 5. Non-goals

- Генерация исправленной конфигурации (вручную — синхронно); подбор
  оптимума — дизайн-ассистент (E1) или `stairs:optimize`.
- Manufacturing/Pricing анализ — D3/D4.
- Новые нормативные профили/EDR-0002 расширения.

---

# 6. Decision

Реализовано в Phase D, D2 (commit `feat(d2)`): `engineeringExpert` в
`internal/application/assistant/engineering.go` (blocking- и
неблокирующая ветка, нормативы STANDARD, rating, подсказки),
`AnalysisRequest` в `entity.go`, маршрут `POST /api/v1/assistant/engineering`
в транспортном слое, unit-тесты (соответствие нормативам, blocking) и
транспортный тест; EDR-0037 и метка ROADMAP.