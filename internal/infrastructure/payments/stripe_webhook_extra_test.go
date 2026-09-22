package payments

// S-133: ветки HandleStripeWebhook (failed/expired/default/dedup-err/no-processor)
// + mock/adapter остатки.

import (
	"context"
	"errors"
	"testing"
)

func TestStripeWebhookFailedAndExpired(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	for _, tc := range []struct {
		name string
		typ  string
	}{
		{"failed", "checkout.failed"},
		{"expired", "checkout.session.expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
			proc := &recordingProcessor{}
			svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(newFakeDeduper())

			raw := webhookRaw("evt-"+tc.name, "cs-"+tc.name)
			raw["type"] = tc.typ
			payload, signature := signedPayloadFor(t, secret, raw)
			if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
				t.Fatalf("HandleStripeWebhook: %v", err)
			}
			if !proc.applied {
				t.Fatal("event must be applied")
			}
		})
	}
}

func TestStripeWebhookUnhandledTypeIgnored(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	proc := &recordingProcessor{}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(newFakeDeduper())

	raw := webhookRaw("evt-unknown", "cs-unknown")
	raw["type"] = "customer.created"
	payload, signature := signedPayloadFor(t, secret, raw)
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("unhandled must not error: %v", err)
	}
	if proc.applied {
		t.Fatal("unhandled event must not be applied")
	}
}

func TestStripeWebhookDedupError(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	deduper.checkAndMark = func(_, _ string) (bool, error) {
		return false, errors.New("redis down")
	}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(&recordingProcessor{}).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt-dedup-err", "cs-x"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err == nil {
		t.Fatal("dedup error: want error")
	}
}

func TestStripeWebhookNoProcessor(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	svc := NewStripeWebhookService(provider, nil).WithEventDeduper(newFakeDeduper())

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt-noproc", "cs-x"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("no processor must be ignored: %v", err)
	}
}

func TestMockProviderNameAndNilCtx(t *testing.T) {
	p := NewMockProvider("https://pay.test")
	if p.Name() != "mock" {
		t.Fatalf("Name = %q", p.Name())
	}
	if _, _, err := p.CreateCheckout(nil, 100, "USD"); err == nil {
		t.Fatal("nil ctx: want error")
	}
	if _, _, err := p.CreateCheckout(context.Background(), 0, "USD"); err == nil {
		t.Fatal("zero amount: want error")
	}
}

func TestStripeAdapterCreateCheckoutSuccess(t *testing.T) {
	a := testCBAdapter(t, okCheckoutHandler(t))
	// Тот же пакет — доступ к inner напрямую.
	id, url, err := a.inner.CreateCheckout(context.Background(), 2000, "EUR")
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if id != "cs_cb_1" || url == "" {
		t.Fatalf("got %q %q", id, url)
	}
}
