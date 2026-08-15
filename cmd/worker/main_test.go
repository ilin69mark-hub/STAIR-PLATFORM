package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"stairplatform/internal/application/integrations"
	"stairplatform/internal/infrastructure/queue"
)

// TestRegistryUnknownJobType: неизвестный тип задания → ошибка (retry).
func TestRegistryUnknownJobType(t *testing.T) {
	r := newRegistry(nil, nil, nil, 90)
	job, err := queue.NewJob("unknown.type", nil)
	if err != nil {
		t.Fatalf("new job: %v", err)
	}
	if err := r.Handle(context.Background(), job); err == nil {
		t.Fatalf("expected error for unknown job type")
	}
}

// TestEnqueueCleanup: schedule ставит все три типа заданий очистки.
func TestEnqueueCleanup(t *testing.T) {
	q := queue.NewMemoryQueue()
	ctx := context.Background()

	enqueueCleanup(ctx, q)

	want := map[string]int{
		queue.JobCleanupSessions:  1,
		queue.JobCleanupSsoStates: 1,
		queue.JobCleanupAudit:     1,
	}
	for i := 0; i < len(want); i++ {
		job, ok, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("dequeue: %v", err)
		}
		if !ok {
			t.Fatalf("expected a cleanup job, queue empty after %d dequeues", i)
		}
		want[job.Type]--
	}
	for typ, n := range want {
		if n != 0 {
			t.Fatalf("type %q enqueued %d times, want 1", typ, n)
		}
	}
}

// TestProcessJobPermanentFailure: задание с попытками == max не реенкьюится
// и не паникует (окончательный отказ, EDR-0020 инвариант 3).
func TestProcessJobPermanentFailure(t *testing.T) {
	q := queue.NewMemoryQueue()
	ctx := context.Background()

	job := queue.Job{ID: "j1", Type: "unknown.type", Attempts: queue.DefaultMaxAttempts, MaxAttempts: queue.DefaultMaxAttempts, CreatedAt: time.Now().UTC()}
	// Не должен зависнуть: процесс просто залогирует permanent failure.
	done := make(chan struct{})
	go func() {
		r := newRegistry(nil, nil, nil, 90)
		processJob(ctx, q, r, job)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("processJob hung on permanent failure")
	}
}

// TestConsumeEmptyQueue: consume не блокируется на пустой in-memory очереди
// (внешний ctx с таймаутом — consume выходит по cancel).
func TestConsumeStopsOnCancel(t *testing.T) {
	q := queue.NewMemoryQueue()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r := newRegistry(nil, nil, nil, 90)
	done := make(chan struct{})
	go func() {
		consume(ctx, q, r)
		close(done)
	}()
	select {
	case <-done:
		// Ок: consume завершился по отмене контекста.
	case <-time.After(2 * time.Second):
		t.Fatalf("consume should stop on ctx cancel")
	}
}

// ---- quote_send (EDR-0023 §3.4) ----

// memIntegrationRepo — in-memory порт integrations.Repository для воркера.
type memIntegrationRepo struct {
	mu         sync.Mutex
	endpoints  []*integrations.Endpoint
	deliveries []*integrations.Delivery
}

