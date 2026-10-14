package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	apppayments "stairplatform/internal/application/payments"
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
				"id":             "cs_test_123",
				"status":         "complete",
				"payment_status": "paid",
				"amount_total":   1000,
				"currency":       "usd",
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
	// #nosec G101 -- test-only fake Stripe webhook secret
	secret := "whsec_test_secret"
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	svc := NewStripeWebhookService(provider, nil)

	// Use Stripe's data.object format
	raw := map[string]interface{}{
		"id":   "evt_test_1",
		"type": "checkout.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":             "cs_test_123",
				"status":         "complete",
				"payment_status": "paid",
				"amount_total":   1000,
				"currency":       "usd",
			},
		},
		"created": 1234567890,
	}
	payload, _ := json.Marshal(raw)

	// Generate valid Stripe signature: t=now,v1=signature (tolerance window,
	// P1-1: статический timestamp из 2009 больше не проходит)
	ts := fmt.Sprintf("%d", time.Now().Unix())
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

func TestStripeWebhookServiceHandleStaleSignature(t *testing.T) {
	// #nosec G101 -- test-only fake Stripe webhook secret
	secret := "whsec_test_secret"
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	svc := NewStripeWebhookService(provider, nil)

	raw := map[string]interface{}{
		"id":   "evt_test_stale",
		"type": "checkout.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":             "cs_test_456",
				"status":         "complete",
				"payment_status": "paid",
				"amount_total":   1000,
				"currency":       "usd",
			},
		},
		"created": 1234567890,
	}
	payload, _ := json.Marshal(raw)

	// Подпись отличная (HMAC валиден), но timestamp устарел (2009 год):
	// деploy П1-1 требует отклонения replay даже при валидной подписи.
	ts := fmt.Sprintf("%d", 1234567890)
	signedPayload := ts + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))
	signature := fmt.Sprintf("t=%s,v1=%s", ts, sig)

	err := svc.HandleStripeWebhook(context.Background(), payload, signature)
	if err == nil {
		t.Fatal("expected error for stale (replayed) signature")
	}
}

func TestParseStripeEvent(t *testing.T) {
	raw := map[string]interface{}{
		"type": "checkout.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":             "cs_test_123",
				"status":         "complete",
				"payment_status": "paid",
				"amount_total":   2500,
				"currency":       "usd",
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

// ---- Дедупликация webhook-событий (P1-1) ----

type fakeDeduper struct {
	seen         map[string]bool
	cleared      map[string]bool
	checkAndMark func(provider, eventID string) (bool, error)
}

func (f *fakeDeduper) CheckAndMark(_ context.Context, provider, eventID string) (bool, error) {
	if f.checkAndMark != nil {
		return f.checkAndMark(provider, eventID)
	}
	if f.seen[provider+":"+eventID] {
		return false, nil
	}
	f.seen[provider+":"+eventID] = true
	return true, nil
}

func (f *fakeDeduper) Clear(_ context.Context, provider, eventID string) error {
	f.cleared[provider+":"+eventID] = true
	return nil
}

func newFakeDeduper() *fakeDeduper {
	return &fakeDeduper{seen: map[string]bool{}, cleared: map[string]bool{}}
}

func signedPayloadFor(t *testing.T, secret string, raw map[string]interface{}) ([]byte, string) {
	t.Helper()
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(payload)))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload, fmt.Sprintf("t=%s,v1=%s", ts, sig)
}

func webhookRaw(evtID, checkoutID string) map[string]interface{} {
	return map[string]interface{}{
		"id":   evtID,
		"type": "checkout.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":             checkoutID,
				"status":         "complete",
				"payment_status": "paid",
				"amount_total":   1000,
				"currency":       "usd",
			},
		},
		"created": time.Now().Unix(),
	}
}

type recordingProcessor struct {
	applied  bool
	applyErr error
}

func (r *recordingProcessor) ApplyVerifiedEvent(_ context.Context, _, _, _ string, _ int64, _ string, _ []byte) error {
	r.applied = true
	return r.applyErr
}

// readerProcessor — IntentProcessor + IntentReader (S-141 №3): применяет
// событие и отдаёт состояние интента для сверки при дубликате.
type readerProcessor struct {
	recordingProcessor
	intent   *apppayments.PaymentIntent
	readErr  error
	readDone int
}

func (r *readerProcessor) GetIntentByProviderCheckout(_ context.Context, _, _ string) (*apppayments.PaymentIntent, error) {
	r.readDone++
	if r.readErr != nil {
		return nil, r.readErr
	}
	if r.intent == nil {
		return nil, apppayments.ErrNotFound
	}
	return r.intent, nil
}

func TestStripeWebhookDeduplicates(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура, не реальный секрет
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	proc := &recordingProcessor{}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_dup", "cs_dup"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("first delivery failed: %v", err)
	}
	if !proc.applied {
		t.Fatal("expected first event to be applied")
	}

	// Повторная доставка того же события: должна быть проигнорирована без
	// повторного применения и без ошибки (Stripe считает 2xx успехом).
	proc.applied = false
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("duplicate delivery should not error: %v", err)
	}
	if proc.applied {
		t.Fatal("duplicate event must not be applied twice")
	}
}

func TestStripeWebhookDedupMarkerClearedOnFailure(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура, не реальный секрет
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	failing := &recordingProcessor{}
	failing.applyErr = fmt.Errorf("boom")
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(failing).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_fail", "cs_fail"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err == nil {
		t.Fatal("expected error when processor fails")
	}
	// Метка должна быть снята, чтобы Stripe-retry того же события сработал.
	if !deduper.cleared["stripe:evt_fail"] {
		t.Fatal("expected dedup marker to be cleared on failure")
	}
}

