package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"stairplatform/internal/application/payments"
)

// Регрессия CRITICAL-04 (2026-09-27): платёжный журнал не должен лгать.
//
// ДО фикса ApplyVerifiedEventTx при отклонённом переходе (RowsAffected == 0)
// писал warn-лог и ВСЁ РАВНО писал событие в payment_events. То есть поздний
// checkout.session.expired оставлял в журнале запись payment.failed для
// интента, который оставался в статусе paid, а вызывающий получал `err == nil`
// и `intent.Status = failed`. Три источника правды (БД, журнал, объект на
// выходе) расходились между собой, и расхождение было невидимым.
//
// Проверяется на живой БД; без STAIR_TEST_DATABASE_URL тест скипается.

func c004PaidIntent(t *testing.T, payRepo *PaymentRepository, checkoutID string) *payments.PaymentIntent {
	t.Helper()
	intent := &payments.PaymentIntent{
		TenantID: "unused", AmountMinor: 90_000, Currency: "RUB",
		Status: payments.StatusPending, Provider: "stripe", ProviderCheckoutID: checkoutID,
	}
	return intent
}

// c004CountEvents — число записей журнала по интенту.
func c004CountEvents(t *testing.T, payRepo *PaymentRepository, tenant, intentID string) int {
	t.Helper()
	var n int
	err := payRepo.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM payment_events WHERE tenant_id = $1 AND intent_id = $2`,
		tenant, intentID).Scan(&n)
	if err != nil {
		t.Fatalf("count payment_events: %v", err)
	}
	return n
}

// TestCRIT004_RejectedTransitionWritesNoEvent — ядро дефекта: отклонённый
// переход не оставляет записи в журнале и возвращает ErrStatusConflict.
func TestCRIT004_RejectedTransitionWritesNoEvent(t *testing.T) {
	payRepo, projRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projRepo)
	projectID := testProject(t, ctx, projRepo, tenant, "C004 conflict")
	owner := testOwnerID(t, projRepo, tenant)

	intent := c004PaidIntent(t, payRepo, "c004-conflict-1")
	intent.TenantID, intent.ProjectID, intent.UserID = tenant, projectID, owner
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("create intent: %v", err)
	}

	// Первое событие: оплата прошла, журнал пополнился.
	paidAt := time.Now().UTC()
	ok := &payments.PaymentEvent{
		TenantID: tenant, IntentID: intent.ID,
		EventType: payments.EventTypePaymentSucceeded, Payload: []byte(`{"ok":1}`),
	}
	if err := payRepo.ApplyVerifiedEventTx(ctx, tenant, intent.ID, payments.StatusPaid, &paidAt, ok); err != nil {
		t.Fatalf("first transition must succeed: %v", err)
	}
	before := c004CountEvents(t, payRepo, tenant, intent.ID)
	if before != 1 {
		t.Fatalf("expected 1 event after payment, got %d", before)
	}

	// Позднее событие: попытка перевести paid -> failed. Терминальный
	// статус не переопределяется.
	late := &payments.PaymentEvent{
		TenantID: tenant, IntentID: intent.ID,
		EventType: payments.EventTypePaymentFailed, Payload: []byte(`{"late":1}`),
	}
	err := payRepo.ApplyVerifiedEventTx(ctx, tenant, intent.ID, payments.StatusFailed, nil, late)
	if !errors.Is(err, payments.ErrStatusConflict) {
		t.Fatalf("want ErrStatusConflict, got %v", err)
	}

	// Статус не тронут.
	got, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got.Status != payments.StatusPaid {
		t.Errorf("status = %s, want paid — отклонённый переход не должен его менять", got.Status)
	}
	if got.PaidAt == nil {
		t.Error("paid_at must be preserved")
	}

	// И главное: в журнале НЕ появилось событие, которого не было.
	after := c004CountEvents(t, payRepo, tenant, intent.ID)
	if after != before {
		t.Fatalf("CRITICAL-04: journal grew from %d to %d on a rejected transition", before, after)
	}
}

// TestCRIT004_UpdateStatusReportsRejection — резервный путь (репозиторий без
// EventApplier) шёл через UpdateStatus, который на отклонении делал warn-лог и
// `return nil`. Вызывающий не мог отличить успех от отказа.
func TestCRIT004_UpdateStatusReportsRejection(t *testing.T) {
	payRepo, projRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projRepo)
	projectID := testProject(t, ctx, projRepo, tenant, "C004 update")
	owner := testOwnerID(t, projRepo, tenant)

	intent := c004PaidIntent(t, payRepo, "c004-update-1")
	intent.TenantID, intent.ProjectID, intent.UserID = tenant, projectID, owner
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("create intent: %v", err)
	}
	paidAt := time.Now().UTC()
	if err := payRepo.UpdateStatus(ctx, tenant, intent.ID, payments.StatusPaid, &paidAt); err != nil {
		t.Fatalf("update to paid: %v", err)
	}

	err := payRepo.UpdateStatus(ctx, tenant, intent.ID, payments.StatusFailed, nil)
	if !errors.Is(err, payments.ErrStatusConflict) {
		t.Fatalf("UpdateStatus must report the rejection, got %v", err)
	}
	got, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got.Status != payments.StatusPaid {
		t.Errorf("status = %s, want paid", got.Status)
	}
}

// TestCRIT004_MissingIntentIsNotFoundNotConflict — «интента нет» и «переход
// отклонён» — разные ситуации: обработчик webhook на первом отвечает 404, на
// втором подтверждает доставку (200). Смешивать их нельзя.
func TestCRIT004_MissingIntentIsNotFoundNotConflict(t *testing.T) {
	payRepo, projRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projRepo)

	missing := "00000000-0000-0000-0000-00000000c004"
	ev := &payments.PaymentEvent{
		TenantID: tenant, IntentID: missing,
		EventType: payments.EventTypePaymentSucceeded, Payload: []byte(`{}`),
	}
	err := payRepo.ApplyVerifiedEventTx(ctx, tenant, missing, payments.StatusPaid, nil, ev)
	if !errors.Is(err, payments.ErrNotFound) {
		t.Fatalf("want ErrNotFound for a missing intent, got %v", err)
	}
	if errors.Is(err, payments.ErrStatusConflict) {
		t.Error("missing intent must not be reported as a status conflict")
	}

	err = payRepo.UpdateStatus(ctx, tenant, missing, payments.StatusPaid, nil)
	if !errors.Is(err, payments.ErrNotFound) {
		t.Fatalf("UpdateStatus: want ErrNotFound, got %v", err)
	}
}

// TestCRIT004_IdempotentRedeliveryIsAcceptedAndRecordedOnce — повторная
// доставка ТОГО ЖЕ терминального события — не конфликт (SQL-гард разрешает
// «статус в статус»). Она обязана проходить и писать в журнал, иначе
// платёжная система теряет подтверждение при ретраях PSP.
func TestCRIT004_IdempotentRedeliveryIsAcceptedAndRecordedOnce(t *testing.T) {
	payRepo, projRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projRepo)
	projectID := testProject(t, ctx, projRepo, tenant, "C004 retry")
	owner := testOwnerID(t, projRepo, tenant)

	intent := c004PaidIntent(t, payRepo, "c004-retry-1")
	intent.TenantID, intent.ProjectID, intent.UserID = tenant, projectID, owner
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("create intent: %v", err)
	}
	paidAt := time.Now().UTC()
	for i := 1; i <= 2; i++ {
		ev := &payments.PaymentEvent{
			TenantID: tenant, IntentID: intent.ID,
			EventType: payments.EventTypePaymentSucceeded, Payload: []byte(`{"n":1}`),
		}
		if err := payRepo.ApplyVerifiedEventTx(ctx, tenant, intent.ID, payments.StatusPaid, &paidAt, ev); err != nil {
			t.Fatalf("redelivery %d must be accepted (idempotent), got: %v", i, err)
		}
	}
	if n := c004CountEvents(t, payRepo, tenant, intent.ID); n != 2 {
		t.Errorf("events = %d, want 2 (both deliveries journaled)", n)
	}
	got, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got.Status != payments.StatusPaid {
		t.Errorf("status = %s, want paid", got.Status)
	}
}
