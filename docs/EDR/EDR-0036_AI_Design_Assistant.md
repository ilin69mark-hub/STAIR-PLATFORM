# STAIR PLATFORM

**Document:** EDR-0036_AI_Design_Assistant.md

**ID:** EDR-0036

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** AI

---

# 1. Purpose

Документ фиксирует Phase D, элемент D1 — **AI Design Assistant**
(EDR-0036) — и одновременно «фундацию» AI Layer: общий каркас ассистентов
(`internal/application/assistant`), который будут переиспользовать
Engineering (D2, EDR-0037), Manufacturing (D3, EDR-0038) и Pricing
(D4, EDR-0039).

Цель D1 — дать пользователю интеллектуальную рекомендацию по выбору типа
лестничного марша: детерминированный перебор маршей (прямой, L-, П-,
спиральный), оптимизация каждого через существующий движок
(`stair.Service.Optimize`, EDR-0032) и ранжирование по приоритету
(цена/себестоимость/материал/комфорт).

Модель AI Layer, фиксируемая этим EDR (общая для всех ассистентов D):

- **AI не является источником истины** (AI-0000): ассистент не содержит
  предметной бизнес-логики — вся «истина» в существующих конвейерах.
- **AI работает только через Tool Calling**: единственные действия —
  `Calculate`/`Optimize`/`ValidateConfig` порта `stairCalculator`
  (обычно `*stair.Service`).
- **Ответ двухчастный** (ADR-0003): детерминированная структура
  (Recommendation/Rating/Alternatives/Findings/Suggestions/Tradeoffs/Notes)
  строится из результатов тулов; текстовый комментарий — локальным
  детерминированным генератором либо, при конфигурации `STAIR_AI_*`,
  первичным OpenAI-совместимым бэкендом с фолбэком на локальный (AI-0003:
  primary → local → error).
- **Полный аудит** (AI-0001): каждый запрос пишет событие аудита
  `ai.assist.<kind>` (result ok/failed) через существующий `audit.Service`
  (EDR-0013). Аудит best-effort: сбой журнала не ломает ответ.
- **Наблюдаемость**: гистограмма `stair_assistant_duration_seconds` по
  `kind`/`valid`.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase D — AI: AI Design Assistant, D1)
- AI-0000 (документ 00_AI_MANIFEST) — AI не источник истины, только Tool
  Calling, аудируемость
- AI-0003 (03_MODEL_ROUTER) — fallback primary → local → error
- AI-0017 (17_AI_CONFIGURATION) — конфигурация через env (`STAIR_AI_*`)
- EDR-0032 (Optimization) — `stair.Service.Optimize`, целевые метрики
- ADR-0003 (детерминизм) — фиксированный порядок перебора, стабильная
  сортировка, Temperature=0 у LLM
- EDR-0013 (Audit System) — `audit.Event`, best-effort запись
- `internal/application/stair` (конвейер), `internal/application/audit`
  (аудит), `internal/transport/http` (маршруты)

---

# 3. Model

## 3.1 Package `internal/application/assistant`

Мета-уровень, не зависит от HTTP/БД/UI (ADR-0006). Слои внутри пакета:

- `entity.go` — `Kind` (design|engineering|manufacturing|pricing с
  `Action()` → `ai.assist.*`), `Request`, `Response`, `Result`,
  `Finding`/`Suggestion`/`Alternative`, `intent`, `Service`.
- `Service.Ask(ctx, tenantID, userID, kind, req)` — единая точка входа:
  планирование → `expert.analyze(tools, req)` (Tool Calling) → сборка
  `Response` → `ModelRouter.Infer` (комментарий) → аудит + метрика.
- `tools.go` — порт `stairCalculator` и `Tools` (Calculate/Optimize/
  ValidateConfig).
- `router.go` — `Backend`, `Prompt`, `Answer`, `ModelRouter` (primary
  → local → error).
- `comments.go` — `localComment` — детерминированный русскоязычный
  генератор комментария из `Response` (дефолтный бэкенд); `localFallback`
  — страховка от пустоты.
- `openai.go` — первичный бэкенд `OpenAI` (Chat Completions API,
  stdlib-only), подключается только при `STAIR_AI_BASE_URL`.
- `design.go` — D1-эксперт (см. §3.2).
- `metrics.go` — гистограмма `stair_assistant_duration_seconds{kind,valid}`.

`Service` инверсирует зависимости: получает `stairCalculator` и
опциональный `*audit.Service`; тесты подменяют порт фейком без перезаписи
логики экспертов.

## 3.2 Design Expert (D1)

Вход — `DesignRequest{Config stair.Config, Preferences{Priority}}`.
Алгоритм (детерминированный, ADR-0003):

1. **Приоритет**: `price` (по умолчанию) / `cost` / `material` /
   `comfort`; mapping `comfort → TargetPrice` (оптимизатор не имеет цели
   «комфорт» — для comfort оптимизируем по цене, а ранжируем варианты по
   шагу 2h+b).
