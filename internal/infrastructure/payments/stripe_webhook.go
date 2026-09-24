package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	apppayments "stairplatform/internal/application/payments"
)

// IntentProcessor применяет подтверждённое подписью событие PSP к платёжному
// интенту (EDR-0027 §3.4). Реализуется application/service платежей; подпись
// к событию уже проверена выше.
type IntentProcessor interface {
	// ApplyVerifiedEvent переводит интент в новый статус по событию провайдера
	// (provider и checkoutID уже извлечены из подписанного события).
	ApplyVerifiedEvent(ctx context.Context, provider, checkoutID, status string, amountMinor int64, currency string, raw []byte) error
}

// IntentReader — опциональное чтение интента для сверки с событием при
// дубликате (S-141 №3, CWE-367/703): когда CheckAndMark вернул duplicate,
// сервис сверяет состояние интента с событием и при расхождении применяет
// событие (reconcile, self-healing crash-window). Реализуется
// application.Service. Если процессор ридер не реализует — деградация к
// старому поведению (дубликат игнорируется молча).
type IntentReader interface {
	GetIntentByProviderCheckout(ctx context.Context, provider, checkoutID string) (*apppayments.PaymentIntent, error)
}

// StripeWebhookServiceImpl обрабатывает Stripe webhook events: проверяет
// подпись, парсит event и передаёт результат в IntentProcessor для перевода
// интента в новый статус.
type StripeWebhookServiceImpl struct {
	provider  *StripeAdapter
	processor IntentProcessor
	deduper   EventDeduper
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

// WithEventDeduper подключает защиту от повторной обработки событий (P1-1).
// Без дедупера обработка полагается на идемпотентность ApplyVerifiedEvent.
func (s *StripeWebhookServiceImpl) WithEventDeduper(d EventDeduper) *StripeWebhookServiceImpl {
	s.deduper = d
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

	// Дедупликация по event.id (P1-1): повторная доставка или replay того же
	// события не должна приводить к повторному переводу интента. Метка
	// снимается при ошибке, чтобы Stripe-retry применился с новым id.
	if s.deduper != nil && event.EventID != "" {
		processed, dedupErr := s.deduper.CheckAndMark(ctx, event.Provider, event.EventID)
		if dedupErr != nil {
			s.logger.Error("stripe webhook dedup check failed", "event_id", event.EventID, "error", dedupErr)
			return fmt.Errorf("stripe webhook: dedup check failed: %w", dedupErr)
		}
		if !processed {
			// S-141 №3 (CWE-367/703): crash-window — метка проставлена, но
			// ApplyVerifiedEvent не выполнен (процесс убит между
			// CheckAndMark и применением), Clear не вызывался. НЕ отбрасываем
			// молча: сверяем интент с событием и при расхождении применяем.
			return s.handleDuplicate(ctx, event, payload)
		}
	}

	if s.processor == nil {
		s.logger.Warn("stripe webhook: no intent processor configured, event ignored")
		return nil
	}

	return s.applyEvent(ctx, event, payload)
}

// handleDuplicate — ветка дубликата (S-141 №3): вместо молчаливого игнора
// сверяем состояние интента с событием. Интент в целевом статусе → чистый
// дубликат (nil, как раньше). Интент НЕ отражает событие (статус иной или
// интента нет) → применяем событие (reconcile inline) — self-healing
// crash-window между CheckAndMark и Apply. Метка на ошибке снимается тем же
// defer-ом, что и для новых событий (см. applyEvent).
func (s *StripeWebhookServiceImpl) handleDuplicate(ctx context.Context, event *StripeWebhookEvent, payload []byte) error {
	if s.processor == nil {
		s.logger.Warn("stripe webhook: duplicate event ignored (no processor)",
			"event_id", event.EventID, "checkout_id", event.CheckoutID)
		return nil
	}

	reader, ok := s.processor.(IntentReader)
	if !ok {
		// Деградация к старому поведению: ридер не реализован — нечем
		// сверить интент, дубликат игнорируется молча.
		s.logger.Warn("stripe webhook: duplicate event ignored (intent reader unavailable)",
			"event_id", event.EventID, "checkout_id", event.CheckoutID)
		return nil
	}

	intent, err := reader.GetIntentByProviderCheckout(ctx, event.Provider, event.CheckoutID)
	if err != nil {
		if errors.Is(err, apppayments.ErrNotFound) {
			// Интента нет — событию нечему соответствовать: применяем как
			// обычно; ApplyVerifiedEvent вернёт ErrNotFound, метка снимется
			// (defer Clear) и Stripe-retry повторит доставку — событие не
			// теряется молча.
			s.logger.Warn("stripe webhook: duplicate event, intent missing — reconcile will surface not-found",
				"event_id", event.EventID, "checkout_id", event.CheckoutID)
			return s.applyEvent(ctx, event, payload)
		}
		s.logger.Error("stripe webhook: duplicate reconcile: read intent failed",
			"event_id", event.EventID, "error", err)
		return fmt.Errorf("stripe webhook: duplicate reconcile: read intent: %w", err)
	}

	target := reconcileTarget(event)
	if target != "" && intent.Status == target {
		// Чистый дубликат: интент уже отражает событие — повторно не применяем.
		s.logger.Info("stripe webhook: duplicate event ignored (intent in target status)",
			"event_id", event.EventID, "checkout_id", event.CheckoutID,
			"intent_status", intent.Status)
		return nil
	}

	// Интент не отражает событие (crash-window): применяем повторно.
	s.logger.Warn("stripe webhook: duplicate event reconciled (intent does not reflect event)",
		"event_id", event.EventID, "checkout_id", event.CheckoutID,
		"intent_status", intent.Status, "target_status", target)
	return s.applyEvent(ctx, event, payload)
}

// applyEvent применяет событие к интенту (переключатель по типу события).
// При ошибке метка дедупликации снимается (defer Clear), чтобы Stripe-retry
// того же события применился (P1-1) — семантика единая для нового события и
// для reconcile дубликата (S-141 №3).
func (s *StripeWebhookServiceImpl) applyEvent(ctx context.Context, event *StripeWebhookEvent, payload []byte) (err error) {
	if s.deduper != nil && event.EventID != "" {
		defer func() {
			if err != nil {
				if cerr := s.deduper.Clear(ctx, event.Provider, event.EventID); cerr != nil {
					s.logger.Error("stripe webhook: clear dedup marker failed", "event_id", event.EventID, "error", cerr)
				}
			}
		}()
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

// reconcileTarget — целевой статус интента для события (тот же маппинг, что
// в applyEvent): checkout.completed → paid; failed/expired → failed.
// Пустая строка для необрабатываемых типов — сверка не имеет смысла.
func reconcileTarget(event *StripeWebhookEvent) apppayments.Status {
	switch event.EventType {
	case WebhookEventCheckoutCompleted:
		return apppayments.StatusPaid
	case WebhookEventCheckoutFailed, WebhookEventCheckoutExpired:
		return apppayments.StatusFailed
	default:
		return ""
	}
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
