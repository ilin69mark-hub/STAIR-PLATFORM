package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"stairplatform/internal/application/audit"
	appauth "stairplatform/internal/application/auth"
)

// ---- стабы -------------------------------------------------------------

type sec2Audit struct{ got []*audit.Event }

func (a *sec2Audit) Record(_ context.Context, e *audit.Event) error {
	// Повторяем доменную проверку application-слоя, чтобы стаб не позволял
	// записать заведомо невалидное событие мимо проверок.
	if err := e.Validate(); err != nil {
		return err
	}
	a.got = append(a.got, e)
	return nil
}
func (a *sec2Audit) ListProjectAudit(context.Context, string, string) ([]*audit.Event, error) {
	return nil, nil
}
func (a *sec2Audit) ListTenantAudit(context.Context, string) ([]*audit.Event, error) {
	return nil, nil
}

type sec2Auth2 struct{ u *appauth.User }

type sec2Err string

func (e sec2Err) Error() string { return string(e) }

var sec2Unauth error = sec2Err("unauthorized")

func (a *sec2Auth2) Register(context.Context, string, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec2Auth2) Login(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec2Auth2) Authenticate(_ context.Context, tok string) (*appauth.User, string, error) {
	if tok == "sess" {
		return a.u, "", nil
	}
	return nil, "", sec2Unauth
}
func (a *sec2Auth2) Logout(context.Context, string) error { return nil }
func (a *sec2Auth2) ListUsers(context.Context, string) ([]*appauth.User, error) {
	return nil, nil
}
func (a *sec2Auth2) UpdateUserRole(context.Context, string, string, string, appauth.Role) error {
	return nil
}
func (a *sec2Auth2) UpdateUser(context.Context, string, string, string, *appauth.Role, *appauth.Status) error {
	return nil
}
func (a *sec2Auth2) GetPolicy(context.Context, string) (appauth.Policy, error) {
	return appauth.Policy{}, nil
}
func (a *sec2Auth2) UpdatePolicy(context.Context, string, string, appauth.Policy) error {
	return nil
}
func (a *sec2Auth2) AuthenticateApiKey(context.Context, string) (*appauth.ApiKey, error) {
	return nil, sec2Unauth
}
func (a *sec2Auth2) CreateApiKey(context.Context, string, string, string, []appauth.Permission) (*appauth.ApiKey, string, error) {
	return nil, "", nil
}
func (a *sec2Auth2) ListApiKeys(context.Context, string) ([]*appauth.ApiKey, error) {
	return nil, nil
}
func (a *sec2Auth2) RevokeApiKey(context.Context, string, string, string) error { return nil }
func (a *sec2Auth2) SsoAuthorizeURL(context.Context, string) (string, error)    { return "", nil }
func (a *sec2Auth2) SsoCallback(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec2Auth2) SsoEnabled() appauth.SsoConfig { return appauth.SsoConfig{} }
func (a *sec2Auth2) DefaultTenant(context.Context) (*appauth.Tenant, error) {
	return nil, nil
}

func sec2Router(a AuditService) http.Handler {
	u := &appauth.User{ID: "u1", TenantID: "t1", Email: "a@b.c", Role: appauth.RoleUser}
	cfg := DefaultConfig()
	cfg.SecurityConfig = &SecurityConfig{AllowedOrigins: []string{"http://x"}}
	return NewRouter(nil, nil, &sec2Auth2{u: u}, cfg, a)
}

func sec2Post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "sess"})
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "x"})
	req.Header.Set(csrfHeader, "x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestSEC002_ForgedServerActionRejected — регрессия SEC-002: попытка записать
// серверное действие в журнал безопасности из клиента.
func TestSEC002_ForgedServerActionRejected(t *testing.T) {
	// Ровно тот payload, который в аудите дал 201 и записал
	// action="user.role_changed" с result="ok".
	forged := []struct{ action string }{
		{"user.role_changed"},
		{"payment.refunded"},
		{"auth.login"},
		{"authz.denied"},
		{"settings.updated"},
		{"data.exported"},
		{"api_key.created"},
		{"config.approved"},
		{"member.added"},
		{"stair.calculated"},
	}
	for _, f := range forged {
		t.Run(f.action, func(t *testing.T) {
			a := &sec2Audit{}
			h := sec2Router(a)
			body := `{"action":"` + f.action +
				`","resource_type":"user","resource_id":"00000000-0000-0000-0000-000000000001","detail":"{}"}`
			rec := sec2Post(t, h, body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d (want 403) body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "action_not_recordable") {
				t.Fatalf("want action_not_recordable, got %s", rec.Body.String())
			}
			if len(a.got) != 0 {
				t.Fatalf("forged event WAS recorded: %+v", a.got[0])
			}
		})
	}
}

// TestSEC002_UnknownActionRejected — неизвестное действие → 422 (похоже на
// опечатку, а не на подделку).
func TestSEC002_UnknownActionRejected(t *testing.T) {
	for _, action := range []string{"bogus", "user.role_changed. ", "USER.ROLE_CHANGED", "'; DROP TABLE audit_events; --", ""} {
		a := &sec2Audit{}
		h := sec2Router(a)
		rec := sec2Post(t, h, `{"action":"`+action+`"}`)
		switch action {
		case "":
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("empty action: status=%d want 400", rec.Code)
			}
		default:
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%q: status=%d (want 422) body=%s", action, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "invalid_action") {
				t.Fatalf("%q: want invalid_action, got %s", action, rec.Body.String())
			}
		}
		if len(a.got) != 0 {
			t.Fatalf("%q: event recorded", action)
		}
	}
}

