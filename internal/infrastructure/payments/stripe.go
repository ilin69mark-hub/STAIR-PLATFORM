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
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// webhookTolerance — допустимое отклонение timestamp подписи webhook от NOW:
// защита от replay перехваченного запроса (P1-1). События Stripe, доставленные
// повторной попыткой, подписываются заново (новый t), поэтому retry не страдает.
const webhookTolerance = 5 * time.Minute

// StripeProvider — реализация Provider для Stripe.
type StripeProvider struct {
	secretKey     string
	webhookSecret string
	baseURL       string
	client        *http.Client
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
		"mode":                                "payment",
		"success_url":                         params.SuccessURL,
		"cancel_url":                          params.CancelURL,
		"customer_email":                      params.Email,
		"line_items[0][price_data][currency]": params.Currency,
		"line_items[0][price_data][product_data][name]": params.Description,
		"line_items[0][price_data][unit_amount]":        fmt.Sprintf("%d", params.Amount),
		"line_items[0][quantity]":                       "1",
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stripe: get session: %s", string(body))
	}

	var result struct {
		ID            string            `json:"id"`
		Status        string            `json:"status"`
		AmountTotal   int64             `json:"amount_total"`
		Currency      string            `json:"currency"`
		PaymentStatus string            `json:"payment_status"`
		CreatedAt     int64             `json:"created"`
		Metadata      map[string]string `json:"metadata"`
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
		ID:        result.ID,
		Status:    status,
		Amount:    result.AmountTotal,
		Currency:  result.Currency,
		Metadata:  result.Metadata,
		CreatedAt: createdAt,
	}, nil
}

// VerifyWebhookSignature проверяет подпись Stripe webhook: HMAC-SHA256
// payload вместе с timestamp (Stripe-формат Signature t=ts,v1=sig) И окно
// времени (P1-1): подпись старше webhookTolerance отклоняется — это защита
// от replay перехваченного запроса. Заголовок может содержать НЕСКОЛЬКО
// v1= (ротация ключей Stripe: подписи старым и новым секретом, CWE-754) —
// принимаем, если ЛЮБАЯ v1 совпадает с настроенным webhookSecret (S-141 №9);
// timestamp берём из первого t=. Пустой/битый заголовок, отсутствие t или
// v1, несовпадение всех v1 и просрочка tolerance по-прежнему отклоняются
// (поведение не ослабляется).
func (p *StripeProvider) VerifyWebhookSignature(payload []byte, signature string) error {
	// Stripe использует формат: t=timestamp,v1=signature. Пар может быть
	// несколько (ротация ключей): t=<ts>,v1=<подпись-старым>,v1=<подпись-новым>.
	parts := strings.Split(signature, ",")
	if len(parts) < 2 {
		return fmt.Errorf("stripe: invalid signature format")
	}

	var timestamp string
	sigs := make([]string, 0, len(parts))
	tSeen := false
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			// Timestamp — первый t в заголовке.
			if !tSeen {
				timestamp = kv[1]
				tSeen = true
			}
		case "v1":
			sigs = append(sigs, kv[1])
		}
	}

	if timestamp == "" || len(sigs) == 0 {
		return fmt.Errorf("stripe: missing timestamp or signature")
	}

	// Формируем подписанный payload
	signedPayload := timestamp + "." + string(payload)

	// Вычисляем HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// При ротации ключей Stripe присылает подписи старым и новым секретом —
	// достаточно совпадения ЛЮБОЙ v1 с настроенным webhookSecret.
	matched := false
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), []byte(expectedSig)) {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("stripe: invalid signature")
	}

	// Окно времени: timestamp передаётся открыто в подписи, но подпись
	// защищает его от подмены — проверка корректна.
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("stripe: invalid signature timestamp: %w", err)
	}
	eventTime := time.Unix(ts, 0)
	if d := time.Since(eventTime); d > webhookTolerance || d < -webhookTolerance {
		return fmt.Errorf("stripe: signature timestamp outside tolerance window (age %s)", d.Round(time.Second))
	}

	return nil
}

// ParseWebhookEvent парсит Stripe webhook event.
func (p *StripeProvider) ParseWebhookEvent(payload []byte) (*StripeWebhookEvent, error) {
	var event struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID            string            `json:"id"`
				Status        string            `json:"status"`
				PaymentStatus string            `json:"payment_status"`
				AmountTotal   int64             `json:"amount_total"`
				Currency      string            `json:"currency"`
				CreatedAt     int64             `json:"created"`
				Metadata      map[string]string `json:"metadata"`
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
		EventID:     event.ID,
		EventType:   webhookType,
		Provider:    "stripe",
		CheckoutID:  event.Data.Object.ID,
		Status:      status,
		AmountMinor: event.Data.Object.AmountTotal,
		Currency:    event.Data.Object.Currency,
		CreatedAt:   time.Unix(event.CreatedAt, 0),
	}, nil
}

// makeRequest выполняет HTTP запрос к Stripe API с trace context propagation.
func (p *StripeProvider) makeRequest(ctx context.Context, method, path string, data map[string]string) (*http.Response, error) {
	var body io.Reader
	if data != nil {
		// Формируем form-urlencoded data с правильным URL-кодированием
		values := url.Values{}
		for k, v := range data {
			values.Set(k, v)
		}
		body = strings.NewReader(values.Encode())
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
