package database

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"stairplatform/internal/application/order"
)

func newOrderRepo(t *testing.T) (*OrderRepository, *ProjectRepository) {
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
	return NewOrderRepository(pool), NewProjectRepository(pool)
}

func sampleOrder(t *testing.T, ctx context.Context, repo *OrderRepository, tenant, userID string) *order.Order {
	t.Helper()
	o := &order.Order{
		TenantID: tenant, UserID: userID, Kind: order.KindOrder, Status: order.StatusNew,
		Contact:    order.Contact{Name: "Иван", Email: "i@ex.ru", Phone: "+7 900"},
		ConfigJSON: json.RawMessage(`{"width_mm":900}`),
		PriceJSON:  json.RawMessage(`{"final_price":100}`),
	}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return o
}

// sampleConsultation создаёт анонимную консультацию (без user_id и цены).
func sampleConsultation(t *testing.T, ctx context.Context, repo *OrderRepository, tenant string) *order.Order {
	t.Helper()
	o := &order.Order{
		TenantID: tenant, Kind: order.KindConsultation, Status: order.StatusNew,
		Contact:    order.Contact{Name: "Мария", Email: "m@ex.ru", Phone: "+7 000"},
		ConfigJSON: json.RawMessage(`{"type":"consultation","question":"Сколько стоит?"}`),
	}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return o
}

// TestOrderCRUD: создание заказа и чтение по ID внутри tenant.
func TestOrderCRUD(t *testing.T) {
	repo, pr := newOrderRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)

	o := sampleOrder(t, ctx, repo, tenant, owner)
	if o.ID == "" {
		t.Fatal("expected generated id")
	}
	got, err := repo.Get(ctx, tenant, o.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != order.StatusNew || got.Contact.Name != "Иван" {
		t.Fatalf("unexpected order: %+v", got)
	}
	var cfg struct {
		WidthMM int `json:"width_mm"`
	}
	if err := json.Unmarshal(got.ConfigJSON, &cfg); err != nil {
		t.Fatalf("config json: %v", err)
	}
	if cfg.WidthMM != 900 {
		t.Fatalf("width = %d, want 900", cfg.WidthMM)
	}
}

// TestOrderScopedByTenant: заказ не виден из другого tenant.
func TestOrderScopedByTenant(t *testing.T) {
	repo, pr := newOrderRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)
	o := sampleOrder(t, ctx, repo, tenant, owner)

	if _, err := repo.Get(ctx, "other-tenant", o.ID); err == nil {
		t.Fatal("expected ErrNotFound from foreign tenant")
	}
}

// TestOrderListByUserAndAll: списки пользователя и весь tenant.
func TestOrderListByUserAndAll(t *testing.T) {
	repo, pr := newOrderRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)

	sampleOrder(t, ctx, repo, tenant, owner)
	sampleOrder(t, ctx, repo, tenant, owner)

	byUser, err := repo.ListByUser(ctx, tenant, owner)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(byUser) != 2 {
		t.Fatalf("ListByUser count = %d, want 2", len(byUser))
	}

	all, err := repo.ListAll(ctx, tenant)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("ListAll count = %d, want >= 2", len(all))
	}
}

// TestOrderUpdateStatus: смена статуса менеджером.
func TestOrderUpdateStatus(t *testing.T) {
	repo, pr := newOrderRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)
	o := sampleOrder(t, ctx, repo, tenant, owner)

	if err := repo.UpdateStatus(ctx, tenant, o.ID, order.StatusConfirmed); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err := repo.Get(ctx, tenant, o.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != order.StatusConfirmed {
		t.Fatalf("status = %s, want confirmed", got.Status)
	}
}

// TestOrderUpdateStatusNotFound — смена статуса несуществующего заказа.
func TestOrderUpdateStatusNotFound(t *testing.T) {
	repo, pr := newOrderRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	if err := repo.UpdateStatus(ctx, tenant, "00000000-0000-0000-0000-000000000000", order.StatusPriced); err == nil {
		t.Fatal("expected ErrNotFound")
	}
}

// TestOrderConsultation: анонимная консультация без пользователя и цены.
func TestOrderConsultation(t *testing.T) {
	repo, pr := newOrderRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	o := sampleConsultation(t, ctx, repo, tenant)
	if o.ID == "" {
		t.Fatal("expected generated id")
	}
	got, err := repo.Get(ctx, tenant, o.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Kind != order.KindConsultation {
		t.Fatalf("kind = %s, want consultation", got.Kind)
	}
	if got.UserID != "" {
		t.Fatalf("user_id = %q, want empty", got.UserID)
	}
	if len(got.PriceJSON) != 0 {
		t.Fatalf("price should be empty, got %s", got.PriceJSON)
	}
	var cfg struct {
		Type     string `json:"type"`
		Question string `json:"question"`
	}
	if err := json.Unmarshal(got.ConfigJSON, &cfg); err != nil {
		t.Fatalf("config json: %v", err)
	}
	if cfg.Type != "consultation" || cfg.Question == "" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

// TestOrderCRUDRoundtripKind: заказ сохраняет kind=order через Create.
func TestOrderCRUDRoundtripKind(t *testing.T) {
	repo, pr := newOrderRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)
	o := sampleOrder(t, ctx, repo, tenant, owner)
	got, err := repo.Get(ctx, tenant, o.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Kind != order.KindOrder {
		t.Fatalf("kind = %s, want order", got.Kind)
	}
}
