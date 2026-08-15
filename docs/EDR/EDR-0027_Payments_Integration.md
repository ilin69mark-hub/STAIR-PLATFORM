# STAIR PLATFORM

**Document:** EDR-0027_Payments_Integration.md

**ID:** EDR-0027

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Integrations

---

# 1. Purpose

Документ фиксирует Payment-интеграцию (Phase E, E6) — прием платежей за
проект через внешний платёжный провайдер (PSP). Платформа — получатель
webhook с подтверждением оплаты: PSP (реальный или mock-эмулятор) создаёт
checkout-сессию, покупатель «оплачивает», провайдер присылает подписанный
webhook, платформа верифицирует подпись (переиспользуя формат EDR-0023
§3.1 — HMAC-SHA256 c `X-Stair-Signature`/`X-Stair-Timestamp`) и фиксирует
статус платежа.

Состав:

1. **`payment_intents` / `payment_events`** — БД-модель платежей и журнала
   событий (миграция `000015`).
2. **Mock-провайдер** — `internal/infrastructure/payments`: эмулятор PSP
   (checkout-сессия + генерация подписанного webhook для тестов/demo).
3. **Pаyments-сервис** — `internal/application/payments`: создание платёжного
   интента, верификация входящего webhook, переход статуса, журнал событий.
4. **Транспорт** — создание checkout (`POST /projects/{id}/checkout`), список
   платежей, статус интента и публичный входящий webhook
   (`POST /api/v1/payments/webhook`).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase E — Integrations: Payments)
- EDR-0023 (Webhook-платформа: формат подписи и верификации) — переиспользуется
  для входящего webhook провайдера
- ADR-0006 (Layered Architecture)
- DEV-0009 (Offline builds — только stdlib, mock-эмулятор без SDK)
- EDR-0022 (CAD Export — оплата за расчёт/проект соседствует с экспортами)

---

# 3. Model

## 3.1 БД (`migrations/000015_payments`)

```sql
payment_intents (
  id UUID PK, tenant_id → tenants, project_id → projects (SET NULL),
  user_id → users,
  amount_minor BIGINT, currency TEXT DEFAULT 'USD',
  status TEXT: pending|paid|failed|refunded,
  provider TEXT, provider_checkout_id TEXT,
  created_at/updated_at, paid_at TIMESTAMPTZ
)
UNIQUE (provider, provider_checkout_id)

payment_events (
  id UUID PK, tenant_id, intent_id → payment_intents (CASCADE),
  event_type TEXT, payload JSONB, created_at
)
```

- Деньги в `amount_minor` (минимальная денежная единица; ADR-0008).
- `provider_checkout_id` — идентификатор checkout-сессии во внешнем PSP
  (уникален в рамках провайдера → идемпотентность webhook).
- `payment_events` — журнал входящих событий (аудит, EDR-0013).

## 3.2 Infrastructure: mock-провайдер (`internal/infrastructure/payments/mock.go`)

```go
// MockProvider — эмуляция PSP: создаёт checkout-сессию и умеет собрать
// подписанный webhook (для тестов и demo). Ведёт себя как удалённый PSP.
func NewMockProvider(baseURL string) *MockProvider
func (p *MockProvider) Name() string                       // "mock"
func (p *MockProvider) CreateCheckout(ctx, amountMinor int64, currency string) (checkoutID, checkoutURL string, err error)

// SignWebhook сериализует событие и подписывает его как PSP
// (HMAC-SHA256 via integrations.Sign, заголовки сам расставляет платформа).
func (p *MockProvider) SignWebhook(secret string, ts int64, event any) (body []byte, err error)
```

- `checkoutID` — крипто-random hex (stdlib only).
- `checkoutURL = baseURL + "/pay/" + checkoutID` (в prod это редирект на PSP).

## 3.3 Application-сервис (`internal/application/payments`)

```go
type Status string // pending|paid|failed|refunded
type PaymentIntent struct { ID, TenantID, ProjectID, UserID, AmountMinor,
  Currency, Status, Provider, ProviderCheckoutID string; CreatedAt, UpdatedAt,
  PaidAt }

type Provider interface {
    Name() string
    CreateCheckout(ctx context.Context, amountMinor int64, currency string) (checkoutID, checkoutURL string, err error)
}

type WebhookVerifier interface {
    // Верифицирует HMAC-SHA256 подпись (X-Stair-Signature) входящего webhook.
    Verify(secret string, tsUnix, sigValue string, body []byte, maxAge time.Duration) error
}

type Repository interface {
    CreateIntent(ctx, *PaymentIntent) error
    GetIntent(ctx, tenantID, id) (*PaymentIntent, error)
    GetIntentByProviderCheckout(ctx, provider, checkoutID string) (*PaymentIntent, error)
    ListByProject(ctx, tenantID, projectID string) ([]*PaymentIntent, error)
    UpdateStatus(ctx, tenantID, id, status string, paidAt *time.Time) error
    AppendEvent(ctx, *PaymentEvent) error
}

// CreateCheckout создаёт pending-интент через Provider и возвращает
// checkout URL для редиректа покупателя.
func (s *Service) CreateCheckout(ctx, tenantID, projectID, userID string, amountMinor int64, currency string) (*PaymentIntent, error)

// ListByProject возвращает платежи проекта (tenant-скоуп).
func (s *Service) ListByProject(ctx, tenantID, projectID string) ([]*PaymentIntent, error)

// Get возвращает интент по ID (tenant-скоуп).
func (s *Service) Get(ctx, tenantID, id) (*PaymentIntent, error)

// HandleWebhook верифицирует подпись, находит интент по (provider, checkout_id),
// обновляет статус и пишет событие в журнал.
func (s *Service) HandleWebhook(ctx, secret string, tsUnix, sigValue string, body []byte) (*PaymentEvent, error)
```

