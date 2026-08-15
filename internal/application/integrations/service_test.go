package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"stairplatform/internal/infrastructure/queue"
)

// fakeRepo — тестовая реализация Repository в памяти.
type fakeRepo struct {
	endpoints  []*Endpoint
	deliveries []*Delivery
	nextID     int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{} }

func (f *fakeRepo) id() string {
	f.nextID++
	return fmt.Sprintf("id-%d", f.nextID)
}

func (f *fakeRepo) CreateEndpoint(_ context.Context, e *Endpoint) error {
	e.ID = f.id()
	e.CreatedAt = time.Now().UTC()
	f.endpoints = append(f.endpoints, e)
	return nil
}
func (f *fakeRepo) ListEndpoints(_ context.Context, tenantID string) ([]*Endpoint, error) {
	var out []*Endpoint
	for _, e := range f.endpoints {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeRepo) GetEndpoint(_ context.Context, tenantID, id string) (*Endpoint, error) {
	for _, e := range f.endpoints {
		if e.ID == id && e.TenantID == tenantID {
			return e, nil
		}
	}
	return nil, ErrNotFound
}
func (f *fakeRepo) DeleteEndpoint(_ context.Context, tenantID, id string) error {
	for i, e := range f.endpoints {
		if e.ID == id && e.TenantID == tenantID {
			f.endpoints = append(f.endpoints[:i], f.endpoints[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
func (f *fakeRepo) FindEndpointByKind(_ context.Context, tenantID string, kind Kind) (*Endpoint, error) {
	for _, e := range f.endpoints {
		if e.TenantID == tenantID && e.Kind == kind {
			return e, nil
		}
	}
	return nil, ErrNoEndpoint
}
func (f *fakeRepo) CreateDelivery(_ context.Context, d *Delivery) error {
	d.ID = f.id()
	d.CreatedAt = time.Now().UTC()
	f.deliveries = append(f.deliveries, d)
	return nil
}
func (f *fakeRepo) UpdateDeliveryStatus(_ context.Context, tenantID, id string, s Status, attempts int, lastErr string, deliveredAt *time.Time) error {
	for _, d := range f.deliveries {
		if d.ID == id && d.TenantID == tenantID {
			d.Status, d.Attempts, d.LastError, d.DeliveredAt = s, attempts, lastErr, deliveredAt
			return nil
		}
	}
	return ErrNotFound
}
func (f *fakeRepo) GetDelivery(_ context.Context, tenantID, id string) (*Delivery, error) {
	for _, d := range f.deliveries {
		if d.ID == id && d.TenantID == tenantID {
			return d, nil
		}
	}
	return nil, ErrNotFound
}

// fakeQueue — записывает постановку заданий в память.
type fakeQueue struct {
	jobs []queue.Job
}

func (q *fakeQueue) Enqueue(_ context.Context, job queue.Job) error {
	q.jobs = append(q.jobs, job)
	return nil
}
func (q *fakeQueue) Dequeue(context.Context) (queue.Job, bool, error) {
	return queue.Job{}, false, nil
}

const testTenant = "t-1"

func newTestService() (*Service, *fakeRepo, *fakeQueue) {
	repo := newFakeRepo()
	q := &fakeQueue{}
	return NewService(repo, q), repo, q
}

func TestRegisterEndpoint(t *testing.T) {
	svc, repo, _ := newTestService()
	ep, err := svc.RegisterEndpoint(context.Background(), testTenant, "ERP Kanban", "erp", "https://erp.example.com/hook", "secret-1")
	if err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}
	if ep.ID == "" || ep.Kind != KindERP || ep.SecretEnc != "secret-1" {
		t.Fatalf("unexpected endpoint: %+v", ep)
	}
	if got, _ := repo.GetEndpoint(context.Background(), testTenant, ep.ID); got == nil {
		t.Fatal("endpoint not persisted")
	}
}

func TestRegisterEndpointInvalid(t *testing.T) {
	svc, _, _ := newTestService()
	cases := []struct {
		name string
		kind string
		url  string
		sec  string
	}{
		{"", "erp", "https://x", "s"},
		{"A", "bogus", "https://x", "s"},
		{"A", "erp", "not-a-url", "s"},
		{"A", "erp", "https://x", ""},
	}
	for _, c := range cases {
		if _, err := svc.RegisterEndpoint(context.Background(), testTenant, c.name, c.kind, c.url, c.sec); !errors.Is(err, ErrInvalid) {
			t.Errorf("RegisterEndpoint(%q,%q,%q): err = %v, want ErrInvalid", c.name, c.kind, c.url, err)
		}
	}
}

func TestSendQuoteHappyPath(t *testing.T) {
	svc, _, q := newTestService()
	_, err := svc.RegisterEndpoint(context.Background(), testTenant, "erp", "erp", "https://erp.example.com/hook", "s")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"quote":1}`)
	d, err := svc.SendQuote(context.Background(), testTenant, "p-1", payload)
	if err != nil {
		t.Fatalf("SendQuote: %v", err)
	}
	if d.Status != StatusPending || d.EventType != EventTypeQuoteSend || d.ProjectID != "p-1" {
		t.Fatalf("unexpected delivery: %+v", d)
	}
	if len(q.jobs) != 1 || q.jobs[0].Type != queue.JobQuoteSend {
		t.Fatalf("expected one quote_send job, got %+v", q.jobs)
	}
	var jp struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(q.jobs[0].Payload, &jp); err != nil || jp.EventID != d.ID {
		t.Fatalf("job payload mismatch: %+v", q.jobs[0].Payload)
	}
}

func TestSendQuoteNoEndpoint(t *testing.T) {
	svc, _, q := newTestService()
	if _, err := svc.SendQuote(context.Background(), testTenant, "p-1", []byte("{}")); !errors.Is(err, ErrNoEndpoint) {
		t.Fatalf("err = %v, want ErrNoEndpoint", err)
	}
	if len(q.jobs) != 0 {
		t.Fatal("no job should be enqueued without endpoint")
	}
}

func TestSendQuoteNoQueue(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)
	_, err := svc.RegisterEndpoint(context.Background(), testTenant, "erp", "erp", "https://erp.example.com", "s")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendQuote(context.Background(), testTenant, "p-1", []byte("{}")); err == nil {
		t.Fatal("expected error without queue")
	}
}

func TestSyncProjectHappyPath(t *testing.T) {
	svc, _, q := newTestService()
	if _, err := svc.RegisterEndpoint(context.Background(), testTenant, "crm", "crm", "https://crm.example.com/hook", "s"); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"project_id":"p-1"}`)
	d, err := svc.SyncProject(context.Background(), testTenant, "p-1", payload)
	if err != nil {
		t.Fatalf("SyncProject: %v", err)
	}
	if d.Status != StatusPending || d.EventType != EventTypeProjectSync || d.ProjectID != "p-1" {
		t.Fatalf("unexpected delivery: %+v", d)
	}
	if len(q.jobs) != 1 || q.jobs[0].Type != queue.JobProjectSync {
		t.Fatalf("expected one project_sync job, got %+v", q.jobs)
	}
	var jp struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(q.jobs[0].Payload, &jp); err != nil || jp.EventID != d.ID {
		t.Fatalf("job payload mismatch: %+v", q.jobs[0].Payload)
	}
}

func TestSyncProjectNoEndpoint(t *testing.T) {
	svc, _, q := newTestService()
	if _, err := svc.SyncProject(context.Background(), testTenant, "p-1", []byte("{}")); !errors.Is(err, ErrNoEndpoint) {
		t.Fatalf("err = %v, want ErrNoEndpoint", err)
	}
	if len(q.jobs) != 0 {
		t.Fatal("no job should be enqueued without endpoint")
	}
}

func TestSyncProjectNoQueue(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)
	if _, err := svc.RegisterEndpoint(context.Background(), testTenant, "crm", "crm", "https://crm.example.com", "s"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncProject(context.Background(), testTenant, "p-1", []byte("{}")); err == nil {
		t.Fatal("expected error without queue")
	}
}

func TestSendManufacturingOrderHappyPath(t *testing.T) {
	svc, _, q := newTestService()
	if _, err := svc.RegisterEndpoint(context.Background(), testTenant, "mes", "mes", "https://mes.example.com/hook", "s"); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"project_id":"p-1","parts":[]}`)
	d, err := svc.SendManufacturingOrder(context.Background(), testTenant, "p-1", payload)
	if err != nil {
		t.Fatalf("SendManufacturingOrder: %v", err)
	}
	if d.Status != StatusPending || d.EventType != EventTypeOrderSend || d.ProjectID != "p-1" {
		t.Fatalf("unexpected delivery: %+v", d)
	}
	if len(q.jobs) != 1 || q.jobs[0].Type != queue.JobOrderSend {
		t.Fatalf("expected one order_send job, got %+v", q.jobs)
	}
	var jp struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(q.jobs[0].Payload, &jp); err != nil || jp.EventID != d.ID {
		t.Fatalf("job payload mismatch: %+v", q.jobs[0].Payload)
	}
}

func TestSendManufacturingOrderNoEndpoint(t *testing.T) {
	svc, _, q := newTestService()
	if _, err := svc.SendManufacturingOrder(context.Background(), testTenant, "p-1", []byte("{}")); !errors.Is(err, ErrNoEndpoint) {
		t.Fatalf("err = %v, want ErrNoEndpoint", err)
	}
	if len(q.jobs) != 0 {
		t.Fatal("no job should be enqueued without endpoint")
	}
}

func TestSendManufacturingOrderNoQueue(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil)
	if _, err := svc.RegisterEndpoint(context.Background(), testTenant, "mes", "mes", "https://mes.example.com", "s"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendManufacturingOrder(context.Background(), testTenant, "p-1", []byte("{}")); err == nil {
		t.Fatal("expected error without queue")
	}
}

func TestDeleteEndpoint(t *testing.T) {
	svc, _, _ := newTestService()
	ep, _ := svc.RegisterEndpoint(context.Background(), testTenant, "erp", "erp", "https://erp.example.com", "s")
	if err := svc.DeleteEndpoint(context.Background(), testTenant, ep.ID); err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}
	if err := svc.DeleteEndpoint(context.Background(), testTenant, ep.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestMarkDeliveredFailed(t *testing.T) {
	svc, repo, q := newTestService()
	_, _ = svc.RegisterEndpoint(context.Background(), testTenant, "erp", "erp", "https://erp.example.com", "s")
	d, _ := svc.SendQuote(context.Background(), testTenant, "p-1", []byte("{}"))

	if err := svc.MarkDelivered(context.Background(), testTenant, d.ID); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}
	got, _ := repo.GetDelivery(context.Background(), testTenant, d.ID)
	if got.Status != StatusDelivered || got.DeliveredAt == nil {
		t.Fatalf("after MarkDelivered: %+v", got)
	}

	if err := svc.MarkFailed(context.Background(), testTenant, d.ID, 3, "boom"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got, _ = repo.GetDelivery(context.Background(), testTenant, d.ID)
	if got.Status != StatusDLQ || got.LastError != "boom" {
		t.Fatalf("after MarkFailed(max): %+v", got)
	}
	_ = q
}
