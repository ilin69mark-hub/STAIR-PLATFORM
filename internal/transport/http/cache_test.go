package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCacheMiddleware_NoCache(t *testing.T) {
	policies := map[string]CachePolicy{
		"/api/v1/": CacheNoCache,
	}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected no-store, got %q", got)
	}
}

func TestCacheMiddleware_Short(t *testing.T) {
	policies := map[string]CachePolicy{
		"/api/v1/": CacheShort,
	}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	got := w.Header().Get("Cache-Control")
	if got != "max-age=300, private, must-revalidate" {
		t.Errorf("expected max-age=300 private must-revalidate, got %q", got)
	}
}

func TestCacheMiddleware_Long(t *testing.T) {
	policies := map[string]CachePolicy{
		"/static/": CacheLong,
	}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/static/bundle.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "max-age=86400" {
		t.Errorf("expected max-age=86400, got %q", got)
	}
}

func TestCacheMiddleware_Immutable(t *testing.T) {
	policies := map[string]CachePolicy{
		"/assets/": CacheImmutable,
	}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/assets/app.abc123.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "max-age=31536000" {
		t.Errorf("expected max-age=31536000, got %q", got)
	}
}

func TestCacheMiddleware_ExactMatch(t *testing.T) {
	policies := map[string]CachePolicy{
		"/api/v1/auth/login": CacheNoCache,
		"/api/v1/":           CacheShort,
	}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected no-store for exact match, got %q", got)
	}
}

func TestCacheMiddleware_DefaultNoCache(t *testing.T) {
	policies := map[string]CachePolicy{
		"/api/": CacheShort,
	}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected no-store for unknown path, got %q", got)
	}
}

func TestCacheHeaders_NoStore(t *testing.T) {
	w := httptest.NewRecorder()
	CacheHeaders(w, CacheNoCache, time.Time{})

	if got := w.Header().Get("Pragma"); got != "no-cache" {
		t.Errorf("expected Pragma no-cache, got %q", got)
	}
	if got := w.Header().Get("Expires"); got != "0" {
		t.Errorf("expected Expires 0, got %q", got)
	}
}

func TestCacheHeaders_LastModified(t *testing.T) {
	w := httptest.NewRecorder()
	lastMod := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	CacheHeaders(w, CacheShort, lastMod)

	if got := w.Header().Get("Last-Modified"); got != "Thu, 15 Jan 2026 10:30:00 GMT" {
		t.Errorf("expected Last-Modified header, got %q", got)
	}
}

func TestCachePolicy_String(t *testing.T) {
	tests := []struct {
		policy CachePolicy
		want   string
	}{
		{CacheNoCache, "no-store"},
		{CacheShort, "max-age=300, private, must-revalidate"},
		{CacheMedium, "max-age=3600, private"},
		{CacheLong, "max-age=86400"},
		{CacheImmutable, "max-age=31536000"},
	}
	for _, tt := range tests {
		if got := tt.policy.String(); got != tt.want {
			t.Errorf("CachePolicy.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestFindCachePolicy_LongestPrefix(t *testing.T) {
	policies := map[string]CachePolicy{
		"/api/":           CacheShort,
		"/api/v1/auth/":   CacheNoCache,
		"/api/v1/stairs/": CacheMedium,
	}

	tests := []struct {
		path string
		want CachePolicy
	}{
		{"/api/v1/auth/login", CacheNoCache},
		{"/api/v1/stairs/123", CacheMedium},
		{"/api/v1/projects", CacheShort},
	}
	for _, tt := range tests {
		got := findCachePolicy(tt.path, policies)
		if got != tt.want {
			t.Errorf("findCachePolicy(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// S-107: Private-политика не должна оставлять ответ в браузерном кеше, если
// запрос несёт identity (session-cookie) — иначе данные утекают после logout.
func TestCacheMiddleware_PrivateDowngradedWithIdentity(t *testing.T) {
	policies := map[string]CachePolicy{
		"/api/v1/projects/": CacheShort,
		"/api/v1/":          CacheNoCache,
	}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/123", nil)
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("authenticated GET must be no-store, got %q", got)
	}

	// Без identity политика сохраняется (downgrade скоупнут на identity).
	anon := httptest.NewRequest(http.MethodGet, "/api/v1/projects/123", nil)
	aw := httptest.NewRecorder()
	handler.ServeHTTP(aw, anon)
	if got := aw.Header().Get("Cache-Control"); got != "max-age=300, private, must-revalidate" {
		t.Fatalf("anonymous GET must keep policy, got %q", got)
	}
}

// S-107: публичные статические ассеты не должны терять кеш из-за cookie.
func TestCacheMiddleware_PublicAssetKeepsCacheWithIdentity(t *testing.T) {
	policies := map[string]CachePolicy{"/assets/": CacheImmutable}
	handler := CacheMiddleware(policies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/assets/app.abc123.js", nil)
	req.AddCookie(testCookie(sessionCookieName, "token-1"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "max-age=31536000" {
		t.Fatalf("public asset must stay cacheable, got %q", got)
	}
}

// S-107 (PROJECTS-BROWSER-CACHE-LEAK): реальный роутер на аутентифицированном
// GET /api/v1/projects не должен отдавать max-age/private.
func TestProjectsAuthenticatedResponseIsNotBrowserCached(t *testing.T) {
	req := authedRequest(http.MethodGet, "/api/v1/projects", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(newFakeProjectService()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := rec.Header().Get("Cache-Control")
	if got != "no-store" {
		t.Fatalf("projects response must be no-store, got %q", got)
	}
}