- `HandleWebhook` mapping: невалидная подпись → `ErrInvalidSignature`;
  интент не найден → `ErrNotFound`; неверная сумма/валюта события →
  `ErrInvalid`.
- Журналируется каждое принятое событие (включая `paid`, `failed`).

## 3.4 Transport (`internal/transport/http/payments.go`)

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| POST | `/api/v1/payments/webhook` | публичный (подпись) | Входящий webhook PSP: верификация HMAC, обновление статуса, журнал события |
| POST | `/api/v1/projects/{id}/checkout` | auth/член проекта | Создать checkout-сессию на сумму за проект; 201 — `{id, checkout_url, status}` |
| GET | `/api/v1/projects/{id}/payments` | auth/член проекта | Список платежей проекта |
| GET | `/api/v1/payments/{id}` | auth/tenant | Статус платёжного интента |

- Webhook читает сырое тело + заголовки `X-Stair-Signature`/`X-Stair-Timestamp`;
  секрет — `STAIR_PAYMENT_WEBHOOK_SECRET`; окно — `integrations.MaxTimestampAge`.
- `POST /api/v1/payments/webhook` — единственный маршрут, доступный без
  `requireAuth` (его вызывает внешняя система); безопасность — за счёт
  HMAC-подписи.

---

# 4. Invariants

```
1. Секрет webhook (STAIR_PAYMENT_WEBHOOK_SECRET) не логируется и не
   попадает в JSON-ответы.
2. Сумма/валюта в webhook сравниваются с созданным интентом — расхождение
   отклоняется (ErrInvalid).
3. (provider, provider_checkout_id) уникальны — повторный webhook идемпотентен.
4. Все методы записи tenant-скоупed; webhook находит интент глобально по
   provider+checkout_id (без раскрытия чужих данных в ответе).
5. Трубка подписи — та же, что в EDR-0023 §3.1/§3.2 (Sign/Verify).
6. Деньги целыми минимальными единицами (amount_minor, ADR-0008).
```

---

# 5. Schema

- `payment_intents` — один платёжный интент (pending/paid/failed/refunded).
- `payment_events` — журнал событий webhook (audit-совместимый, EDR-0013).
- Каскады: tenant → CASCADE; project → SET NULL; user → (без каскада,
  значение не содержит-чувствительных данных, проверяемо).

---

# 6. API

Создание checkout:

```
POST /api/v1/projects/{id}/checkout
{ "amount_minor": 5000, "currency": "USD" }
→ 201 { "id": "...", "checkout_url": "http://psp/pay/abc", "status": "pending",
        "amount_minor": 5000, "currency": "USD" }
```

Входящий webhook:

```
POST /api/v1/payments/webhook
X-Stair-Timestamp: <unix>
X-Stair-Signature: v1:<hex>
{ "event_type": "payment.succeeded", "provider": "mock",
  "checkout_id": "abc", "status": "paid",
  "amount_minor": 5000, "currency": "USD" }
→ 200 { "status": "ok" } ; 401 signature invalid ; 404 intent not found ;
422 invalid payload ; 409 amount mismatch
```

Список платежей:

```
GET /api/v1/projects/{id}/payments → 200 [ {id,status,amount_minor,currency,...} ]
GET /api/v1/payments/{id} → 200 {id,status,...} | 404
```

---

# 7. Tests

1. Infrastructure mock: `CreateCheckout` даёт checkoutID + URL; `SignWebhook`
   событие проходит `integrations.Verify`.
2. Application service: CreateCheckout (валидный/невалидный ввод, нет очереди);
   HandleWebhook — подпись ок / невалидная → ErrInvalidSignature / чужой
   checkout_id → ErrNotFound / расхождение суммы → ErrInvalid; статус переходит
   pending → paid c paid_at; событие журналируется.
3. Транспорт: checkout 201/400/403/404/422; list 200/403/404; get 200/404;
   webhook 200/401/422/404/409.
4. Репозиторий (DB-интеграция обычная): CRUD-интентов, UpdateStatus
   (pending→paid c paid_at), AppendEvent, уникальность (provider, checkout_id).

---

# 8. Migration rollback

`000015_payments.down.sql` удаляет `payment_events`, затем `payment_intents`.

---

APPROVED