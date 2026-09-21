package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/payments"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

// ---- validation helpers ----

func TestValidationHelpers(t *testing.T) {
	if err := ValidateRequired("name", "  "); err == nil || err.Field != "name" {
		t.Fatalf("ValidateRequired empty expected error, got %v", err)
	}
	if err := ValidateRequired("name", "ok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateMinLength("field", "ab", 3); err == nil {
		t.Fatal("expected min length error")
	}
	if err := ValidateMinLength("field", "abc", 3); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateMaxLength("field", "abcd", 3); err == nil {
		t.Fatal("expected max length error")
	}
	if err := ValidateMaxLength("field", "ab", 3); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateEmail("email", ""); err != nil {
		t.Fatalf("empty email should be nil, got %v", err)
	}
	if err := ValidateEmail("email", "bad"); err == nil {
		t.Fatal("expected email error")
	}
	if err := ValidateEmail("email", "a@b.co"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateEmail("email", "a@b.c"); err == nil {
		t.Fatal("expected email error (TLD < 2 chars)")
	}
	if err := ValidateMinValue("num", 1, 5); err == nil {
		t.Fatal("expected min value error")
	}
	if err := ValidateMinValue("num", 5, 5); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateMaxValue("num", 10, 5); err == nil {
		t.Fatal("expected max value error")
	}
	if err := ValidateMaxValue("num", 5, 5); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateOneOf("role", "user", "admin", "user"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateOneOf("role", "guest", "admin", "user"); err == nil {
		t.Fatal("expected oneOf error")
	}
	collected := CollectErrors(ValidateRequired("a", ""), nil, ValidateEmail("e", "bad"))
	if len(collected) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(collected))
	}
	if collected.Error() == "" {
		t.Fatal("expected non-empty Error()")
	}
	ve := ValidationErrors{{Field: "x", Message: "msg"}}
	if ve.Error() != "x: msg" {
		t.Fatalf("Error string mismatch: %q", ve.Error())
	}
}

// ---- isSafeRedirect ----

func TestIsSafeRedirect(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"/dashboard", true},
		{"/projects?x=1", true},
		{"/", true},
		{"https://evil.com", false},
		{"http://evil.com/phish", false},
		{"//evil.com", false},
		{"dashboard", false},
		{"", false},
		{"///evil", false},
		// SSO-OPEN-REDIRECT-BACKSLASH (S-106): браузер трактует "\" как "/".
		{`/\evil.com`, false},
		{`/%5Cevil.com`, false},
		{`/%5cevil.com`, false},
		{`/a\b`, false},
		{`/legit/path`, true},
	}
	for _, c := range cases {
		if got := isSafeRedirect(c.input); got != c.want {
			t.Errorf("isSafeRedirect(%q)=%v want %v", c.input, got, c.want)
		}
	}
	// Invalid URL parse error path: trigger url.Parse error with bad percent
	// Note: url.Parse is permissive; we test malformed with control char
	if isSafeRedirect("http://[invalid") {
		t.Error("expected false for invalid URL")
	}
}

// ---- sso callback safe redirect integration ----

func TestSsoCallbackSafeRedirect(t *testing.T) {
	fa := newFakeAuth()
	fa.authUser = &auth.User{ID: "u-1", TenantID: "t-1", Email: "sso@example.com", Role: auth.RoleUser}
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	// safe redirect should be honored
	resp, err := client.Get(srv.URL + "/api/v1/auth/sso/callback?code=abc&state=xyz&redirect=%2Fdashboard")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status=%d want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/dashboard" {
		t.Fatalf("location=%q want /dashboard", loc)
	}
	// unsafe redirect should fallback to "/"
	resp2, err := client.Get(srv.URL + "/api/v1/auth/sso/callback?code=abc&state=xyz&redirect=https%3A%2F%2Fevil.com")
	if err != nil {
		t.Fatalf("GET2: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if loc := resp2.Header.Get("Location"); loc != "/" {
		t.Fatalf("unsafe redirect location=%q want /", loc)
	}
}

func TestSsoBeginInternalError(t *testing.T) {
	fa := newFakeAuth()
	fa.ssoBeginErr = errors.New("boom")
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/auth/sso")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", resp.StatusCode)
	}
}

