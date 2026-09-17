package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// IntentProcessor применяет подтверждённое подписью событие PSP к платёжному
// интенту (EDR-0027 §3.4). Реализуется application/service платежей; подпись
// к событию уже проверена выше.
type IntentProcessor interface {
	// ApplyVerifiedEvent переводит интент в новый статус по событию провайдера
	// (provider и checkoutID уже извлечены из подписанного события).
	ApplyVerifiedEvent(ctx context.Context, provider, checkoutID, status string, amountMinor int64, currency string, raw []byte) error
}

// StripeWebhookServiceImpl обрабатывает Stripe webhook events: проверяет
// подпись, парсит event и передаёт результат в IntentProcessor для перевода
// интента в новый статус.
type StripeWebhookServiceImpl struct {
	provider  *StripeAdapter
	processor IntentProcessor
	logger    *slog.Logger
}

// NewStripeWebhookService создаёт сервис обработки Stripe webhook.
// Processor подключается отдельно (WithIntentProcessor) — сервис может
// работать как логгер событий и без него.
func NewStripeWebhookService(provider *StripeAdapter, logger *slog.Logger) *StripeWebhookServiceImpl {
	if logger == nil {
		logger = slog.Default()
	}
	return &StripeWebhookServiceImpl{
		provider: provider,
		logger:   logger,
	}
}

// WithIntentProcessor подключает обработчик интентов (application payments.Service).
func (s *StripeWebhookServiceImpl) WithIntentProcessor(p IntentProcessor) *StripeWebhookServiceImpl {
	s.processor = p
	return s
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

	if s.processor == nil {
		s.logger.Warn("stripe webhook: no intent processor configured, event ignored")
		return nil
	}

	// Обрабатываем event в зависимости от типа
	switch event.EventType {
	case WebhookEventCheckoutCompleted:
		s.logger.Info("checkout completed", "checkout_id", event.CheckoutID,
			"amount", event.AmountMinor, "currency", event.Currency)
		if err := s.processor.ApplyVerifiedEvent(ctx, event.Provider, event.CheckoutID, "succeeded",
			event.AmountMinor, event.Currency, payload); err != nil {
			s.logger.Error("stripe webhook: apply completed event failed", "error", err)
			return fmt.Errorf("stripe webhook: apply completed event: %w", err)
		}
	case WebhookEventCheckoutFailed:
		s.logger.Warn("checkout failed", "checkout_id", event.CheckoutID)
		if err := s.processor.ApplyVerifiedEvent(ctx, event.Provider, event.CheckoutID, "failed",
			event.AmountMinor, event.Currency, payload); err != nil {
			s.logger.Error("stripe webhook: apply failed event failed", "error", err)
			return fmt.Errorf("stripe webhook: apply failed event: %w", err)
		}
	case WebhookEventCheckoutExpired:
		s.logger.Warn("checkout expired", "checkout_id", event.CheckoutID)
		if err := s.processor.ApplyVerifiedEvent(ctx, event.Provider, event.CheckoutID, "failed",
			event.AmountMinor, event.Currency, payload); err != nil {
			s.logger.Error("stripe webhook: apply expired event failed", "error", err)
			return fmt.Errorf("stripe webhook: apply expired event: %w", err)
		}
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
			ID        string            `json:"id"`
			Status    string            `json:"status"`
			Amount    int64             `json:"amount_total"`
			Currency  string            `json:"currency"`
			Metadata  map[string]string `json:"metadata"`
			CreatedAt int64             `json:"created"`
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
		EventType:   raw.Type,
		Provider:    "stripe",
		CheckoutID:  raw.Data.Object.ID,
		Status:      raw.Data.Object.Status,
		AmountMinor: raw.Data.Object.Amount,
		Currency:    raw.Data.Object.Currency,
	}, nil
}
