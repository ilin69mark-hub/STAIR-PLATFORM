package payments

// S-133: покрытие ListByProject/Get/ApplyVerifiedEvent и веток ошибок
// CreateCheckout/applyVerifiedEvent/HandleWebhook.

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

// failRepo — fakeRepo с инжектируемыми ошибками (не трогаем базовый fake).
type failRepo struct {
	fakeRepo
	failCreate error
	failUpdate error
	failAppend error
}

func (f *failRepo) CreateIntent(ctx context.Context, p *PaymentIntent) error {
	if f.failCreate != nil {
		return f.failCreate
	}
	return f.fakeRepo.CreateIntent(ctx, p)
}

func (f *failRepo) UpdateStatus(ctx context.Context, tenantID, id string, s Status, paidAt *time.Time) error {
	if f.failUpdate != nil {
		return f.failUpdate
	}
	return f.fakeRepo.UpdateStatus(ctx, tenantID, id, s, paidAt)
}

func (f *failRepo) AppendEvent(ctx context.Context, e *PaymentEvent) error {
	if f.failAppend != nil {
		return f.failAppend
	}
	return f.fakeRepo.AppendEvent(ctx, e)
}

type failProvider struct {
	fakeProvider
	err error
}

func (f *failProvider) CreateCheckout(_ context.Context, _ int64, _ string) (string, string, error) {
	if f.err != nil {
		return "", "", f.err
	}
	return "chk-123", "https://pay.example.com/pay/chk-123", nil
}

func seedIntent(t *testing.T, repo *failRepo, tenant, project, checkout string, amount int64) *PaymentIntent {
	t.Helper()
	p := &PaymentIntent{
		TenantID:           tenant,
		ProjectID:          project,
		UserID:             "u-1",
		AmountMinor:        amount,
		Currency:           "USD",
		Status:             StatusPending,
		Provider:           "mock",
		ProviderCheckoutID: checkout,
	}
	if err := repo.CreateIntent(context.Background(), p); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return p
}

func TestListByProject(t *testing.T) {
	repo := &failRepo{}
	svc := NewService(repo, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	ctx := context.Background()
	seedIntent(t, repo, "t-1", "p-1", "chk-1", 1000)
	seedIntent(t, repo, "t-1", "p-1", "chk-2", 2000)
	seedIntent(t, repo, "t-1", "p-2", "chk-3", 3000)
	seedIntent(t, repo, "t-9", "p-1", "chk-4", 4000)

	got, err := svc.ListByProject(ctx, "t-1", "p-1")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d intents, want 2 (tenant+project scoped)", len(got))
	}

	empty, err := svc.ListByProject(ctx, "t-1", "p-404")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty project: got %d, err %v", len(empty), err)
	}
}

