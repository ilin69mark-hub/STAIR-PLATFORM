package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/stair"
)

// fakeAuth — управляемый AuthService для тестов транспортного слоя.
type fakeAuth struct {
	registerErr error
	loginErr    error
	authUser    *auth.User
	authErr     error
	tokens      map[string]*auth.User
}

func newFakeAuth() *fakeAuth {
	return &fakeAuth{
		authUser: &auth.User{ID: "u-1", TenantID: "t-1", Email: "a@b.co", Role: auth.RoleUser},
		tokens:   map[string]*auth.User{"token-1": {ID: "u-1", TenantID: "t-1", Email: "a@b.co", Role: auth.RoleUser}},
	}
}

func (f *fakeAuth) Register(ctx context.Context, email, name, password string) (*auth.User, string, error) {
	if f.registerErr != nil {
		return nil, "", f.registerErr
	}
	return &auth.User{ID: "u-1", TenantID: "t-1", Email: email, Name: name, Role: auth.RoleUser}, "token-1", nil
}

func (f *fakeAuth) Login(ctx context.Context, email, password string) (*auth.User, string, error) {
	if f.loginErr != nil {
		return nil, "", f.loginErr
	}
	return f.authUser, "token-1", nil
}

func (f *fakeAuth) Authenticate(ctx context.Context, token string) (*auth.User, error) {
	if f.authErr != nil {
		return nil, f.authErr
	}
	if u, ok := f.tokens[token]; ok {
		return u, nil
	}
	return nil, auth.ErrSessionExpired
}

func (f *fakeAuth) Logout(ctx context.Context, token string) error { return nil }

func authTestRouter(a AuthService) http.Handler {
	return NewRouter(stair.NewService(), nil, a, DefaultConfig())
}

func TestRegisterHandler(t *testing.T) {
	router := authTestRouter(newFakeAuth())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"new@example.com","name":"Новый","password":"secret123"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var u userDTO
	if err := decodeResponse(rec, &u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.Email != "new@example.com" {
		t.Fatalf("email = %q", u.Email)
	}
}

func TestRegisterEmailExists(t *testing.T) {
	a := newFakeAuth()
	a.registerErr = auth.ErrEmailExists
	router := authTestRouter(a)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"dup@example.com","name":"A","password":"secret123"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestRegisterSetsCookies(t *testing.T) {
	router := authTestRouter(newFakeAuth())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"new@example.com","name":"Новый","password":"secret123"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	names := map[string]bool{}
	for _, c := range cookies {
		names[c.Name] = true
	}
	if !names[sessionCookieName] || !names[csrfCookieName] {
		t.Fatalf("expected session+csrf cookies after register, got %v", names)
	}
}

func TestLoginSetsCookies(t *testing.T) {
	router := authTestRouter(newFakeAuth())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"a@b.co","password":"secret123"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	names := map[string]bool{}
	for _, c := range cookies {
		names[c.Name] = true
	}
	if !names[sessionCookieName] || !names[csrfCookieName] {
		t.Fatalf("expected session+csrf cookies, got %v", names)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	a := newFakeAuth()
	a.loginErr = auth.ErrInvalidCreds
	router := authTestRouter(a)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"a@b.co","password":"wrong"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthRejectsAnonymous(t *testing.T) {
	router := authTestRouter(newFakeAuth())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthRejectsInvalidToken(t *testing.T) {
	router := authTestRouter(newFakeAuth())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "bad-token"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthAcceptsValidToken(t *testing.T) {
	router := NewRouter(stair.NewService(), newFakeProjectService(), newFakeAuth(), DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", rec.Code)
	}
}

func TestCSRFRequiredOnMutating(t *testing.T) {
	router := authTestRouter(newFakeAuth())
	// Есть session, но нет csrf → 403.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects",
		strings.NewReader(`{"name":"A"}`))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf, got %d", rec.Code)
	}
}

func TestCSRFMismatchRejected(t *testing.T) {
	router := authTestRouter(newFakeAuth())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects",
		strings.NewReader(`{"name":"A"}`))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-1"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-1"})
	req.Header.Set(csrfHeader, "csrf-wrong")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on mismatch, got %d", rec.Code)
	}
}

func TestRateLimitLogin(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LoginRateLimit = 3
	router := authTestRouterWithConfig(cfg)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			strings.NewReader(`{"email":"a@b.co","password":"secret123"}`))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"a@b.co","password":"secret123"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d", rec.Code)
	}
}

func authTestRouterWithConfig(cfg Config) http.Handler {
	return NewRouter(stair.NewService(), nil, newFakeAuth(), cfg)
}

func decodeResponse(rec *httptest.ResponseRecorder, dst any) error {
	return json.Unmarshal(rec.Body.Bytes(), dst)
}

// testRateLimiterWindow — проверка сброса окна.
func TestRateLimitWindowResets(t *testing.T) {
	l := newRateLimiter(1, 10*time.Millisecond)
	if !l.allow("1.2.3.4") {
		t.Fatal("first request must be allowed")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("second request must be blocked")
	}
	time.Sleep(20 * time.Millisecond)
	if !l.allow("1.2.3.4") {
		t.Fatal("request after window must be allowed")
	}
}
