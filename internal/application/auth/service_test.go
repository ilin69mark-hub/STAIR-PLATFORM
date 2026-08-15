package auth

import (
	"context"
	"testing"
	"time"
)

// fakeRepo — тестовая реализация Repository в памяти.
type fakeRepo struct {
	users    map[string]*User // by email
	byID     map[string]*User
	sessions map[string]*Session // by token hash
	tenant   *Tenant
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

	if _, _, err := svc.Login(context.Background(), "off@example.com", "password123"); err != ErrUserDisabled {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
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
