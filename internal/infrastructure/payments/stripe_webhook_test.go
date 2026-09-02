package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
)

func TestStripeWebhookServiceCreation(t *testing.T) {
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", "whsec_test_secret"))
	svc := NewStripeWebhookService(provider, nil)

	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestStripeWebhookServiceHandleBadSignature(t *testing.T) {
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", "whsec_test_secret"))
	svc := NewStripeWebhookService(provider, nil)

	// Use Stripe's data.object format
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

	err := svc.HandleStripeWebhook(context.Background(), payload, "bad-signature")
	if err == nil {
		t.Fatal("expected error for bad signature")
	}
}

func TestStripeWebhookServiceHandleGoodSignature(t *testing.T) {
	secret := "whsec_test_secret"
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	svc := NewStripeWebhookService(provider, nil)

	// Use Stripe's data.object format
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

	// Generate valid Stripe signature: t=timestamp,v1=signature
	ts := fmt.Sprintf("%d", 1234567890)
	signedPayload := ts + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))
	signature := fmt.Sprintf("t=%s,v1=%s", ts, sig)

	err := svc.HandleStripeWebhook(context.Background(), payload, signature)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseStripeEvent(t *testing.T) {
	raw := map[string]interface{}{
		"type": "checkout.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":            "cs_test_123",
				"status":        "complete",
				"payment_status": "paid",
				"amount_total":  2500,
				"currency":      "usd",
			},
		},
		"created": 1234567890,
	}
	payload, _ := json.Marshal(raw)

	event, err := ParseStripeEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventType != "checkout.completed" {
		t.Fatalf("expected event type 'checkout.completed', got %q", event.EventType)
	}
	if event.CheckoutID != "cs_test_123" {
		t.Fatalf("expected checkout ID 'cs_test_123', got %q", event.CheckoutID)
	}
	if event.AmountMinor != 2500 {
		t.Fatalf("expected amount 2500, got %d", event.AmountMinor)
	}
}

func TestParseStripeEventInvalid(t *testing.T) {
	_, err := ParseStripeEvent([]byte("invalid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
