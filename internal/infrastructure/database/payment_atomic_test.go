package database

import (
	"context"
	"testing"
	"time"

	"stairplatform/internal/application/payments"
)

// TestApplyVerifiedEventTxAtomic (DB-002, forensic 2026-09-24) — статус
// интента и запись в журнал payment_events атомарны: если журнал не пишется,
// статус не должен остаться изменённым.
func TestApplyVerifiedEventTxAtomic(t *testing.T) {
	payRepo, projRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projRepo)
	projectID := testProject(t, ctx, projRepo, tenant, "Pay atomic")
	owner := testOwnerID(t, projRepo, tenant)

	intent := &payments.PaymentIntent{
		TenantID: tenant, ProjectID: projectID, UserID: owner,
		AmountMinor: 90_000, Currency: "RUB", Status: payments.StatusPending,
		Provider: "stripe", ProviderCheckoutID: "chk-atomic-1",
	}
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("create intent: %v", err)
	}

	// Событие с несуществующим intent_id: вставка в журнал упадёт по FK уже
	// после UPDATE статуса внутри транзакции → статус обязан откатиться.
	bad := &payments.PaymentEvent{
		TenantID:  tenant,
		IntentID:  "00000000-0000-0000-0000-000000000000",
		EventType: payments.EventTypePaymentSucceeded,
		Payload:   []byte(`{}`),
	}
	paidAt := time.Now().UTC()
	if err := payRepo.ApplyVerifiedEventTx(ctx, tenant, intent.ID, payments.StatusPaid, &paidAt, bad); err == nil {
		t.Fatal("want error when journal insert fails")
	}

	got, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got.Status != payments.StatusPending {
		t.Fatalf("status must roll back to pending, got %s", got.Status)
	}
	if got.PaidAt != nil {
		t.Fatalf("paid_at must roll back too, got %v", got.PaidAt)
	}

	// Успешный путь: статус меняется, событие журналируется в той же транзакции.
	okEvent := &payments.PaymentEvent{
		TenantID:  tenant,
		IntentID:  intent.ID,
		EventType: payments.EventTypePaymentSucceeded,
		Payload:   []byte(`{"x":1}`),
	}
	if err := payRepo.ApplyVerifiedEventTx(ctx, tenant, intent.ID, payments.StatusPaid, &paidAt, okEvent); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	got2, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil || got2.Status != payments.StatusPaid {
		t.Fatalf("want paid, got %+v err=%v", got2, err)
	}
	if okEvent.ID == "" {
		t.Fatal("event id must be returned from the transaction")
	}
}

func TestMarkRefundedRollsBackWhenEventInsertFails(t *testing.T) {
	payRepo, projRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projRepo)
	intent := &payments.PaymentIntent{
		TenantID: tenant, AmountMinor: 900, Currency: "RUB", Status: payments.StatusPaid,
		Provider: "mock", ProviderCheckoutID: "refund-atomic-" + itoaUD(),
	}
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("create intent: %v", err)
	}
	badEvent := &payments.PaymentEvent{
		TenantID: tenant, IntentID: "00000000-0000-0000-0000-000000000000",
		EventType: payments.EventTypePaymentRefunded, Payload: []byte(`{}`),
	}

	if _, err := payRepo.MarkRefunded(ctx, tenant, intent.ID, badEvent); err == nil {
		t.Fatal("want event insert error")
	}
	got, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got.Status != payments.StatusPaid {
		t.Fatalf("status must roll back to paid, got %s", got.Status)
	}
}
