# STAIR PLATFORM

**Document:** EDR-0039_AI_Pricing_Assistant.md

**ID:** EDR-0039

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** AI

---

# 1. Purpose

Документ фиксирует Phase D, элемент D4 (завершающий Phase D) — **AI Pricing
Assistant**: экономическая экспертиза конфигурации по структуре цены
(`PriceBreakdown`, PRC-0004): доля материалов/машины/труда/накладных в
себестоимости, адекватность маржи, удельная цена на м² проступи и на кг,
тонкая маржа. Ассистент не пересчитывает цену — «истина» в PriceBreakdown
полного конвейера (AI-0000). Пороги — эмпирические, зафиксированы в EDR.

# 2. Related Artifacts

- ROADMAP-0011 (Phase D — AI: AI Pricing Assistant, D4)
- EDR-0036/0037/0038 (каркас AI Layer: Service/ModelRouter/Tools/audit,
  `AnalysisRequest` — переиспользуются)
- PRC-0004 (PriceBreakdown, Cost Categories), PRC-0002 (конвейер цены),
  EDR-0010/BC-007 (производственный пакет: PartArea/Mass)
- ADR-0003 (детерминизм), AI-0000 (истина в конвейере)
- `internal/domain/pricing/pricing.go`,
  `internal/application/assistant/pricing.go`

---

# 3. Model

## 3.1 Expert `pricingExpert`

Алгоритм (детерминированный):

1. `ValidateConfig` → `ErrInvalid` при невалидном входе.
2. `Calculate` полного конвейера (включая цену).
3. **Конвейер остановлен/`Price == nil`**: рекомендация «ценовая экспертиза
   недоступна», Rating=0, замечания валидации + подсказки; intent без
   ценовых данных (nil-безопасно).
4. **Анализ структуры цены** (пороги, все — в §3.3):
   - доля материалов в себестоимости > 60% → warning «материалы»;
   - накладные > 30% себестоимости → info «накладные»;
   - маржа > 40% итоговой цены → info «маржа» (проверить адекватность);
   - маржа < 10% (но > 0) → warning «маржа» (тонкая прибыль);
   - нулевая себестоимость → warning.
5. `Rating` (0..1): 1.0 − 0.3·warning − 0.1·info (clamped).
6. `Recommendation` (сбалансирована / требует внимания), `Suggestions`
   (снижение материалоёмкости, пересмотр тонкой маржи), `Notes`
   (итог/себестоимость/маржа%/удельная цена на м² и кг).

## 3.2 Transport

`POST /api/v1/assistant/pricing` (auth) — тело как у `stairs:calculate`
(включая rates; дефолтные ставки — те же, что в расчёте). Обработчик
собирает `AnalysisRequest` через `toOptions`. Ответ:
`{kind, response{...}, commentary}`.

## 3.3 Пороги ценовой экспертизы (фиксированы)

| метрика | порог | severity/элемент |
|---|---|---|
| доля материалов в ProductionCost | > 60% | warning «материалы» |
| доля накладных в ProductionCost | > 30% | info «накладные» |
| доля маржи в FinalPrice | > 40% | info «маржа» |
| доля маржи в FinalPrice | < 10% | warning «маржа» |
| ProductionCost | ≤ 0 | warning «себестоимость» |

Разумные ставки по умолчанию дают материалоёмкость ≤ 60% и маржу 10–40%;
девиация означает нестандартную конфигурацию или ставки — ассистент даёт
подсказку.

## 3.4 Audit / metrics

Записи `ai.assist.pricing` (ok/failed), гистограмма
`stair_assistant_duration_seconds{kind="pricing",valid}` — штатно через
каркас (EDR-0036 §3.1). С Phase D завершаются все 4 action AI-аудита.

---

# 4. Invariants

1. **Истина в конвейере**: экспертиза читает только `PriceBreakdown`/
   `ManufacturingCostDataset` из `Calculate`; пороги — константы пакета.
2. **Детерминизм** (ADR-0003): фиксированные пороги и порядок проверок.
3. **Без цены ⇒ Rating 0** и отсутствие финансовых чисел; intent без
   dereference nil-Price.
4. **Комментарий не влияет на структуру** (каркас D1).
5. Пользовательские rates влияют на цену так же, как в расчёте, — анализ
   согласован с итогом.

---

# 5. Non-goals

- Пересформирование цены/ставок — существующий движок Pricing.
- Оптимизация по цене — `stairs:optimize` (target=price) или design.
- Скидки/промо/валютная конверсия — вне MVP (PRC-0013).
- Рекомендации по «снижению» себестоимости на уровне материалов — отсылка
  к manufacturing-ассистенту.

---

# 6. Decision

Реализовано в Phase D, D4 (commit `feat(d4)`): `pricingExpert` в
`internal/application/assistant/pricing.go` (структура цены, пороги, rating,
подсказки, удельные метрики), маршрут `POST /api/v1/assistant/pricing`,
unit-тесты (сбалансированная/материалоёмкая/тонкая маржа/blocked,
nil-Price intent) и транспортный тест; EDR-0039 и метка ROADMAP.
**Phase D — полностью CLOSED.**