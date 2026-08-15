package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/stair"
)

// ssoTestRouter собирает роутер для SSO-тестов.
func ssoTestRouter(a AuthService) http.Handler {
	return NewRouter(stair.NewService(), nil, a, DefaultConfig())
}

func TestSsoConfig(t *testing.T) {
	fa := newFakeAuth()
	fa.ssoEnabled = true
	fa.ssoProvider = "example-idp"
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/auth/sso/config")
	if err != nil {
		t.Fatalf("GET config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var cfg auth.SsoConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !cfg.Enabled || cfg.Provider != "example-idp" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestSsoConfigDisabled(t *testing.T) {
	fa := newFakeAuth()
	fa.ssoEnabled = false
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/auth/sso/config")
	if err != nil {
		t.Fatalf("GET config: %v", err)
	}
	defer resp.Body.Close()
	var cfg auth.SsoConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("config must be disabled when provider not configured")
	}
}

func TestSsoBeginRedirects(t *testing.T) {
	fa := newFakeAuth()
	fa.ssoURL = "https://idp.example/authorize?x=1"
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/api/v1/auth/sso?redirect=%2Fprojects")
	if err != nil {
		t.Fatalf("GET sso: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://idp.example/authorize?x=1" {
		t.Fatalf("location = %q", loc)
	}
}

func TestSsoBeginDisabled(t *testing.T) {
	fa := newFakeAuth()
	fa.ssoBeginErr = auth.ErrSsoNotConfigured
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/auth/sso")
	if err != nil {
		t.Fatalf("GET sso: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestSsoCallbackSetsSession(t *testing.T) {
	fa := newFakeAuth()
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/api/v1/auth/sso/callback?code=abc&state=xyz")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Fatalf("location = %q, want /", loc)
	}
	// session cookie (httpOnly) должен быть выставлен.
	var hasSession bool
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName && c.Value == "sso-token" && c.HttpOnly {
			hasSession = true
		}
	}
	if !hasSession {
		t.Fatal("expected httpOnly session cookie after SSO callback")
	}
}

func TestSsoCallbackMissingParams(t *testing.T) {
	fa := newFakeAuth()
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/api/v1/auth/sso/callback")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc == "" {
		t.Fatal("expected redirect to frontend with error")
	}
}

func TestSsoCallbackDenied(t *testing.T) {
	fa := newFakeAuth()
	fa.ssoErr = auth.ErrSsoDenied
	srv := httptest.NewServer(ssoTestRouter(fa))
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/api/v1/auth/sso/callback?code=abc&state=xyz")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}
