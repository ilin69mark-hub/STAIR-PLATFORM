// Package auth реализует прикладной слой Identity/Authentication
// (SEC-0003) и базовую модель Authorization (SEC-0004): учётные записи,
// роли, серверные сессии с opaque-токенами, tenant-привязку пользователя
// (SEC-0005). Transport зависит от интерфейса Service; БД — от порта
// Repository (BE-0005, инверсия зависимостей DOM-0008).
package auth

import (
	"context"
	"fmt"
	"time"
)

// Role — роль пользователя (SEC-0004).
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Permission — именованное право на операцию (EDR-0015 §3.1). Системные
// права владеет роль пользователя (auth), проектные — роль участника
// (project package). Проверка — через Role.HasPermission.
type Permission string

const (
	// PermissionAuditReadAll — чтение глобального журнала аудита (EDR-0013).
	PermissionAuditReadAll Permission = "audit.read_all"
	// PermissionUsersList — просмотр пользователей tenant (EDR-0015 §3.4).
	PermissionUsersList Permission = "users.list"
	// PermissionUsersUpdateRole — смена роли пользователя (EDR-0015 §3.4).
	PermissionUsersUpdateRole Permission = "users.update_role"
	// PermissionUsersManage — смена роли и статуса (EDR-0016 §3.1).
	PermissionUsersManage Permission = "users.manage"
	// PermissionSettingsRead — чтение политик безопасности (EDR-0016).
	PermissionSettingsRead Permission = "settings.read"
	// PermissionSettingsWrite — изменение политик безопасности (EDR-0016).
	PermissionSettingsWrite Permission = "settings.write"
	// PermissionDataExport — экспорт данных tenant (EDR-0016).
	PermissionDataExport Permission = "data.export"
	// PermissionApiKeysManage — управление API-ключами (EDR-0016).
	PermissionApiKeysManage Permission = "api_keys.manage"
	// PermissionIntegrationsManage — управление интеграционными эндпоинтами
	// (EDR-0023 §3.5, Phase E).
	PermissionIntegrationsManage Permission = "integrations.manage"
)

// Permissions возвращает набор прав роли (матрица EDR-0015 §3.2,
// расширена EDR-0016 §3.1).
func (r Role) Permissions() []Permission {
	switch r {
	case RoleAdmin:
		return []Permission{
			PermissionAuditReadAll, PermissionUsersList, PermissionUsersUpdateRole,
			PermissionUsersManage, PermissionSettingsRead, PermissionSettingsWrite,
			PermissionDataExport, PermissionApiKeysManage, PermissionIntegrationsManage,
		}
	default:
		return nil
	}
}

// HasPermission возвращает true, если роль обладает правом p.
func (r Role) HasPermission(p Permission) bool {
	for _, perm := range r.Permissions() {
		if perm == p {
			return true
		}
	}
	return false
}

// ParseRole нормализует роль из строки DTO; ErrUnknownRole — невалидная.
func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleUser, RoleAdmin:
		return Role(s), nil
	default:
		return "", ErrUnknownRole
	}
}

// Status — состояние учётной записи (SEC-0003).
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Tenant — граница изоляции данных (SEC-0005).
type Tenant struct {
	ID        string
	Name      string
	Slug      string
	CreatedAt time.Time
}