2. **Валидация**: `ValidateConfig` — невалидный вход → `ErrInvalid` (422).
3. **Перебор типов марша** в фиксированном порядке (straight → l_shape →
   u_shape → spiral); для спирали без наружного радиуса подставляется
   детерминированная эвристика `R = 2·W` (с пометкой в Notes); тип не
   выполнимый при данных параметрах — пропускается.
4. **Оптимизация** каждого типа через тул `Optimize` (целевая метрика из
   приоритета) с выводом полного расчёта лучшей конфигурации.
5. **Ранжирование** стабильной сортировкой по метрике приоритета:
   цена/себестоимость/материал — Objective оптимизатора; комфорт —
   отклонение `|630 − S|` от центра норматива 600–640.
6. **Ответ**: `Recommendation` (тип + n/h/b/S/цена), `Rating` (0..1),
   `Alternatives` (остальные типы с рейтингом и причиной),
   `Findings` (валидационные/геометрические замечания лучшего кандидата),
   `Tradeoffs` (ценовые/комфортные различия), `Notes`.

Нет допустимых типов → `ErrNoFeasible` (422).

## 3.3 Model Router (комментарий)

- По умолчанию используется локальный детерминированный комментатор
  (`localComment`) — платформа работает offline без LLM.
- При наличии `STAIR_AI_BASE_URL` (и опционально `STAIR_AI_API_KEY`/
  `STAIR_AI_MODEL`) подключается первичный `OpenAI`-бэкенд: POST
  `{base}/chat/completions`, `temperature:0`, Bearer-авторизация,
  таймаут 15s, лимит ответа 256 KiB. Не влияет на структурный ответ.
- Фолбэк: при сбое или пустом ответе первичного бэкенда комментарий
  генерируется локально (AI-0003); сбой локального — провал запроса.
- Промпт собирается из `intent`: kind, системная инструкция, задача
  пользователя, нарратив тулов и строгий JSON-контекст. Персональные и
  аудируемые данные в промпт не попадают (AI-0012).

## 3.4 Transport

- `POST /api/v1/assistant/{kind}` (auth; kind ∈ design|engineering|
  manufacturing|pricing) — тело: поля расчёта (как у `stairs:calculate`)
  + `priority` (для design). Ответ `200`: `{kind, response{...}, commentary}`.
  D1 реализует только `design`; остальные kind вернут 422 до D2–D4.
- Конфигурация маршрута — `Config.Assistant` (nil → маршруты не
  регистрируются). Аутентификация — существующий `requireAuth`
  (session-cookie или API-ключ); tenant/actor берутся из контекста,
  никогда из тела (SEC-0013).
- Проводка в `cmd/api`: `assistant.NewService(stairSvc, auditSvc)`, при
  `STAIR_AI_BASE_URL` → `WithPrimaryBackend(oai)`.

## 3.5 Audit

Новые действия: `ai.assist.design`, `ai.assist.engineering`,
`ai.assist.manufacturing`, `ai.assist.pricing` (анонсированы в D1, фактические
записи для engineering/manufacturing/pricing появятся в D2–D4). События:
result=ok (detail=контекст тулов) / result=failed (detail=ошибка).

---

# 4. Invariants

1. **Истина только в конвейере** (AI-0000): эксперт не рассчитывает ничего
   сам — только вызывает тулы `stair.Service`.
2. **Детерминизм** (ADR-0003): фиксированный порядок перебора, стабильная
   сортировка, `Temperature=0`; повторный запрос идентичен.
3. **Комментарий не влияет на структуру**: `Response` строится из тулов
   до `ModelRouter`; сбой LLM меняет только текстовую часть.
4. **Локальный бэкенд всегда доступен**: без `STAIR_AI_*` платформа
   полностью работоспособна (offline-режим).
5. **Аудит best-effort**: сбой `audit.Record` не проваливает ответ
   ассистента.
6. **Триада результатов**: рекомендация старше всех альтернатив, rating
   ∈ [0,1], commentary непустой.

---

# 5. Non-goals

- Engineering/Manufacturing/Pricing ассистенты — D2–D4 (тот же каркас,
  свои эксперты и EDR).
- RAG/память/агентские циклы/онлайн-обучение (AI-0006..0011) — вне
  текущей модели; ассистенты stateless.
- Генерация кода, мутация доменных данных через AI — запрещено (AI-0000).
- Локальный LLM (Ollama/ONNX) как встроенный — потенциально через тот же
  OpenAI-совместимый интерфейс.

---

# 6. Decision

Реализовано в Phase D, D1 (commit `feat(d1)`): новый пакет
`internal/application/assistant` (entity/metrics/tools/router/comments/
design/openai), `Service`/`ModelRouter`/`DesignExpert`, тулы поверх
`stair.Service`, option-бэкенд `OpenAI`, audit-действия `ai.assist.*`,
эндпоинт `POST /api/v1/assistant/design`, проводка в `cmd/api`,
unit-тесты эксперта/роутера/OpenAI и транспортные тесты; метка D1 в
ROADMAP.