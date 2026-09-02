package payments

import (
	"testing"
)

func TestStripeProviderCreation(t *testing.T) {
	provider := NewStripeProvider("sk_test_key", "whsec_test_secret")

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.Name() != "stripe" {
		t.Errorf("expected name 'stripe', got %q", provider.Name())
	}
}

func TestStripeProviderVerifyWebhookSignature(t *testing.T) {
	provider := NewStripeProvider("sk_test_key", "whsec_test_secret")

	// Тест с невалидной подписью
	err := provider.VerifyWebhookSignature([]byte("test"), "invalid_signature")
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestStripeProviderVerifyWebhookSignatureFormat(t *testing.T) {
	provider := NewStripeProvider("sk_test_key", "whsec_test_secret")

	// Тест с невалидным форматом
	err := provider.VerifyWebhookSignature([]byte("test"), "t=123,v1=")
	if err == nil {
		t.Error("expected error for empty signature")
	}
}

func TestStripeProviderParseWebhookEvent(t *testing.T) {
	provider := NewStripeProvider("sk_test_key", "whsec_test_secret")

	// Тест парсинга checkout.session.completed
	payload := []byte(`{
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "cs_test_123",
				"status": "complete",
				"payment_status": "paid",
				"created": 1234567890,
				"metadata": {
					"order_id": "order-123"
				}
			}
		},
		"created": 1234567890
	}`)

	event, err := provider.ParseWebhookEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != WebhookEventCheckoutCompleted {
		t.Errorf("expected type %q, got %q", WebhookEventCheckoutCompleted, event.EventType)
	}
	if event.CheckoutID != "cs_test_123" {
		t.Errorf("expected session ID 'cs_test_123', got %q", event.CheckoutID)
	}
	if event.Status != CheckoutStatusCompleted {
		t.Errorf("expected status %q, got %q", CheckoutStatusCompleted, event.Status)
	}
}

func TestStripeProviderParseWebhookEventExpired(t *testing.T) {
	provider := NewStripeProvider("sk_test_key", "whsec_test_secret")

	// Тест парсинга checkout.session.expired
	payload := []byte(`{
		"type": "checkout.session.expired",
		"data": {
			"object": {
				"id": "cs_test_456",
				"status": "expired",
				"payment_status": "unpaid",
				"created": 1234567890,
				"metadata": {}
			}
		},
		"created": 1234567890
	}`)

	event, err := provider.ParseWebhookEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != WebhookEventCheckoutExpired {
		t.Errorf("expected type %q, got %q", WebhookEventCheckoutExpired, event.EventType)
	}
	if event.Status != CheckoutStatusPending {
		t.Errorf("expected status %q, got %q", CheckoutStatusPending, event.Status)
	}
}

func TestStripeProviderParseWebhookEventInvalidJSON(t *testing.T) {
	provider := NewStripeProvider("sk_test_key", "whsec_test_secret")

	payload := []byte(`invalid json`)

	_, err := provider.ParseWebhookEvent(payload)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCheckoutSessionStatuses(t *testing.T) {
	statuses := []string{
		CheckoutStatusPending,
		CheckoutStatusActive,
		CheckoutStatusCompleted,
		CheckoutStatusFailed,
		CheckoutStatusExpired,
		CheckoutStatusCancelled,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("expected non-empty status")
		}
	}
}

func TestWebhookEventTypes(t *testing.T) {
	types := []string{
		WebhookEventCheckoutCompleted,
		WebhookEventCheckoutFailed,
		WebhookEventCheckoutExpired,
	}

	for _, eventType := range types {
		if eventType == "" {
			t.Error("expected non-empty event type")
		}
	}
}
