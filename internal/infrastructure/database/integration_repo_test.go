package database

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"stairplatform/internal/application/integrations"
)

// integrationRepo готовит IntegrationRepository на тестовой БД.
func newIntegrationRepo(t *testing.T) (*IntegrationRepository, *ProjectRepository) {
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
	return NewIntegrationRepository(pool), NewProjectRepository(pool)
}

// TestIntegrationEndpointCRUD: создание/чтение/список/удаление эндпоинта.
func TestIntegrationEndpointCRUD(t *testing.T) {
	ir, pr := newIntegrationRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	ep := &integrations.Endpoint{
		TenantID:  tenant,
		Name:      "ERP Kanban",
		Kind:      integrations.KindERP,
		URL:       "https://erp.example.com/hook",
		SecretEnc: "secret",
	}
	if err := ir.CreateEndpoint(ctx, ep); err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.ID == "" {
		t.Fatal("expected assigned id")
	}

	got, err := ir.GetEndpoint(ctx, tenant, ep.ID)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}
	if got.Kind != integrations.KindERP || got.URL != ep.URL || got.SecretEnc != "secret" {
		t.Fatalf("unexpected endpoint: %+v", got)
	}

	list, err := ir.ListEndpoints(ctx, tenant)
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if len(list) < 1 {
		t.Fatalf("expected at least 1 endpoint, got %d", len(list))
	}

	found, err := ir.FindEndpointByKind(ctx, tenant, integrations.KindERP)
	if err != nil {
		t.Fatalf("FindEndpointByKind: %v", err)
	}
	if found.ID != ep.ID {
		t.Fatalf("expected found id %s, got %s", ep.ID, found.ID)
	}

	if err := ir.DeleteEndpoint(ctx, tenant, ep.ID); err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}
	if _, err := ir.GetEndpoint(ctx, tenant, ep.ID); err != integrations.ErrNotFound {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
}

// TestIntegrationFindEndpointWrongKind: другой kind → ErrNoEndpoint.
func TestIntegrationFindEndpointWrongKind(t *testing.T) {
	ir, pr := newIntegrationRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	if _, err := ir.FindEndpointByKind(ctx, tenant, integrations.KindCRM); err != integrations.ErrNoEndpoint {
		t.Fatalf("err = %v, want ErrNoEndpoint", err)
	}
}

// TestIntegrationDeliveryStatus: создание события и переход delivered.
func TestIntegrationDeliveryStatus(t *testing.T) {
	ir, pr := newIntegrationRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)

	ep := &integrations.Endpoint{TenantID: tenant, Name: "ERP", Kind: integrations.KindERP,
		URL: "https://erp.example.com", SecretEnc: "s"}
	if err := ir.CreateEndpoint(ctx, ep); err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	d := &integrations.Delivery{
		TenantID:   tenant,
		EndpointID: ep.ID,
		EventType:  integrations.EventTypeQuoteSend,
		Payload:    []byte(`{"quote":1}`),
	}
	if err := ir.CreateDelivery(ctx, d); err != nil {
		t.Fatalf("CreateDelivery: %v", err)
	}
	if d.ID == "" || d.Status != integrations.StatusPending {
		t.Fatalf("unexpected delivery: %+v", d)
	}

	if err := ir.UpdateDeliveryStatus(ctx, tenant, d.ID, integrations.StatusDelivered, 1, "", nil); err != nil {
		t.Fatalf("UpdateDeliveryStatus: %v", err)
	}
	got, err := ir.GetDelivery(ctx, tenant, d.ID)
	if err != nil {
		t.Fatalf("GetDelivery: %v", err)
	}
	if got.Status != integrations.StatusDelivered || got.Attempts != 1 {
		t.Fatalf("unexpected delivery after update: %+v", got)
	}
	var real map[string]any
	if err := json.Unmarshal(got.Payload, &real); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if real["quote"].(float64) != 1 {
		t.Fatalf("payload mismatch: %v", real)
	}
}
