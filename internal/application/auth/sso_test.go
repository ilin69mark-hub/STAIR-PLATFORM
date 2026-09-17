package auth

import (
	"context"
	"errors"
	"net/url"
	"testing"
)

// ssoState returns list helper — извлекает raw state из authorization URL.
func rawStateFromURL(t *testing.T, svc *Service) string {
	t.Helper()
	u, err := svc.SsoAuthorizeURL(context.Background(), "/")
	if err != nil {
		t.Fatalf("SsoAuthorizeURL: %v", err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	return parsed.Query().Get("state")
}

// fakeOIDC — тестовая реализация OIDCProvider (без сети).
type fakeOIDC struct {
	enabled bool
	name    string
	authURL string
	raw     string
	idtok   *IDTokenClaims
	exErr   error
	verErr  error
}

func (f *fakeOIDC) Enabled() bool { return f.enabled }
func (f *fakeOIDC) Name() string  { return f.name }
func (f *fakeOIDC) AuthCodeURL(state, nonce, challenge, method string) (string, error) {
	return f.authURL + "?state=" + state + "&nonce=" + nonce, nil
}
func (f *fakeOIDC) Exchange(_ context.Context, _ string, _ string) (string, error) {
	if f.exErr != nil {
		return "", f.exErr
	}
	return f.raw, nil
}
func (f *fakeOIDC) VerifyIDToken(_ context.Context, _ string, _ string) (*IDTokenClaims, error) {
	if f.verErr != nil {
		return nil, f.verErr
	}
	return f.idtok, nil
}

func ssoSvc(repo Repository, p OIDCProvider) *Service {
	return NewService(repo, 0).WithOIDCProvider(p)
}

func TestSsoDisabled(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if _, err := svc.SsoAuthorizeURL(context.Background(), "/"); err != ErrSsoNotConfigured {
		t.Fatalf("expected ErrSsoNotConfigured, got %v", err)
	}
	if _, _, err := svc.SsoCallback(context.Background(), "code", "state"); err != ErrSsoNotConfigured {
		t.Fatalf("expected ErrSsoNotConfigured, got %v", err)
	}
	if cfg := svc.SsoEnabled(); cfg.Enabled {
		t.Fatal("SsoEnabled must be false when not configured")
	}
}

func TestSsoAuthorizeURLPersistsState(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{enabled: true, name: "testidp", authURL: "https://idp/auth"})
	u, err := svc.SsoAuthorizeURL(context.Background(), "/projects")
	if err != nil {
		t.Fatalf("SsoAuthorizeURL: %v", err)
	}
	if u == "" {
		t.Fatal("expected authorization URL")
	}
	if len(repo.ssoStates) != 1 {
		t.Fatalf("expected 1 sso state stored, got %d", len(repo.ssoStates))
	}
	st := repo.ssoStates[0]
	if st.Redirect != "/projects" {
		t.Fatalf("redirect = %q, want /projects", st.Redirect)
	}
	if st.PKCEVerifier == "" || st.Nonce == "" {
		t.Fatal("expected nonce and pkce verifier")
	}
	if st.ExpiresAt.IsZero() {
		t.Fatal("expected expiry on sso state")
	}
}

func TestSsoCallbackProvisionsNewUser(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw-token",
		idtok:   &IDTokenClaims{Issuer: "https://iss", Subject: "sub-1", Email: "sso@example.com", Name: "SSO User"},
	})
	// Предварительно "начинаем" вход, чтобы state существовал.
	state := rawStateFromURL(t, svc)
	u, token, err := svc.SsoCallback(context.Background(), "auth-code", state)
	if err != nil {
		t.Fatalf("SsoCallback: %v", err)
	}
	if u.Email != "sso@example.com" {
		t.Fatalf("email = %q", u.Email)
	}
	if u.Role != RoleUser || u.Status != StatusActive {
		t.Fatalf("role/status = %q/%q", u.Role, u.Status)
	}
	if token == "" {
		t.Fatal("expected session token after SSO")
	}
	if u.PasswordHash == "" || u.PasswordHash == "sso@example.com" {
		t.Fatal("SSO user must have a password stub hash")
	}
	// Привязка сохранена.
	acct, err := repo.GetOAuthAccountByProviderSubject(context.Background(), "testidp", "sub-1")
	if err != nil {
		t.Fatalf("GetOAuthAccount: %v", err)
	}
	if acct.UserID != u.ID {
		t.Fatalf("oauth user=%q, want %q", acct.UserID, u.ID)
	}
	// Состояние израсходовано.
	if len(repo.ssoStates) != 0 {
		t.Fatalf("expected sso state consumed, got %d", len(repo.ssoStates))
	}
}

func TestSsoCallbackLinksExistingUserByEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw-token",
		idtok:   &IDTokenClaims{Issuer: "https://iss", Subject: "sub-2", Email: "existing@example.com", Name: "Existing"},
	})
	// Регистрируем существующего пользователя с тем же email.
	pre, _, err := svc.Register(context.Background(), "existing@example.com", "Pre", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	state := rawStateFromURL(t, svc)
	u, _, err := svc.SsoCallback(context.Background(), "auth-code", state)
	if err != nil {
		t.Fatalf("SsoCallback: %v", err)
	}
	if u.ID != pre.ID {
		t.Fatalf("SSO must link to existing user (got %q, want %q)", u.ID, pre.ID)
	}
	if len(repo.users) != 1 {
		t.Fatalf("expected 1 user total, got %d", len(repo.users))
	}
	acct, err := repo.GetOAuthAccountByProviderSubject(context.Background(), "testidp", "sub-2")
	if err != nil {
		t.Fatalf("linked account: %v", err)
	}
	if acct.UserID != pre.ID {
		t.Fatalf("oauth user=%q, want %q", acct.UserID, pre.ID)
	}
}

func TestSsoCallbackRejectsBadState(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{enabled: true, name: "testidp"})
	_, _, err := svc.SsoCallback(context.Background(), "code", "unknown-state")
	if !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied, got %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatal("no session should be created on bad state")
	}
}

func TestSsoCallbackRejectsExchangeFailure(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		exErr:   errors.New("token endpoint down"),
	})
	state := rawStateFromURL(t, svc)
	if _, _, err := svc.SsoCallback(context.Background(), "code", state); !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied on exchange failure, got %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatal("no session should be created on exchange failure")
	}
}

func TestSsoCallbackRejectsInvalidIdToken(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw-token",
		verErr:  errors.New("oidc: signatures mismatch"),
	})
	state := rawStateFromURL(t, svc)
	if _, _, err := svc.SsoCallback(context.Background(), "code", state); !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied on invalid id_token, got %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatal("no session should be created on invalid id_token")
	}
}

func TestSsoCallbackRejectsMissingEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw-token",
		idtok:   &IDTokenClaims{Issuer: "https://iss", Subject: "sub-3"},
	})
	state := rawStateFromURL(t, svc)
	if _, _, err := svc.SsoCallback(context.Background(), "code", state); !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied when no email claim, got %v", err)
	}
}

func TestSsoCallbackRejectsDisabledUser(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw-token",
		idtok:   &IDTokenClaims{Issuer: "https://iss", Subject: "sub-4", Email: "disabled@sso.example"},
	})
	// Первый вход — создаёт пользователя.
	state := rawStateFromURL(t, svc)
	u, _, err := svc.SsoCallback(context.Background(), "code", state)
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	// Блокируем пользователя.
	if err := repo.UpdateUserStatus(context.Background(), u.TenantID, u.ID, StatusDisabled); err != nil {
		t.Fatalf("disable: %v", err)
	}
	// Повторный вход через SSO отклоняется.
	state2 := rawStateFromURL(t, svc)
	if _, _, err := svc.SsoCallback(context.Background(), "code", state2); !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied for disabled user, got %v", err)
	}
}

func TestSsoStateCanBeConsumedOnce(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw-token",
		idtok:   &IDTokenClaims{Issuer: "https://iss", Subject: "sub-5", Email: "sso1@example.com"},
	})
	state := rawStateFromURL(t, svc)
	if _, _, err := svc.SsoCallback(context.Background(), "code", state); err != nil {
		t.Fatalf("first callback: %v", err)
	}
	// Повторное использование того же state отклоняется (state израсходован).
	if _, _, err := svc.SsoCallback(context.Background(), "code", state); !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied on reuse, got %v", err)
	}
}
