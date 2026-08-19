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
	ErrNotFound         = errors.New("auth: not found")
	ErrEmailExists      = errors.New("auth: email already registered")
	ErrInvalidEmail     = errors.New("auth: invalid email")
	ErrWeakPassword     = errors.New("auth: password too weak (min 8 chars)")
	ErrInvalidCreds     = errors.New("auth: invalid credentials")
	ErrUserDisabled     = errors.New("auth: user disabled")
	ErrSessionExpired   = errors.New("auth: session expired or invalid")
	ErrForbidden        = errors.New("auth: forbidden")
	ErrUnknownRole      = errors.New("auth: unknown role")
	ErrInvalidPolicy    = errors.New("auth: invalid policy")
	ErrInvalidStatus    = errors.New("auth: invalid status")
	ErrKeyRevoked       = errors.New("auth: api key revoked")
	ErrOAuthExists      = errors.New("auth: oauth account already linked")
	ErrSsoState         = errors.New("auth: sso state invalid or expired")
	ErrSsoDenied        = errors.New("auth: sso login denied")
	ErrSsoNotConfigured = errors.New("auth: sso not configured")
)

// Service — прикладной сервис auth (BE-0002 Use Cases). Не зависит от
// транспорта и БД (инверсия зависимостей).
type Service struct {
	repo       Repository
	sessionTTL time.Duration
	audit      *audit.Service
	sso        OIDCProvider
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

// sessionTTLFor возвращает TTL сессии по политике tenant (EDR-0016 §3.2).
// При отсутствии настроек — дефолт конструктора (sessionTTL).
func (s *Service) sessionTTLFor(ctx context.Context, tenantID string) time.Duration {
	if p, err := s.repo.GetPolicy(ctx, tenantID); err == nil {
		return time.Duration(p.SessionTTLSeconds) * time.Second
	}
	return s.sessionTTL
}

// passwordValid проверяет пароль по политике tenant (EDR-0016 §3.2):
// длина и, если задано, наличие цифры/заглавной буквы.
func (s *Service) passwordValid(ctx context.Context, tenantID, password string) bool {
	p, err := s.repo.GetPolicy(ctx, tenantID)
	if err != nil {
		p = DefaultPolicy()
	}
	if len(password) < p.MinPasswordLength {
		return false
	}
	if p.RequireNumber {
		hasDigit := false
		for i := 0; i < len(password); i++ {
			if password[i] >= '0' && password[i] <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			return false
		}
	}
	if p.RequireUpper {
		hasUpper := false
		for i := 0; i < len(password); i++ {
			if password[i] >= 'A' && password[i] <= 'Z' {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return false
		}
	}
	return true
}

// DefaultTenant возвращает дефолтный tenant (slug "default"). Используется
// публичными маршрутами без аутентификации (регистрация, консультации store).
func (s *Service) DefaultTenant(ctx context.Context) (*Tenant, error) {
	tenant, err := s.repo.DefaultTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: default tenant: %w", err)
	}
	return tenant, nil
}

// Register создаёт пользователя в дефолтном tenant (SEC-0005): для MVP
// регистрация всегда помещает пользователя в единственный tenant.
// Роль — user (SEC-0004: админ создаётся только через БД/seed).
// Сразу создаёт сессию (авто-вход): возвращает user и session-токен.
func (s *Service) Register(ctx context.Context, email, name, password string) (*User, string, error) {
	if !emailPattern.MatchString(email) {
		return nil, "", ErrInvalidEmail
	}
	tenant, err := s.repo.DefaultTenant(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("auth: default tenant: %w", err)
	}
	if !s.passwordValid(ctx, tenant.ID, password) {
		return nil, "", ErrWeakPassword
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
		ExpiresAt: time.Now().UTC().Add(s.sessionTTLFor(ctx, tenant.ID)),
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
		ExpiresAt: time.Now().UTC().Add(s.sessionTTLFor(ctx, u.TenantID)),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, "", fmt.Errorf("auth: create session: %w", err)
	}
	s.record(ctx, u.ID, u.TenantID, audit.ActionAuthLogin, audit.ResultOK, "login ok")
	return u, token, nil
}

// Authenticate проверяет session-токен: хеширует, ищет сессию, проверяет
// срок действия и статус пользователя. Возвращает пользователя и, при
// ротации сессии (EDR-0014 §3.1), новый session-токен (пустая строка —
// ротации не было). Транспорт обязан при ротации выставить новый cookie
// и использовать новый токен вместо старого.
func (s *Service) Authenticate(ctx context.Context, token string) (*User, string, error) {
	if token == "" {
		return nil, "", ErrSessionExpired
	}
	sess, err := s.repo.GetSessionByTokenHash(ctx, HashToken(token))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, "", ErrSessionExpired
		}
		return nil, "", err
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.repo.DeleteSessionByTokenHash(ctx, sess.TokenHash)
		return nil, "", ErrSessionExpired
	}
	u, err := s.repo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, "", err
	}
	if u.Status != StatusActive {
		return nil, "", ErrUserDisabled
	}
	// Ротация: если сессия старше половины TTL, выдаём новый токен и
	// удаляем старую сессию (защита от session fixation, EDR-0014 §3.1).
	if time.Now().UTC().After(sess.CreatedAt.Add(s.sessionTTL / 2)) {
		newToken, err := newToken()
		if err != nil {
			return nil, "", err
		}
		rotated := &Session{
			UserID:    u.ID,
			TokenHash: HashToken(newToken),
			ExpiresAt: time.Now().UTC().Add(s.sessionTTLFor(ctx, u.TenantID)),
		}
		if err := s.repo.CreateSession(ctx, rotated); err != nil {
			return nil, "", fmt.Errorf("auth: rotate session: %w", err)
		}
		_ = s.repo.DeleteSessionByTokenHash(ctx, sess.TokenHash)
		return u, newToken, nil
	}
	return u, "", nil
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