func TestGet(t *testing.T) {
	repo := &failRepo{}
	svc := NewService(repo, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	ctx := context.Background()
	seed := seedIntent(t, repo, "t-1", "p-1", "chk-1", 1000)

	got, err := svc.Get(ctx, "t-1", seed.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ProviderCheckoutID != "chk-1" {
		t.Fatalf("wrong intent: %+v", got)
	}
	if _, err := svc.Get(ctx, "t-1", "int-404"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: got %v, want ErrNotFound", err)
	}
	if _, err := svc.Get(ctx, "t-OTHER", seed.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign tenant: got %v, want ErrNotFound", err)
	}
}

func TestApplyVerifiedEventPaidAndFailed(t *testing.T) {
	repo := &failRepo{}
	svc := NewService(repo, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	ctx := context.Background()
	seedIntent(t, repo, "t-1", "p-1", "chk-paid", 5000)
	seedIntent(t, repo, "t-1", "p-1", "chk-fail", 5000)

	if err := svc.ApplyVerifiedEvent(ctx, "mock", "chk-paid", "paid", 5000, "usd", []byte(`{}`)); err != nil {
		t.Fatalf("paid: %v", err)
	}
	got, _ := svc.Get(ctx, "t-1", "int-1")
	if got.Status != StatusPaid || got.PaidAt == nil {
		t.Fatalf("paid intent wrong: %+v", got)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventTypePaymentSucceeded {
		t.Fatalf("events wrong: %+v", repo.events)
	}

	if err := svc.ApplyVerifiedEvent(ctx, "mock", "chk-fail", "failed", 5000, "USD", []byte(`{}`)); err != nil {
		t.Fatalf("failed: %v", err)
	}
	got2, _ := svc.Get(ctx, "t-1", "int-2")
	if got2.Status != StatusFailed || got2.PaidAt != nil {
		t.Fatalf("failed intent wrong: %+v", got2)
	}
}

func TestApplyVerifiedEventErrors(t *testing.T) {
	repo := &failRepo{}
	svc := NewService(repo, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	ctx := context.Background()
	seedIntent(t, repo, "t-1", "p-1", "chk-1", 5000)

	for name, args := range map[string][6]string{
		"empty provider":   {"", "chk-1", "paid", "5000", "USD", ""},
		"unknown checkout": {"mock", "chk-404", "paid", "5000", "USD", ""},
		"amount mismatch":  {"mock", "chk-1", "paid", "1", "USD", ""},
		"bad status":       {"mock", "chk-1", "pending", "5000", "USD", ""},
	} {
		a := args
		var amt int64 = 5000
		if a[3] == "1" {
			amt = 1
		}
		if err := svc.ApplyVerifiedEvent(ctx, a[0], a[1], a[2], amt, a[4], []byte(a[5])); err == nil {
			t.Fatalf("%s: want error", name)
		}
	}
}

func TestHandleWebhookVerifierNilAndBadBody(t *testing.T) {
	repo := &failRepo{}
	svc := NewService(repo, &fakeProvider{name: "mock"}, nil, 0)
	if _, err := svc.HandleWebhook(context.Background(), "s", "ts", "sig", []byte(`{}`)); err == nil {
		t.Fatal("nil verifier: want error")
	}

	svc2 := NewService(repo, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	if _, err := svc2.HandleWebhook(context.Background(), "s", "ts", "sig", []byte(`{bad`)); err == nil {
		t.Fatal("bad json: want error")
	}
}

func TestCreateCheckoutProviderAndRepoErrors(t *testing.T) {
	ctx := context.Background()
	badProv := NewService(&failRepo{}, &failProvider{err: errBoom}, &fakeVerifier{}, 0)
	if _, err := badProv.CreateCheckout(ctx, "t-1", "p-1", "u-1", 100, "USD"); !errors.Is(err, errBoom) {
		t.Fatalf("provider err: got %v", err)
	}

	badRepo := NewService(&failRepo{failCreate: errBoom}, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	if _, err := badRepo.CreateCheckout(ctx, "t-1", "p-1", "u-1", 100, "USD"); !errors.Is(err, errBoom) {
		t.Fatalf("repo err: got %v", err)
	}
}

func TestApplyVerifiedEventRepoErrors(t *testing.T) {
	ctx := context.Background()
	newSeeded := func(r *failRepo) {
		p := &PaymentIntent{TenantID: "t-1", ProjectID: "p-1", UserID: "u-1", AmountMinor: 5000, Currency: "USD", Status: StatusPending, Provider: "mock", ProviderCheckoutID: "chk-1"}
		if err := r.fakeRepo.CreateIntent(ctx, p); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	updFail := &failRepo{failUpdate: errBoom}
	newSeeded(updFail)
	svc := NewService(updFail, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	if err := svc.ApplyVerifiedEvent(ctx, "mock", "chk-1", "paid", 5000, "USD", []byte(`{}`)); !errors.Is(err, errBoom) {
		t.Fatalf("update err: got %v", err)
	}

	appFail := &failRepo{failAppend: errBoom}
	newSeeded(appFail)
	svc2 := NewService(appFail, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0)
	if err := svc2.ApplyVerifiedEvent(ctx, "mock", "chk-1", "paid", 5000, "USD", []byte(`{}`)); !errors.Is(err, errBoom) {
		t.Fatalf("append err: got %v", err)
	}
}
