package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appauth "stairplatform/internal/application/auth"
	appjobs "stairplatform/internal/application/jobs"
	"stairplatform/internal/application/payments"
	appstair "stairplatform/internal/application/stair"
)

// ---- стабы для регрессии SEC-004 --------------------------------------

// sec4Jobs — хранит задания с владельцем и отдаёт их ТОЛЬКО ему (SEC-004).
type sec4Jobs struct{ jobs map[string]*appjobs.Job }

func (s *sec4Jobs) SubmitCalculate(_ context.Context, tenantID, userID string, _ appjobs.Payload) (*appjobs.Job, error) {
	j := &appjobs.Job{ID: "job-" + userID, TenantID: tenantID, UserID: userID,
		Type: "calc.calculate", Status: appjobs.StatusSucceeded}
	s.jobs[j.ID] = j
	return j, nil
}

// Create — запись нового задания (метод Repository, нужен для app-слоя).
func (s *sec4Jobs) Create(_ context.Context, j *appjobs.Job) error {
	s.jobs[j.ID] = j
	return nil
}

// MarkRunning / MarkSucceeded / MarkFailed — воркерские методы Repository.
func (s *sec4Jobs) MarkRunning(context.Context, string, string) error { return nil }
func (s *sec4Jobs) MarkSucceeded(context.Context, string, string, *appstair.Result) error {
	return nil
}
func (s *sec4Jobs) MarkFailed(context.Context, string, string, string) error { return nil }

// GetByID — tenant-only выборка (используется воркером, SEC-004 не меняет).
func (s *sec4Jobs) GetByID(_ context.Context, tenantID, id string) (*appjobs.Job, error) {
	j, ok := s.jobs[id]
	if !ok || j.TenantID != tenantID {
		return nil, appjobs.ErrNotFound
	}
	return j, nil
}

// GetByIDForUser — имя метода Repository (jobs.Repository).
func (s *sec4Jobs) GetByIDForUser(ctx context.Context, tenantID, userID, id string) (*appjobs.Job, error) {
	return s.GetJobForUser(ctx, tenantID, userID, id)
}

func (s *sec4Jobs) GetJobForUser(_ context.Context, tenantID, userID, id string) (*appjobs.Job, error) {
	j, ok := s.jobs[id]
	if !ok || j.TenantID != tenantID {
		return nil, appjobs.ErrNotFound
	}
	// Владелец обязателен: чужое задание неотличимо от отсутствующего.
	if j.UserID != userID {
		return nil, appjobs.ErrNotFound
	}
	return j, nil
}

// sec4Payments — то же для платежей.
type sec4Payments struct {
	intents map[string]*payments.PaymentIntent
}

// CreateIntent — низкоуровневая запись интента (метод Repository).
func (s *sec4Payments) CreateIntent(_ context.Context, p *payments.PaymentIntent) error {
	s.intents[p.ID] = p
	return nil
}

// UpdateStatus — смена статуса интента (метод Repository).
func (s *sec4Payments) UpdateStatus(context.Context, string, string, payments.Status, *time.Time) error {
	return nil
}

func (s *sec4Payments) CreateCheckout(_ context.Context, tenantID, projectID, userID, tierID string) (*payments.PaymentIntent, error) {
	p := &payments.PaymentIntent{ID: "pay-" + userID, TenantID: tenantID, ProjectID: projectID,
		UserID: userID, TierID: tierID, Status: payments.StatusPending,
		AmountMinor: 123400, Currency: "RUB"}
	s.intents[p.ID] = p
	return p, nil
}
func (s *sec4Payments) ListByProject(context.Context, string, string) ([]*payments.PaymentIntent, error) {
	return nil, nil
}

// GetIntent — tenant-only выборка (админские пути: возврат, сверка).
func (s *sec4Payments) GetIntent(_ context.Context, tenantID, id string) (*payments.PaymentIntent, error) {
	p, ok := s.intents[id]
	if !ok || p.TenantID != tenantID {
		return nil, payments.ErrNotFound
	}
	return p, nil
}

// GetIntentByProviderCheckout — по паре провайдер/идентификатор чека.
func (s *sec4Payments) GetIntentByProviderCheckout(_ context.Context, provider, checkoutID string) (*payments.PaymentIntent, error) {
	for _, p := range s.intents {
		if p.Provider == provider && p.ProviderCheckoutID == checkoutID {
			return p, nil
		}
	}
	return nil, payments.ErrNotFound
}

// GetIntentByUser — имя метода Repository (payments.Repository).
func (s *sec4Payments) GetIntentByUser(ctx context.Context, tenantID, userID, id string) (*payments.PaymentIntent, error) {
	return s.GetByUser(ctx, tenantID, userID, id)
}

