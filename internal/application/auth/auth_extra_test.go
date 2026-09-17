package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
)

type overRepo struct {
	*fakeRepo
	acctErr         error
	tenantErr       error
	createAcctErr   error
	createUserErr   error
	consumeErr      error
	createStateErr  error
	createSessErr   error
	sessErr         error
	byIDErr         error
	policyErr       error
	updatePolicyErr error
	createKeyErr    error
	emailErrSet     []error
}

func (r *overRepo) DefaultTenant(ctx context.Context) (*Tenant, error) {
	if r.tenantErr != nil {
		return nil, r.tenantErr
	}
	return r.fakeRepo.DefaultTenant(ctx)
}

func (r *overRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if len(r.emailErrSet) > 0 {
		e := r.emailErrSet[0]
		r.emailErrSet = r.emailErrSet[1:]
		if e != nil {
			return nil, e
		}
	}
	return r.fakeRepo.GetUserByEmail(ctx, email)
}

func (r *overRepo) GetUserByID(ctx context.Context, id string) (*User, error) {
	if r.byIDErr != nil {
		return nil, r.byIDErr
	}
	return r.fakeRepo.GetUserByID(ctx, id)
}

func (r *overRepo) GetOAuthAccountByProviderSubject(ctx context.Context, provider, subject string) (*OAuthAccount, error) {
	if r.acctErr != nil {
		return nil, r.acctErr
	}
	return r.fakeRepo.GetOAuthAccountByProviderSubject(ctx, provider, subject)
}

func (r *overRepo) CreateUser(ctx context.Context, u *User) error {
	if r.createUserErr != nil {
		return r.createUserErr
	}
	return r.fakeRepo.CreateUser(ctx, u)
}

func (r *overRepo) CreateOAuthAccount(ctx context.Context, a *OAuthAccount) error {
	if r.createAcctErr != nil {
		return r.createAcctErr
	}
	return r.fakeRepo.CreateOAuthAccount(ctx, a)
}

func (r *overRepo) CreateSsoState(ctx context.Context, s *SsoState) error {
	if r.createStateErr != nil {
		return r.createStateErr
	}
	return r.fakeRepo.CreateSsoState(ctx, s)
}

func (r *overRepo) ConsumeSsoState(ctx context.Context, stateHash string) (*SsoState, error) {
	if r.consumeErr != nil {
		return nil, r.consumeErr
	}
	return r.fakeRepo.ConsumeSsoState(ctx, stateHash)
}

func (r *overRepo) CreateSession(ctx context.Context, s *Session) error {
	if r.createSessErr != nil {
		return r.createSessErr
	}
	return r.fakeRepo.CreateSession(ctx, s)
}

func (r *overRepo) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	if r.sessErr != nil {
		return nil, r.sessErr
	}
	return r.fakeRepo.GetSessionByTokenHash(ctx, tokenHash)
}

func (r *overRepo) GetPolicy(ctx context.Context, tenantID string) (Policy, error) {
	if r.policyErr != nil {
		return Policy{}, r.policyErr
	}
	return r.fakeRepo.GetPolicy(ctx, tenantID)
}

func (r *overRepo) UpdatePolicy(ctx context.Context, tenantID string, p Policy) error {
	if r.updatePolicyErr != nil {
		return r.updatePolicyErr
	}
	return r.fakeRepo.UpdatePolicy(ctx, tenantID, p)
}

func (r *overRepo) CreateApiKey(ctx context.Context, k *ApiKey) error {
	if r.createKeyErr != nil {
		return r.createKeyErr
	}
	return r.fakeRepo.CreateApiKey(ctx, k)
}

type failAuthURLOIDC struct {
	name    string
	authErr error
}

func (f failAuthURLOIDC) Enabled() bool                                 { return true }
func (f failAuthURLOIDC) Name() string                                  { return f.name }
func (f failAuthURLOIDC) AuthCodeURL(_, _, _, _ string) (string, error) { return "", f.authErr }
func (f failAuthURLOIDC) Exchange(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (f failAuthURLOIDC) VerifyIDToken(_ context.Context, _, _ string) (*IDTokenClaims, error) {
	return nil, errors.New("unused")
}

type auditRecorder struct {
	events []*audit.Event
	err    error
}

func (a *auditRecorder) Insert(_ context.Context, e *audit.Event) error {
	if a.err != nil {
		return a.err
	}
	a.events = append(a.events, e)
	return nil
}
func (a *auditRecorder) ListByProject(context.Context, string, string) ([]*audit.Event, error) {
	return nil, nil
}
func (a *auditRecorder) ListByTenant(context.Context, string) ([]*audit.Event, error) {
	return nil, nil
}

func TestSsoCallbackMissingCodeOrState(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{enabled: true, name: "testidp"})
	if _, _, err := svc.SsoCallback(context.Background(), "", ""); !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied for empty code/state, got %v", err)
	}
}