// ListUsers возвращает пользователей tenant (EDR-0015 §3.4). Требуется
// право users.list (проверка в транспорте).
func (s *Service) ListUsers(ctx context.Context, tenantID string) ([]*User, error) {
	return s.repo.ListUsers(ctx, tenantID)
}

// UpdateUserRole меняет роль пользователя tenant (EDR-0015 §3.4).
// Требуется право users.update_role (проверка в транспорте); роль — только
// user|admin. Запрещено менять собственную роль (инвариант админа).
func (s *Service) UpdateUserRole(ctx context.Context, tenantID, actorID, userID string, role Role) error {
	if actorID == userID {
		s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied, "change own role")
		return ErrForbidden
	}
	if err := s.repo.UpdateUserRole(ctx, tenantID, userID, role); err != nil {
		if errors.Is(err, ErrNotFound) {
			s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied, "user not found in tenant")
		}
		return err
	}
	s.record(ctx, actorID, tenantID, audit.ActionUserRoleChanged, audit.ResultOK, "user role changed to "+string(role))
	return nil
}

// ParseStatus нормализует статус из строки DTO; ErrInvalidStatus — невалидна.
func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusActive, StatusDisabled:
		return Status(s), nil
	default:
		return "", ErrInvalidStatus
	}
}

// UpdateUser меняет роль и/или статус пользователя tenant (EDR-0016 §3.1).
// Требуется право users.manage (проверка в транспорте); роль — только
// user|admin, статус — только active|disabled. Запрещено менять
// собственные роль/статус (инвариант админа). Блокировка немедленно
// ревокает активные сессии пользователя.
func (s *Service) UpdateUser(ctx context.Context, tenantID, actorID, userID string, role *Role, status *Status) error {
	if actorID == userID {
		s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied, "change own role/status")
		return ErrForbidden
	}
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied, "user not found in tenant")
		}
		return err
	}
	if u.TenantID != tenantID {
		s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied, "user not found in tenant")
		return ErrNotFound
	}
	if role != nil {
		if err := s.repo.UpdateUserRole(ctx, tenantID, userID, *role); err != nil {
			return err
		}
		s.record(ctx, actorID, tenantID, audit.ActionUserRoleChanged, audit.ResultOK, "user role changed to "+string(*role))
	}
	if status != nil {
		if err := s.repo.UpdateUserStatus(ctx, tenantID, userID, *status); err != nil {
			return err
		}
		if *status == StatusDisabled {
			_ = s.repo.DeleteUserSessions(ctx, userID)
		}
		s.record(ctx, actorID, tenantID, audit.ActionUserStatusChanged, audit.ResultOK, "user status changed to "+string(*status))
	}
	return nil
}