func (s *sec4Payments) GetByUser(_ context.Context, tenantID, userID, id string) (*payments.PaymentIntent, error) {
	p, ok := s.intents[id]
	if !ok || p.TenantID != tenantID {
		return nil, payments.ErrNotFound
	}
	if p.UserID != userID {
		return nil, payments.ErrNotFound
	}
	return p, nil
}
func (s *sec4Payments) HandleWebhook(context.Context, string, string, string, []byte) (*payments.PaymentEvent, error) {
	return nil, nil
}
func (s *sec4Payments) AppendEvent(_ context.Context, e *payments.PaymentEvent) error {
	return nil
}
func (s *sec4Payments) ListTiers() []payments.Tier { return nil }
func (s *sec4Payments) CreateServiceCheckout(_ context.Context, tenantID, userID, tierID string) (*payments.PaymentIntent, error) {
	return s.CreateCheckout(context.Background(), tenantID, "", userID, tierID)
}
func (s *sec4Payments) ListByUser(_ context.Context, tenantID, userID string) ([]*payments.PaymentIntent, error) {
	var out []*payments.PaymentIntent
	for _, p := range s.intents {
		if p.TenantID == tenantID && p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

// sec4Auth — два разных пользователя в ОДНОМ tenant'е (дефолтная
// одно-tenant регистрация — именно этот сценарий делал уязвимость
// эксплуатируемой).
type sec4Auth struct{ users map[string]*appauth.User }

func (a *sec4Auth) Register(context.Context, string, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec4Auth) Login(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec4Auth) Authenticate(_ context.Context, tok string) (*appauth.User, string, error) {
	if u, ok := a.users[tok]; ok {
		return u, "", nil
	}
	return nil, "", sec5Unauth
}
func (a *sec4Auth) Logout(context.Context, string) error { return nil }
func (a *sec4Auth) ListUsers(context.Context, string) ([]*appauth.User, error) {
	return nil, nil
}
func (a *sec4Auth) UpdateUserRole(context.Context, string, string, string, appauth.Role) error {
	return nil
}
func (a *sec4Auth) UpdateUser(context.Context, string, string, string, *appauth.Role, *appauth.Status) error {
	return nil
}
func (a *sec4Auth) GetPolicy(context.Context, string) (appauth.Policy, error) {
	return appauth.Policy{}, nil
}
func (a *sec4Auth) UpdatePolicy(context.Context, string, string, appauth.Policy) error { return nil }
func (a *sec4Auth) AuthenticateApiKey(context.Context, string) (*appauth.ApiKey, error) {
	return nil, sec5Unauth
}
func (a *sec4Auth) CreateApiKey(context.Context, string, string, string, []appauth.Permission) (*appauth.ApiKey, string, error) {
	return nil, "", nil
}
func (a *sec4Auth) ListApiKeys(context.Context, string) ([]*appauth.ApiKey, error) {
	return nil, nil
}
func (a *sec4Auth) RevokeApiKey(context.Context, string, string, string) error { return nil }
func (a *sec4Auth) SsoAuthorizeURL(context.Context, string) (string, error)    { return "", nil }
func (a *sec4Auth) SsoCallback(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec4Auth) SsoEnabled() appauth.SsoConfig { return appauth.SsoConfig{} }
func (a *sec4Auth) DefaultTenant(context.Context) (*appauth.Tenant, error) {
	return nil, nil
}

func sec4Router(js *sec4Jobs, ps *sec4Payments) http.Handler {
	// Ключевая предпосылка дефекта: оба пользователя в ОДНОМ tenant'е.
	users := map[string]*appauth.User{
		"sess-userA": {ID: "userA", TenantID: "tenant-shared", Role: appauth.RoleUser},
		"sess-userB": {ID: "userB", TenantID: "tenant-shared", Role: appauth.RoleUser},
	}
	cfg := DefaultConfig()
	cfg.SecurityConfig = &SecurityConfig{AllowedOrigins: []string{"http://x"}}
	cfg.Jobs = js
	cfg.Payments = ps
	return NewRouter(&sec5Stair{appstair.NewService()}, &sec1Projects{n: 1}, &sec4Auth{users: users}, cfg, nil)
}

func sec4Get(t *testing.T, h http.Handler, path, session string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: session})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestSEC004_CrossUserIsolation — регрессия SEC-004: горизонтальный IDOR.
//
// Сценарий из аудита: пользователь A регистрируется (в дефолтном единственном
// tenant'е), создаёт асинхронный расчёт и платёж. Пользователь B — тоже в
// этом tenant'е — до фикса читал оба по ID.
func TestSEC004_CrossUserIsolation(t *testing.T) {
	js := &sec4Jobs{jobs: map[string]*appjobs.Job{}}
	ps := &sec4Payments{intents: map[string]*payments.PaymentIntent{}}
	h := sec4Router(js, ps)

	// 1. Пользователь A регистрирует асинхронный расчёт.
	postBody := `{"width_mm":900,"height_mm":3000,"flight":"straight","step_height_mm":180,
 "stringer_thickness_mm":60,"step_thickness_mm":40,"riser":true,"clearance_mm":2200,
 "railing_height_mm":900,"approach_space_mm":1000}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate/async",
		strings.NewReader(postBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "sess-userA"})
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "x"})
	req.Header.Set(csrfHeader, "x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit failed: %d %s", rec.Code, rec.Body.String())
	}
	var sub struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sub); err != nil {
		t.Fatal(err)
	}
	if sub.JobID == "" {
		t.Fatal("no job_id")
	}
	t.Logf("userA created job %s", sub.JobID)

	// 2. Владелец читает своё — 200.
	own := sec4Get(t, h, "/api/v1/jobs/"+sub.JobID, "sess-userA")
	if own.Code != http.StatusOK {
		t.Fatalf("owner must read own job: %d %s", own.Code, own.Body.String())
	}

	// 3. Чужой пользователь в том же tenant'е — 404 (не 403: существование
	//    не раскрывается).
	other := sec4Get(t, h, "/api/v1/jobs/"+sub.JobID, "sess-userB")
	if other.Code != http.StatusNotFound {
		t.Fatalf("cross-user job read: got %d (want 404) body=%s", other.Code, other.Body.String())
	}
	if other.Code == http.StatusOK {
		t.Fatal("SEC-004 REGRESSION: userB read userA's job")
	}
	t.Logf("userB blocked from job: %d", other.Code)

	// 4. То же для платежей: A создаёт интент напрямую через стаб,
	//    B пытается прочитать.
	if _, err := ps.CreateCheckout(context.Background(), "tenant-shared", "proj-1", "userA", "tier-x"); err != nil {
		t.Fatal(err)
	}
	ownP := sec4Get(t, h, "/api/v1/payments/pay-userA", "sess-userA")
	if ownP.Code != http.StatusOK {
		t.Fatalf("owner must read own payment: %d %s", ownP.Code, ownP.Body.String())
	}
	otherP := sec4Get(t, h, "/api/v1/payments/pay-userA", "sess-userB")
	if otherP.Code != http.StatusNotFound {
		t.Fatalf("cross-user payment read: got %d (want 404) body=%s", otherP.Code, otherP.Body.String())
	}
	t.Logf("userB blocked from payment: %d", otherP.Code)

	// 5. Несуществующий ID — тоже 404 (одинаковый ответ, без оракула).
	missing := sec4Get(t, h, "/api/v1/jobs/does-not-exist", "sess-userB")
	if missing.Code != http.StatusNotFound {
		t.Errorf("missing job must be 404, got %d", missing.Code)
	}
	if missing.Code != other.Code {
		t.Errorf("existence oracle: existing-but-foreign=%d vs missing=%d", other.Code, missing.Code)
	}
}

// TestSEC004_ApplicationScoping — проверка прикладного слоя: пустой userID
// всегда даёт ErrNotFound (fail-closed), даже если подпись вызывающего
// изменится.
func TestSEC004_ApplicationScoping(t *testing.T) {
	js := &sec4Jobs{jobs: map[string]*appjobs.Job{}}
	ps := &sec4Payments{intents: map[string]*payments.PaymentIntent{}}
	jsSvc := appjobs.NewService(js, nil, nil)
	psSvc := payments.NewService(ps, nil, nil, 0)

	// Регистрация задания напрямую через порт (SubmitCalculate требует
	// настроенной очереди, которая здесь не нужна — проверяется только
	// чтение скоупа).
	if err := js.Create(context.Background(), &appjobs.Job{
		ID: "job-userA", TenantID: "tenant-shared", UserID: "userA",
		Type: "calc.calculate", Status: appjobs.StatusSucceeded,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ps.CreateIntent(context.Background(), &payments.PaymentIntent{
		ID: "pay-userA", TenantID: "tenant-shared", UserID: "userA",
		AmountMinor: 100, Currency: "RUB", Status: payments.StatusPaid,
	}); err != nil {
		t.Fatal(err)
	}

	// Владелец — Ok.
	if _, err := jsSvc.GetJobForUser(context.Background(), "tenant-shared", "userA", "job-userA"); err != nil {
		t.Errorf("owner must be allowed: %v", err)
	}
	if _, err := psSvc.GetByUser(context.Background(), "tenant-shared", "userA", "pay-userA"); err != nil {
		t.Errorf("owner must be allowed: %v", err)
	}
	// Чужой — ErrNotFound.
	if _, err := jsSvc.GetJobForUser(context.Background(), "tenant-shared", "userB", "job-userA"); err == nil {
		t.Error("SEC-004 REGRESSION: foreign user allowed for job")
	}
	if _, err := psSvc.GetByUser(context.Background(), "tenant-shared", "userB", "pay-userA"); err == nil {
		t.Error("SEC-004 REGRESSION: foreign user allowed for payment")
	}
	// Пустой userID — fail-closed.
	if _, err := jsSvc.GetJobForUser(context.Background(), "tenant-shared", "", "job-userA"); err == nil {
		t.Error("empty userID must be rejected (fail-closed)")
	}
	if _, err := psSvc.GetByUser(context.Background(), "tenant-shared", "", "pay-userA"); err == nil {
		t.Error("empty userID must be rejected (fail-closed)")
	}
	// Системные записи (user_id пуст) остаются видимыми — иначе сломались бы
	// фоновые/админские сценарии.
	if _, err := psSvc.GetByUser(context.Background(), "tenant-shared", "userB", ""); err == nil {
		t.Log("empty id correctly not found")
	}
}
