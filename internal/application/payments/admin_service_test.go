package payments

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type adminRepoStub struct {
	intent        *PaymentIntent
	list          []*PaymentIntent
	getErr        error
	markErr       error
	listTenant    string
	getCalls      int
	markCalls     int
	recordedEvent *PaymentEvent
}

func (r *adminRepoStub) GetIntent(_ context.Context, tenantID, id string) (*PaymentIntent, error) {
	r.getCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.intent == nil || r.intent.TenantID != tenantID || r.intent.ID != id {
		return nil, ErrNotFound
	}
	return r.intent, nil
}

func (r *adminRepoStub) ListAll(_ context.Context, tenantID string) ([]*PaymentIntent, error) {
	r.listTenant = tenantID
	return r.list, nil
}

func (r *adminRepoStub) MarkRefunded(_ context.Context, _, intentID string, event *PaymentEvent) (*PaymentIntent, error) {
	r.markCalls++
	r.recordedEvent = event
	if r.intent == nil || r.intent.ID != intentID {
		return nil, ErrNotFound
	}
	if r.markErr != nil {
		if errors.Is(r.markErr, ErrInvalidStatus) {
			r.intent.Status = StatusRefunded
		}
		return nil, r.markErr
	}
	r.intent.Status = StatusRefunded
	return r.intent, nil
}

type adminRefundProvider struct {
	name           string
	refundID       string
	refundErr      error
	refundCalls    int
	checkoutID     string
	idempotencyKey string
}

func (p *adminRefundProvider) Name() string { return p.name }

func (p *adminRefundProvider) CreateCheckout(context.Context, int64, string) (string, string, error) {
	return "checkout-new", "https://pay.example/checkout-new", nil
}

func (p *adminRefundProvider) Refund(_ context.Context, checkoutID, idempotencyKey string) (string, error) {
	p.refundCalls++
	p.checkoutID = checkoutID
	p.idempotencyKey = idempotencyKey
	return p.refundID, p.refundErr
}

type checkoutOnlyProvider struct{ name string }

func (p checkoutOnlyProvider) Name() string { return p.name }
func (checkoutOnlyProvider) CreateCheckout(context.Context, int64, string) (string, string, error) {
	return "checkout", "https://pay.example/checkout", nil
}

func paidIntent() *PaymentIntent {
	paidAt := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	return &PaymentIntent{
		ID:                 "intent-1",
		TenantID:           "tenant-1",
		AmountMinor:        90_000,
		Currency:           "RUB",
		Status:             StatusPaid,
		Provider:           "stripe",
		ProviderCheckoutID: "cs_1",
		PaidAt:             &paidAt,
	}
}

func TestAdminServiceListAllKeepsTenantScope(t *testing.T) {
	repo := &adminRepoStub{list: []*PaymentIntent{paidIntent()}}
	svc := NewAdminService(repo, &adminRefundProvider{name: "stripe"})

	for _, tenantID := range []string{"tenant-1", ""} {
		got, err := svc.ListAll(context.Background(), tenantID)
		if err != nil {
			t.Fatalf("ListAll(%q): %v", tenantID, err)
		}
		if repo.listTenant != tenantID || len(got) != 1 {
			t.Fatalf("ListAll(%q) lost tenant scope: tenant=%q len=%d", tenantID, repo.listTenant, len(got))
		}
	}
}

