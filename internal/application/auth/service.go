package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"stairplatform/internal/application/audit"
)

// Ошибки auth (SEC-0003).
var (
	ErrNotFound       = errors.New("auth: not found")
	ErrEmailExists    = errors.New("auth: email already registered")
	ErrInvalidEmail   = errors.New("auth: invalid email")
	ErrWeakPassword   = errors.New("auth: password too weak (min 8 chars)")
	ErrInvalidCreds   = errors.New("auth: invalid credentials")
	ErrUserDisabled   = errors.New("auth: user disabled")
	ErrSessionExpired = errors.New("auth: session expired or invalid")
	ErrForbidden      = errors.New("auth: forbidden")
	ErrUnknownRole    = errors.New("auth: unknown role")
)

// Service — прикладной сервис auth (BE-0002 Use Cases). Не зависит от
// транспорта и БД (инверсия зависимостей).
type Service struct {
	repo       Repository
	sessionTTL time.Duration
	audit      *audit.Service
}

// NewService создаёт сервис auth. sessionTTL — время жизни сессии
// (0 → дефолт 24h). audit — необязательный журнал событий (EDR-0013);
// nil — запись аудита отключена.
func NewService(repo Repository, sessionTTL time.Duration, auditSvc ...*audit.Service) *Service {
	if sessionTTL <= 0 {
		sessionTTL = 24 * time.Hour
	}
	s := &Service{repo: repo, sessionTTL: sessionTTL}
	if len(auditSvc) > 0 {
		s.audit = auditSvc[0]
	}
	return s
}

// record пишет событие аудита (best-effort, EDR-0013 §4.1). Ошибка
// журнала не ломает бизнес-операцию: только логируется.
func (s *Service) record(ctx context.Context, actorID, tenantID string, action audit.Action, result audit.Result, detail string) {
	if s.audit == nil {
		return
	}
	m := audit.MetaFrom(ctx)
	_ = s.audit.Record(ctx, &audit.Event{
		ActorID:   actorID,
		TenantID:  tenantID,
		Action:    action,
		Result:    result,
		Detail:    detail,
		RequestID: m.RequestID,
		IP:        m.IP,
	})
}

// EmailPattern — минимальная проверка формата email.
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Register создаёт пользователя в дефолтном tenant (SEC-0005): для MVP
// регистрация всегда помещает пользователя в единственный tenant.
// Роль — user (SEC-0004: админ создаётся только через БД/seed).
// Сразу создаёт сессию (авто-вход): возвращает user и session-токен.
func (s *Service) Register(ctx context.Context, email, name, password string) (*User, string, error) {
	if !emailPattern.MatchString(email) {
		return nil, "", ErrInvalidEmail
	}
	if len(password) < 8 {
		return nil, "", ErrWeakPassword
	}
	tenant, err := s.repo.DefaultTenant(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("auth: default tenant: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("auth: hash password: %w", err)
	}
	u := &User{
		TenantID:     tenant.ID,
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		Role:         RoleUser,
		Status:       StatusActive,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		if errors.Is(err, ErrEmailExists) {
			return nil, "", ErrEmailExists
		}
		return nil, "", fmt.Errorf("auth: create user: %w", err)
	}
	token, err := newToken()
	if err != nil {
		return nil, "", err
	}
	sess := &Session{
		UserID:    u.ID,
		TokenHash: HashToken(token),
		ExpiresAt: time.Now().UTC().Add(s.sessionTTL),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, "", fmt.Errorf("auth: create session: %w", err)
	}
	s.record(ctx, u.ID, u.TenantID, audit.ActionAuthRegister, audit.ResultOK, "user registered")
	return u, token, nil
}

// Login проверяет учётные данные и создаёт сессию. Возвращает пользователя
// и открытый session-токен (только здесь; в БД хранится его хеш).
func (s *Service) Login(ctx context.Context, email, password string) (*User, string, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.record(ctx, "", "", audit.ActionAuthLoginDenied, audit.ResultDenied, "unknown email: "+email)
			return nil, "", ErrInvalidCreds
		}
		return nil, "", err
	}
	if u.Status != StatusActive {
		s.record(ctx, u.ID, u.TenantID, audit.ActionAuthLoginDenied, audit.ResultDenied, "user disabled")
		return nil, "", ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.record(ctx, u.ID, u.TenantID, audit.ActionAuthLoginDenied, audit.ResultDenied, "invalid password")
		return nil, "", ErrInvalidCreds
	}
	token, err := newToken()
	if err != nil {
		return nil, "", err
	}
	sess := &Session{
		UserID:    u.ID,
		TokenHash: HashToken(token),
		ExpiresAt: time.Now().UTC().Add(s.sessionTTL),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, "", fmt.Errorf("auth: create session: %w", err)
	}
	s.record(ctx, u.ID, u.TenantID, audit.ActionAuthLogin, audit.ResultOK, "login ok")
	return u, token, nil
}

// Authenticate проверяет session-токен: хеширует, ищет сессию, проверяет
// срок действия и статус пользователя. Возвращает пользователя.
func (s *Service) Authenticate(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, ErrSessionExpired
	}
	sess, err := s.repo.GetSessionByTokenHash(ctx, HashToken(token))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrSessionExpired
		}
		return nil, err
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.repo.DeleteSessionByTokenHash(ctx, sess.TokenHash)
		return nil, ErrSessionExpired
	}
	u, err := s.repo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	if u.Status != StatusActive {
		return nil, ErrUserDisabled
	}
	return u, nil
}

// Logout удаляет сессию (мгновенная ревокация). Токен — хеш session-токена.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	hash := HashToken(token)
	sess, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err == nil {
		// Аудит выхода: actor — владелец сессии (EDR-0013).
		if u, uerr := s.repo.GetUserByID(ctx, sess.UserID); uerr == nil {
			s.record(ctx, u.ID, u.TenantID, audit.ActionAuthLogout, audit.ResultOK, "logout")
		}
	}
	return s.repo.DeleteSessionByTokenHash(ctx, hash)
}

// newToken генерирует opaque-токен: 32 случайных байта в hex (64 символа).
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken возвращает SHA-256 (hex) от токена — то, что хранится в БД.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
