package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
)

func newPaymentRepo(t *testing.T) (*PaymentRepository, *ProjectRepository) {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return NewPaymentRepository(pool), NewProjectRepository(pool)
}

// testProject создаёт проект для платежных тестов и возвращает его ID.
func testProject(t *testing.T, ctx context.Context, repo *ProjectRepository, tenant string, name string) string {
	t.Helper()
	owner := testOwnerID(t, repo, tenant)
	p := &project.Project{Name: name, Description: "payments", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p.ID
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

var uniqueCounter int

// itoaUD возвращает уникальный суффикс для provider_checkout_id: тестовая БД
// стойкая между прогонами, а (provider, checkout_id) уникален.
func itoaUD() string {
	uniqueCounter++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), uniqueCounter)
}

// TestPaymentIntentCRUD: создание checkout-интента и чтение по ID.
func TestPaymentIntentCRUD(t *testing.T) {
	prRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	owner := testOwnerID(t, projectRepo, tenant)
	projID := testProject(t, ctx, projectRepo, tenant, "Payments project")

	p := &payments.PaymentIntent{
		TenantID:           tenant,
		ProjectID:          projID,
		UserID:             owner,
		AmountMinor:        5000,
		Currency:           "USD",
		Status:             payments.StatusPending,
		Provider:           "mock",
		ProviderCheckoutID: "chk-crud-" + itoaUD(),
	}
	if err := prRepo.CreateIntent(ctx, p); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected assigned id")
	}

	got, err := prRepo.GetIntent(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if got.AmountMinor != 5000 || got.Currency != "USD" || got.Status != payments.StatusPending {
		t.Fatalf("unexpected intent: %+v", got)
	}
	if got.ProjectID != projID || got.ProviderCheckoutID != p.ProviderCheckoutID {
		t.Fatalf("unexpected intent: %+v", got)
	}

	// Чужой tenant — ErrNotFound.
	if _, err := prRepo.GetIntent(ctx, "00000000-0000-0000-0000-000000000000", p.ID); err != payments.ErrNotFound {
		t.Fatalf("foreign tenant: err = %v, want ErrNotFound", err)
	}
}

// TestPaymentGetByProviderCheckout: поиск по (provider, checkout_id).
func TestPaymentGetByProviderCheckout(t *testing.T) {
	prRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	owner := testOwnerID(t, projectRepo, tenant)
	projID := testProject(t, ctx, projectRepo, tenant, "checkout")

	if err := prRepo.CreateIntent(ctx, &payments.PaymentIntent{
		TenantID: tenant, ProjectID: projID, UserID: owner,
		AmountMinor: 100, Currency: "USD", Status: payments.StatusPending,
		Provider: "mock", ProviderCheckoutID: "chk-xyz-" + itoaUD(),
	}); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}

	got, err := prRepo.GetIntentByProviderCheckout(ctx, "mock", "chk-xyz-"+itoaUD())
	if err == nil {
		t.Fatalf("expected ErrNotFound for wrong checkout, got %+v", got)
	}
	if err != payments.ErrNotFound {
		t.Fatalf("ghost checkout: err = %v, want ErrNotFound", err)
	}
}

// TestPaymentListByProject: список интентов проекта.
func TestPaymentListByProject(t *testing.T) {
	prRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	owner := testOwnerID(t, projectRepo, tenant)
	projID := testProject(t, ctx, projectRepo, tenant, "list")

	for i := 0; i < 3; i++ {
		if err := prRepo.CreateIntent(ctx, &payments.PaymentIntent{
			TenantID: tenant, ProjectID: projID, UserID: owner,
			AmountMinor: 100, Currency: "USD", Status: payments.StatusPending,
			Provider: "mock", ProviderCheckoutID: "chk-" + itoa(i) + "-" + itoaUD(),
		}); err != nil {
			t.Fatalf("CreateIntent: %v", err)
		}
	}

	list, err := prRepo.ListByProject(ctx, tenant, projID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 intents, got %d", len(list))
	}
}