// User — учётная запись (SEC-0003).
type User struct {
	ID           string
	TenantID     string
	Email        string
	Name         string
	PasswordHash string
	Role         Role
	Status       Status
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session — серверная сессия. TokenHash — SHA-256 от opaque-токена;
// открытый токен клиент держит только в cookie (httpOnly).
type Session struct {
	ID        string
	UserID    string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Policy — настраиваемые политики безопасности tenant (EDR-0016 §3.2).
// Хранится как JSONB в tenant_settings; нулевые значения означают дефолт.
type Policy struct {
	MinPasswordLength    int  `json:"min_password_length"`
	RequireNumber        bool `json:"require_number"`
	RequireUpper         bool `json:"require_upper"`
	SessionTTLSeconds    int  `json:"session_ttl_seconds"`
	LoginRateLimitPerMin int  `json:"login_rate_limit_per_min"`
}

// DefaultPolicy возвращает политику по умолчанию (совпадает с поведением
// MVP: min 8 символов, TTL 24ч, лимит входа 10/мин).
func DefaultPolicy() Policy {
	return Policy{
		MinPasswordLength:    8,
		RequireNumber:        false,
		RequireUpper:         false,
		SessionTTLSeconds:    86400,
		LoginRateLimitPerMin: 10,
	}
}

// WithDefaults заполняет нулевые поля значениями по умолчанию.
func (p Policy) WithDefaults() Policy {
	d := DefaultPolicy()
	if p.MinPasswordLength <= 0 {
		p.MinPasswordLength = d.MinPasswordLength
	}
	if p.SessionTTLSeconds <= 0 {
		p.SessionTTLSeconds = d.SessionTTLSeconds
	}
	if p.LoginRateLimitPerMin <= 0 {
		p.LoginRateLimitPerMin = d.LoginRateLimitPerMin
	}
	return p
}

// Validate проверяет диапазоны политики; ErrInvalidPolicy — невалидна.
func (p Policy) Validate() error {
	if p.MinPasswordLength < 8 || p.MinPasswordLength > 128 {
		return fmt.Errorf("%w: min_password_length must be in [8,128]", ErrInvalidPolicy)
	}
	if p.SessionTTLSeconds < 300 || p.SessionTTLSeconds > 86400 {
		return fmt.Errorf("%w: session_ttl_seconds must be in [300,86400]", ErrInvalidPolicy)
	}
	if p.LoginRateLimitPerMin < 1 || p.LoginRateLimitPerMin > 1000 {
		return fmt.Errorf("%w: login_rate_limit_per_min must be in [1,1000]", ErrInvalidPolicy)
	}
	return nil
}

// ApiKey — долгоживущий service-токен для интеграций (EDR-0016 §3.3).
// TokenHash — SHA-256 от opaque-токена; открытый токен отдаётся один раз
// при создании. RevokedAt — мягкий отзыв (мгновенная инвалидация).
type ApiKey struct {
	ID         string
	TenantID   string
	Name       string
	TokenHash  string
	Scopes     []string
	CreatedBy  string
	CreatedAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
}

// OAuthAccount — привязка внешнего identity (SSO, EDR-0017 §3.1).
// Уникальность (provider, subject): один внешний identity — одна учётная
// запись. Subject — `sub` claim IdP.
type OAuthAccount struct {
	ID        string
	Provider  string
	Subject   string
	UserID    string // владелец учётной записи (FK users)
	CreatedAt time.Time
}

// SsoState — одноразовое OIDC-состояние начала входа (EDR-0017 §3.3).
// StateHash — SHA-256 от state; хранится в БД до 10 минут (TTL), расходуется
// при колбэке.
type SsoState struct {
	ID           string
	StateHash    string
	Nonce        string
	PKCEVerifier string
	Redirect     string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// Active возвращает true, если ключ не отозван.
func (k *ApiKey) Active() bool {
	return k.RevokedAt == nil
}

// HasScope возвращает true, если ключ обладает правом scope.
func (k *ApiKey) HasScope(scope Permission) bool {
	for _, s := range k.Scopes {
		if Permission(s) == scope {
			return true
		}
	}
	return false
}

// Repository — порт доступа к данным auth (BE-0005).
type Repository interface {
	// DefaultTenant возвращает дефолтный tenant (slug "default").
	DefaultTenant(ctx context.Context) (*Tenant, error)

	// CreateUser создаёт пользователя; ErrEmailExists — email занят.
	CreateUser(ctx context.Context, u *User) error
	// GetUserByEmail возвращает пользователя по email; ErrNotFound — нет.
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	// GetUserByID возвращает пользователя по ID; ErrNotFound — нет.
	GetUserByID(ctx context.Context, id string) (*User, error)
	// ListUsers возвращает пользователей tenant (EDR-0015 §3.4).
	ListUsers(ctx context.Context, tenantID string) ([]*User, error)
	// UpdateUserRole меняет роль пользователя tenant; ErrNotFound — нет.
	UpdateUserRole(ctx context.Context, tenantID, userID string, role Role) error
	// UpdateUserStatus меняет статус пользователя tenant (EDR-0016 §3.1);
	// ErrNotFound — нет.
	UpdateUserStatus(ctx context.Context, tenantID, userID string, status Status) error

	// CreateSession сохраняет сессию (токен уже захэширован).
	CreateSession(ctx context.Context, s *Session) error
	// GetSessionByTokenHash возвращает сессию по хешу токена;
	// ErrNotFound — сессии нет.
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	// DeleteSessionByTokenHash удаляет сессию (лог-аут).
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
	// DeleteUserSessions удаляет все сессии пользователя (блокировка,
	// EDR-0016 §3.1; немедленная ревокация).
	DeleteUserSessions(ctx context.Context, userID string) error

	// GetPolicy возвращает политику безопасности tenant (EDR-0016 §3.2);
	// ErrNotFound — настройки отсутствуют (дефолтная политика).
	GetPolicy(ctx context.Context, tenantID string) (Policy, error)
	// UpdatePolicy сохраняет политику безопасности tenant (upsert).
	UpdatePolicy(ctx context.Context, tenantID string, p Policy) error

	// CreateApiKey сохраняет API-ключ (EDR-0016 §3.3).
	CreateApiKey(ctx context.Context, k *ApiKey) error
	// ListApiKeys возвращает ключи tenant (по убыванию created_at).
	ListApiKeys(ctx context.Context, tenantID string) ([]*ApiKey, error)
	// GetApiKeyByTokenHash возвращает ключ по хешу токена; ErrNotFound — нет.
	GetApiKeyByTokenHash(ctx context.Context, tokenHash string) (*ApiKey, error)
	// RevokeApiKey отзывает ключ (мягко: revoked_at); ErrNotFound — нет.
	RevokeApiKey(ctx context.Context, tenantID, keyID string) error
	// TouchApiKey обновляет last_used_at (использование ключа).
	TouchApiKey(ctx context.Context, keyID string) error

	// CreateOAuthAccount сохраняет привязку внешнего identity (EDR-0017);
	// ErrEmailExists-эквивалент ErrOAuthExists — (provider,subject) занят.
	CreateOAuthAccount(ctx context.Context, a *OAuthAccount) error
	// GetOAuthAccountByProviderSubject возвращает привязку по внешнему
	// identity; ErrNotFound — нет.
	GetOAuthAccountByProviderSubject(ctx context.Context, provider, subject string) (*OAuthAccount, error)
	// ListOAuthAccounts возвращает привязки пользователя.
	ListOAuthAccounts(ctx context.Context, userID string) ([]*OAuthAccount, error)

	// CreateSsoState сохраняет одноразовый OIDC-состояние (state hash,
	// nonce, PKCE verifier). ExpiresAt — TTL 10 минут.
	CreateSsoState(ctx context.Context, s *SsoState) error
	// ConsumeSsoState извлекает и удаляет состояние по хешу state
	// (одноразовое использование); ErrNotFound — нет/истекло.
	ConsumeSsoState(ctx context.Context, stateHash string) (*SsoState, error)
}
