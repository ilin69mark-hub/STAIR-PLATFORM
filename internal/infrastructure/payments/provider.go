package payments

import (
	"context"
	"time"
)

// Provider — интерфейс для платежного провайдера.
type Provider interface {
	// Name возвращает имя провайдера.
	Name() string

	// CreateCheckoutSession создает сессию оплаты.
	CreateCheckoutSession(ctx context.Context, params CheckoutParams) (*CheckoutSession, error)

	// GetSession получает информацию о сессии.
	GetSession(ctx context.Context, sessionID string) (*CheckoutSession, error)

	// VerifyWebhookSignature проверяет подпись webhook.
	VerifyWebhookSignature(payload []byte, signature string) error

	// ParseWebhookEvent парсит webhook event.
	ParseWebhookEvent(payload []byte) (*StripeWebhookEvent, error)
}

// CheckoutParams — параметры создания сессии оплаты.
type CheckoutParams struct {
	OrderID     string            `json:"orderId"`
	Amount      int64             `json:"amount"` // в копейках
	Currency    string            `json:"currency"`
	Email       string            `json:"email"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	SuccessURL  string            `json:"successUrl"`
	CancelURL   string            `json:"cancelUrl"`
}

// CheckoutSession — сессия оплаты.
type CheckoutSession struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Amount      int64             `json:"amount"`
	Currency    string            `json:"currency"`
	CheckoutURL string            `json:"checkoutUrl"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	ExpiresAt   *time.Time        `json:"expiresAt,omitempty"`
}

// StripeWebhookEvent — Stripe webhook event.
type StripeWebhookEvent struct {
	// EventID — уникальный идентификатор события PSA (de dup): у Stripe — id
	// верхнеуровневого объекта event (evt_…). Пуст, если провайдер не отдаёт
	// идентификатор — тогда защита от повторов строится на идемпотентности
	// ApplyVerifiedEvent.
	EventID     string    `json:"event_id"`
	EventType   string    `json:"event_type"`
	Provider    string    `json:"provider"`
	CheckoutID  string    `json:"checkout_id"`
	Status      string    `json:"status"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
}

// CheckoutSessionStatus — статусы сессии.
const (
	CheckoutStatusPending   = "pending"
	CheckoutStatusActive    = "active"
	CheckoutStatusCompleted = "completed"
	CheckoutStatusFailed    = "failed"
	CheckoutStatusExpired   = "expired"
	CheckoutStatusCancelled = "cancelled"
)

// WebhookEventTypes — типы webhook events.
const (
	WebhookEventCheckoutCompleted = "checkout.completed"
	WebhookEventCheckoutFailed    = "checkout.failed"
	WebhookEventCheckoutExpired   = "checkout.expired"
)
