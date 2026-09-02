package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// StripeWebhookService обрабатывает Stripe webhook events.
type StripeWebhookServiceImpl struct {
	provider *StripeAdapter
	logger   *slog.Logger
}

// NewStripeWebhookService создаёт сервис обработки Stripe webhook.
func NewStripeWebhookService(provider *StripeAdapter, logger *slog.Logger) *StripeWebhookServiceImpl {
	if logger == nil {
		logger = slog.Default()
	}
	return &StripeWebhookServiceImpl{
		provider: provider,
		logger:   logger,
	}
}

// HandleStripeWebhook обрабатывает входящий Stripe webhook.
func (s *StripeWebhookServiceImpl) HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
	// Верифицируем подпись Stripe
	if err := s.provider.VerifyWebhookSignature(payload, signature); err != nil {
		s.logger.Error("stripe webhook signature verification failed", "error", err)
		return fmt.Errorf("stripe webhook: signature verification failed: %w", err)
	}

	// Парсим event
	event, err := s.provider.ParseWebhookEvent(payload)
	if err != nil {
		s.logger.Error("stripe webhook parse failed", "error", err)
		return fmt.Errorf("stripe webhook: parse failed: %w", err)
	}

	s.logger.Info("stripe webhook received",
		"event_type", event.EventType,
		"checkout_id", event.CheckoutID,
		"status", event.Status,
	)

	// Обрабатываем event в зависимости от типа
	switch event.EventType {
	case "checkout.completed":
		s.logger.Info("checkout completed", "checkout_id", event.CheckoutID, "amount", event.AmountMinor)
	case "checkout.failed":
		s.logger.Warn("checkout failed", "checkout_id", event.CheckoutID)
	case "checkout.expired":
		s.logger.Warn("checkout expired", "checkout_id", event.CheckoutID)
	default:
		s.logger.Info("unhandled stripe event type", "event_type", event.EventType)
	}

	return nil
}

// stripeWebhookEventInternal — внутренняя структура для парсинга Stripe event.
// Stripe оборачивает объект в data.object.
type stripeWebhookEventInternal struct {
	Data struct {
		Object struct {
			ID         string            `json:"id"`
			Status     string            `json:"status"`
			Amount     int64             `json:"amount_total"`
			Currency   string            `json:"currency"`
			Metadata   map[string]string `json:"metadata"`
			CreatedAt  int64             `json:"created"`
		} `json:"object"`
	} `json:"data"`
	Type      string `json:"type"`
	CreatedAt int64  `json:"created"`
}

// ParseStripeEvent парсит raw Stripe webhook event.
func ParseStripeEvent(payload []byte) (*StripeWebhookEvent, error) {
	var raw stripeWebhookEventInternal
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("stripe: unmarshal event: %w", err)
	}

	return &StripeWebhookEvent{
		EventType:  raw.Type,
		Provider:   "stripe",
		CheckoutID: raw.Data.Object.ID,
		Status:     raw.Data.Object.Status,
		AmountMinor: raw.Data.Object.Amount,
		Currency:   raw.Data.Object.Currency,
	}, nil
}
