package payments

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	intents []*PaymentIntent
	events  []*PaymentEvent
	nextID  int
}

func (f *fakeRepo) CreateIntent(_ context.Context, p *PaymentIntent) error {
	f.nextID++
	p.ID = "int-" + itoa(f.nextID)
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	f.intents = append(f.intents, p)
	return nil
}

func (f *fakeRepo) GetIntent(_ context.Context, tenantID, id string) (*PaymentIntent, error) {
	for _, p := range f.intents {
		if p.ID == id && p.TenantID == tenantID {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) GetIntentByProviderCheckout(_ context.Context, provider, checkoutID string) (*PaymentIntent, error) {
	for _, p := range f.intents {
		if p.Provider == provider && p.ProviderCheckoutID == checkoutID {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) ListByUser(_ context.Context, tenantID, userID string) ([]*PaymentIntent, error) {
	var out []*PaymentIntent
	for _, p := range f.intents {
		if p.TenantID == tenantID && p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListByProject(_ context.Context, tenantID, projectID string) ([]*PaymentIntent, error) {
	var out []*PaymentIntent
	for _, p := range f.intents {
		if p.TenantID == tenantID && p.ProjectID == projectID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateStatus(_ context.Context, tenantID, id string, s Status, paidAt *time.Time) error {
	for _, p := range f.intents {
		if p.ID == id && p.TenantID == tenantID {
			p.Status = s
			p.PaidAt = paidAt
			p.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) AppendEvent(_ context.Context, e *PaymentEvent) error {
	f.nextID++
	e.ID = "ev-" + itoa(f.nextID)
	e.CreatedAt = time.Now().UTC()
	f.events = append(f.events, e)
	return nil
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}

type fakeProvider struct {
	name string
	// lastAmount/lastCurrency — что сервис реально отдал провайдеру (S-150:
	// сумма обязана прийти из каталога, а не из аргументов вызова).
	lastAmount   int64
	lastCurrency string
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) CreateCheckout(_ context.Context, amount int64, currency string) (string, string, error) {
	f.lastAmount = amount
	f.lastCurrency = currency
	return "chk-123", "https://pay.example.com/pay/chk-123", nil
}

type fakeVerifier struct{ wantErr error }

func (f *fakeVerifier) Verify(_ string, _, _ string, _ []byte, _ time.Duration) error {
	return f.wantErr
}

func newTestService(verifier WebhookVerifier) (*Service, *fakeRepo) {
	repo := &fakeRepo{}
	svc := NewService(repo, &fakeProvider{name: "mock"}, verifier, 0)
	svc.WithCatalog(NewCatalog(Tier{ID: "test", AmountMinor: 5000, Currency: "USD"}))
	return svc, repo
}

func TestCreateCheckoutHappyPath(t *testing.T) {
	svc, repo := newTestService(&fakeVerifier{})
	p, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", "test")
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if p.ID == "" || p.Status != StatusPending {
		t.Fatalf("unexpected intent: %+v", p)
	}
	if p.Provider != "mock" || p.ProviderCheckoutID != "chk-123" {
		t.Fatalf("provider fields wrong: %+v", p)
	}
	if p.CheckoutURL != "https://pay.example.com/pay/chk-123" {
		t.Fatalf("checkout url wrong: %q", p.CheckoutURL)
	}
	if p.Currency != "USD" || p.AmountMinor != 5000 {
		t.Fatalf("amount wrong: %+v", p)
	}
	if len(repo.intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(repo.intents))
	}
}

// TestCreateCheckoutPriceFromCatalog (S-150) — red-team «платное бесплатно»:
// цена берётся из серверного каталога; провайдеру уходит именно она, а не
// то, что прислал клиент (в API такой параметр вообще не принимается).
func TestCreateCheckoutPriceFromCatalog(t *testing.T) {
	prov := &fakeProvider{name: "mock"}
	svc := NewService(&fakeRepo{}, prov, &fakeVerifier{}, 0).
		WithCatalog(NewCatalog(Tier{ID: TierBasic, AmountMinor: 90_000, Currency: "RUB"}))
	p, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", TierBasic)
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if p.AmountMinor != 90_000 || p.Currency != "RUB" {
		t.Fatalf("intent price must come from catalog: %+v", p)
	}
	if prov.lastAmount != 90_000 || prov.lastCurrency != "RUB" {
		t.Fatalf("provider got %d/%s, want catalog price 90000/RUB", prov.lastAmount, prov.lastCurrency)
	}
}

// TestCreateCheckoutUnknownTierRejected (S-150) — неизвестный/пустой tier
// отвергается; «подсунуть свою сумму» больше нечем.
func TestCreateCheckoutUnknownTierRejected(t *testing.T) {
	svc, _ := newTestService(&fakeVerifier{})
	for _, tier := range []string{"", "free", "basic", "../../etc/passwd"} {
		if _, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", tier); !errors.Is(err, ErrInvalid) {
			t.Fatalf("tier %q: err = %v, want ErrInvalid", tier, err)
		}
	}
}

func TestCreateCheckoutInvalid(t *testing.T) {
	svc, _ := newTestService(&fakeVerifier{})
	cases := []struct {
		name    string
		tenant  string
		project string
		tier    string
	}{
		{"empty tenant", "", "p-1", "test"},
		{"empty project", "t-1", "", "test"},
		{"empty tier", "t-1", "p-1", ""},
		{"unknown tier", "t-1", "p-1", "free"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.CreateCheckout(context.Background(), c.tenant, c.project, "u-1", c.tier)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestCreateCheckoutNormalizesCatalogCurrency(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeProvider{name: "mock"}, &fakeVerifier{}, 0).
		WithCatalog(NewCatalog(Tier{ID: "t", AmountMinor: 100, Currency: " usd "}))
	p, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", "t")
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if p.Currency != "USD" {
		t.Fatalf("expected normalized catalog currency USD, got %q", p.Currency)
	}
}

func TestHandleWebhookHappyPath(t *testing.T) {
	svc, repo := newTestService(&fakeVerifier{})
	if _, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", "test"); err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}

	ev, err := svc.HandleWebhook(context.Background(), "secret", "12345", "v1:abc", []byte(`{
		"event_type": "payment.succeeded", "provider": "mock",
		"checkout_id": "chk-123", "status": "paid",
		"amount_minor": 5000, "currency": "USD"
	}`))
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if ev.EventType != EventTypePaymentSucceeded {
		t.Fatalf("event type = %q", ev.EventType)
	}
	if len(repo.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.events))
	}
	intent := repo.intents[0]
	if intent.Status != StatusPaid || intent.PaidAt == nil {
		t.Fatalf("intent not paid: %+v", intent)
	}
}

func TestHandleWebhookStatusSucceededAlias(t *testing.T) {
	svc, repo := newTestService(&fakeVerifier{})
	if _, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", "test"); err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	_, err := svc.HandleWebhook(context.Background(), "secret", "1", "v1:abc", []byte(`{
		"event_type": "payment.failed", "provider": "mock",
		"checkout_id": "chk-123", "status": "declined",
		"amount_minor": 5000, "currency": "USD"
	}`))
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if repo.intents[0].Status != StatusFailed {
		t.Fatalf("expected failed, got %s", repo.intents[0].Status)
	}
}

func TestHandleWebhookInvalidSignature(t *testing.T) {
	svc, _ := newTestService(&fakeVerifier{wantErr: errors.New("bad sig")})
	_, err := svc.HandleWebhook(context.Background(), "secret", "1", "v1:bad", []byte(`{}`))
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want ErrInvalidSignature", err)
	}
}

func TestHandleWebhookNotFound(t *testing.T) {
	svc, _ := newTestService(&fakeVerifier{})
	_, err := svc.HandleWebhook(context.Background(), "secret", "1", "v1:abc", []byte(`{
		"event_type": "payment.succeeded", "provider": "mock",
		"checkout_id": "nope", "status": "paid",
		"amount_minor": 100, "currency": "USD"
	}`))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestHandleWebhookAmountMismatch(t *testing.T) {
	svc, _ := newTestService(&fakeVerifier{})
	if _, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", "test"); err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	_, err := svc.HandleWebhook(context.Background(), "secret", "1", "v1:abc", []byte(`{
		"event_type": "payment.succeeded", "provider": "mock",
		"checkout_id": "chk-123", "status": "paid",
		"amount_minor": 9999, "currency": "USD"
	}`))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestHandleWebhookBadStatus(t *testing.T) {
	svc, _ := newTestService(&fakeVerifier{})
	if _, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", "test"); err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	_, err := svc.HandleWebhook(context.Background(), "secret", "1", "v1:abc", []byte(`{
		"event_type": "payment.unknown", "provider": "mock",
		"checkout_id": "chk-123", "status": "weird",
		"amount_minor": 5000, "currency": "USD"
	}`))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestHandleWebhookMissingFields(t *testing.T) {
	svc, _ := newTestService(&fakeVerifier{})
	_, err := svc.HandleWebhook(context.Background(), "secret", "1", "v1:abc", []byte(`{"event_type":"payment.succeeded"}`))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestWebhookPayloadRawPreserved(t *testing.T) {
	svc, repo := newTestService(&fakeVerifier{})
	if _, err := svc.CreateCheckout(context.Background(), "t-1", "p-1", "u-1", "test"); err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	raw := `{"event_type":"payment.succeeded","provider":"mock","checkout_id":"chk-123","status":"paid","amount_minor":5000,"currency":"USD"}`
	if _, err := svc.HandleWebhook(context.Background(), "secret", "1", "v1:abc", []byte(raw)); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if strings.TrimSpace(string(repo.events[0].Payload)) == "" {
		t.Fatal("expected raw payload stored")
	}
}