func TestSsoCallbackInternalError(t *testing.T) {
	fa := newFakeAuth()
	fa.ssoErr = errors.New("boom")
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/api/v1/auth/sso/callback?code=abc&state=xyz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", resp.StatusCode)
	}
}

// ---- auditRequestID and handleRecordAudit ----

func TestAuditRequestID(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey, "req-123")
	if got := auditRequestID(ctx); got != "req-123" {
		t.Fatalf("got %q want req-123", got)
	}
	if got := auditRequestID(context.Background()); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

type fakeAuditRecord struct {
	fakeAuditService
	recordErr error
	recorded  *audit.Event
}

func (f *fakeAuditRecord) Record(_ context.Context, e *audit.Event) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.recorded = e
	f.events = append(f.events, e)
	return nil
}

func TestHandleRecordAudit(t *testing.T) {
	// success via router
	t.Run("success", func(t *testing.T) {
		svc := &fakeAuditRecord{}
		router := NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig(), svc)
		req := authedRequest(http.MethodPost, "/api/v1/audit", `{"action":"stair.calculated","detail":"x"}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		if svc.recorded == nil || svc.recorded.Action != audit.Action("stair.calculated") {
			t.Fatalf("recorded event mismatch: %+v", svc.recorded)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		svc := &fakeAuditRecord{}
		// handler directly without auth context -> userID empty -> 401
		h := handleRecordAudit(svc)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/audit", strings.NewReader(`{"action":"x"}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		svc := &fakeAuditRecord{}
		router := NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig(), svc)
		req := authedRequest(http.MethodPost, "/api/v1/audit", `{bad`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty_action", func(t *testing.T) {
		svc := &fakeAuditRecord{}
		router := NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig(), svc)
		req := authedRequest(http.MethodPost, "/api/v1/audit", `{"action":""}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("internal_error", func(t *testing.T) {
		svc := &fakeAuditRecord{recordErr: errors.New("boom")}
		router := NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig(), svc)
		req := authedRequest(http.MethodPost, "/api/v1/audit", `{"action":"stair.calculated"}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

// ---- Stripe webhook ----

type fakeStripeWebhook struct {
	err error
}

func (f *fakeStripeWebhook) HandleStripeWebhook(_ context.Context, _ []byte, _ string) error {
	return f.err
}

func TestHandleStripeWebhook(t *testing.T) {
	cases := []struct {
		name       string
		sig        string
		svcErr     error
		wantStatus int
	}{
		{"success", "t=123,v1=abc", nil, http.StatusOK},
		{"missing_signature", "", nil, http.StatusBadRequest},
		{"invalid_signature", "t=123,v1=abc", payments.ErrInvalidSignature, http.StatusUnauthorized},
		{"not_found", "t=123,v1=abc", payments.ErrNotFound, http.StatusNotFound},
		{"invalid", "t=123,v1=abc", payments.ErrInvalid, http.StatusConflict},
		{"internal", "t=123,v1=abc", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.StripeWebhookService = &fakeStripeWebhook{err: tc.svcErr}
			router := NewRouter(stair.NewService(), nil, testAuth{}, cfg)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/stripe/webhook", strings.NewReader(`{"type":"payment_intent.succeeded"}`))
			if tc.sig != "" {
				req.Header.Set("Stripe-Signature", tc.sig)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleStripeWebhookDirect(t *testing.T) {
	// cover direct handler without router (exercises body read)
	h := handleStripeWebhook(&fakeStripeWebhook{err: nil})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/stripe/webhook", strings.NewReader(`{"ok":true}`))
	req.Header.Set("Stripe-Signature", "t=1,v1=sig")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// ---- swagger handlers ----

func TestSwaggerHandlers(t *testing.T) {
	t.Run("ui", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
		rec := httptest.NewRecorder()
		handleSwaggerUI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("Content-Type=%q want text/html", ct)
		}
		if !strings.Contains(rec.Body.String(), "swagger-ui") {
			t.Fatalf("body missing swagger-ui: %s", rec.Body.String())
		}
	})
	t.Run("spec", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/openapi/swagger.yaml", nil)
		rec := httptest.NewRecorder()
		handleSwaggerSpec(rec, req)
		// spec may be 200 or 404 depending on embedded file presence; both cover branches
		if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
			t.Fatalf("expected 200 or 404, got %d", rec.Code)
		}
		if rec.Code == http.StatusOK {
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
				t.Fatalf("Content-Type=%q want yaml", ct)
			}
		}
	})
}

// ---- debug endpoints ----

func TestDebugEndpoints(t *testing.T) {
	RegisterCB("test-cb", &DebugCBStatus{Name: "test-cb", State: "closed", Failures: 0})
	if _, ok := circuitBreakerRegistry.breakers["test-cb"]; !ok {
		t.Fatal("RegisterCB failed")
	}
	cache := &responseCache{
		entries: make(map[string]*ResponseCacheEntry),
		config:  ResponseCacheConfig{MaxEntries: 10, DefaultTTL: time.Minute},
	}
	cache.Set("k", &ResponseCacheEntry{Body: []byte("x"), StatusCode: 200, ExpiresAt: time.Now().Add(time.Minute)})
	h := HandleDebugCache(cache)
	req := httptest.NewRequest(http.MethodGet, "/debug/cache", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["entries"] == nil {
		t.Fatal("expected entries in body")
	}
}

// ---- response cache extra ----

func TestResponseCacheInvalidate(t *testing.T) {
	cache := &responseCache{
		entries: make(map[string]*ResponseCacheEntry),
		config:  ResponseCacheConfig{MaxEntries: 10, DefaultTTL: time.Minute},
	}
	cache.Set("/a?x=1", &ResponseCacheEntry{Body: []byte("a"), StatusCode: 200, ExpiresAt: time.Now().Add(time.Minute)})
	cache.Set("/a?x=2", &ResponseCacheEntry{Body: []byte("b"), StatusCode: 200, ExpiresAt: time.Now().Add(time.Minute)})
	cache.Set("/b?x=1", &ResponseCacheEntry{Body: []byte("c"), StatusCode: 200, ExpiresAt: time.Now().Add(time.Minute)})

	if !cache.Invalidate("/a?x=1") {
		t.Fatal("expected invalidate true")
	}
	if cache.Invalidate("/nonexistent") {
		t.Fatal("expected false for missing key")
	}
	if n := cache.InvalidatePrefix("/a"); n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
	if cache.Size() != 1 {
		t.Fatalf("expected size 1, got %d", cache.Size())
	}
	cache.Clear()
	if cache.Size() != 0 {
		t.Fatalf("expected 0 after Clear, got %d", cache.Size())
	}
}

func TestCacheWriterUnwrap(t *testing.T) {
	inner := httptest.NewRecorder()
	cw := &cacheWriter{ResponseWriter: inner, body: &bytes.Buffer{}, headers: make(http.Header)}
	if cw.Unwrap() != inner {
		t.Fatal("Unwrap mismatch")
	}
}

func TestCompressionWriterUnwrap(t *testing.T) {
	inner := httptest.NewRecorder()
	cw := &compressionWriter{ResponseWriter: inner}
	if cw.Unwrap() != inner {
		t.Fatal("Unwrap mismatch")
	}
}

// ---- preview project ----

func TestHandlePreviewProjectViaRouter(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	router := NewRouter(stair.NewService(), svc, testAuth{}, DefaultConfig())

	// success
	body := `{"width_mm": 900, "height_mm": 2700, "flight": "straight"}`
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/preview", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// invalid json
	req2 := authedRequest(http.MethodPost, "/api/v1/projects/p-1/preview", `{bad`)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec2.Code)
	}
	// not found
	req3 := authedRequest(http.MethodPost, "/api/v1/projects/notfound/preview", body)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec3.Code)
	}
	// forbidden
	svc.calculateErr = project.ErrForbidden
	req4 := authedRequest(http.MethodPost, "/api/v1/projects/p-1/preview", body)
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec4.Code)
	}
	svc.calculateErr = nil
	// invalid input (simulate validation error via calculateErr)
	svc.calculateErr = errors.New("invalid input")
	req5 := authedRequest(http.MethodPost, "/api/v1/projects/p-1/preview", body)
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec5.Code, rec5.Body.String())
	}
}

// ---- admin overview permission extra ----

func TestAdminOverviewExtended(t *testing.T) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	router := NewRouter(stair.NewService(), projects, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/overview", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var stats map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats["tenant_id"] == nil {
		t.Fatal("expected tenant_id")
	}
}