// TestPaymentServiceCheckoutAndListByUser — покупка услуги с витрины (этап 4):
// интент без проекта, с кодом услуги; кабинет видит покупки пользователя и не
// видит чужие (user_id фильтр, tenant-скоуп).
func TestPaymentServiceCheckoutAndListByUser(t *testing.T) {
	prRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	owner := testOwnerID(t, projectRepo, tenant)

	if err := prRepo.CreateIntent(ctx, &payments.PaymentIntent{
		TenantID: tenant, UserID: owner, TierID: "pro",
		AmountMinor: 180_000, Currency: "RUB", Status: payments.StatusPending,
		Provider: "mock", ProviderCheckoutID: "svc-" + itoaUD(),
	}); err != nil {
		t.Fatalf("CreateIntent(service): %v", err)
	}

	mine, err := prRepo.ListByUser(ctx, tenant, owner)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(mine) != 1 {
		t.Fatalf("want 1 покупка, got %d", len(mine))
	}
	if mine[0].TierID != "pro" || mine[0].AmountMinor != 180_000 {
		t.Fatalf("покупка потеряла код услуги или сумму: %+v", mine[0])
	}
	if mine[0].ProjectID != "" {
		t.Errorf("покупка услуги не привязана к проекту: %q", mine[0].ProjectID)
	}

	// Чужой пользователь не видит покупки.
	other := testOwnerID(t, projectRepo, tenant)
	otherUser := other
	if otherUser == owner {
		otherUser = "00000000-0000-0000-0000-000000000000"
	}
	none, err := prRepo.ListByUser(ctx, tenant, otherUser)
	if err != nil {
		t.Fatalf("ListByUser(other): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("чужие покупки не должны показываться, got %d", len(none))
	}
}

// TestPaymentUpdateStatus: переход pending → paid с paid_at; терминальный
// статус (paid) не перезаписывается другим статусом (S-141 №4): поздний
// checkout.session.expired (failed) не переворачивает оплаченный интент,
// paid_at не едет при отклонённом переходе.
func TestPaymentUpdateStatus(t *testing.T) {
	prRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	owner := testOwnerID(t, projectRepo, tenant)
	projID := testProject(t, ctx, projectRepo, tenant, "paid")

	p := &payments.PaymentIntent{
		TenantID: tenant, ProjectID: projID, UserID: owner,
		AmountMinor: 250, Currency: "EUR", Status: payments.StatusPending,
		Provider: "mock", ProviderCheckoutID: "chk-paid-" + itoaUD(),
	}
	if err := prRepo.CreateIntent(ctx, p); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}

	paidAt := time.Now().UTC()
	if err := prRepo.UpdateStatus(ctx, tenant, p.ID, payments.StatusPaid, &paidAt); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := prRepo.GetIntent(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if got.Status != payments.StatusPaid || got.PaidAt == nil {
		t.Fatalf("intent not paid: %+v", got)
	}
	paidBefore := *got.PaidAt

	// Поздний expired → failed: переход отклонён guard'ом (терминальный paid
	// не перезаписывается), статус и paid_at не трогаются.
	if err := prRepo.UpdateStatus(ctx, tenant, p.ID, payments.StatusFailed, nil); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	got, _ = prRepo.GetIntent(ctx, tenant, p.ID)
	if got.Status != payments.StatusPaid {
		t.Fatalf("paid intent flipped to %q by late failed event: %+v", got.Status, got)
	}
	if got.PaidAt == nil || !got.PaidAt.Equal(paidBefore) {
		t.Fatalf("paid_at must not move on rejected transition: %v vs %v", got.PaidAt, paidBefore)
	}
}

// TestPaymentUpdateStatusTerminalGuard: повтор paid → paid разрешён
// (идемпотентная доставка «succeeded→succeeded»); pending → failed разрешён;
// failed → paid отклонён (терминальный статус нельзя перезаписать другим).
func TestPaymentUpdateStatusTerminalGuard(t *testing.T) {
	prRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	owner := testOwnerID(t, projectRepo, tenant)
	projID := testProject(t, ctx, projectRepo, tenant, "guard")

	paid := &payments.PaymentIntent{
		TenantID: tenant, ProjectID: projID, UserID: owner,
		AmountMinor: 100, Currency: "USD", Status: payments.StatusPaid,
		Provider: "mock", ProviderCheckoutID: "chk-g-guard-" + itoaUD(),
	}
	if err := prRepo.CreateIntent(ctx, paid); err != nil {
		t.Fatalf("CreateIntent paid: %v", err)
	}

	// paid → paid (повтор того же события) разрешён: не ошибка, статус paid.
	paidAt := time.Now().UTC()
	if err := prRepo.UpdateStatus(ctx, tenant, paid.ID, payments.StatusPaid, &paidAt); err != nil {
		t.Fatalf("paid→paid repeat must be allowed: %v", err)
	}
	got, err := prRepo.GetIntent(ctx, tenant, paid.ID)
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if got.Status != payments.StatusPaid {
		t.Fatalf("repeat paid→paid: status = %q", got.Status)
	}

	// failed-интент из pending: pending → failed разрешён (обычный отказ),
	// затем failed → paid отклонён (терминальный failed не перезаписывается).
	failed := &payments.PaymentIntent{
		TenantID: tenant, ProjectID: projID, UserID: owner,
		AmountMinor: 100, Currency: "USD", Status: payments.StatusPending,
		Provider: "mock", ProviderCheckoutID: "chk-f-guard-" + itoaUD(),
	}
	if err := prRepo.CreateIntent(ctx, failed); err != nil {
		t.Fatalf("CreateIntent failed: %v", err)
	}
	if err := prRepo.UpdateStatus(ctx, tenant, failed.ID, payments.StatusFailed, nil); err != nil {
		t.Fatalf("pending→failed must be allowed: %v", err)
	}
	// failed → paid отклонён.
	if err := prRepo.UpdateStatus(ctx, tenant, failed.ID, payments.StatusPaid, &paidAt); err != nil {
		t.Fatalf("UpdateStatus failed→paid: %v", err)
	}
	gotF, err := prRepo.GetIntent(ctx, tenant, failed.ID)
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if gotF.Status != payments.StatusFailed {
		t.Fatalf("failed intent flipped to %q by paid event: %+v", gotF.Status, gotF)
	}
}

// TestPaymentAppendEvent: журнал webhook-событий.
func TestPaymentAppendEvent(t *testing.T) {
	prRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	owner := testOwnerID(t, projectRepo, tenant)
	projID := testProject(t, ctx, projectRepo, tenant, "events")

	p := &payments.PaymentIntent{
		TenantID: tenant, ProjectID: projID, UserID: owner,
		AmountMinor: 500, Currency: "USD", Status: payments.StatusPending,
		Provider: "mock", ProviderCheckoutID: "chk-ev-" + itoaUD(),
	}
	if err := prRepo.CreateIntent(ctx, p); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}

	e := &payments.PaymentEvent{
		TenantID:  tenant,
		IntentID:  p.ID,
		EventType: payments.EventTypePaymentSucceeded,
		Payload:   []byte(`{"status":"paid"}`),
	}
	if err := prRepo.AppendEvent(ctx, e); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if e.ID == "" || e.CreatedAt.IsZero() {
		t.Fatalf("expected assigned id/created_at: %+v", e)
	}
}

