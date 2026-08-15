# STAIR PLATFORM

**Document:** EDR-0024_CRM_Integration.md

**ID:** EDR-0024

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Integrations

---

# 1. Purpose

Документ фиксирует CRM-интеграцию (Phase E, E3) — синхронизацию проектной
информации во внешнюю CRM-систему по защищённому webhook. E3 использует
webhook-каркас, построенный в E2 (EDR-0023): WebhookClient c HMAC-SHA256
подписью, реестр эндпоинтов `integration_endpoints` и события доставки
`integration_events` (миграция `000014_integrations`).

CRM (в отличие от ERP quote в E2) синхронизирует **метаданные проекта**
(название, описание, статус жизненного цикла, владелец), чтобы внешняя
CRM могла отслеживать проект как сделку/объект:

1. **Событие** `crm.project_sync` — канонический документ проекта (проекция
   `project.Project`, EDR-0010 статусы).
2. **Транспорт** — `POST /api/v1/projects/{id}/crm-sync` (owner/editor, как
   quote-send в E2).
3. **Доставка** — то же задание из каркаса E2 (`JobQuoteSend` рефакторится в
   общий `JobEventDeliver` на событийный маршрут `crm.project_sync`):
   retry/backoff/max-attempts/DLQ политика EDR-0020.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase E — Integrations: CRM)
- EDR-0020 (Distributed Infrastructure — JobQueue/worker ретрай)
- EDR-0023 (ERP Integration — webhook-каркас E2, переиспользуется)
- API-0015 (Webhooks: HMAC, retry, exponential backoff, DLQ)
- API-0014 (Event API: immutable events, idempotent consumers)
- ADR-0006 (Layered Architecture)
- DEV-0009 (Offline builds — только stdlib)

---

# 3. Model

## 3.1 Событие

| Поле | Значение |
|------|----------|
| event_type | `crm.project_sync` |
| endpoint kind | `crm` |
| payload | канонический документ проекта (см. §3.3) |
| инициируется | `POST /api/v1/projects/{id}/crm-sync` |

Жизненный цикл события идентичен E2: `pending` → (воркер) → `delivered` /
`failed` → `dlq` при исчерпании попыток (EDR-0020).

## 3.2 Прикладной сервис

В `internal/application/integrations` добавляется метод:

```go
// SyncProject ставит задание crm.project_sync для проекта
// (EDR-0024): event-строка + payload=документ проекта.
func (s *Service) SyncProject(ctx, tenantID, projectID string, payload []byte) (*Delivery, error)
```

`SyncProject` симметричен `SendQuote` (E2): находит единственный активный
эндпоинт `kind=crm` для tenant (нет → `ErrNoEndpoint`), создаёт
`integration_events` (status=`pending`, event_type=`crm.project_sync`) и
enqueue задание с payload `{event_id, endpoint_id, tenant_id}`.

Для устранения дублирования постановки в очередь общая логика
`SendQuote`/`SyncProject` выносится в приватный `enqueue(ctx, tenantID,
projectID string, kind Kind, eventType string, payload []byte)`.

## 3.3 Документ проекта (payload)

Канонический документ — проекция `project.Project` (edr-0023 конвенция
канонических документов):

| Поле | JSON | Источник |
|------|------|----------|
| project_id | `project_id` | Project.ID |
| name | `name` | Project.Name |
| description | `description` | Project.Description |
| status | `status` | Project.Status (EDR-0010) |
| owner_id | `owner_id` | Project.OwnerID |
| created_at | `created_at` | Project.CreatedAt (RFC3339) |
| updated_at | `updated_at` | Project.UpdatedAt (RFC3339) |

## 3.4 Воркер

`cmd/worker/registry.go`: обработчик `crm.project_sync` использует ту же
процедуру доставки, что и `erp.quote_send` (прочитать delivery+endpoint →
WebhookClient.Send → `MarkDelivered`/`MarkFailed`). Реализация общего
обработчика `deliverEvent` (renamed из `quoteSend`), реестр заданий выводит
оба типа на него.

## 3.5 Транспорт

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| POST | `/api/v1/projects/{id}/crm-sync` | owner/editor | Синхронизация проекта в CRM |

Payload формируется транспортом из `projects.GetProject` (членство проверяет
сервис проектов). Ответ 202 — `deliveryDTO`; 403 — нет доступа к проекту;
404 — нет проекта; 422 — нет CRM-эндпоинта (`no_endpoint`).

---

# 4. Invariants

```
1. Подпись HMAC-SHA256 over (timestamp + "." + body); constant-time проверка.
2. Доставка асинхронна через JobQueue: retry/backoff/max-attempts/DLQ (EDR-0020).
3. payload = канонический документ проекта (EDR-0024 §3.3), события идемпотентны.
4. CRM-эндпоинт — единственный активный kind=crm для tenant (ErrNoEndpoint).
5. Секрет эндпоинта не логируется и не попадает в JSON-ответы.
6. Новая миграция не требуется: таблицы 000014 поддерживают kind=crm.
```

---

# 5. Schema

Изменений нет: `integration_endpoints.kind` и `integration_events.event_type`
(TEXT) уже моделируют `crm` и `crm.project_sync` (миграция `000014_integrations`).

---

# 6. API

`POST /api/v1/projects/{id}/crm-sync` — body пустой; ответ:

```json
{ "id": "<event_id>", "endpoint_id": "<ep_id>", "project_id": "<p_id>",
  "event_type": "crm.project_sync", "status": "pending", "created_at": "..." }
```

---

# 7. Tests

1. `application/integrations` — SyncProject создаёт event `crm.project_sync` +
   задание; ErrNoEndpoint без crm-эндпоинта; без очереди — ошибка.
2. `cmd/worker` — доставка `crm.project_sync` на stub-клиенте: delivered/failed.
3. Транспорт — crm-sync: 202/403/404/422; payload содержит поля документа.
4. Рефактор `SendQuote`→`enqueue` верифицируется существующими тестами E2
   (erp.quote_send демостронстрация той же ветки доставки).

---

APPROVED