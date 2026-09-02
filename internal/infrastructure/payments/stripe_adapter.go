package payments

import (
	"context"
	"fmt"
)

// StripeAdapter адаптирует StripeProvider к application-level payments.Provider.
// Используется в main.go для выбора между Mock и Stripe при запуске.
type StripeAdapter struct {
	provider *StripeProvider
}

// NewStripeAdapter создаёт адаптер для Stripe.
func NewStripeAdapter(provider *StripeProvider) *StripeAdapter {
	return &StripeAdapter{provider: provider}
}

// Name возвращает имя провайдера.
func (a *StripeAdapter) Name() string { return "stripe" }

// CreateCheckout создаёт checkout-сессию через Stripe API.
func (a *StripeAdapter) CreateCheckout(ctx context.Context, amountMinor int64, currency string) (checkoutID, checkoutURL string, err error) {
	session, err := a.provider.CreateCheckoutSession(ctx, CheckoutParams{
		Amount:   amountMinor,
		Currency: currency,
	})
	if err != nil {
		return "", "", fmt.Errorf("stripe adapter: %w", err)
	}
	return session.ID, session.CheckoutURL, nil
}

// VerifyWebhookSignature проверяет подпись Stripe webhook.
func (a *StripeAdapter) VerifyWebhookSignature(payload []byte, signature string) error {
	return a.provider.VerifyWebhookSignature(payload, signature)
}

// ParseWebhookEvent парсит Stripe webhook event.
func (a *StripeAdapter) ParseWebhookEvent(payload []byte) (*StripeWebhookEvent, error) {
	return a.provider.ParseWebhookEvent(payload)
}
