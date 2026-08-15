// Package payments implements the payments infrastructure (Phase E, EDR-0027):
// a mock PSP (payment service provider) emulator that the platform uses to
// create checkout sessions and to produce signed webhook events (for tests and
// demo). Standard library only (DEV-0009). The mock mimics a remote PSP: the
// platform never exposes the provider secret to clients.
package payments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"stairplatform/internal/infrastructure/integrations"
)

// MockProvider — эмуляция PSP (EDR-0027 §3.2). Создаёт checkout-сессии и умеет
// собрать подписанный webhook-запрос вида, который отправил бы реальный PSP.
type MockProvider struct {
	name    string
	baseURL string
	now     func() time.Time
}

// NewMockProvider создаёт mock-провайдера. baseURL — база ссылок checkout
// (напр. https://pay.example.com или http://localhost:8081).
func NewMockProvider(baseURL string) *MockProvider {
	return &MockProvider{name: "mock", baseURL: baseURL, now: time.Now}
}

// Name — идентификатор провайдера ("mock").
func (p *MockProvider) Name() string { return p.name }

// CreateCheckout создаёт checkout-сессию: идентификатор — крипто-random hex,
// URL редиректа покупателя — baseURL/pay/<id> (здесь это «страница оплаты»
// mock-провайдера).
func (p *MockProvider) CreateCheckout(ctx context.Context, amountMinor int64, currency string) (checkoutID, checkoutURL string, err error) {
	if amountMinor <= 0 {
		return "", "", errors.New("payments/mock: amount must be positive")
	}
	if ctx == nil {
		return "", "", errors.New("payments/mock: nil context")
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("payments/mock: rand: %w", err)
	}
	checkoutID = hex.EncodeToString(raw)
	checkoutURL = p.baseURL + "/pay/" + checkoutID
	return checkoutID, checkoutURL, nil
}

// WebhookEvent — событие, которое «отправляет» mock-провайдер на
// POST /api/v1/payments/webhook (EDR-0027 §3.4).
type WebhookEvent struct {
	EventType   string `json:"event_type"`
	Provider    string `json:"provider"`
	CheckoutID  string `json:"checkout_id"`
	Status      string `json:"status"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

// SignWebhook сериализует событие и подписывает его как сделал бы PSP
// (трубка подписи идентична EDR-0023 §3.1 — HMAC-SHA256). Возвращает тело и
// значения заголовков X-Stair-Timestamp / X-Stair-Signature.
func (p *MockProvider) SignWebhook(secret string, ev WebhookEvent) (body []byte, tsUnix, sigValue string, err error) {
	body, err = json.Marshal(ev)
	if err != nil {
		return nil, "", "", fmt.Errorf("payments/mock: marshal webhook: %w", err)
	}
	ts := p.now().Unix()
	sig, err := integrations.Sign(secret, ts, body)
	if err != nil {
		return nil, "", "", err
	}
	return body, fmt.Sprintf("%d", ts), integrations.FormatSignature(sig), nil
}