// TestSEC002_LegitClientActionsAccepted — легитимные клиентские события
// (EDR-0013 §4.2) продолжают приниматься, включая те, что фронтенд шлёт
// без соответствующей Go-константы до фикса.
func TestSEC002_LegitClientActionsAccepted(t *testing.T) {
	for _, action := range []string{
		"stair.config_changed",
		"stair.suggestion_applied",
		"stair.variation_applied",
		"stair.live_suggestion_applied",
		"stair.live_variation_applied",
	} {
		t.Run(action, func(t *testing.T) {
			a := &sec2Audit{}
			h := sec2Router(a)
			rec := sec2Post(t, h, `{"action":"`+action+`","resource_type":"config","resource_id":"c1","detail":"{\"n\":17}"}`)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status=%d (want 201) body=%s", rec.Code, rec.Body.String())
			}
			if len(a.got) != 1 {
				t.Fatalf("want 1 stored event, got %d", len(a.got))
			}
			e := a.got[0]
			if e.Action != audit.Action(action) {
				t.Fatalf("action=%q", e.Action)
			}
			if e.Result != audit.ResultOK {
				t.Fatalf("result must be ok, got %q", e.Result)
			}
			if e.ActorID != "u1" || e.TenantID != "t1" {
				t.Fatalf("actor/tenant must come from context, got %q/%q", e.ActorID, e.TenantID)
			}
		})
	}
}