func TestPaymentListAllIsTenantScoped(t *testing.T) {
	payRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)

	var otherTenant string
	err := projectRepo.pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`,
		"Payments other", "payments-other-"+itoaUD()).Scan(&otherTenant)
	if err != nil {
		t.Fatalf("create other tenant: %v", err)
	}

	mine := &payments.PaymentIntent{
		TenantID: tenant, AmountMinor: 100, Currency: "RUB", Status: payments.StatusPaid,
		Provider: "mock", ProviderCheckoutID: "list-mine-" + itoaUD(),
	}
	other := &payments.PaymentIntent{
		TenantID: otherTenant, AmountMinor: 200, Currency: "RUB", Status: payments.StatusPaid,
		Provider: "mock", ProviderCheckoutID: "list-other-" + itoaUD(),
	}
	if err := payRepo.CreateIntent(ctx, mine); err != nil {
		t.Fatalf("create mine: %v", err)
	}
	if err := payRepo.CreateIntent(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}

	list, err := payRepo.ListAll(ctx, tenant)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	foundMine, foundOther := false, false
	for _, intent := range list {
		foundMine = foundMine || intent.ID == mine.ID
		foundOther = foundOther || intent.ID == other.ID
	}
	if !foundMine || foundOther {
		t.Fatalf("tenant list leaked or missed intent: mine=%v other=%v", foundMine, foundOther)
	}
	empty, err := payRepo.ListAll(ctx, "")
	if err != nil {
		t.Fatalf("ListAll(empty tenant): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty tenant must stay scoped, got %d intents", len(empty))
	}
}

func TestPaymentMarkRefundedAtomicAndIdempotent(t *testing.T) {
	payRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	intent := &payments.PaymentIntent{
		TenantID: tenant, AmountMinor: 900, Currency: "RUB", Status: payments.StatusPaid,
		Provider: "mock", ProviderCheckoutID: "refund-" + itoaUD(),
	}
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}
	event := &payments.PaymentEvent{
		TenantID: tenant, IntentID: intent.ID, EventType: payments.EventTypePaymentRefunded,
		Payload: []byte(`{"provider":"mock","idempotency_key":"refund:intent"}`),
	}

	got, err := payRepo.MarkRefunded(ctx, tenant, intent.ID, event)
	if err != nil {
		t.Fatalf("MarkRefunded: %v", err)
	}
	if got.Status != payments.StatusRefunded {
		t.Fatalf("MarkRefunded returned status %q, want refunded", got.Status)
	}
	stored, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if stored.Status != payments.StatusRefunded {
		t.Fatalf("stored status = %q, want refunded", stored.Status)
	}

	repeatEvent := &payments.PaymentEvent{
		TenantID: tenant, IntentID: intent.ID, EventType: payments.EventTypePaymentRefunded,
		Payload: []byte(`{"repeat":true}`),
	}
	got, err = payRepo.MarkRefunded(ctx, tenant, intent.ID, repeatEvent)
	if err != nil {
		t.Fatalf("repeat MarkRefunded: %v", err)
	}
	if got.Status != payments.StatusRefunded {
		t.Fatalf("repeat returned status %q", got.Status)
	}
	var eventCount int
	if err := payRepo.pool.QueryRow(ctx,
		`SELECT count(*) FROM payment_events WHERE intent_id = $1`, intent.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("refund event count = %d, want exactly 1", eventCount)
	}
}

func TestPaymentMarkRefundedRejectsMismatchedEvent(t *testing.T) {
	payRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	intent := &payments.PaymentIntent{
		TenantID: tenant, AmountMinor: 100, Currency: "RUB", Status: payments.StatusPaid,
		Provider: "mock", ProviderCheckoutID: "refund-event-" + itoaUD(),
	}
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}
	event := &payments.PaymentEvent{
		TenantID: tenant, IntentID: intent.ID, EventType: payments.EventTypePaymentSucceeded,
		Payload: []byte(`{}`),
	}

	if _, err := payRepo.MarkRefunded(ctx, tenant, intent.ID, event); err == nil {
		t.Fatal("want mismatched event error")
	}
	got, err := payRepo.GetIntent(ctx, tenant, intent.ID)
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if got.Status != payments.StatusPaid {
		t.Fatalf("status = %q, want paid", got.Status)
	}
}

func TestPaymentMarkRefundedRejectsInvalidStatus(t *testing.T) {
	payRepo, projectRepo := newPaymentRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, projectRepo)
	intent := &payments.PaymentIntent{
		TenantID: tenant, AmountMinor: 100, Currency: "RUB", Status: payments.StatusPending,
		Provider: "mock", ProviderCheckoutID: "refund-pending-" + itoaUD(),
	}
	if err := payRepo.CreateIntent(ctx, intent); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}
	event := &payments.PaymentEvent{
		TenantID: tenant, IntentID: intent.ID, EventType: payments.EventTypePaymentRefunded,
		Payload: []byte(`{}`),
	}

	if _, err := payRepo.MarkRefunded(ctx, tenant, intent.ID, event); !errors.Is(err, payments.ErrInvalidStatus) {
		t.Fatalf("error = %v, want ErrInvalidStatus", err)
	}
}
