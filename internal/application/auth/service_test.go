package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRepo — тестовая реализация Repository в памяти.
type fakeRepo struct {
	users     map[string]*User // by email
	byID      map[string]*User
	sessions  map[string]*Session // by token hash
	tenant    *Tenant
	policy    *Policy
	apiKeys   []*ApiKey
	oauths    []*OAuthAccount
	ssoStates []*SsoState
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:    map[string]*User{},
		byID:     map[string]*User{},
		sessions: map[string]*Session{},
		tenant:   &Tenant{ID: "t-1", Slug: "default"},
	}
}

func (f *fakeRepo) DefaultTenant(ctx context.Context) (*Tenant, error) { return f.tenant, nil }

func (f *fakeRepo) CreateUser(ctx context.Context, u *User) error {
	if _, ok := f.users[u.Email]; ok {
		return ErrEmailExists
	}
	u.ID = "u-" + u.Email
	f.users[u.Email] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u, ok := f.users[email]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) GetUserByID(ctx context.Context, id string) (*User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) ListUsers(ctx context.Context, tenantID string) ([]*User, error) {
	var out []*User
	for _, u := range f.byID {
		if u.TenantID == tenantID {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateUserRole(ctx context.Context, tenantID, userID string, role Role) error {
	u, ok := f.byID[userID]
	if !ok || u.TenantID != tenantID {
		return ErrNotFound
	}
	u.Role = role
	return nil
}

func (f *fakeRepo) UpdateUserStatus(ctx context.Context, tenantID, userID string, status Status) error {
	u, ok := f.byID[userID]
	if !ok || u.TenantID != tenantID {
		return ErrNotFound
	}
	u.Status = status
	return nil
}

func (f *fakeRepo) DeleteUserSessions(ctx context.Context, userID string) error {
	for h, s := range f.sessions {
		if s.UserID == userID {
			delete(f.sessions, h)
		}
	}
	return nil
}

// ---- политики безопасности (EDR-0016) ----

func (f *fakeRepo) GetPolicy(ctx context.Context, tenantID string) (Policy, error) {
	if f.policy == nil {
		return DefaultPolicy(), ErrNotFound
	}
	return f.policy.WithDefaults(), nil
}

func (f *fakeRepo) UpdatePolicy(ctx context.Context, tenantID string, p Policy) error {
	f.policy = &p
	return nil
}

// ---- API-ключи (EDR-0016) ----

func (f *fakeRepo) CreateApiKey(ctx context.Context, k *ApiKey) error {
	k.ID = "key-" + k.TokenHash
	f.apiKeys = append(f.apiKeys, k)
	return nil
}

func (f *fakeRepo) ListApiKeys(ctx context.Context, tenantID string) ([]*ApiKey, error) {
	var out []*ApiKey
	for _, k := range f.apiKeys {
		if k.TenantID == tenantID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetApiKeyByTokenHash(ctx context.Context, tokenHash string) (*ApiKey, error) {
	for _, k := range f.apiKeys {
		if k.TokenHash == tokenHash {
			return k, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) RevokeApiKey(ctx context.Context, tenantID, keyID string) error {
	for _, k := range f.apiKeys {
		if k.ID == keyID && k.TenantID == tenantID {
			now := time.Now().UTC()
			k.RevokedAt = &now
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) TouchApiKey(ctx context.Context, keyID string) error {
	for _, k := range f.apiKeys {
		if k.ID == keyID {
			now := time.Now().UTC()
			k.LastUsedAt = &now
			return nil
		}
	}
	return nil
}

// ---- SSO / OAuthAccount (EDR-0017) ----

func (f *fakeRepo) CreateOAuthAccount(ctx context.Context, a *OAuthAccount) error {
	for _, ex := range f.oauths {
		if ex.Provider == a.Provider && ex.Subject == a.Subject {
			return ErrOAuthExists
		}
	}
	a.ID = "oa-" + a.Provider + "-" + a.Subject
	a.CreatedAt = time.Now().UTC()
	f.oauths = append(f.oauths, a)
	return nil
}

func (f *fakeRepo) GetOAuthAccountByProviderSubject(ctx context.Context, provider, subject string) (*OAuthAccount, error) {
	for _, a := range f.oauths {
		if a.Provider == provider && a.Subject == subject {
			return a, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) ListOAuthAccounts(ctx context.Context, userID string) ([]*OAuthAccount, error) {
	var out []*OAuthAccount
	for _, a := range f.oauths {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeRepo) CreateSsoState(ctx context.Context, s *SsoState) error {
	s.ID = "st-" + s.StateHash
	s.CreatedAt = time.Now().UTC()
	f.ssoStates = append(f.ssoStates, s)
	return nil
}

func (f *fakeRepo) ConsumeSsoState(ctx context.Context, stateHash string) (*SsoState, error) {
	for i, s := range f.ssoStates {
		if s.StateHash == stateHash {
			if time.Now().UTC().After(s.ExpiresAt) {
				return nil, ErrNotFound
			}
			f.ssoStates = append(f.ssoStates[:i], f.ssoStates[i+1:]...)
			return s, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) CreateSession(ctx context.Context, s *Session) error {
	s.ID = "s-" + s.TokenHash
	s.CreatedAt = time.Now().UTC()
	f.sessions[s.TokenHash] = s
	return nil
}

func (f *fakeRepo) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	s, ok := f.sessions[tokenHash]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (f *fakeRepo) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	delete(f.sessions, tokenHash)
	return nil
}

func TestRegisterCreatesUserInDefaultTenant(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	u, token, err := svc.Register(context.Background(), "user@example.com", "Иван", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.TenantID != "t-1" {
		t.Fatalf("tenant = %q, want t-1", u.TenantID)
	}
	if u.Role != RoleUser || u.Status != StatusActive {
		t.Fatalf("role/status = %q/%q", u.Role, u.Status)
	}
	if u.PasswordHash == "" || u.PasswordHash == "password123" {
		t.Fatal("password must be hashed")
	}
	if token == "" {
		t.Fatal("expected session token from register (auto-login)")
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if _, _, err := svc.Register(context.Background(), "not-an-email", "A", "password123"); err != ErrInvalidEmail {
		t.Fatalf("invalid email: expected ErrInvalidEmail, got %v", err)
	}
	if _, _, err := svc.Register(context.Background(), "a@b.co", "A", "short"); err != ErrWeakPassword {
		t.Fatalf("weak password: expected ErrWeakPassword, got %v", err)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	_, _, err := svc.Register(context.Background(), "dup@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, _, err := svc.Register(context.Background(), "dup@example.com", "B", "password123"); err != ErrEmailExists {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
}

func TestLoginAndAuthenticate(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)
	_, _, err := svc.Register(context.Background(), "login@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	u, token, err := svc.Login(context.Background(), "login@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if u.Email != "login@example.com" {
		t.Fatalf("email = %q", u.Email)
	}
	if token == "" {
		t.Fatal("expected session token")
	}

	// Сессия сохранена как хеш.
	if _, ok := repo.sessions[HashToken(token)]; !ok {
		t.Fatal("session must be stored by token hash")
	}

	got, rotated, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if rotated != "" {
		t.Fatalf("fresh session must not rotate, got new token %q", rotated)
	}
	if got.ID != u.ID {
		t.Fatalf("user id = %q, want %q", got.ID, u.ID)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	_, _, _ = svc.Register(context.Background(), "a@b.co", "A", "password123")
	if _, _, err := svc.Login(context.Background(), "a@b.co", "wrongpass"); err != ErrInvalidCreds {
		t.Fatalf("expected ErrInvalidCreds, got %v", err)
	}
}

// TestSessionRotation — старая сессия ротируется (EDR-0014 §3.1):
// выдан новый токен, старая сессия удалена, новый токен валиден.
func TestSessionRotation(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)
	_, _, _ = svc.Register(context.Background(), "rot@example.com", "A", "password123")
	_, token, err := svc.Login(context.Background(), "rot@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Состарим сессию сверх половины TTL (TTL=1ч, половина — 30 мин).
	if s, ok := repo.sessions[HashToken(token)]; ok {
		s.CreatedAt = time.Now().UTC().Add(-time.Hour)
	}

	u, rotated, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if rotated == "" {
		t.Fatal("expected session rotation for old session")
	}
	if u.ID == "" {
		t.Fatal("expected user")
	}
	// Старая сессия удалена, новый токен валиден.
	if _, ok := repo.sessions[HashToken(token)]; ok {
		t.Fatal("old session must be deleted after rotation")
	}
	if got, r2, err := svc.Authenticate(context.Background(), rotated); err != nil || got == nil {
		t.Fatalf("rotated token must authenticate (err=%v)", err)
	} else if r2 != "" {
		t.Fatalf("fresh rotated session must not rotate again, got %q", r2)
	}
}

// TestSessionRotationFreshNone — свежая сессия не ротируется.
func TestSessionRotationFreshNone(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)
	_, _, _ = svc.Register(context.Background(), "fresh@example.com", "A", "password123")
	_, token, _ := svc.Login(context.Background(), "fresh@example.com", "password123")

	_, rotated, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if rotated != "" {
		t.Fatalf("fresh session must not rotate, got %q", rotated)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if _, _, err := svc.Login(context.Background(), "nobody@example.com", "x12345678"); err != ErrInvalidCreds {
		t.Fatalf("expected ErrInvalidCreds, got %v", err)
	}
}

func TestAuthenticateExpiredSession(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, time.Millisecond) // TTL 1мс — истекает мгновенно
	_, _, _ = svc.Register(context.Background(), "exp@example.com", "A", "password123")
	_, token, err := svc.Login(context.Background(), "exp@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, _, err := svc.Authenticate(context.Background(), token); err != ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestAuthenticateUnknownToken(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if _, _, err := svc.Authenticate(context.Background(), "no-such-token"); err != ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)
	_, _, _ = svc.Register(context.Background(), "logout@example.com", "A", "password123")
	_, token, _ := svc.Login(context.Background(), "logout@example.com", "password123")

	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, _, err := svc.Authenticate(context.Background(), token); err != ErrSessionExpired {
		t.Fatalf("token must be revoked: expected ErrSessionExpired, got %v", err)
	}
}

func TestDisabledUserCannotLogin(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "off@example.com", "A", "password123")
	u.Status = StatusDisabled

	// S-113: disabled-аккаунт неотличим от неверного пароля — ErrInvalidCreds,
	// а не ErrUserDisabled (состояние не раскрывается на login-эндпоинте).
	// И с верным, и с неверным паролем — одинаковый ответ.
	if _, _, err := svc.Login(context.Background(), "off@example.com", "password123"); err != ErrInvalidCreds {
		t.Fatalf("expected ErrInvalidCreds for disabled user (correct password), got %v", err)
	}
	if _, _, err := svc.Login(context.Background(), "off@example.com", "wrong-password"); err != ErrInvalidCreds {
		t.Fatalf("expected ErrInvalidCreds for disabled user (wrong password), got %v", err)
	}
}

// TestAuthenticateDisabledUser — аутентифицированный путь (валидная сессия +
// disabled-аккаунт) ВОЗВРАЩАЕТ ErrUserDisabled: пользователь уже доказал
// владение сессией, состояние можно сообщить (S-113).
func TestAuthenticateDisabledUser(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)
	u, token, err := svc.Register(context.Background(), "off@example.com", "A", "password123")
	if err != nil {
		t.Fatal(err)
	}
	u.Status = StatusDisabled

	if _, _, err := svc.Authenticate(context.Background(), token); err != ErrUserDisabled {
		t.Fatalf("expected ErrUserDisabled from Authenticate, got %v", err)
	}
}

// --- EDR-0015: Permission-модель и admin-операции ---

func TestRolePermissionsMatrix(t *testing.T) {
	cases := []struct {
		role    Role
		perm    Permission
		allowed bool
	}{
		{RoleAdmin, PermissionAuditReadAll, true},
		{RoleAdmin, PermissionUsersList, true},
		{RoleAdmin, PermissionUsersUpdateRole, true},
		{RoleUser, PermissionAuditReadAll, false},
		{RoleUser, PermissionUsersList, false},
		{RoleUser, PermissionUsersUpdateRole, false},
	}
	for _, c := range cases {
		if got := c.role.HasPermission(c.perm); got != c.allowed {
			t.Errorf("%s.HasPermission(%s) = %v, want %v", c.role, c.perm, got, c.allowed)
		}
	}
}

func TestParseRole(t *testing.T) {
	if _, err := ParseRole("admin"); err != nil {
		t.Fatalf("admin: %v", err)
	}
	if _, err := ParseRole("user"); err != nil {
		t.Fatalf("user: %v", err)
	}
	if _, err := ParseRole("superadmin"); err != ErrUnknownRole {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}

func TestListUsersScopedToTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	_, _, _ = svc.Register(context.Background(), "one@example.com", "A", "password123")
	_, _, _ = svc.Register(context.Background(), "two@example.com", "B", "password123")

	users, err := svc.ListUsers(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users in tenant t-1, got %d", len(users))
	}
	other, err := svc.ListUsers(context.Background(), "t-2")
	if err != nil {
		t.Fatalf("ListUsers t-2: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("expected 0 users in tenant t-2, got %d", len(other))
	}
}

func TestUpdateUserRole(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "up@example.com", "A", "password123")

	if err := svc.UpdateUserRole(context.Background(), u.TenantID, "admin-actor", u.ID, RoleAdmin); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}
	got, err := repo.GetUserByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Role != RoleAdmin {
		t.Fatalf("expected role admin, got %s", got.Role)
	}
}

func TestUpdateUserRoleCannotChangeOwnRole(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "self@example.com", "A", "password123")

	if err := svc.UpdateUserRole(context.Background(), u.TenantID, u.ID, u.ID, RoleAdmin); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for self-role change, got %v", err)
	}
}

func TestUpdateUserRoleNotFoundInTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "miss@example.com", "A", "password123")

	err := svc.UpdateUserRole(context.Background(), "t-other", "admin", u.ID, RoleAdmin)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for foreign tenant, got %v", err)
	}
}

// --- EDR-0016: Enterprise Controls ---

func TestAdminPermissionsMatrix(t *testing.T) {
	for _, perm := range []Permission{
		PermissionUsersManage, PermissionSettingsRead, PermissionSettingsWrite,
		PermissionDataExport, PermissionApiKeysManage, PermissionOrdersList, PermissionOrdersManage,
	} {
		if !RoleAdmin.HasPermission(perm) {
			t.Errorf("admin must have %s", perm)
		}
		if RoleUser.HasPermission(perm) {
			t.Errorf("user must NOT have %s", perm)
		}
	}
}

func TestParseStatus(t *testing.T) {
	if _, err := ParseStatus("active"); err != nil {
		t.Fatalf("active: %v", err)
	}
	if _, err := ParseStatus("disabled"); err != nil {
		t.Fatalf("disabled: %v", err)
	}
	if _, err := ParseStatus("banned"); err != ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestUpdateUserStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "st@example.com", "A", "password123")

	disabled := StatusDisabled
	if err := svc.UpdateUser(context.Background(), u.TenantID, "admin", u.ID, nil, &disabled); err != nil {
		t.Fatalf("UpdateUser status: %v", err)
	}
	got, _ := repo.GetUserByID(context.Background(), u.ID)
	if got.Status != StatusDisabled {
		t.Fatalf("expected status disabled, got %s", got.Status)
	}
}

func TestUpdateUserRoleAndStatusCombined(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "both@example.com", "A", "password123")

	admin := RoleAdmin
	active := StatusActive
	if err := svc.UpdateUser(context.Background(), u.TenantID, "admin", u.ID, &admin, &active); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got, _ := repo.GetUserByID(context.Background(), u.ID)
	if got.Role != RoleAdmin || got.Status != StatusActive {
		t.Fatalf("expected admin/active, got %s/%s", got.Role, got.Status)
	}
}

func TestUpdateUserCannotChangeOwnRoleOrStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "self2@example.com", "A", "password123")

	admin := RoleAdmin
	if err := svc.UpdateUser(context.Background(), u.TenantID, u.ID, u.ID, &admin, nil); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for self-change, got %v", err)
	}
}

func TestUpdateUserNotFoundInTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, _, _ := svc.Register(context.Background(), "miss2@example.com", "A", "password123")

	admin := RoleAdmin
	err := svc.UpdateUser(context.Background(), "t-other", "admin", u.ID, &admin, nil)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for foreign tenant, got %v", err)
	}
}

func TestDisableUserRevokesSessions(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	u, token, _ := svc.Register(context.Background(), "blk@example.com", "A", "password123")
	if _, ok := repo.sessions[HashToken(token)]; !ok {
		t.Fatal("expected active session")
	}

	disabled := StatusDisabled
	if err := svc.UpdateUser(context.Background(), u.TenantID, "admin", u.ID, nil, &disabled); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatal("expected all sessions revoked after disable")
	}
}

func TestPolicyDefaults(t *testing.T) {
	if got := DefaultPolicy(); got.MinPasswordLength != 8 || got.SessionTTLSeconds != 86400 || got.LoginRateLimitPerMin != 10 {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	partial := Policy{MinPasswordLength: 12}
	filled := partial.WithDefaults()
	if filled.MinPasswordLength != 12 || filled.SessionTTLSeconds != 86400 {
		t.Fatalf("WithDefaults: %+v", filled)
	}
}

func TestPolicyValidate(t *testing.T) {
	if err := (Policy{MinPasswordLength: 8, SessionTTLSeconds: 300, LoginRateLimitPerMin: 1}).Validate(); err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}
	if err := (Policy{MinPasswordLength: 5, SessionTTLSeconds: 300, LoginRateLimitPerMin: 1}).Validate(); err == nil {
		t.Fatal("min_password_length < 8 must be invalid")
	}
	if err := (Policy{MinPasswordLength: 8, SessionTTLSeconds: 100, LoginRateLimitPerMin: 1}).Validate(); err == nil {
		t.Fatal("session_ttl < 300 must be invalid")
	}
	if err := (Policy{MinPasswordLength: 8, SessionTTLSeconds: 300, LoginRateLimitPerMin: 0}).Validate(); err == nil {
		t.Fatal("login_rate_limit < 1 must be invalid")
	}
}

func TestRegisterEnforcesTenantPolicy(t *testing.T) {
	repo := newFakeRepo()
	repo.policy = &Policy{MinPasswordLength: 12, RequireNumber: true, RequireUpper: true,
		SessionTTLSeconds: 3600, LoginRateLimitPerMin: 5}
	svc := NewService(repo, time.Hour)

	if _, _, err := svc.Register(context.Background(), "weak@example.com", "A", "tooshort"); err != ErrWeakPassword {
		t.Fatalf("expected ErrWeakPassword for too-short, got %v", err)
	}
	if _, _, err := svc.Register(context.Background(), "noup@example.com", "A", "alllowercasenumber"); err != ErrWeakPassword {
		t.Fatalf("expected ErrWeakPassword for missing upper, got %v", err)
	}
	if _, _, err := svc.Register(context.Background(), "nodig@example.com", "A", "NoDigitsHere"); err != ErrWeakPassword {
		t.Fatalf("expected ErrWeakPassword for missing digit, got %v", err)
	}
	_, token, err := svc.Register(context.Background(), "ok@example.com", "A", "Str0ngPass12")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// TTL сессии — из политики (3600s), а не из конструктора.
	if s, ok := repo.sessions[HashToken(token)]; ok {
		remaining := s.ExpiresAt.Sub(time.Now().UTC())
		if remaining < 50*time.Minute || remaining > time.Hour {
			t.Fatalf("session TTL ~ %v, want ~1h", remaining)
		}
	}
}

func TestUpdatePolicy(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)

	p := Policy{MinPasswordLength: 12, SessionTTLSeconds: 7200, LoginRateLimitPerMin: 20}
	if err := svc.UpdatePolicy(context.Background(), "t-1", "admin", p); err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}
	got, err := svc.GetPolicy(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if got.MinPasswordLength != 12 || got.SessionTTLSeconds != 7200 {
		t.Fatalf("policy mismatch: %+v", got)
	}
}

func TestUpdatePolicyInvalid(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	p := Policy{MinPasswordLength: 4, SessionTTLSeconds: 300, LoginRateLimitPerMin: 1}
	if err := svc.UpdatePolicy(context.Background(), "t-1", "admin", p); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("expected ErrInvalidPolicy, got %v", err)
	}
}