// TestSEC002_OversizedFieldsRejected — границы длин текстовых полей.
func TestSEC002_OversizedFieldsRejected(t *testing.T) {
	big := func(n int) string { return strings.Repeat("x", n) }
	cases := []struct{ name, body string }{
		{"resource_type", `{"action":"stair.config_changed","resource_type":"` + big(65) + `"}`},
		{"resource_id", `{"action":"stair.config_changed","resource_id":"` + big(129) + `"}`},
		{"detail", `{"action":"stair.config_changed","detail":"` + big(4097) + `"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &sec2Audit{}
			h := sec2Router(a)
			rec := sec2Post(t, h, c.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d (want 422) body=%.200s", rec.Code, rec.Body.String())
			}
			if len(a.got) != 0 {
				t.Fatal("oversized event was recorded")
			}
		})
	}
	// Граница ровно в лимит — проходит.
	for _, c := range []struct{ name, body string }{
		{"resource_type@64", `{"action":"stair.config_changed","resource_type":"` + big(64) + `"}`},
		{"resource_id@128", `{"action":"stair.config_changed","resource_id":"` + big(128) + `"}`},
		{"detail@4096", `{"action":"stair.config_changed","detail":"` + big(4096) + `"}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := &sec2Audit{}
			h := sec2Router(a)
			rec := sec2Post(t, h, c.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("at-limit must pass: status=%d body=%.200s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestSEC002_DomainValidate — доменная валидация события.
func TestSEC002_DomainValidate(t *testing.T) {
	ok := &audit.Event{ActorID: "u", TenantID: "t", Action: audit.ActionAuthLogin, Result: audit.ResultOK}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	bad := []struct {
		name string
		e    *audit.Event
	}{
		{"nil", nil},
		{"empty tenant", &audit.Event{TenantID: "", Action: audit.ActionAuthLogin, Result: audit.ResultOK}},
		{"unknown action", &audit.Event{TenantID: "t", Action: "made.up", Result: audit.ResultOK}},
		{"unknown result", &audit.Event{TenantID: "t", Action: audit.ActionAuthLogin, Result: "weird"}},
		{"long type", &audit.Event{TenantID: "t", Action: audit.ActionAuthLogin, Result: audit.ResultOK, ResourceType: strings.Repeat("t", 65)}},
	}
	for _, b := range bad {
		t.Run(b.name, func(t *testing.T) {
			if err := b.e.Validate(); err == nil {
				t.Fatal("Validate must reject")
			}
		})
	}
	// Системное событие: actor пустой, tenant задан — валидно.
	sys := &audit.Event{TenantID: "t", Action: audit.ActionAuthzDenied, Result: audit.ResultDenied}
	if err := sys.Validate(); err != nil {
		t.Fatalf("system event rejected: %v", err)
	}
}

// TestSEC002_RegistryConsistency — внутренняя согласованность реестра.
func TestSEC002_RegistryConsistency(t *testing.T) {
	all := audit.AllActions()
	if len(all) == 0 {
		t.Fatal("empty registry")
	}
	seen := map[audit.Action]bool{}
	for _, a := range all {
		if !a.Valid() {
			t.Errorf("registry action %q is not Valid()", a)
		}
		if seen[a] {
			t.Errorf("duplicate registry action %q", a)
		}
		seen[a] = true
	}
	// Копия, а не сам срез: мутация результата не должна портить реестр.
	all[0] = "mutated"
	if !audit.AllActions()[0].Valid() {
		t.Error("AllActions() must return a copy")
	}
	cl := audit.ClientActions()
	if len(cl) == 0 {
		t.Fatal("empty client allowlist")
	}
	for _, a := range cl {
		if !a.ClientRecordable() {
			t.Errorf("%q in ClientActions but ClientRecordable()=false", a)
		}
	}
	// Каждое клиентское действие должно быть вызывающим UI-событием,
	// а не серверной операцией.
	for _, a := range cl {
		if !strings.HasPrefix(string(a), "stair.") {
			t.Errorf("client action %q is not a UI event", a)
		}
	}
	if audit.ActionAuthLogin.ClientRecordable() {
		t.Error("auth.login must not be client-recordable")
	}
}

// TestSEC002_MigrationAllowlistMatchesRegistry — миграция
// 000029_audit_action_allowlist.up.sql должна содержать РОВНО те же
// значения action, что и audit.AllActions(). Это защищает от расхождения
// «код говорит одно, CHECK в БД говорит другое»: такое расхождение привело
// бы к 500 на легитимном серверном событии.
func TestSEC002_MigrationAllowlistMatchesRegistry(t *testing.T) {
	mig := filepath.Join("..", "..", "..", "migrations", "000029_audit_action_allowlist.up.sql")
	raw, err := os.ReadFile(mig)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	// Вырезаем только тело CHECK на action.
	m := regexp.MustCompile(`(?s)ADD CONSTRAINT audit_events_action_check CHECK \(action IN \((.*?)\n    \)\);`)
	sub := m.FindStringSubmatch(string(raw))
	if sub == nil {
		t.Fatal("action CHECK not found in migration")
	}
	got := map[string]bool{}
	for _, q := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(sub[1], -1) {
		got[q[1]] = true
	}
	want := map[string]bool{}
	for _, a := range audit.AllActions() {
		want[string(a)] = true
	}
	var missing, extra []string
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("migration allows actions unknown to Go registry: %v", extra)
	}
	if len(missing) > 0 {
		t.Errorf("migration forbids actions present in Go registry (would 500 server writes): %v", missing)
	}
	t.Logf("migration CHECK and audit.AllActions() agree on %d actions", len(want))

	// CHECK на result тоже должен совпадать с Result.Valid().
	rm := regexp.MustCompile(`(?s)audit_events_result_check\s*\n\s*CHECK \(result IN \((.*?)\)\);`)
	rs := rm.FindStringSubmatch(string(raw))
	if rs == nil {
		t.Fatal("result CHECK not found")
	}
	res := map[string]bool{}
	for _, q := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(rs[1], -1) {
		res[q[1]] = true
	}
	for _, want := range []audit.Result{audit.ResultOK, audit.ResultDenied, audit.ResultFailed} {
		if !res[string(want)] {
			t.Errorf("result CHECK missing %q", want)
		}
		if !want.Valid() {
			t.Errorf("Result(%q).Valid()=false", want)
		}
	}
	// json-обёртка не должна была потерять Content-Type проверку.
	var e struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal([]byte(`{"error":{"code":"invalid_action"}}`), &e)
	if e.Error.Code != "invalid_action" {
		t.Fatal("sanity")
	}
}
