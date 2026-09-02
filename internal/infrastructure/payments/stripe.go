package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// StripeProvider — реализация Provider для Stripe.
type StripeProvider struct {
	secretKey      string
	webhookSecret  string
	baseURL        string
	client         *http.Client
}

// NewStripeProvider создаёт новый Stripe provider.
func NewStripeProvider(secretKey, webhookSecret string) *StripeProvider {
	return &StripeProvider{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		baseURL:       "https://api.stripe.com/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name возвращает имя провайдера.
func (p *StripeProvider) Name() string {
	return "stripe"
}

// CreateCheckoutSession создает сессию оплаты в Stripe.
func (p *StripeProvider) CreateCheckoutSession(ctx context.Context, params CheckoutParams) (*CheckoutSession, error) {
	// Формируем данные для Stripe API
	data := map[string]string{
		"mode":                    "payment",
		"success_url":             params.SuccessURL,
		"cancel_url":              params.CancelURL,
		"customer_email":          params.Email,
		"line_items[0][price_data][currency]": params.Currency,
		"line_items[0][price_data][product_data][name]": params.Description,
		"line_items[0][price_data][unit_amount]":        fmt.Sprintf("%d", params.Amount),
		"line_items[0][quantity]":                        "1",
		"metadata[order_id]":                            params.OrderID,
	}

	// Добавляем дополнительные метаданные
	for k, v := range params.Metadata {
		data["metadata["+k+"]"] = v
	}

	// Отправляем запрос в Stripe
	resp, err := p.makeRequest(ctx, "POST", "/checkout/sessions", data)
	if err != nil {
		return nil, fmt.Errorf("stripe: create checkout session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stripe: create checkout session: %s", string(body))
	}

	var result struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		Status    string `json:"status"`
		CreatedAt int64  `json:"created"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("stripe: decode response: %w", err)
	}

	createdAt := time.Unix(result.CreatedAt, 0)

	return &CheckoutSession{
		ID:          result.ID,
		Status:      CheckoutStatusPending,
		Amount:      params.Amount,
		Currency:    params.Currency,
		CheckoutURL: result.URL,
		Metadata:    params.Metadata,
		CreatedAt:   createdAt,
	}, nil
}

// GetSession получает информацию о сессии из Stripe.
func (p *StripeProvider) GetSession(ctx context.Context, sessionID string) (*CheckoutSession, error) {
	resp, err := p.makeRequest(ctx, "GET", "/checkout/sessions/"+sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("stripe: get session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stripe: get session: %s", string(body))
	}

	var result struct {
		ID           string            `json:"id"`
		Status       string            `json:"status"`
		AmountTotal  int64             `json:"amount_total"`
		Currency     string            `json:"currency"`
		PaymentStatus string           `json:"payment_status"`
		CreatedAt    int64             `json:"created"`
		Metadata     map[string]string `json:"metadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("stripe: decode response: %w", err)
	}

	createdAt := time.Unix(result.CreatedAt, 0)

	// Маппим статус Stripe на наш статус
	status := CheckoutStatusPending
	switch result.PaymentStatus {
	case "paid":
		status = CheckoutStatusCompleted
	case "unpaid":
		status = CheckoutStatusPending
	case "no_payment_required":
		status = CheckoutStatusCompleted
	}

	return &CheckoutSession{
		ID:         result.ID,
		Status:     status,
		Amount:     result.AmountTotal,
		Currency:   result.Currency,
		Metadata:   result.Metadata,
		CreatedAt:  createdAt,
	}, nil
}

// VerifyWebhookSignature проверяет подпись Stripe webhook.
func (p *StripeProvider) VerifyWebhookSignature(payload []byte, signature string) error {
	// Stripe использует формат: t=timestamp,v1=signature
	parts := strings.Split(signature, ",")
	if len(parts) != 2 {
		return fmt.Errorf("stripe: invalid signature format")
	}

	var timestamp, sig string
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			sig = kv[1]
		}
	}

	if timestamp == "" || sig == "" {
		return fmt.Errorf("stripe: missing timestamp or signature")
	}

	// Формируем подписанный payload
	signedPayload := timestamp + "." + string(payload)

	// Вычисляем HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return fmt.Errorf("stripe: invalid signature")
	}

	return nil
}

// ParseWebhookEvent парсит Stripe webhook event.
func (p *StripeProvider) ParseWebhookEvent(payload []byte) (*StripeWebhookEvent, error) {
	var event struct {
		Type   string `json:"type"`
		Data   struct {
			Object struct {
				ID           string `json:"id"`
				Status       string `json:"status"`
				PaymentStatus string `json:"payment_status"`
				CreatedAt    int64  `json:"created"`
				Metadata     map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
		CreatedAt int64 `json:"created"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("stripe: parse event: %w", err)
	}

	// Маппим тип Stripe event на наш тип
	webhookType := ""
	switch event.Type {
	case "checkout.session.completed":
		webhookType = WebhookEventCheckoutCompleted
	case "checkout.session.expired":
		webhookType = WebhookEventCheckoutExpired
	default:
		webhookType = event.Type
	}

	// Маппим статус
	status := CheckoutStatusPending
	switch event.Data.Object.PaymentStatus {
	case "paid":
		status = CheckoutStatusCompleted
	case "unpaid":
		status = CheckoutStatusPending
	}

	return &StripeWebhookEvent{
		EventType:  webhookType,
		CheckoutID: event.Data.Object.ID,
		Status:     status,
		CreatedAt:  time.Unix(event.CreatedAt, 0),
	}, nil
}

// makeRequest выполняет HTTP запрос к Stripe API с trace context propagation.
func (p *StripeProvider) makeRequest(ctx context.Context, method, path string, data map[string]string) (*http.Response, error) {
	var body io.Reader
	if data != nil {
		// Формируем form-urlencoded data
		form := make([]string, 0, len(data))
		for k, v := range data {
			form = append(form, k+"="+v)
		}
		body = strings.NewReader(strings.Join(form, "&"))
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	if data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Inject trace context into outgoing headers (W3C Trace Context)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	return p.client.Do(req)
}
