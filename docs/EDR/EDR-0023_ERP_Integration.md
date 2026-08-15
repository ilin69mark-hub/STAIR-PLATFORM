# STAIR PLATFORM

**Document:** EDR-0023_ERP_Integration.md

**ID:** EDR-0023

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Integrations

---

# 1. Purpose

Документ фиксирует ERP-интеграцию (Phase E, E2) — отправку коммерческого
предложения (quote) досмотренного проекта во внешнюю ERP-систему по защищённому
webhook. Вместе с ним строится фундамент интеграций платформы (EDR-0022
заложил CAD; E2 добавляет каркас outgoing/incoming webhook):

1. **Webhook-платформа** — `internal/infrastructure/integrations`:
   WebhookClient (HMAC-SHA256-подпись, таймаут, ретрай через TaskQueue EDR-0020)
   и WebhookReceiver (проверка подписи входящих webhook).
2. **Модель данных** — регистрация интеграционных эндпоинтов и событий
   доставки (миграция `000014_integrations`).
3. **ERP quote** — задание `erp.quote_send` (JobQueue EDR-0020) и endpoint
   `GET /api/v1/projects/{id}/quote-send` (или внутренний триггер): сервис
   берёт результат расчёта (Snapshot, EDR-0022 §3.1), формирует payload и
   отдаёт его в ERP через webhook с exponential backoff и DLQ.

Первый релиз — **ERP quote**; CRM (E3), MES (E4), Storage (E5) и Payments (E6)
добавляют собственные типы событий на том же каркасе.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase E — Integrations: ERP)
- EDR-0020 (Distributed Infrastructure — JobQueue/worker ретрай)
- EDR-0022 (CAD Export — Snapshot как канонический результат)
- API-0012 (Partner API: quotation, webhook-based)
- API-0015 (Webhooks: HMAC, retry, exponential backoff, DLQ)
- API-0014 (Event API: immutable events, idempotent consumers)
- ADR-0006 (Layered Architecture)
- DEV-0009 (Offline builds — только stdlib)

---

# 3. Model

## 3.1 Webhook-клиент (outgoing)

`internal/infrastructure/integrations/client.go`:

```go
package integrations

// Client — HTTP-клиент для отправки webhook наружу (EDR-0023 §3.1).
// Подпись — HMAC-SHA256 over (timestamp + "." + body); заголовок
// X-Stair-Signature: t=<unix>.<hex>. Ретрай — по политике воркера.
type Client struct { /* http.Client, чёрный список URL? */ }

func NewClient(timeout time.Duration) *Client

// Send выполняет POST payload по url с подписью. Не-2xx — ошибка.
func (c *Client) Send(ctx context.Context, url, secret string, payload []byte) error
```

- `https://` принудительно вне localhost (в production); в dev/тестах
  разрешён `http://127.0.0.1`.
- Секрет — HMAC-ключ эндпоинта, хранится в БД (не логируется).
- Таймаут по умолчанию 10s.

## 3.2 Webhook-приёмник (incoming)

`internal/infrastructure/integrations/receiver.go` — проверка подписи
входящих webhook (используется E6 Payments, в перспективе — Partner API):

```go
// Verify проверяет подпись входящего запроса по секрету эндпоинта.
func Verify(secret string, tsUnix string, sigHex string, body []byte, maxAge time.Duration) error
```

- Формат подписи симметричен клиенту: HMAC-SHA256 over (timestamp + "." + body).
- Timestamp-верификация (защита от replay, window ≤ 5 мин) и constant-time
  сравнение подписи (анти-fuzing).
- `X-Stair-Signature` и `X-Stair-Timestamp` — заголовки по API-0015.

## 3.3 Хранилище (`migrations/000014_integrations.up.sql`)

### `integration_endpoints`

| Колонка | Тип | Описание |
|---------|-----|----------|
| id | UUID PK | |
| tenant_id | UUID FK tenants | владелец эндпоинта |
| name | TEXT | человекочитаемое имя (ERP Kanban и т.п.) |
| kind | TEXT | `erp`, `crm`, `mes`, `payment` (первый релиз: erp) |
| url | TEXT | целевой webhook |
| secret_enc | TEXT | HMAC-секрет (хранение — приложение) |
| created_at | timestamptz | |
| updated_at | timestamptz | |

### `integration_events`

