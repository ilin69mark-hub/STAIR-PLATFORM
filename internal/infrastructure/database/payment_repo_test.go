package database

import (
	"context"
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

// TestPaymentUpdateStatus: переход pending → paid с paid_at.
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

	// paid_at не затирается при последующем failed (COALESCE).
	if err := prRepo.UpdateStatus(ctx, tenant, p.ID, payments.StatusFailed, nil); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	got, _ = prRepo.GetIntent(ctx, tenant, p.ID)
	if got.PaidAt == nil {
		t.Fatal("paid_at must survive status update")
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
