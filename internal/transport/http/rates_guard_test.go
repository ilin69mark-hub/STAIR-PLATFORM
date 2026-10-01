package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appast "stairplatform/internal/application/assistant"
	appauth "stairplatform/internal/application/auth"
	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/stair"
)

// ---- стабы для регрессии SEC-001 -------------------------------------

type sec1Auth struct{ u *appauth.User }

type sec1Err string

func (e sec1Err) Error() string { return string(e) }

var sec1Unauth error = sec1Err("unauthorized")

func (a *sec1Auth) Register(context.Context, string, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec1Auth) Login(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec1Auth) Authenticate(_ context.Context, tok string) (*appauth.User, string, error) {
	if tok == "sess" {
		return a.u, "", nil
	}
	return nil, "", sec1Unauth
}
func (a *sec1Auth) Logout(context.Context, string) error { return nil }
func (a *sec1Auth) ListUsers(context.Context, string) ([]*appauth.User, error) {
	return nil, nil
}
func (a *sec1Auth) UpdateUserRole(context.Context, string, string, string, appauth.Role) error {
	return nil
}
func (a *sec1Auth) UpdateUser(context.Context, string, string, string, *appauth.Role, *appauth.Status) error {
	return nil
}
func (a *sec1Auth) GetPolicy(context.Context, string) (appauth.Policy, error) {
	return appauth.Policy{}, nil
}
func (a *sec1Auth) UpdatePolicy(context.Context, string, string, appauth.Policy) error {
	return nil
}
func (a *sec1Auth) AuthenticateApiKey(context.Context, string) (*appauth.ApiKey, error) {
	return nil, sec1Unauth
}
func (a *sec1Auth) CreateApiKey(context.Context, string, string, string, []appauth.Permission) (*appauth.ApiKey, string, error) {
	return nil, "", nil
}
func (a *sec1Auth) ListApiKeys(context.Context, string) ([]*appauth.ApiKey, error) {
	return nil, nil
}
func (a *sec1Auth) RevokeApiKey(context.Context, string, string, string) error { return nil }
func (a *sec1Auth) SsoAuthorizeURL(context.Context, string) (string, error)    { return "", nil }
func (a *sec1Auth) SsoCallback(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}

// RevokeApiKeysByUser — отзыв всех ключей пользователя (SEC-007).
func (a *sec1Auth) RevokeApiKeysByUser(context.Context, string, string) error { return nil }
func (a *sec1Auth) SsoEnabled() appauth.SsoConfig                             { return appauth.SsoConfig{} }
func (a *sec1Auth) DefaultTenant(context.Context) (*appauth.Tenant, error) {
	return nil, nil
}

type sec1Stair struct{ s *stair.Service }

func (a *sec1Stair) Calculate(c context.Context, x stair.Config, o stair.Options) (*stair.Result, error) {
	return a.s.Calculate(c, x, o)
}
func (a *sec1Stair) Validate(c context.Context, x stair.Config, o stair.Options) (*stair.Result, error) {
	return a.s.Validate(c, x, o)
}
func (a *sec1Stair) Optimize(c context.Context, x stair.Config, o stair.Options, r stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return a.s.Optimize(c, x, o, r)
}

// sec1Jobs / sec1Assistant — минимальные стабы, чтобы маршруты
// stairs:calculate/async и assistant/{kind} были зарегистрированы
// (иначе NewRouter их не монтирует и тест получил бы 405 вместо 422).
type sec1Jobs struct{}

func (sec1Jobs) SubmitCalculate(context.Context, string, string, jobs.Payload) (*jobs.Job, error) {
	return &jobs.Job{ID: "j1", Status: jobs.StatusPending}, nil
}
func (sec1Jobs) GetJobForUser(context.Context, string, string, string) (*jobs.Job, error) {
	return nil, jobs.ErrNotFound
}

type sec1Assistant struct{}

func (sec1Assistant) Ask(context.Context, string, string, appast.Kind, appast.Request) (*appast.Result, error) {
	return &appast.Result{}, nil
}
func (sec1Assistant) Forget(context.Context, string, string, string) (int64, error) { return 0, nil }

