package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/integrations"
	paymentsinfra "stairplatform/internal/infrastructure/payments"
)

// fakePaymentService — тестовая реализация PaymentService.
type fakePaymentService struct {
	intents     []*payments.PaymentIntent
	events      []*payments.PaymentEvent
	webhookErr  error
	checkoutErr error
	lastTier    string
}

func newFakePaymentService() *fakePaymentService {
	return &fakePaymentService{}
}

func (f *fakePaymentService) CreateCheckout(ctx context.Context, tenantID, projectID, userID, tierID string) (*payments.PaymentIntent, error) {
	if f.checkoutErr != nil {
		return nil, f.checkoutErr
	}
	// Фейк повторяет серверный каталог (S-150): цена не приходит из тела.
	tier, err := payments.DefaultCatalog().Resolve(tierID)
	if err != nil {
		return nil, err
	}
	f.lastTier = tierID
	p := &payments.PaymentIntent{
		ID: "pay-1", TenantID: tenantID, ProjectID: projectID, UserID: userID,
		AmountMinor: tier.AmountMinor, Currency: tier.Currency,
		Status: payments.StatusPending, Provider: "mock", ProviderCheckoutID: "chk-1",
		CheckoutURL: "https://pay.example.com/pay/chk-1",
	}
	f.intents = append(f.intents, p)
	return p, nil
}

func (f *fakePaymentService) CreateServiceCheckout(ctx context.Context, tenantID, userID, tierID string) (*payments.PaymentIntent, error) {
	if f.checkoutErr != nil {
		return nil, f.checkoutErr
	}
	tier, err := payments.DefaultCatalog().Resolve(tierID)
	if err != nil {
		return nil, err
	}
	f.lastTier = tierID
	p := &payments.PaymentIntent{
		ID: "pay-svc-1", TenantID: tenantID, UserID: userID, TierID: tier.ID,
		AmountMinor: tier.AmountMinor, Currency: tier.Currency,
		Status: payments.StatusPending, Provider: "mock", ProviderCheckoutID: "chk-svc-1",
		CheckoutURL: "https://pay.example.com/pay/chk-svc-1",
	}
	f.intents = append(f.intents, p)
	return p, nil
}

