package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appauth "stairplatform/internal/application/auth"
	"stairplatform/internal/application/stair"
)

// ---- стабы для регрессии SEC-005 -------------------------------------

// sec5Err / sec5Unauth — ошибка аутентификации стаба.
type sec5Err string

func (e sec5Err) Error() string { return string(e) }

var sec5Unauth error = sec5Err("unauthorized")

// sec5Auth — аутентификатор, выдающий пользователя по сессии "sess-user" и
// API-ключ по Bearer "Bearer valid-key".
type sec5Auth struct{ u *appauth.User }

func (a *sec5Auth) Register(context.Context, string, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec5Auth) Login(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}
func (a *sec5Auth) Authenticate(_ context.Context, tok string) (*appauth.User, string, error) {
	if tok == "sess-user" {
		return a.u, "", nil
	}
	return nil, "", sec5Unauth
}
func (a *sec5Auth) Logout(context.Context, string) error { return nil }
func (a *sec5Auth) ListUsers(context.Context, string) ([]*appauth.User, error) {
	return nil, nil
}
func (a *sec5Auth) UpdateUserRole(context.Context, string, string, string, appauth.Role) error {
	return nil
}
func (a *sec5Auth) UpdateUser(context.Context, string, string, string, *appauth.Role, *appauth.Status) error {
	return nil
}
func (a *sec5Auth) GetPolicy(context.Context, string) (appauth.Policy, error) {
	return appauth.Policy{}, nil
}
func (a *sec5Auth) UpdatePolicy(context.Context, string, string, appauth.Policy) error {
	return nil
}
func (a *sec5Auth) AuthenticateApiKey(_ context.Context, tok string) (*appauth.ApiKey, error) {
	if tok == "valid-key" {
		return &appauth.ApiKey{
			ID: "key-1", TenantID: "tenant-1", Name: "ci-integration",
			Scopes: []string{"project.read"}, TokenHash: "SHOULD-NEVER-LEAK",
		}, nil
	}
	return nil, sec5Unauth
}
func (a *sec5Auth) CreateApiKey(context.Context, string, string, string, []appauth.Permission) (*appauth.ApiKey, string, error) {
	return nil, "", nil
}
func (a *sec5Auth) ListApiKeys(context.Context, string) ([]*appauth.ApiKey, error) {
	return nil, nil
}
func (a *sec5Auth) RevokeApiKey(context.Context, string, string, string) error { return nil }
func (a *sec5Auth) SsoAuthorizeURL(context.Context, string) (string, error)    { return "", nil }
func (a *sec5Auth) SsoCallback(context.Context, string, string) (*appauth.User, string, error) {
	return nil, "", nil
}

// RevokeApiKeysByUser — отзыв всех ключей пользователя (SEC-007).
func (a *sec5Auth) RevokeApiKeysByUser(context.Context, string, string) error { return nil }
func (a *sec5Auth) SsoEnabled() appauth.SsoConfig                             { return appauth.SsoConfig{} }
func (a *sec5Auth) DefaultTenant(context.Context) (*appauth.Tenant, error) {
	return nil, nil
}

type sec5Stair struct{ s *stair.Service }

func (a *sec5Stair) Calculate(c context.Context, x stair.Config, o stair.Options) (*stair.Result, error) {
	return a.s.Calculate(c, x, o)
}
func (a *sec5Stair) Validate(c context.Context, x stair.Config, o stair.Options) (*stair.Result, error) {
	return a.s.Validate(c, x, o)
}
func (a *sec5Stair) Optimize(c context.Context, x stair.Config, o stair.Options, r stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return a.s.Optimize(c, x, o, r)
}

func sec5Router() http.Handler {
	u := &appauth.User{ID: "u-1", TenantID: "t-1", Email: "u@example.com", Name: "U", Role: appauth.RoleUser}
	cfg := DefaultConfig()
	cfg.SecurityConfig = &SecurityConfig{AllowedOrigins: []string{"http://x"}}
	return NewRouter(&sec5Stair{stair.NewService()}, &sec1Projects{n: 3}, &sec5Auth{u: u}, cfg, nil)
}