func TestSsoCallbackExistingAccountLogin(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw-token",
		idtok:   &IDTokenClaims{Subject: "sub-ex", Email: "sso.existing@example.com"},
	})
	u, _, err := svc.Register(context.Background(), "sso.existing@example.com", "E", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	repo.oauths = append(repo.oauths, &OAuthAccount{Provider: "testidp", Subject: "sub-ex", UserID: u.ID})
	state := rawStateFromURL(t, svc)
	got, token, err := svc.SsoCallback(context.Background(), "code", state)
	if err != nil {
		t.Fatalf("SsoCallback: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("user = %q, want existing %q", got.ID, u.ID)
	}
	if token == "" {
		t.Fatal("expected session token")
	}
	if _, ok := repo.sessions[HashToken(token)]; !ok {
		t.Fatal("expected stored session")
	}
}

func TestSsoCallbackExistingAccountGetUserError(t *testing.T) {
	repo := newFakeRepo()
	over := &overRepo{fakeRepo: repo, byIDErr: errors.New("db down")}
	svc := ssoSvc(over, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw",
		idtok:   &IDTokenClaims{Subject: "sub-b", Email: "a@b.co"},
	})
	repo.oauths = append(repo.oauths, &OAuthAccount{Provider: "testidp", Subject: "sub-b", UserID: "u-1"})
	state := rawStateFromURL(t, svc)
	_, _, err := svc.SsoCallback(context.Background(), "code", state)
	if err == nil || errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected raw db error, got %v", err)
	}
}

func TestSsoCallbackEmailNotInDefaultTenant(t *testing.T) {
	repo := newFakeRepo()
	seed := &User{ID: "u-99", TenantID: "t-99", Email: "tenantx@example.com", Role: RoleUser, Status: StatusActive}
	repo.users[seed.Email] = seed
	repo.byID[seed.ID] = seed
	svc := ssoSvc(repo, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw",
		idtok:   &IDTokenClaims{Subject: "sub-t", Email: "tenantx@example.com"},
	})
	state := rawStateFromURL(t, svc)
	if _, _, err := svc.SsoCallback(context.Background(), "code", state); !errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected ErrSsoDenied for foreign tenant email, got %v", err)
	}
}

func TestSsoCallbackAccountLookupError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), acctErr: errors.New("db down")}
	svc := ssoSvc(over, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw",
		idtok:   &IDTokenClaims{Subject: "sub-l", Email: "a@b.co"},
	})
	state := rawStateFromURL(t, svc)
	_, _, err := svc.SsoCallback(context.Background(), "code", state)
	if err == nil || errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected raw account lookup error, got %v", err)
	}
}

func TestSsoCallbackDefaultTenantError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), tenantErr: errors.New("no tenant")}
	svc := ssoSvc(over, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw",
		idtok:   &IDTokenClaims{Subject: "sub-d", Email: "a@b.co"},
	})
	state := rawStateFromURL(t, svc)
	_, _, err := svc.SsoCallback(context.Background(), "code", state)
	if err == nil || errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected raw tenant error, got %v", err)
	}
}

func TestSsoCallbackGetUserByEmailError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), emailErrSet: []error{errors.New("db down")}}
	svc := ssoSvc(over, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw",
		idtok:   &IDTokenClaims{Subject: "sub-e", Email: "a@b.co"},
	})
	state := rawStateFromURL(t, svc)
	_, _, err := svc.SsoCallback(context.Background(), "code", state)
	if err == nil || errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected raw email lookup error, got %v", err)
	}
}

func TestSsoCallbackConsumeStateError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), consumeErr: errors.New("db down")}
	svc := ssoSvc(over, &fakeOIDC{enabled: true, name: "testidp"})
	_, _, err := svc.SsoCallback(context.Background(), "code", "some-state")
	if err == nil || errors.Is(err, ErrSsoDenied) {
		t.Fatalf("expected raw consume error, got %v", err)
	}
}