func (m *memIntegrationRepo) CreateEndpoint(_ context.Context, e *integrations.Endpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endpoints = append(m.endpoints, e)
	return nil
}
func (m *memIntegrationRepo) ListEndpoints(_ context.Context, _ string) ([]*integrations.Endpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*integrations.Endpoint(nil), m.endpoints...), nil
}
func (m *memIntegrationRepo) GetEndpoint(_ context.Context, _, id string) (*integrations.Endpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.endpoints {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, integrations.ErrNotFound
}
func (m *memIntegrationRepo) DeleteEndpoint(_ context.Context, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endpoints = nil
	return nil
}
func (m *memIntegrationRepo) FindEndpointByKind(_ context.Context, _ string, kind integrations.Kind) (*integrations.Endpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.endpoints {
		if e.Kind == kind {
			return e, nil
		}
	}
	return nil, integrations.ErrNotFound
}
func (m *memIntegrationRepo) CreateDelivery(_ context.Context, d *integrations.Delivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d.ID = "d1"
	m.deliveries = append(m.deliveries, d)
	return nil
}
func (m *memIntegrationRepo) UpdateDeliveryStatus(_ context.Context, _, id string, s integrations.Status, attempts int, lastErr string, deliveredAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.deliveries {
		if d.ID == id {
			d.Status, d.Attempts, d.LastError, d.DeliveredAt = s, attempts, lastErr, deliveredAt
		}
	}
	return nil
}
func (m *memIntegrationRepo) GetDelivery(_ context.Context, _, id string) (*integrations.Delivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.deliveries {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, integrations.ErrNotFound
}
func (m *memIntegrationRepo) get(id string) *integrations.Delivery {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.deliveries {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// stubSender — подставной webhook-отправитель.
type stubSender struct {
	mu    sync.Mutex
	urls  []string
	fail  bool
	count int
}

func (s *stubSender) Send(_ context.Context, url, _ string, _ []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls = append(s.urls, url)
	s.count++
	if s.fail {
		return errors.New("boom")
	}
	return nil
}

func quoteSender(t *testing.T) (*registry, *integrations.Service, *memIntegrationRepo, *stubSender) {
	t.Helper()
	repo := &memIntegrationRepo{}
	ep := &integrations.Endpoint{ID: "e1", Kind: integrations.KindERP, URL: "https://erp.example.com/hook", SecretEnc: "secret"}
	if err := repo.CreateEndpoint(context.Background(), ep); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	svc := integrations.NewService(repo, queue.NewMemoryQueue())
	r := newRegistry(nil, nil, repo, 90)
	s := &stubSender{}
	r.overrideWebhook(s)
	return r, svc, repo, s
}

func quoteJob(t *testing.T, eventID string) queue.Job {
	t.Helper()
	job, err := queue.NewJob(queue.JobQuoteSend, map[string]string{
		"event_id": eventID, "endpoint_id": "e1", "tenant_id": "t-1",
	})
	if err != nil {
		t.Fatalf("new job: %v", err)
	}
	return job
}

func TestRegistryQuoteSendDelivered(t *testing.T) {
	r, svc, repo, s := quoteSender(t)
	d, err := svc.SendQuote(context.Background(), "t-1", "p-1", []byte(`{"quote":1}`))
	if err != nil {
		t.Fatalf("SendQuote: %v", err)
	}
	if err := r.Handle(context.Background(), quoteJob(t, d.ID)); err != nil {
		t.Fatalf("Handle quote_send: %v", err)
	}
	got := repo.get(d.ID)
	if got.Status != integrations.StatusDelivered || got.DeliveredAt == nil {
		t.Fatalf("expected delivered, got %+v", got)
	}
	if len(s.urls) != 1 || s.urls[0] != "https://erp.example.com/hook" {
		t.Fatalf("webhook not sent to endpoint: %+v", s.urls)
	}
}

func TestRegistryQuoteSendFailed(t *testing.T) {
	r, svc, repo, s := quoteSender(t)
	d, err := svc.SendQuote(context.Background(), "t-1", "p-1", []byte(`{"quote":1}`))
	if err != nil {
		t.Fatalf("SendQuote: %v", err)
	}
	s.fail = true
	if err := r.Handle(context.Background(), quoteJob(t, d.ID)); err == nil {
		t.Fatal("expected error from failed webhook")
	}
	got := repo.get(d.ID)
	if got.Status != integrations.StatusFailed || got.LastError == "" {
		t.Fatalf("expected failed status, got %+v", got)
	}
	if got.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", got.Attempts)
	}
}

func TestRegistryQuoteSendUnknownEvent(t *testing.T) {
	r, _, _, _ := quoteSender(t)
	if err := r.Handle(context.Background(), quoteJob(t, "missing")); err == nil {
		t.Fatal("expected error for unknown event")
	}
}

func TestRegistryProjectSyncDelivered(t *testing.T) {
	r, svc, repo, s := quoteSender(t)
	// quoteSender регистрирует ERP-эндпоинт; для CRM нужен свой kind.
	ep := &integrations.Endpoint{ID: "e2", Kind: integrations.KindCRM, URL: "https://crm.example.com/hook", SecretEnc: "secret"}
	if err := repo.CreateEndpoint(context.Background(), ep); err != nil {
		t.Fatalf("create crm endpoint: %v", err)
	}
	d, err := svc.SyncProject(context.Background(), "t-1", "p-1", []byte(`{"project_id":"p-1"}`))
	if err != nil {
		t.Fatalf("SyncProject: %v", err)
	}
	job, err := queue.NewJob(queue.JobProjectSync, map[string]string{
		"event_id": d.ID, "endpoint_id": ep.ID, "tenant_id": "t-1",
	})
	if err != nil {
		t.Fatalf("new job: %v", err)
	}
	if err := r.Handle(context.Background(), job); err != nil {
		t.Fatalf("Handle crm.project_sync: %v", err)
	}
	got := repo.get(d.ID)
	if got.Status != integrations.StatusDelivered || got.DeliveredAt == nil {
		t.Fatalf("expected delivered, got %+v", got)
	}
	if len(s.urls) != 1 || s.urls[0] != "https://crm.example.com/hook" {
		t.Fatalf("webhook not sent to crm endpoint: %+v", s.urls)
	}
}

func TestRegistryProjectSyncFailed(t *testing.T) {
	r, svc, repo, s := quoteSender(t)
	ep := &integrations.Endpoint{ID: "e2", Kind: integrations.KindCRM, URL: "https://crm.example.com/hook", SecretEnc: "secret"}
	if err := repo.CreateEndpoint(context.Background(), ep); err != nil {
		t.Fatalf("create crm endpoint: %v", err)
	}
	d, err := svc.SyncProject(context.Background(), "t-1", "p-1", []byte(`{}`))
	if err != nil {
		t.Fatalf("SyncProject: %v", err)
	}
	s.fail = true
	job, err := queue.NewJob(queue.JobProjectSync, map[string]string{
		"event_id": d.ID, "endpoint_id": ep.ID, "tenant_id": "t-1",
	})
	if err != nil {
		t.Fatalf("new job: %v", err)
	}
	if err := r.Handle(context.Background(), job); err == nil {
		t.Fatal("expected error from failed webhook")
	}
	got := repo.get(d.ID)
	if got.Status != integrations.StatusFailed || got.LastError == "" {
		t.Fatalf("expected failed status, got %+v", got)
	}
}