func (f *fakePaymentService) ListByUser(ctx context.Context, tenantID, userID string) ([]*payments.PaymentIntent, error) {
	var out []*payments.PaymentIntent
	for _, p := range f.intents {
		if p.TenantID == tenantID && p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakePaymentService) ListTiers() []payments.Tier {
	return payments.DefaultCatalog().List()
}

func (f *fakePaymentService) ListByProject(ctx context.Context, tenantID, projectID string) ([]*payments.PaymentIntent, error) {
	var out []*payments.PaymentIntent
	for _, p := range f.intents {
		if p.TenantID == tenantID && p.ProjectID == projectID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakePaymentService) Get(ctx context.Context, tenantID, id string) (*payments.PaymentIntent, error) {
	for _, p := range f.intents {
		if p.TenantID == tenantID && p.ID == id {
			return p, nil
		}
	}
	return nil, payments.ErrNotFound
}

func (f *fakePaymentService) HandleWebhook(ctx context.Context, secret, tsUnix, sigValue string, body []byte) (*payments.PaymentEvent, error) {
	if f.webhookErr != nil {
		return nil, f.webhookErr
	}
	ev := &payments.PaymentEvent{ID: "ev-1", TenantID: "t-1", IntentID: "pay-1",
		EventType: payments.EventTypePaymentSucceeded}
	f.events = append(f.events, ev)
	return ev, nil
}

func testRouterWithPayments(p ProjectService, s PaymentService) http.Handler {
	cfg := DefaultConfig()
	cfg.Payments = s
	cfg.PaymentsWebhookSecret = "test-secret"
	return NewRouter(stair.NewService(), p, testAuth{}, cfg)
}

func paymentsTestSetup() (*fakePaymentService, *fakeProjectService) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	return newFakePaymentService(), projects
}

// signedWebhookRequest строит автентично подписанный webhook-запрос через
// mock-провайдера (бэкенд, отправленный «внешним PSP»). Webhook-запросы всегда
// идут методом POST на фиксированный путь.
func signedWebhookRequest(p *paymentsinfra.MockProvider, ev paymentsinfra.WebhookEvent) *http.Request {
	body, ts, sig, err := p.SignWebhook("test-secret", ev)
	if err != nil {
		panic(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", io.NopCloser(strings.NewReader(string(body))))
	r.Header.Set(integrations.HeaderTimestamp, ts)
	r.Header.Set(integrations.HeaderSignature, sig)
	return r
}

func TestCheckout(t *testing.T) {
	s, projects := paymentsTestSetup()
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/checkout", `{"tier_id": "basic"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "checkout_url") || !strings.Contains(rec.Body.String(), "pending") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

// TestCheckoutPriceNotClientControlled (S-150) — red-team «платное бесплатно»:
// клиент больше не может диктовать сумму (amount_minor игнорируется), а
// неизвестный tier_id отвергается 422.
func TestCheckoutPriceNotClientControlled(t *testing.T) {
	s, projects := paymentsTestSetup()
	router := testRouterWithPayments(projects, s)

	// Попытка «заплатить 1 копейку»: amount_minor в теле не влияет.
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/checkout",
		`{"tier_id":"basic","amount_minor":1,"currency":"RUB"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"amount_minor":90000`) {
		t.Fatalf("price must come from server catalog, got %s", rec.Body.String())
	}
	if s.lastTier != "basic" {
		t.Fatalf("service got tier %q", s.lastTier)
	}

	// Старый клиент без tier_id → 422 (цену больше нельзя прислать).
	req2 := authedRequest(http.MethodPost, "/api/v1/projects/p-1/checkout", `{"amount_minor":1}`)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing tier_id must be 422, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestCheckoutNotFound(t *testing.T) {
	s, projects := paymentsTestSetup()
	delete(projects.projects, "p-1")
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/checkout", `{"tier_id": "basic"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCheckoutInvalidJSON(t *testing.T) {
	s, projects := paymentsTestSetup()
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/checkout", `not json`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCheckoutUnprocessable(t *testing.T) {
	s, projects := paymentsTestSetup()
	s.checkoutErr = payments.ErrInvalid
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/checkout", `{"tier_id": "nope"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestCheckoutRequiresAuth(t *testing.T) {
	s, projects := paymentsTestSetup()
	router := testRouterWithPayments(projects, s)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p-1/checkout", strings.NewReader(`{"tier_id": "basic"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListPayments(t *testing.T) {
	s, projects := paymentsTestSetup()
	s.intents = append(s.intents, &payments.PaymentIntent{
		ID: "pay-1", TenantID: "t-1", ProjectID: "p-1", AmountMinor: 5000,
		Currency: "USD", Status: payments.StatusPaid, Provider: "mock",
	})
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/payments", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pay-1") {
		t.Fatalf("missing intent: %s", rec.Body.String())
	}
}

func TestListPaymentsNotFound(t *testing.T) {
	s, projects := paymentsTestSetup()
	delete(projects.projects, "p-1")
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/payments", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetPayment(t *testing.T) {
	s, projects := paymentsTestSetup()
	s.intents = append(s.intents, &payments.PaymentIntent{
		ID: "pay-1", TenantID: "t-1", ProjectID: "p-1", AmountMinor: 100,
		Currency: "USD", Status: payments.StatusPending, Provider: "mock",
	})
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodGet, "/api/v1/payments/pay-1", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetPaymentNotFound(t *testing.T) {
	s, projects := paymentsTestSetup()
	router := testRouterWithPayments(projects, s)
	req := authedRequest(http.MethodGet, "/api/v1/payments/pay-1", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPaymentWebhook(t *testing.T) {
	s, projects := paymentsTestSetup()
	router := testRouterWithPayments(projects, s)
	p := paymentsinfra.NewMockProvider("https://pay.example.com")
	req := signedWebhookRequest(p, paymentsinfra.WebhookEvent{
		EventType: "payment.succeeded", Provider: "mock", CheckoutID: "chk-1",
		Status: "paid", AmountMinor: 5000, Currency: "USD",
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.events) != 1 {
		t.Fatalf("expected 1 webhook event, got %d", len(s.events))
	}
}

func TestPaymentWebhookBadSignature(t *testing.T) {
	s, projects := paymentsTestSetup()
	s.webhookErr = payments.ErrInvalidSignature
	router := testRouterWithPayments(projects, s)
	p := paymentsinfra.NewMockProvider("https://pay.example.com")
	req := signedWebhookRequest(p, paymentsinfra.WebhookEvent{
		EventType: "payment.succeeded", Provider: "mock", CheckoutID: "chk-1",
		Status: "paid", AmountMinor: 5000, Currency: "USD",
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestPaymentWebhookNotFound(t *testing.T) {
	s, projects := paymentsTestSetup()
	s.webhookErr = payments.ErrNotFound
	router := testRouterWithPayments(projects, s)
	p := paymentsinfra.NewMockProvider("https://pay.example.com")
	req := signedWebhookRequest(p, paymentsinfra.WebhookEvent{
		EventType: "payment.succeeded", Provider: "mock", CheckoutID: "ghost",
		Status: "paid", AmountMinor: 5000, Currency: "USD",
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPaymentWebhookInvalidBody(t *testing.T) {
	s, projects := paymentsTestSetup()
	s.webhookErr = payments.ErrInvalid
	router := testRouterWithPayments(projects, s)
	p := paymentsinfra.NewMockProvider("https://pay.example.com")
	req := signedWebhookRequest(p, paymentsinfra.WebhookEvent{
		EventType: "payment.succeeded", Provider: "mock", CheckoutID: "chk-1",
		Status: "weird", AmountMinor: 100, Currency: "USD",
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}