func TestSsoCallbackCreateUserEmailExistsRace(t *testing.T) {
	repo := newFakeRepo()
	seed := &User{ID: "u-seeded", TenantID: "t-1", Email: "race@example.com", Role: RoleUser, Status: StatusActive}
	repo.users[seed.Email] = seed
	repo.byID[seed.ID] = seed
	over := &overRepo{fakeRepo: repo, createUserErr: ErrEmailExists, emailErrSet: []error{ErrNotFound}}
	svc := ssoSvc(over, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw",
		idtok:   &IDTokenClaims{Subject: "sub-r", Email: "race@example.com"},
	})
	state := rawStateFromURL(t, svc)
	u, _, err := svc.SsoCallback(context.Background(), "code", state)
	if err != nil {
		t.Fatalf("SsoCallback: %v", err)
	}
	if u.ID != "u-seeded" {
		t.Fatalf("expected to bind seeded user, got %q", u.ID)
	}
}

func TestSsoCallbackCreateOAuthAccountRace(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), createAcctErr: ErrOAuthExists}
	svc := ssoSvc(over, &fakeOIDC{
		enabled: true,
		name:    "testidp",
		raw:     "raw",
		idtok:   &IDTokenClaims{Subject: "sub-c", Email: "fresh@example.com"},
	})
	state := rawStateFromURL(t, svc)
	u, token, err := svc.SsoCallback(context.Background(), "code", state)
	if err != nil {
		t.Fatalf("SsoCallback: %v", err)
	}
	if u.Email != "fresh@example.com" || token == "" {
		t.Fatalf("expected success with fresh user, got %q / %q", u.Email, token)
	}
}

func TestSsoAuthorizeURLStateStoreError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), createStateErr: errors.New("db down")}
	svc := ssoSvc(over, &fakeOIDC{enabled: true, name: "testidp", authURL: "https://idp/auth"})
	if _, err := svc.SsoAuthorizeURL(context.Background(), "/"); err == nil {
		t.Fatal("expected state-store error")
	}
}

func TestSsoAuthorizeURLDefaultRedirect(t *testing.T) {
	repo := newFakeRepo()
	svc := ssoSvc(repo, &fakeOIDC{enabled: true, name: "testidp", authURL: "https://idp/auth"})
	if _, err := svc.SsoAuthorizeURL(context.Background(), ""); err != nil {
		t.Fatalf("SsoAuthorizeURL: %v", err)
	}
	if got := repo.ssoStates[0].Redirect; got != "/" {
		t.Fatalf("redirect = %q, want /", got)
	}
}

func TestSsoAuthorizeURLAuthURLError(t *testing.T) {
	svc := ssoSvc(newFakeRepo(), failAuthURLOIDC{name: "failauth", authErr: errors.New("auth url failed")})
	if _, err := svc.SsoAuthorizeURL(context.Background(), "/"); err == nil {
		t.Fatal("expected auth URL error")
	}
}

func TestSsoEnabledWithProvider(t *testing.T) {
	svc := ssoSvc(newFakeRepo(), &fakeOIDC{enabled: true, name: "testidp"})
	cfg := svc.SsoEnabled()
	if !cfg.Enabled || cfg.Provider != "testidp" {
		t.Fatalf("SsoEnabled = %+v, want enabled/testidp", cfg)
	}
}

func TestDefaultTenant(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	tnt, err := svc.DefaultTenant(context.Background())
	if err != nil {
		t.Fatalf("DefaultTenant: %v", err)
	}
	if tnt.ID != "t-1" {
		t.Fatalf("tenant = %q, want t-1", tnt.ID)
	}
	over := &overRepo{fakeRepo: newFakeRepo(), tenantErr: errors.New("no tenant")}
	if _, err := NewService(over, 0).DefaultTenant(context.Background()); err == nil {
		t.Fatal("expected DefaultTenant error")
	}
}

func TestRegisterCreateUserError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), createUserErr: errors.New("db down")}
	svc := NewService(over, 0)
	if _, _, err := svc.Register(context.Background(), "fail@example.com", "A", "password123"); err == nil {
		t.Fatal("expected create user error")
	}
}

func TestLoginGetUserError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), emailErrSet: []error{errors.New("db down")}}
	svc := NewService(over, 0)
	_, _, err := svc.Login(context.Background(), "nobody@example.com", "password123")
	if err == nil || errors.Is(err, ErrInvalidCreds) || errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected raw email lookup error, got %v", err)
	}
}

