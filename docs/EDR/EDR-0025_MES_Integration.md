# STAIR PLATFORM

**Document:** EDR-0025_MES_Integration.md

**ID:** EDR-0025

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Integrations

---

# 1. Purpose

Документ фиксирует MES-интеграцию (Phase E, E4) — передачу производственного
заказа (manufacturing package) во внешнюю MES-систему по защищённому webhook.
E4 использует webhook-каркас E2 (EDR-0023): WebhookClient с HMAC-SHA256
подписью, реестр эндпоинтов `integration_endpoints` и события доставки
`integration_events` (миграция `000014_integrations`), а также общий
обработчик доставки `deliverEvent` (EDR-0024 §3.4).

MES (в отличие от CRM в E3) синхронизирует **производственные данные**
посчитанного проекта — комплект Parts/BOM/CutList/Nesting (BC-007):

1. **Событие** `mes.order_send` — канонический документ производственного
   заказа (проекция `Snapshot.Manufacturing`, EDR-0022 §3.1).
2. **Транспорт** — `POST /api/v1/projects/{id}/order-send` (owner/editor, как
   quote-send в E2/crm-sync в E3).
3. **Доставка** — общий воркер-обработчик `deliverEvent`: retry/backoff/
   max-attempts/DLQ (EDR-0020).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase E — Integrations: MES)
- EDR-0020 (Distributed Infrastructure — JobQueue/worker ретрай)
- EDR-0023 (ERP Integration — webhook-каркас E2, переиспользуется)
- EDR-0024 (CRM Integration — общий `enqueue`/`deliverEvent`, переиспользуется)
- API-0015 (Webhooks: HMAC, retry, exponential backoff, DLQ)
- API-0014 (Event API: immutable events, idempotent consumers)
- ADR-0006 (Layered Architecture)
- DEV-0009 (Offline builds — только stdlib)

---

# 3. Model

## 3.1 Событие

| Поле | Значение |
|------|----------|
| event_type | `mes.order_send` |
| endpoint kind | `mes` |
| payload | канонический документ производственного заказа (см. §3.3) |
| инициируется | `POST /api/v1/projects/{id}/order-send` |

Жизненный цикл события идентичен E2/E3: `pending` → (воркер) → `delivered` /
`failed` → `dlq` при исчерпании попыток (EDR-0020).

## 3.2 Прикладной сервис

В `internal/application/integrations` добавляется метод:

```go
// SendManufacturingOrder ставит задание mes.order_send для проекта
// (EDR-0025): event-строка + payload=документ производственного заказа.
func (s *Service) SendManufacturingOrder(ctx, tenantID, projectID string, payload []byte) (*Delivery, error)
```

`SendManufacturingOrder` симметричен `SendQuote` (E2)/`SyncProject` (E3):
через общий `enqueue(ctx, tenantID, projectID, kind=mes, event_type,
payload)` (EDR-0024 §3.2) находит единственный активный эндпоинт `kind=mes`
для tenant (нет → `ErrNoEndpoint`), создаёт `integration_events`
(status=`pending`) и enqueue задание `{event_id, endpoint_id, tenant_id}`.

## 3.3 Документ производственного заказа (payload)

Канонический документ — проекция `Snapshot.Manufacturing`
(`manufacturing.ManufacturingPackage`, EDR-0022 §3.1, BC-007), дополненная
идентификатором проекта:

| Поле | JSON | Источник |
|------|------|----------|
| project_id | `project_id` | Project.ID |
| parts | `parts` | ManufacturingPackage.Parts |
| bom | `bom` | ManufacturingPackage.BOM.Lines |
| cut_list | `cut_list` | ManufacturingPackage.CutList.Items |
| nesting | `nesting` | ManufacturingPackage.Nesting |

Поля деталей/строк сериализуются в snake_case (номер, тип, материал, толщина
и габариты в мм, количество) для консистентности с документом EDR-0024 §3.3.
Отсутствующий manufacturing package (проект без валидного расчёта) — ошибка
422 `no_manufacturing`.

## 3.4 Воркер

`cmd/worker/registry.go`: `JobOrderSend = "mes.order_send"` добавляется к
ветке общего обработчика `deliverEvent` (EDR-0024 §3.4) — нового кода
доставки не требуется.

## 3.5 Транспорт

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| POST | `/api/v1/projects/{id}/order-send` | owner/editor | Передача производственного заказа в MES |

Payload формируется транспортом: `projects.GetResult` → разобрать
`Snapshot.Manufacturing` → собрать документ заказа. Ответ 202 —
`deliveryDTO`; 403 — нет доступа; 404 — нет проекта/расчёта;
422 — нет MES-эндпоинта (`no_endpoint`) или manufacturing package
(`no_manufacturing`).

---

# 4. Invariants

```
1. Подпись HMAC-SHA256 over (timestamp + "." + body); constant-time проверка.
2. Доставка асинхронна через JobQueue: retry/backoff/max-attempts/DLQ (EDR-0020).
3. payload = канонический документ производственного заказа (EDR-0025 §3.3),
   события идемпотентны по event_id.
4. MES-эндпоинт — единственный активный kind=mes для tenant (ErrNoEndpoint).
5. Без валидного manufacturing package (package отсутствует) событие не
   создаётся (422 no_manufacturing).
6. Секрет эндпоинта не логируется и не попадает в JSON-ответы.
7. Новая миграция не требуется: таблицы 000014 поддерживают kind=mes.
```

---

# 5. Schema

Изменений нет: `integration_endpoints.kind` и `integration_events.event_type`
(TEXT) уже моделируют `mes` и `mes.order_send` (миграция `000014_integrations`).

---

# 6. API

`POST /api/v1/projects/{id}/order-send` — body пустой; ответ:

```json
{ "id": "<event_id>", "endpoint_id": "<ep_id>", "project_id": "<p_id>",
  "event_type": "mes.order_send", "status": "pending", "created_at": "..." }
```

---

# 7. Tests

1. `application/integrations` — SendManufacturingOrder создаёт event
   `mes.order_send` + задание; ErrNoEndpoint без mes-эндпоинта; без очереди —
   ошибка.
2. `cmd/worker` — доставка `mes.order_send` через общий deliverEvent на
   stub-клиенте: delivered/failed.
3. Транспорт — order-send: 202/403/404/422 (no_endpoint, no_manufacturing);
   payload содержит документ заказа.
4. Регрессия E2/E3: erp.quote_send/crm.project_sync продолжают проходить
   через общий `deliverEvent`.

---

APPROVED