func TestAdminServiceRefundHappyPath(t *testing.T) {
	intent := paidIntent()
	repo := &adminRepoStub{intent: intent}
	provider := &adminRefundProvider{name: "stripe", refundID: "re_1"}
	svc := NewAdminService(repo, provider)
	fixedNow := time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	got, err := svc.Refund(context.Background(), "tenant-1", intent.ID)
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if got.Status != StatusRefunded {
		t.Fatalf("status = %q, want refunded", got.Status)
	}
	if provider.checkoutID != "cs_1" || provider.idempotencyKey != "refund:intent-1" {
		t.Fatalf("provider call wrong: checkout=%q key=%q", provider.checkoutID, provider.idempotencyKey)
	}
	if repo.markCalls != 1 || repo.recordedEvent == nil {
		t.Fatalf("MarkRefunded calls=%d event=%+v", repo.markCalls, repo.recordedEvent)
	}
	if repo.recordedEvent.EventType != EventTypePaymentRefunded || !repo.recordedEvent.CreatedAt.Equal(fixedNow) {
		t.Fatalf("refund event wrong: %+v", repo.recordedEvent)
	}
	var payload struct {
		Provider         string `json:"provider"`
		ProviderRefundID string `json:"provider_refund_id"`
		IdempotencyKey   string `json:"idempotency_key"`
	}
	if err := json.Unmarshal(repo.recordedEvent.Payload, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if payload.Provider != "stripe" || payload.ProviderRefundID != "re_1" || payload.IdempotencyKey != "refund:intent-1" {
		t.Fatalf("event payload wrong: %+v", payload)
	}
}

func TestAdminServiceRefundAlreadyRefundedIsIdempotent(t *testing.T) {
	intent := paidIntent()
	intent.Status = StatusRefunded
	repo := &adminRepoStub{intent: intent}
	provider := &adminRefundProvider{name: "stripe"}
	svc := NewAdminService(repo, provider)

	got, err := svc.Refund(context.Background(), intent.TenantID, intent.ID)
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if got.Status != StatusRefunded || provider.refundCalls != 0 || repo.markCalls != 0 {
		t.Fatalf("repeat refund made side effects: status=%q provider=%d repo=%d", got.Status, provider.refundCalls, repo.markCalls)
	}
}

func TestAdminServiceRefundRejectsBeforeProviderCall(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		provider Provider
		wantErr  error
	}{
		{name: "pending", status: StatusPending, provider: &adminRefundProvider{name: "stripe"}, wantErr: ErrInvalidStatus},
		{name: "failed", status: StatusFailed, provider: &adminRefundProvider{name: "stripe"}, wantErr: ErrInvalidStatus},
		{name: "mismatch", status: StatusPaid, provider: &adminRefundProvider{name: "mock"}, wantErr: ErrProviderMismatch},
		{name: "unsupported", status: StatusPaid, provider: checkoutOnlyProvider{name: "stripe"}, wantErr: ErrRefundUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := paidIntent()
			intent.Status = tt.status
			repo := &adminRepoStub{intent: intent}
			svc := NewAdminService(repo, tt.provider)

			_, err := svc.Refund(context.Background(), intent.TenantID, intent.ID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if repo.markCalls != 0 {
				t.Fatalf("invalid refund reached repository: calls=%d", repo.markCalls)
			}
			if provider, ok := tt.provider.(*adminRefundProvider); ok && provider.refundCalls != 0 {
				t.Fatalf("invalid refund reached provider: calls=%d", provider.refundCalls)
			}
		})
	}
}

func TestAdminServiceRefundProviderFailureDoesNotMutateDB(t *testing.T) {
	intent := paidIntent()
	repo := &adminRepoStub{intent: intent}
	provider := &adminRefundProvider{name: "stripe", refundErr: errors.New("PSP unavailable")}
	svc := NewAdminService(repo, provider)

	_, err := svc.Refund(context.Background(), intent.TenantID, intent.ID)
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("error = %v, want ErrProviderUnavailable", err)
	}
	if intent.Status != StatusPaid || repo.markCalls != 0 {
		t.Fatalf("provider failure mutated DB: status=%q markCalls=%d", intent.Status, repo.markCalls)
	}
}

func TestAdminServiceRefundHandlesConcurrentRepeat(t *testing.T) {
	intent := paidIntent()
	repo := &adminRepoStub{intent: intent, markErr: ErrInvalidStatus}
	provider := &adminRefundProvider{name: "stripe", refundID: "re_1"}
	svc := NewAdminService(repo, provider)

	got, err := svc.Refund(context.Background(), intent.TenantID, intent.ID)
	if err != nil {
		t.Fatalf("concurrent repeat: %v", err)
	}
	if got.Status != StatusRefunded || provider.refundCalls != 1 {
		t.Fatalf("unexpected concurrent result: status=%q providerCalls=%d", got.Status, provider.refundCalls)
	}
}