// ---- S-141 №3 (CWE-367/703): crash-window self-healing дубликатов ----

// TestStripeWebhookDuplicateReconcilesAfterCrashWindow — тест-ловушка аудита:
// mark → simulated crash (CheckAndMark выполнен, Apply НЕ вызван — процесс
// «убит», Clear не вызывался) → повторная доставка того же event.id →
// дубликат НЕ отбрасывается молча: ApplyVerifiedEvent ВЫЗВАН (reconcile),
// интент переходит в succeeded.
func TestStripeWebhookDuplicateReconcilesAfterCrashWindow(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	deduper.seen["stripe:evt_crash"] = true // метка пережила «крэш» между mark и apply
	proc := &readerProcessor{intent: &apppayments.PaymentIntent{Status: apppayments.StatusPending}}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_crash", "cs_crash"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("reconcile delivery failed: %v", err)
	}
	if !proc.applied {
		t.Fatal("crash-window duplicate must be reconciled: ApplyVerifiedEvent must be called")
	}
	if proc.readDone == 0 {
		t.Fatal("reconcile must read the intent state")
	}
	// Успешный reconcile метку НЕ снимает — дедуп продолжает работать.
	if deduper.cleared["stripe:evt_crash"] {
		t.Fatal("dedup marker must survive successful reconcile")
	}
}

// TestStripeWebhookDuplicateIgnoredWhenIntentInTargetStatus — чистый дубликат:
// интент уже succeeded_ (paid), повтор completed — Apply не вызывается
// повторно, интент untouched.
func TestStripeWebhookDuplicateIgnoredWhenIntentInTargetStatus(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	deduper.seen["stripe:evt_pure"] = true
	paidAt := time.Now().UTC()
	proc := &readerProcessor{intent: &apppayments.PaymentIntent{Status: apppayments.StatusPaid, PaidAt: &paidAt}}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_pure", "cs_pure"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("pure duplicate must not error: %v", err)
	}
	if proc.applied {
		t.Fatal("pure duplicate (intent in target status) must not re-apply")
	}
	if proc.readDone == 0 {
		t.Fatal("reconcile check must still read the intent before deciding")
	}
}

// TestStripeWebhookDuplicateReconcilesWhenIntentStatusDiffers — интент в ином
// (нецелевом) статусе: failed-интент + повтор completed — событие НЕ
// отбрасывается молча, ApplyVerifiedEvent вызывается (репозиторий/guard №4
// решает, применится ли переход; здесь ключевое — reconcile вместо игнора).
func TestStripeWebhookDuplicateReconcilesWhenIntentStatusDiffers(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	deduper.seen["stripe:evt_diff"] = true
	proc := &readerProcessor{intent: &apppayments.PaymentIntent{Status: apppayments.StatusFailed}}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_diff", "cs_diff"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("different-status duplicate must not error: %v", err)
	}
	if !proc.applied {
		t.Fatal("different-status duplicate must be reconciled: ApplyVerifiedEvent must be called")
	}
}

// TestStripeWebhookDuplicateDegradesWithoutReader — ридер не реализован:
// деградация к старому поведению — дубликат игнорируется без ошибки и без
// повторного применения.
func TestStripeWebhookDuplicateDegradesWithoutReader(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	deduper.seen["stripe:evt_noreader"] = true
	proc := &recordingProcessor{}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_noreader", "cs_noreader"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("duplicate without reader must not error: %v", err)
	}
	if proc.applied {
		t.Fatal("duplicate without reader must be ignored (old behaviour)")
	}
}

// TestStripeWebhookDuplicateReconcileReadError — ошибка чтения интента при
// сверке дубликата: ошибка наружу (Stripe ретраит), Apply НЕ вызывается,
// метка НЕ снимается (crash-window защита продолжает действовать).
func TestStripeWebhookDuplicateReconcileReadError(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	deduper.seen["stripe:evt_rderr"] = true
	proc := &readerProcessor{readErr: errors.New("db down")}
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_rderr", "cs_rderr"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); err == nil {
		t.Fatal("intent read error during reconcile must surface")
	}
	if proc.applied {
		t.Fatal("must not apply on intent read error")
	}
	if deduper.cleared["stripe:evt_rderr"] {
		t.Fatal("marker must survive reconcile read error (crash-window protection)")
	}
}

// TestStripeWebhookDuplicateIntentMissingApplies — дубликат при отсутствующем
// интенте: событие применяется (ApplyVerifiedEvent вызовется и вернёт
// ErrNotFound наружу, метка снимется), а не теряется молча.
func TestStripeWebhookDuplicateIntentMissingApplies(t *testing.T) {
	secret := "whsec_test_secret" //nolint:gosec // тестовая фикстура
	provider := NewStripeAdapter(NewStripeProvider("sk_test_xxx", secret))
	deduper := newFakeDeduper()
	deduper.seen["stripe:evt_missing"] = true
	proc := &readerProcessor{intent: nil} // GetIntentByProviderCheckout → ErrNotFound
	proc.applyErr = apppayments.ErrNotFound
	svc := NewStripeWebhookService(provider, nil).WithIntentProcessor(proc).WithEventDeduper(deduper)

	payload, signature := signedPayloadFor(t, secret, webhookRaw("evt_missing", "cs_missing"))
	if err := svc.HandleStripeWebhook(context.Background(), payload, signature); !errors.Is(err, apppayments.ErrNotFound) {
		t.Fatalf("missing intent duplicate: want ErrNotFound, got %v", err)
	}
	if !proc.applied {
		t.Fatal("missing-intent duplicate must attempt apply, not silently drop")
	}
	if !deduper.cleared["stripe:evt_missing"] {
		t.Fatal("marker must be cleared on apply failure so retry can re-apply")
	}
}
