package payments

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/infrastructure/integrations"
)

func TestMockProviderCreateCheckout(t *testing.T) {
	p := NewMockProvider("https://pay.example.com")
	id, url, err := p.CreateCheckout(context.Background(), 5000, "USD")
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if id == "" || len(id) != 32 {
		t.Fatalf("expected 32-hex checkout id, got %q", id)
	}
	if url != "https://pay.example.com/pay/"+id {
		t.Fatalf("checkout url = %q", url)
	}
}

func TestMockProviderCreateCheckoutUnique(t *testing.T) {
	p := NewMockProvider("https://pay.example.com")
	a, _, _ := p.CreateCheckout(context.Background(), 1, "USD")
	b, _, _ := p.CreateCheckout(context.Background(), 1, "USD")
	if a == b {
		t.Fatal("expected distinct checkout ids")
	}
}

func TestMockProviderCreateCheckoutInvalid(t *testing.T) {
	p := NewMockProvider("https://pay.example.com")
	if _, _, err := p.CreateCheckout(context.Background(), 0, "USD"); err == nil {
		t.Fatal("expected error for zero amount")
	}
	if _, _, err := p.CreateCheckout(nil, 100, "USD"); err == nil {
		t.Fatal("expected error for nil context")
	}
}

func TestMockProviderSignWebhookVerifies(t *testing.T) {
	p := NewMockProvider("https://pay.example.com")
	const secret = "topsecret"
	body, ts, sig, err := p.SignWebhook(secret, WebhookEvent{
		EventType:   "payment.succeeded",
		Provider:    "mock",
		CheckoutID:  "chk-123",
		Status:      "paid",
		AmountMinor: 5000,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatalf("SignWebhook: %v", err)
	}
	if !strings.HasPrefix(sig, "v1:") || ts == "" {
		t.Fatalf("bad signature/timestamp: %q / %q", sig, ts)
	}
	if err := integrations.Verify(secret, ts, sig, body, integrations.MaxTimestampAge); err != nil {
		t.Fatalf("Verify produced signature should pass: %v", err)
	}
}

func TestMockProviderSignWebhookTampered(t *testing.T) {
	p := NewMockProvider("https://pay.example.com")
	body, ts, sig, err := p.SignWebhook("secret", WebhookEvent{
		EventType: "payment.succeeded", Provider: "mock", CheckoutID: "chk-123",
		Status: "paid", AmountMinor: 5000, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("SignWebhook: %v", err)
	}
	body = append(body, 'x')
	if err := integrations.Verify("secret", ts, sig, body, integrations.MaxTimestampAge); err == nil {
		t.Fatal("expected verification failure on tampered body")
	}
}