| Колонка | Тип | Описание |
|---------|-----|----------|
| id | UUID PK | |
| tenant_id | UUID FK tenants | |
| endpoint_id | UUID FK integration_endpoints | куда отправлено |
| project_id | UUID FK projects (nullable) | источник (quote) |
| event_type | TEXT | `erp.quote_send` и т.п. |
| payload | JSONB | тело события (snapshot quote) |
| status | TEXT | `pending`/`delivered`/`failed`/`dlq` |
| attempts | int | число попыток |
| last_error | TEXT | причинa последней ошибки |
| created_at | timestamptz | |
| delivered_at | timestamptz (nullable) | момент успешной доставки |

Каждое событие доставки регистрируется как строка `integration_events` и
краткое событие в аудита (EDR-0013, действие `integration.*`).

## 3.4 Прикладной сервис

`internal/application/integrations`:

```go
type Endpoint struct {
    ID, TenantID, Name, Kind, URL string
    Secret []byte // применяется в транспорте; не сериализуется
    CreatedAt, UpdatedAt time.Time
}

type Delivery struct {
    ID, TenantID, EndpointID, ProjectID, EventType string
    Payload    json.RawMessage
    Status     string
    Attempts   int
    LastError  string
    CreatedAt  time.Time
    DeliveredAt *time.Time
}

type Service struct {
    endpoints Repository
    events    EventRepository
    queue     queue.JobQueue
}

// RegisterEndpoint регистрирует webhook-эндпоинт tenant'а.
func (s *Service) RegisterEndpoint(ctx, tenantID, name, kind, url, secret []byte) (*Endpoint, error)

// SendQuote ставит задание erp.quote_send для проекта: event-строка +
// payload=snapshot проекта; немедленно возвращается при успешной постановке
// в очередь.
func (s *Service) SendQuote(ctx, tenantID, projectID string, payload []byte) (*Delivery, error)
```

- `SendQuote` создаёт `integration_events` (status=`pending`) и enqueue
  `erp.quote_send` с payload `{event_id, endpoint_id}`.
- Воркер (EDR-0020) берёт задание → читает endpoint/event → WebhookClient.Send →
  `delivered`/`failed` + `attempts`; ретрай с backoff и max-attempts затем DLQ.
- В первом релизе ERP-эндпоинт выбирается как единственный активный
  `kind=erp` для tenant; не найдено → ошибка `ErrNoEndpoint`.

## 3.5 Транспорт

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| POST | `/api/v1/integrations/endpoints` | admin | Регистрация webhook-эндпоинта |
| GET | `/api/v1/integrations/endpoints` | admin | Список эндпоинтов tenant |
| DELETE | `/api/v1/integrations/endpoints/{id}` | admin | Удаление |
| POST | `/api/v1/projects/{id}/quote-send` | owner/editor | Постановка ERP quote в очередь |

Права:
- Управление эндпоинтами — `data.export`-уровень admin (EDR-0016) либо
  новое право `integration.manage` (добавляется в EDR-0015 permission set).
- Возможность `quote-send` — членство owner/editor (проект), т.к. требует
  актуального результата расчёта.

---

# 4. Invariants

```
1. Подпись HMAC-SHA256 over (timestamp + "." + body); constant-time проверка.
2. Timestamp-окно для входящих webhook ≤ 5 мин (replay-защита).
3. URL эндпоинтов — https вне localhost (dev/tests: http://127.0.0.1).
4. Секрет эндпоинта не логируется и не попадает в JSON-ответы.
5. Доставка асинхронна через JobQueue: retry/backoff/max-attempts/DLQ (EDR-0020).
6. payload quote = Snapshot проекта (EDR-0022), события idempotent по event_id.
7. Каждое событие доставки аудируется (integration.*).
```

---

# 5. Schema

Как в §3.3; две новые таблицы, миграция `000014_integrations`.

---

# 6. API

См. §3.5.

Payload quote (event payload) содержит `project_id`, `revision`,
`name`, `status`, `measurement`, `pricing` (итоговая цена) из Snapshot.

---

# 7. Tests

1. `integrations/client_test.go` — подпись совпадает с эталоном; не-2xx → ошибка;
   https-проверка.
2. `integrations/receiver_test.go` — корректная подпись проходит; поддельная и
   протухший timestamp — отклоняются.
3. `application/integrations` — RegisterEndpoint/SendQuote создают event и задание;
   ErrNoEndpoint.
4. Транспорт — endpoint admin CRUD (авторизация), quote-send (роли owner/editor
   и отсутствие конфигурации).
5. Worker — обработка `erp.quote_send` на stub-клиенте: delivered/failed/attempts.

---

APPROVED