func sec1Router() http.Handler {
	u := &appauth.User{ID: "u1", TenantID: "t1", Email: "a@b.c", Role: appauth.RoleAdmin}
	cfg := DefaultConfig()
	cfg.SecurityConfig = &SecurityConfig{AllowedOrigins: []string{"http://x"}}
	cfg.Jobs = sec1Jobs{}
	cfg.Assistant = sec1Assistant{}
	return NewRouter(&sec1Stair{stair.NewService()}, &sec1Projects{n: 25}, &sec1Auth{u: u}, cfg, nil)
}

const sec1Body = `{
 "width_mm":900,"height_mm":3000,"flight":"straight","step_height_mm":180,
 "stringer_thickness_mm":60,"step_thickness_mm":40,"riser":true,
 "clearance_mm":2200,"railing_height_mm":900,"approach_space_mm":1000,
 "room_width_mm":6000,"room_length_mm":2000}`

// sec1Evil — тот самый payload, который в 2026-09-26 обнулял цену.
const sec1EvilRates = `,
 "rates":{"material_per_kg_rub":{"STEEL-S235":0.01,"WOOD-OAK":0.01,
   "WOOD-WALNUT":0.01,"WOOD-ASH":0.01,"WOOD-SOFT":0.01},
   "machine_per_hour_rub":0.01,"labor_per_hour_rub":0.01,
   "overhead_percent":0.0001,"margin_percent":0.0001,
   "discount_percent":99.999,"tax_percent":0.0001}`

func sec1Post(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "sess"})
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "x"})
	req.Header.Set(csrfHeader, "x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestSEC001_ClientRatesRejectedOnEveryRoute — регрессия SEC-001.
// Каждый маршрут, декодирующий calculateRequest, обязан отклонить
// клиентское переопределение ставок с 422 rates_not_allowed.
func TestSEC001_ClientRatesRejectedOnEveryRoute(t *testing.T) {
	h := sec1Router()
	evil := sec1Body[:len(sec1Body)-1] + sec1EvilRates + "}"

	routes := []struct{ name, path, body string }{
		{"calculate", "/api/v1/stairs:calculate", evil},
		{"validate", "/api/v1/stairs:validate", evil},
		{"optimize", "/api/v1/stairs:optimize", evil},
		{"calculate/async", "/api/v1/stairs:calculate/async", evil},
		{"project/calculate", "/api/v1/projects/p1/calculate", evil},
		{"project/preview", "/api/v1/projects/p1/preview", evil},
		{"project/optimize", "/api/v1/projects/p1/optimize", evil},
		{"assistant/pricing", "/api/v1/assistant/pricing", evil},
		{"public/quote", "/api/v1/public/stairs:quote", evil},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			rec := sec1Post(t, h, rt.path, rt.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status=%d (want 422) body=%s", rt.path, rec.Code, rec.Body.String())
			}
			var e struct {
				Error struct{ Code, Message string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if e.Error.Code != "rates_not_allowed" {
				t.Fatalf("%s: code=%q (want rates_not_allowed)", rt.path, e.Error.Code)
			}
		})
	}
}

// TestSEC001_PriceIsIndependentOfRequest — главная проверка: цена авторизованного
// расчёта обязана быть идентична при наличии/отсутствии чужого `rates`
// и обязана совпадать с публичным расчётом тех же параметров (обе цены
// считаются из одних серверных ставок).
func TestSEC001_PriceIsIndependentOfRequest(t *testing.T) {
	h := sec1Router()
	honest := sec1Post(t, h, "/api/v1/stairs:calculate", sec1Body)
	if honest.Code != 200 {
		t.Fatalf("honest status=%d body=%s", honest.Code, honest.Body.String())
	}
	var hv calculateResponse
	if err := json.Unmarshal(honest.Body.Bytes(), &hv); err != nil {
		t.Fatal(err)
	}
	if hv.Pricing.FinalPriceRub <= 0 {
		t.Fatalf("honest price must be > 0, got %v", hv.Pricing.FinalPriceRub)
	}

	// Ровно тот же расчёт с добавленным `rates` — цена обязана совпасть.
	evil := sec1Post(t, h, "/api/v1/stairs:calculate", sec1Body[:len(sec1Body)-1]+sec1EvilRates+"}")
	if evil.Code != 422 {
		t.Fatalf("evil must be rejected, got %d", evil.Code)
	}

	// И контроль: публичный quote тех же параметров даёт ту же цену.
	pub := sec1Post(t, h, "/api/v1/public/stairs:quote", sec1Body)
	if pub.Code != 200 {
		t.Fatalf("public status=%d body=%s", pub.Code, pub.Body.String())
	}
	var pv publicQuoteDTO
	if err := json.Unmarshal(pub.Body.Bytes(), &pv); err != nil {
		t.Fatal(err)
	}
	if pv.Pricing == nil {
		t.Fatal("public quote has no pricing")
	}
	if diff := pv.Pricing.FinalPriceRub - hv.Pricing.FinalPriceRub; diff > 0.01 || diff < -0.01 {
		t.Fatalf("public price %v != auth price %v", pv.Pricing.FinalPriceRub, hv.Pricing.FinalPriceRub)
	}
	t.Logf("honest=%v public=%v (identical)", hv.Pricing.FinalPriceRub, pv.Pricing.FinalPriceRub)
}

// TestSEC001_ClientRatesPresentMatrix — трактовка «поля не передано».
func TestSEC001_ClientRatesPresentMatrix(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"", false},
		{"null", false},
		{"false", false},
		{"{}", true},
		{"true", true},
		{"0", true},
		{`{"margin_percent":0}`, true},
		{"[]", true},
		{`""`, true},
	}
	for _, c := range cases {
		got := clientRatesPresent(json.RawMessage(c.raw))
		if got != c.want {
			t.Errorf("clientRatesPresent(%q)=%v want %v", c.raw, got, c.want)
		}
	}
}