func TestLoginCreateSessionError(t *testing.T) {
	repo := newFakeRepo()
	over := &overRepo{fakeRepo: repo}
	svc := NewService(over, 0)
	_, _, err := svc.Register(context.Background(), "sess@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	over.createSessErr = errors.New("db down")
	if _, _, err := svc.Login(context.Background(), "sess@example.com", "password123"); err == nil {
		t.Fatal("expected create session error")
	}
}

func TestAuthenticateGetSessionError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), sessErr: errors.New("db down")}
	svc := NewService(over, 0)
	if _, _, err := svc.Authenticate(context.Background(), "some-token"); err == nil || errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected raw session lookup error, got %v", err)
	}
}

func TestAuthenticateGetUserError(t *testing.T) {
	repo := newFakeRepo()
	over := &overRepo{fakeRepo: repo}
	svc := NewService(over, time.Hour)
	_, token, err := svc.Register(context.Background(), "uerr@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	over.byIDErr = errors.New("db down")
	if _, _, err := svc.Authenticate(context.Background(), token); err == nil {
		t.Fatal("expected get-user error")
	}
}

func TestAuthenticateRotationSessionError(t *testing.T) {
	repo := newFakeRepo()
	over := &overRepo{fakeRepo: repo}
	svc := NewService(over, time.Hour)
	_, token, err := svc.Register(context.Background(), "rot2@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if s, ok := repo.sessions[HashToken(token)]; ok {
		s.CreatedAt = time.Now().UTC().Add(-time.Hour)
	}
	over.createSessErr = errors.New("db down")
	if _, _, err := svc.Authenticate(context.Background(), token); err == nil {
		t.Fatal("expected rotation session error")
	}
}

func TestUpdateUserGetUserNotFound(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), byIDErr: ErrNotFound}
	svc := NewService(over, 0)
	admin := RoleAdmin
	if err := svc.UpdateUser(context.Background(), "t-1", "admin", "u-x", &admin, nil); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetPolicyError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), policyErr: errors.New("db down")}
	svc := NewService(over, 0)
	if _, err := svc.GetPolicy(context.Background(), "t-1"); err == nil {
		t.Fatal("expected policy error")
	}
}

func TestUpdatePolicyError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), updatePolicyErr: errors.New("db down")}
	svc := NewService(over, 0)
	p := Policy{MinPasswordLength: 12, SessionTTLSeconds: 7200, LoginRateLimitPerMin: 20}
	if err := svc.UpdatePolicy(context.Background(), "t-1", "admin", p); err == nil {
		t.Fatal("expected update policy error")
	}
}

func TestCreateApiKeyError(t *testing.T) {
	over := &overRepo{fakeRepo: newFakeRepo(), createKeyErr: errors.New("db down")}
	svc := NewService(over, 0)
	if _, _, err := svc.CreateApiKey(context.Background(), "t-1", "u-1", "CI", nil); err == nil {
		t.Fatal("expected create api key error")
	}
}

func TestListApiKeys(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	keys, err := svc.ListApiKeys(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("ListApiKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected empty keys, got %d", len(keys))
	}
}

func TestLogoutMissingAndEmptyToken(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Fatalf("Logout empty: %v", err)
	}
	if err := svc.Logout(context.Background(), "no-such-token"); err != nil {
		t.Fatalf("Logout unknown: %v", err)
	}
}

func TestAuditRecordedOnRegisterAndLoginDenied(t *testing.T) {
	rec := &auditRecorder{}
	svc := NewService(newFakeRepo(), 0, audit.NewService(rec))
	_, _, err := svc.Register(context.Background(), "audit@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(rec.events) != 1 || rec.events[0].Action != audit.ActionAuthRegister {
		t.Fatalf("events = %+v, want one register event", rec.events)
	}
	_, _, err = svc.Login(context.Background(), "nobody@example.com", "password123")
	if !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("Login: %v", err)
	}
	if len(rec.events) != 2 || rec.events[1].Result != audit.ResultDenied {
		t.Fatalf("events = %+v, want denied login event", rec.events)
	}
}

func TestAuditFailureIsBestEffort(t *testing.T) {
	rec := &auditRecorder{err: errors.New("audit down")}
	svc := NewService(newFakeRepo(), 0, audit.NewService(rec))
	u, _, err := svc.Register(context.Background(), "quiet@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("Register must succeed despite audit failure: %v", err)
	}
	if u.Email != "quiet@example.com" {
		t.Fatalf("email = %q", u.Email)
	}
}

func TestNewServiceWithAudit(t *testing.T) {
	svc := NewService(newFakeRepo(), 0, &audit.Service{}, &audit.Service{})
	if svc == nil {
		t.Fatal("expected service")
	}
}
