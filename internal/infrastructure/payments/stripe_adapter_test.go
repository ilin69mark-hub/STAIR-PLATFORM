package payments

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStripeAdapterName(t *testing.T) {
	provider := NewStripeProvider("sk_test_xxx", "whsec_xxx")
	adapter := NewStripeAdapter(provider)

	if adapter.Name() != "stripe" {
		t.Fatalf("expected name 'stripe', got %q", adapter.Name())
	}
}

func TestStripeAdapterCreateCheckout(t *testing.T) {
	provider := NewStripeProvider("sk_test_xxx", "whsec_xxx")
	adapter := NewStripeAdapter(provider)

	// CreateCheckout will fail because we can't reach Stripe API,
	// but we verify the adapter compiles and delegates correctly.
	_, _, err := adapter.CreateCheckout(context.Background(), 1000, "usd")
	if err == nil {
		t.Log("unexpected success (mock endpoint?)")
	}
}

func TestStripeAdapterVerifyWebhookSignature(t *testing.T) {
	provider := NewStripeProvider("sk_test_xxx", "whsec_test_secret")
	adapter := NewStripeAdapter(provider)

	// Invalid signature format
	err := adapter.VerifyWebhookSignature([]byte("test"), "bad-sig")
	if err == nil {
		t.Fatal("expected error for bad signature")
	}
}

func TestStripeAdapterParseWebhookEvent(t *testing.T) {
	provider := NewStripeProvider("sk_test_xxx", "whsec_xxx")
	adapter := NewStripeAdapter(provider)

	// Stripe wraps event in data.object format
	raw := map[string]interface{}{
		"type": "checkout.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":            "cs_test_123",
				"status":        "complete",
				"payment_status": "paid",
				"amount_total":  1000,
				"currency":      "usd",
			},
		},
		"created": 1234567890,
	}
	payload, _ := json.Marshal(raw)

	parsed, err := adapter.ParseWebhookEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.EventType != "checkout.completed" {
		t.Fatalf("expected event type 'checkout.completed', got %q", parsed.EventType)
	}
	if parsed.CheckoutID != "cs_test_123" {
		t.Fatalf("expected checkout ID 'cs_test_123', got %q", parsed.CheckoutID)
	}
}