func TestGetPolicyDefaultsWhenAbsent(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	p, err := svc.GetPolicy(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if p != DefaultPolicy() {
		t.Fatalf("expected defaults, got %+v", p)
	}
}

func TestCreateApiKeyReturnsTokenOnce(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)

	key, token, err := svc.CreateApiKey(context.Background(), "t-1", "u-1", "CI", []Permission{PermissionUsersList})
	if err != nil {
		t.Fatalf("CreateApiKey: %v", err)
	}
	if key == nil || token == "" {
		t.Fatal("expected key and token")
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}
	if key.TokenHash == token {
		t.Fatal("token must be stored as hash, not plaintext")
	}
	if key.TokenHash != HashToken(token) {
		t.Fatal("stored hash mismatch")
	}
	if len(key.Scopes) != 1 || key.Scopes[0] != string(PermissionUsersList) {
		t.Fatalf("scopes = %v", key.Scopes)
	}
}

func TestAuthenticateApiKey(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	_, token, _ := svc.CreateApiKey(context.Background(), "t-1", "u-1", "CI", []Permission{PermissionUsersList})

	key, err := svc.AuthenticateApiKey(context.Background(), token)
	if err != nil {
		t.Fatalf("AuthenticateApiKey: %v", err)
	}
	if key.TenantID != "t-1" || !key.HasScope(PermissionUsersList) {
		t.Fatalf("unexpected key: %+v", key)
	}
	if key.LastUsedAt == nil {
		t.Fatal("expected last_used_at touch")
	}
}

func TestAuthenticateApiKeyInvalid(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if _, err := svc.AuthenticateApiKey(context.Background(), "bogus"); err != ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired for unknown token, got %v", err)
	}
	if _, err := svc.AuthenticateApiKey(context.Background(), ""); err != ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired for empty token, got %v", err)
	}
}

func TestRevokeApiKey(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	_, token, _ := svc.CreateApiKey(context.Background(), "t-1", "u-1", "CI", []Permission{PermissionUsersList})

	if err := svc.RevokeApiKey(context.Background(), "t-1", "u-1", repo.apiKeys[0].ID); err != nil {
		t.Fatalf("RevokeApiKey: %v", err)
	}
	if _, err := svc.AuthenticateApiKey(context.Background(), token); err != ErrKeyRevoked {
		t.Fatalf("expected ErrKeyRevoked after revoke, got %v", err)
	}
}

func TestRevokeApiKeyNotFound(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if err := svc.RevokeApiKey(context.Background(), "t-1", "u-1", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