// GetPolicy возвращает политику безопасности tenant (EDR-0016 §3.2).
// Требуется право settings.read (проверка в транспорте).
func (s *Service) GetPolicy(ctx context.Context, tenantID string) (Policy, error) {
	p, err := s.repo.GetPolicy(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return DefaultPolicy(), nil
		}
		return Policy{}, err
	}
	return p.WithDefaults(), nil
}

// UpdatePolicy сохраняет политику безопасности tenant (EDR-0016 §3.2).
// Требуется право settings.write (проверка в транспорте).
func (s *Service) UpdatePolicy(ctx context.Context, tenantID, actorID string, p Policy) error {
	p = p.WithDefaults()
	if err := p.Validate(); err != nil {
		return err
	}
	if err := s.repo.UpdatePolicy(ctx, tenantID, p); err != nil {
		return err
	}
	s.record(ctx, actorID, tenantID, audit.ActionSettingsUpdated, audit.ResultOK, "security policy updated")
	return nil
}

// CreateApiKey создаёт API-ключ (EDR-0016 §3.3). Требуется право
// api_keys.manage (проверка в транспорте). Открытый токен возвращается
// один раз; в БД хранится только его SHA-256 хеш.
func (s *Service) CreateApiKey(ctx context.Context, tenantID, actorID, name string, scopes []Permission) (*ApiKey, string, error) {
	token, err := newToken()
	if err != nil {
		return nil, "", err
	}
	key := &ApiKey{
		TenantID:  tenantID,
		Name:      name,
		TokenHash: HashToken(token),
		CreatedBy: actorID,
	}
	for _, sc := range scopes {
		key.Scopes = append(key.Scopes, string(sc))
	}
	if err := s.repo.CreateApiKey(ctx, key); err != nil {
		return nil, "", err
	}
	s.record(ctx, actorID, tenantID, audit.ActionApiKeyCreated, audit.ResultOK, "api key created: "+name)
	return key, token, nil
}

// ListApiKeys возвращает ключи tenant (EDR-0016 §3.3). Требуется право
// api_keys.manage (проверка в транспорте).
func (s *Service) ListApiKeys(ctx context.Context, tenantID string) ([]*ApiKey, error) {
	return s.repo.ListApiKeys(ctx, tenantID)
}

// RevokeApiKey отзывает ключ (мягко, мгновенная инвалидация). Требуется
// право api_keys.manage (проверка в транспорте).
func (s *Service) RevokeApiKey(ctx context.Context, tenantID, actorID, keyID string) error {
	if err := s.repo.RevokeApiKey(ctx, tenantID, keyID); err != nil {
		if errors.Is(err, ErrNotFound) {
			s.record(ctx, actorID, tenantID, audit.ActionAuthzDenied, audit.ResultDenied, "revoke: key not found")
		}
		return err
	}
	s.record(ctx, actorID, tenantID, audit.ActionApiKeyRevoked, audit.ResultOK, "api key revoked")
	return nil
}

// AuthenticateApiKey проверяет service-токен: хеширует, ищет ключ,
// проверяет отзыв. Возвращает ключ (без token_hash). Используется
// транспортом для Bearer-аутентификации (EDR-0016 §7).
func (s *Service) AuthenticateApiKey(ctx context.Context, token string) (*ApiKey, error) {
	if token == "" {
		return nil, ErrSessionExpired
	}
	key, err := s.repo.GetApiKeyByTokenHash(ctx, HashToken(token))
	if err != nil {
		return nil, ErrSessionExpired
	}
	if !key.Active() {
		return nil, ErrKeyRevoked
	}
	_ = s.repo.TouchApiKey(ctx, key.ID)
	return key, nil
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
