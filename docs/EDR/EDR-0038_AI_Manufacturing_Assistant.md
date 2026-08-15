# STAIR PLATFORM

**Document:** EDR-0038_AI_Manufacturing_Assistant.md

**ID:** EDR-0038

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** AI

---

# 1. Purpose

Документ фиксирует Phase D, элемент D3 — **AI Manufacturing Assistant**:
оценка производственной готовности конфигурации по данным Manufacturing
Platform (BC-007): полнота и качество раскроя, доля отходов, номенклатура
листовых материалов, размер партии деталей, оценочное машинное/ручное
время. Ассистент не пересчитывает производственную модель сам — «истина»
в `ManufacturingPackage`/`ManufacturingCostDataset`, полученных полным
конвейером `stair.Service.Calculate` через тул (AI-0000).

Анализ ведётся по фиксированным (детерминированным) порогам, зафиксированным
в этом EDR:

| метрика | порог | замечание |
|---|---|---|
| утилизация листа | < 70% | warning «раскрой» |
| доля отходов WasteArea/SheetArea | > 25% | warning «отходы» |
| марок листов | > 2 | info «материалы» (консолидация) |
| БОМ пуст | — | info «спецификация» |
| машинное время > 80% цикла | — | info «время» |

Оценка `Rating` (0..1): база из утилизации (0.4 + util·0.6), штраф 0.15 за
каждое warning. `Recommendation` — «готово» / «требуется доработка»
(с числом замечаний).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase D — AI: AI Manufacturing Assistant, D3)
- EDR-0036/0037 (каркас AI Layer: Service/ModelRouter/Tools/audit,
  `AnalysisRequest` — переиспользуются)
- BC-007/EDR-0010 (Manufacturing Platform), MFG-0012 (NestingResult),
  MFG-0001 (BOM/CutList), MFG-0005 (MaterialCode)
- ADR-0003 (детерминизм), AI-0000 (истина в конвейере)
- `internal/domain/manufacturing/{package,bom,nesting,cost}.go`,
  `internal/application/assistant/manufacturing.go`

---

# 3. Model

## 3.1 Expert `manufacturingExpert`

Алгоритм (детерминированный):

1. `ValidateConfig` → `ErrInvalid` при невалидном входе.
2. `Calculate` полного конвейера.
3. **Конвейер остановлен** (`Blocking` или `Package == nil`): рекомендация
   «производственная подготовка недоступна», Rating=0, замечания
   валидации + подсказки.
4. **Анализ пакета** (см. §1 пороги): замечания по раскрою/отходам/
   материалам/БОМ/времени.
5. `Rating`: `clamp01(0.4 + util·0.6 − 0.15·warnings)`.
6. `Suggestion` по низкой утилизации (скорректировать габариты деталей или
   стандартный лист) и консолидации марок; `Notes` с ключевыми метриками
   (детали/БОМ/листы/утилизация/площадь, время цикла, масса).
7. Единственный чистый (pure) расчёт — доли/площади из данных пакета
   (не инженерия).

## 3.2 Transport

`POST /api/v1/assistant/manufacturing` (auth) — тело как у
`stairs:calculate` (включая rates). Обработчик собирает `AnalysisRequest`
через `toOptions`. Ответ: `{kind, response{...}, commentary}`.

## 3.3 Audit / metrics

Записи `ai.assist.manufacturing` (ok/failed) и гистограмма
`stair_assistant_duration_seconds{kind="manufacturing",valid}` — штатно
через каркас (EDR-0036 §3.1).

---

# 4. Invariants

1. **Истина в конвейере**: экспертиза читает только `Package`/`Cost` из
   `Calculate`; пороги — константы пакета, зафиксированы в §1.
2. **Детерминизм**: фиксированные пороги и порядок проверок (ADR-0003).
3. **Blocking/без пакета ⇒ Rating 0** и отсутствие производственных чисел.
4. **Комментарий не влияет на структуру** (каркас D1).
5. Пользовательские rates не меняют производственный анализ (только цена,
   D4) — analysis согласован.

---

# 5. Non-goals

- Перерасчёт раскроя/времени — существующие движки Manufacturing/Cost.
- Ценовая экспертиза — D4/EDR-0039.
- Оптимизация раскроя (вне MVP; только подсказка).

---

# 6. Decision

Реализовано в Phase D, D3 (commit `feat(d3)`): `manufacturingExpert` в
`internal/application/assistant/manufacturing.go` (blocking-ветка,
пороговый анализ раскроя/отходов/материалов/БОМ/времени, rating из
утилизации, подсказки и ключевые метрики), маршрут
`POST /api/v1/assistant/manufacturing` в транспортном слое, unit-тесты
(готовность, низкая утилизация, blocking) и транспортный тест;
EDR-0038 и метка ROADMAP.