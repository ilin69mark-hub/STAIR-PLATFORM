package payments

import (
	"context"
	"fmt"

	"stairplatform/internal/infrastructure/circuitbreaker"
)

// CBStripeAdapter оборачивает StripeAdapter с CircuitBreaker для отказоустойчивости.
type CBStripeAdapter struct {
	inner *StripeAdapter
	cb    *circuitbreaker.CircuitBreaker
}

// NewCBStripeAdapter создаёт adapter с CircuitBreaker.
func NewCBStripeAdapter(inner *StripeAdapter, cb *circuitbreaker.CircuitBreaker) *CBStripeAdapter {
	return &CBStripeAdapter{inner: inner, cb: cb}
}

// Name возвращает имя провайдера.
func (a *CBStripeAdapter) Name() string { return "stripe" }

// CreateCheckout создаёт checkout-сессию через CircuitBreaker.
func (a *CBStripeAdapter) CreateCheckout(ctx context.Context, amountMinor int64, currency string) (checkoutID, checkoutURL string, err error) {
	err = a.cb.Execute(func() error {
		var cbErr error
		checkoutID, checkoutURL, cbErr = a.inner.CreateCheckout(ctx, amountMinor, currency)
		return cbErr
	})
	if err != nil {
		return "", "", fmt.Errorf("stripe cb: %w", err)
	}
	return checkoutID, checkoutURL, nil
}

// Refund выполняет возврат через CircuitBreaker.
func (a *CBStripeAdapter) Refund(ctx context.Context, providerCheckoutID, idempotencyKey string) (string, error) {
	var refundID string
	err := a.cb.Execute(func() error {
		var cbErr error
		refundID, cbErr = a.inner.Refund(ctx, providerCheckoutID, idempotencyKey)
		return cbErr
	})
	if err != nil {
		return "", fmt.Errorf("stripe cb: %w", err)
	}
	return refundID, nil
}

// VerifyWebhookSignature проверяет подпись через CircuitBreaker.
func (a *CBStripeAdapter) VerifyWebhookSignature(payload []byte, signature string) error {
	return a.cb.Execute(func() error {
		return a.inner.VerifyWebhookSignature(payload, signature)
	})
}

// ParseWebhookEvent парсит event через CircuitBreaker.
func (a *CBStripeAdapter) ParseWebhookEvent(payload []byte) (*StripeWebhookEvent, error) {
	var event *StripeWebhookEvent
	err := a.cb.Execute(func() error {
		var cbErr error
		event, cbErr = a.inner.ParseWebhookEvent(payload)
		return cbErr
	})
	if err != nil {
		return nil, fmt.Errorf("stripe cb: %w", err)
	}
	return event, nil
}

// CircuitBreaker возвращает circuit breaker для метрик.
func (a *CBStripeAdapter) CircuitBreaker() *circuitbreaker.CircuitBreaker {
	return a.cb
}
