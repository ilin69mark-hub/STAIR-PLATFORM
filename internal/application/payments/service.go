package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Service — прикладной сервис платежей (EDR-0027 §3.3). Управляет
// интентами: создаёт checkout-сессии через Provider и обрабатывает
// входящие webhook (верификация подписи → переход статуса → журнал событий).
type Service struct {
	repo          Repository
	provider      Provider
	verifier      WebhookVerifier
	maxWebhookAge time.Duration
	now           func() time.Time
}

// NewService создаёт сервис платежей. maxAge <= 0 — WebhookVerifier решает
// дефолт (у интеграций MaxTimestampAge).
func NewService(repo Repository, provider Provider, verifier WebhookVerifier, maxAge time.Duration) *Service {
	return &Service{
		repo:          repo,
		provider:      provider,
		verifier:      verifier,
		maxWebhookAge: maxAge,
		now:           time.Now,
	}
}

// CreateCheckout создаёт pending-интент через Provider и возвращает
// checkout URL (EDR-0027 §3.4). Валидирует сумму и валюту.
func (s *Service) CreateCheckout(ctx context.Context, tenantID, projectID, userID string, amountMinor int64, currency string) (*PaymentIntent, error) {
	if tenantID == "" || projectID == "" {
		return nil, fmt.Errorf("%w: project required", ErrInvalid)
	}
	if amountMinor <= 0 {
		return nil, fmt.Errorf("%w: amount_minor must be positive", ErrInvalid)
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return nil, fmt.Errorf("%w: currency required", ErrInvalid)
	}

	checkoutID, checkoutURL, err := s.provider.CreateCheckout(ctx, amountMinor, currency)
	if err != nil {
		return nil, err
	}

	p := &PaymentIntent{
		TenantID:           tenantID,
		ProjectID:          projectID,
		UserID:             userID,
		AmountMinor:        amountMinor,
		Currency:           currency,
		Status:             StatusPending,
		Provider:           s.provider.Name(),
		ProviderCheckoutID: checkoutID,
		CheckoutURL:        checkoutURL,
		CreatedAt:          s.now().UTC(),
		UpdatedAt:          s.now().UTC(),
	}
	if err := s.repo.CreateIntent(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListByProject возвращает платежи проекта (tenant-скоуп).
func (s *Service) ListByProject(ctx context.Context, tenantID, projectID string) ([]*PaymentIntent, error) {
	return s.repo.ListByProject(ctx, tenantID, projectID)
}

// Get возвращает интент по ID (tenant-скоуп).
func (s *Service) Get(ctx context.Context, tenantID, id string) (*PaymentIntent, error) {
	return s.repo.GetIntent(ctx, tenantID, id)
}

// GetIntentByProviderCheckout возвращает интент по (provider, checkout_id).
// Публичный доступ к чтению нужен для сверки состояния интента с событием
// при дубликате webhook (S-141 №3, crash-window reconcile): StripeWebhookService
// через опциональный интерфейс IntentReader type-assert'ит Service.
func (s *Service) GetIntentByProviderCheckout(ctx context.Context, provider, checkoutID string) (*PaymentIntent, error) {
	return s.repo.GetIntentByProviderCheckout(ctx, provider, checkoutID)
}

// webhookPayload — тело входящего события PSP (EDR-0027 §3.4).
type webhookPayload struct {
	EventType   string `json:"event_type"`
	Provider    string `json:"provider"`
	CheckoutID  string `json:"checkout_id"`
	Status      string `json:"status"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

// HandleWebhook верифицирует подпись входящего webhook, затем вызывает
// common webhook-конвейер: находит интент по (provider, checkout_id),
// проверяет сумму/валюту, переводит статус и пишет событие в журнал.
// Возвращает созданное событие.
func (s *Service) HandleWebhook(ctx context.Context, secret, tsUnix, sigValue string, body []byte) (*PaymentEvent, error) {
	if s.verifier == nil {
		return nil, fmt.Errorf("%w: verifier not configured", ErrInvalid)
	}
	if err := s.verifier.Verify(secret, tsUnix, sigValue, body, s.maxWebhookAge); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	var ev webhookPayload
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, fmt.Errorf("%w: invalid webhook body", ErrInvalid)
	}
	if ev.Provider == "" || ev.CheckoutID == "" {
		return nil, fmt.Errorf("%w: provider and checkout_id required", ErrInvalid)
	}

	return s.applyVerifiedEvent(ctx, ev.Provider, ev.CheckoutID, ev.Status, ev.AmountMinor, ev.Currency, body)
}

// ApplyVerifiedEvent применяет событие PSP, подпись которого уже проверена
// (Stripe-вебхук: sub-специфичная подпись проверяется в инфраструктуре до
// вызова). Реализация интерфейса IntentProcessor (infrastructure/payments).
// Общая логика с HandleWebhook — переход статуса и журнал событий.
func (s *Service) ApplyVerifiedEvent(ctx context.Context, provider, checkoutID, status string, amountMinor int64, currency string, raw []byte) error {
	_, err := s.applyVerifiedEvent(ctx, provider, checkoutID, status, amountMinor, currency, raw)
	return err
}

// applyVerifiedEvent — общий webhook-конвейер (EDR-0027 §3.4): интент по
// (provider, checkout_id), проверка суммы/валюты, смена статуса, журнал.
func (s *Service) applyVerifiedEvent(ctx context.Context, provider, checkoutID, status string, amountMinor int64, currency string, body []byte) (*PaymentEvent, error) {
	if provider == "" || checkoutID == "" {
		return nil, fmt.Errorf("%w: provider and checkout_id required", ErrInvalid)
	}

	intent, err := s.repo.GetIntentByProviderCheckout(ctx, provider, checkoutID)
	if err != nil {
		return nil, err // ErrNotFound прокидывается.
	}

	// Проверяем сумму/валюту события против созданного интента.
	if amountMinor != intent.AmountMinor || !strings.EqualFold(currency, intent.Currency) {
		return nil, fmt.Errorf("%w: amount mismatch", ErrInvalid)
	}

	newStatus, eventType, err := resolveStatus(status)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	var paidAt *time.Time
	if newStatus == StatusPaid {
		paidAt = &now
	}
	if err := s.repo.UpdateStatus(ctx, intent.TenantID, intent.ID, newStatus, paidAt); err != nil {
		return nil, err
	}
	intent.Status = newStatus
	intent.PaidAt = paidAt

	event := &PaymentEvent{
		TenantID:  intent.TenantID,
		IntentID:  intent.ID,
		EventType: eventType,
		Payload:   append(json.RawMessage(nil), body...),
		CreatedAt: now,
	}
	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

// resolveStatus сопоставляет статус события PSP со статусом интента.
func resolveStatus(raw string) (Status, string, error) {
	switch strings.ToLower(raw) {
	case "paid", "succeeded":
		return StatusPaid, EventTypePaymentSucceeded, nil
	case "failed", "declined":
		return StatusFailed, EventTypePaymentFailed, nil
	default:
		return "", "", fmt.Errorf("%w: unsupported payment status %q", ErrInvalid, raw)
	}
}