// TestSEC001_ToOptionsNeverAppliesRates — юнит-guard на саму функцию:
// toOptions не может вертить Options с непустыми Rates, что бы ни было в DTO.
func TestSEC001_ToOptionsNeverAppliesRates(t *testing.T) {
	evil := calculateRequest{
		ComfortStepMM: 630,
		Rates:         json.RawMessage(`{"margin_percent":0.0001,"discount_percent":99.999}`),
	}
	opts, ok := toOptions(evil)
	if ok {
		t.Error("toOptions accepted client rates (ok=true)")
	}
	if opts.Rates != nil {
		t.Errorf("toOptions produced client rates: %+v", opts.Rates)
	}
	if opts.ComfortStep != 630 {
		t.Errorf("ComfortStep must still be mapped, got %v", opts.ComfortStep)
	}
}

// TestSEC003_SearchSpaceRejectedOverHTTP — регрессия SEC-003 на HTTP-границе.
// Payload из аудита раньше занимал ядро 60.00 с и возвращал 499.
func TestSEC003_SearchSpaceRejectedOverHTTP(t *testing.T) {
	h := sec1Router()
	evil := sec1Body[:len(sec1Body)-1] + `,
 "target":"price","step_count_min":1,"step_count_max":2000000000,
 "comfort_step_min_mm":600,"comfort_step_max_mm":640,"comfort_step_grid_mm":0.0000001}`

	start := time.Now()
	rec := sec1Post(t, h, "/api/v1/stairs:optimize", evil)
	elapsed := time.Since(start)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d (want 422) body=%.300s", rec.Code, rec.Body.String())
	}
	var e struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Error.Code != "search_space_too_large" {
		t.Fatalf("code=%q body=%.300s", e.Error.Code, rec.Body.String())
	}
	// Ответ обязан называть потолки, чтобы клиент знал, как сузить диапазон.
	for _, want := range []string{"48", "16384"} {
		if !strings.Contains(e.Error.Message, want) {
			t.Errorf("message must mention limit %s: %s", want, e.Error.Message)
		}
	}
	if elapsed > 2*time.Second {
		t.Errorf("rejection took %s — search may have started", elapsed)
	}
	t.Logf("rejected in %s: %s", elapsed, e.Error.Message)
}

// TestSEC003_LegitOptimizeStillWorks — защита от регрессии: нормальный
// запрос оптимизации остаётся рабочим.
func TestSEC003_LegitOptimizeStillWorks(t *testing.T) {
	h := sec1Router()
	legit := sec1Body[:len(sec1Body)-1] + `,
 "target":"price","step_count_min":15,"step_count_max":19,
 "comfort_step_min_mm":600,"comfort_step_max_mm":640,"comfort_step_grid_mm":2}`
	rec := sec1Post(t, h, "/api/v1/stairs:optimize", legit)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%.300s", rec.Code, rec.Body.String())
	}
	var out optimizeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Valid {
		t.Fatalf("legit optimize must be valid: %+v", out)
	}
	t.Logf("valid=%v evaluated=%d objective=%.2f", out.Valid, out.Evaluated, out.Objective)
}
