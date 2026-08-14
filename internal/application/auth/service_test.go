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

func (f *fakeRepo) CreateSession(ctx context.Context, s *Session) error {
	s.ID = "s-" + s.TokenHash
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

	got, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
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
	if _, err := svc.Authenticate(context.Background(), token); err != ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestAuthenticateUnknownToken(t *testing.T) {
	svc := NewService(newFakeRepo(), 0)
	if _, err := svc.Authenticate(context.Background(), "no-such-token"); err != ErrSessionExpired {
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
	if _, err := svc.Authenticate(context.Background(), token); err != ErrSessionExpired {
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