// TestSEC005_AuthMeWithAPIKey — регрессия SEC-005: GET /auth/me с валидным
// API-ключом падал (nil-pointer) и отдавал 500.
func TestSEC005_AuthMeWithAPIKey(t *testing.T) {
	h := sec5Router()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d (want 200) body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "500") || strings.Contains(body, "internal") {
		t.Errorf("response must not look like an error: %s", body)
	}
	var out struct {
		SubjectType string    `json:"subject_type"`
		APIKey      apiKeyDTO `json:"api_key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.SubjectType != "api_key" {
		t.Errorf("subject_type=%q want api_key", out.SubjectType)
	}
	if out.APIKey.ID != "key-1" || out.APIKey.TenantID != "tenant-1" {
		t.Errorf("key identity mismatch: %+v", out.APIKey)
	}
	if len(out.APIKey.Scopes) != 1 || out.APIKey.Scopes[0] != "project.read" {
		t.Errorf("scopes mismatch: %v", out.APIKey.Scopes)
	}
	// Секрет хранилища ключа не должен утекать ни в каком виде.
	if strings.Contains(body, "SHOULD-NEVER-LEAK") || strings.Contains(body, "token_hash") {
		t.Errorf("token hash leaked: %s", body)
	}
	t.Logf("ok: %s", body)
}

// TestSEC005_AuthMeWithSession — регрессия: сессионный путь не сломан.
func TestSEC005_AuthMeWithSession(t *testing.T) {
	h := sec5Router()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "sess-user"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		SubjectType string  `json:"subject_type"`
		User        userDTO `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.SubjectType != "user" || out.User.ID != "u-1" {
		t.Errorf("session path broken: %+v", out)
	}
}

// TestSEC005_AuthMeUnauthenticated — 401 без заголовков.
func TestSEC005_AuthMeUnauthenticated(t *testing.T) {
	h := sec5Router()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

// TestSEC005_RequestIDPresentInPanicResponse — регрессия SEC-005b: тело 500
// обязано содержать непустой request_id, совпадающий с заголовком
// X-Request-Id. Раньше recover стоял ВНЕ withLogging и видел контекст до
// вложения request id, поэтому поле всегда было пустым.
func TestSEC005_RequestIDPresentInPanicResponse(t *testing.T) {
	// Роутер, в котором обработчик гарантированно паникует.
	u := &appauth.User{ID: "u-1", TenantID: "t-1", Email: "u@e.com", Role: appauth.RoleUser}
	cfg := DefaultConfig()
	cfg.SecurityConfig = &SecurityConfig{AllowedOrigins: []string{"http://x"}}
	real := NewRouter(&sec5Stair{stair.NewService()}, &sec1Projects{n: 1}, &sec5Auth{u: u}, cfg, nil)

	// Оборачиваем mux-уровень: подменяем обработчик через дополнительный
	// слой ПОСЛЕ withLogging, чтобы паника произошла внутри цепочки.
	// Для этого используем отдельный роутер с одним падающим маршрутом.
	panicRouter := func() http.Handler {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) {
			panic("audit-test panic")
		})
		return buildChainForTest(mux)
	}()

	_ = real

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set("X-Request-Id", "req-fixed-123")
	panicRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", rec.Code)
	}
	var out struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if out.Error.RequestID == "" {
		t.Fatalf("request_id in 500 body is EMPTY (SEC-005b regression)")
	}
	if out.Error.RequestID != "req-fixed-123" {
		t.Errorf("request_id=%q want the value from X-Request-Id", out.Error.RequestID)
	}
	if hdr := rec.Header().Get(requestIDHeader); hdr != "req-fixed-123" {
		t.Errorf("%s header=%q want req-fixed-123", requestIDHeader, hdr)
	}
	t.Logf("500 body carries request_id=%q", out.Error.RequestID)
}

// buildChainForTest собирает ту же цепочку middleware, что и NewRouter, но
// вокруг произвольного mux. Держит тест регрессии SEC-005b независимым от
// состава боевых маршрутов.
func buildChainForTest(mux *http.ServeMux) http.Handler {
	cfg := DefaultConfig()
	cachePolicies := map[string]CachePolicy{}
	respCacheMW, _ := NewResponseCacheWithInvalidation(ResponseCacheConfig{MaxEntries: 8, DefaultTTL: 0})
	dedup := DeduplicateMiddleware(DeduplicateByKey)
	sec := SecurityMiddleware(cfg.SecurityConfig)
	bodyLimit := BodySizeLimit(cfg.MaxBodyBytes)
	compress := CompressionMiddleware
	timeout := RouteTimeoutMiddleware(DefaultAPIRouteTimeouts())
	version := VersionMiddleware

	var h http.Handler = withLogging(PanicRecoveryMiddleware(mux))
	h = sec(h)
	h = dedup(h)
	h = respCacheMW(h)
	h = CacheMiddleware(cachePolicies)(h)
	h = bodyLimit(h)
	h = compress(h)
	h = timeout(h)
	h = version(h)
	return TraceMiddleware(h)
}
