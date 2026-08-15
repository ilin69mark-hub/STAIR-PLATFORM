package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/auth"
)

// AuthRepository — реализация auth.Repository на PostgreSQL (BE-0005).
type AuthRepository struct {
	pool *pgxpool.Pool
}

// NewAuthRepository создаёт репозиторий auth поверх пула.
func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

var _ auth.Repository = (*AuthRepository)(nil)

// DefaultTenant возвращает дефолтный tenant (slug "default").
func (r *AuthRepository) DefaultTenant(ctx context.Context) (*auth.Tenant, error) {
	var t auth.Tenant
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, created_at FROM tenants WHERE slug = 'default'`,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: default tenant: %w", err)
	}
	return &t, nil
}

func (r *AuthRepository) CreateUser(ctx context.Context, u *auth.User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (tenant_id, email, name, password_hash, role, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		u.TenantID, u.Email, u.Name, u.PasswordHash, u.Role, u.Status,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return auth.ErrEmailExists
		}
		return fmt.Errorf("auth: create user: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	u, err := r.scanUser(r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, email, name, password_hash, role, status, created_at, updated_at
		 FROM users WHERE email = $1`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get user by email: %w", err)
	}
	return u, nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, id string) (*auth.User, error) {
	u, err := r.scanUser(r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, email, name, password_hash, role, status, created_at, updated_at
		 FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get user by id: %w", err)
	}
	return u, nil
}

func (r *AuthRepository) scanUser(row pgx.Row) (*auth.User, error) {
	var u auth.User
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.PasswordHash,
		&u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsers возвращает пользователей tenant (EDR-0015 §3.4).
func (r *AuthRepository) ListUsers(ctx context.Context, tenantID string) ([]*auth.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, email, name, password_hash, role, status, created_at, updated_at
		 FROM users WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("auth: list users: %w", err)
	}
	defer rows.Close()
	var out []*auth.User
	for rows.Next() {
		var u auth.User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.PasswordHash,
			&u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("auth: scan user: %w", err)
		}
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: list users rows: %w", err)
	}
	return out, nil
}

// UpdateUserRole меняет роль пользователя tenant (EDR-0015 §3.4).
// ErrNotFound — пользователь не найден в tenant.
func (r *AuthRepository) UpdateUserRole(ctx context.Context, tenantID, userID string, role auth.Role) error {
	if !isUUID(userID) {
		return auth.ErrNotFound
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET role = $1, updated_at = now()
		 WHERE id = $2 AND tenant_id = $3`, role, userID, tenantID)
	if err != nil {
		return fmt.Errorf("auth: update user role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

// UpdateUserStatus меняет статус пользователя tenant (EDR-0016 §3.1).
// ErrNotFound — пользователь не найден в tenant.
func (r *AuthRepository) UpdateUserStatus(ctx context.Context, tenantID, userID string, status auth.Status) error {
	if !isUUID(userID) {
		return auth.ErrNotFound
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET status = $1, updated_at = now()
		 WHERE id = $2 AND tenant_id = $3`, status, userID, tenantID)
	if err != nil {
		return fmt.Errorf("auth: update user status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

// DeleteUserSessions удаляет все сессии пользователя (блокировка, EDR-0016).
func (r *AuthRepository) DeleteUserSessions(ctx context.Context, userID string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("auth: delete user sessions: %w", err)
	}
	return nil
}

func (r *AuthRepository) CreateSession(ctx context.Context, s *auth.Session) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		s.UserID, s.TokenHash, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("auth: create session: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*auth.Session, error) {
	var s auth.Session
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, created_at, expires_at
		 FROM sessions WHERE token_hash = $1`, tokenHash,
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.CreatedAt, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get session: %w", err)
	}
	return &s, nil
}

func (r *AuthRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}

// ---- политики безопасности (EDR-0016 §3.2) ----

// GetPolicy возвращает политику tenant; ErrNotFound — настройки отсутствуют.
func (r *AuthRepository) GetPolicy(ctx context.Context, tenantID string) (auth.Policy, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT policy FROM tenant_settings WHERE tenant_id = $1`, tenantID,
	).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.DefaultPolicy(), auth.ErrNotFound
	}
	if err != nil {
		return auth.Policy{}, fmt.Errorf("auth: get policy: %w", err)
	}
	var p auth.Policy
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return auth.Policy{}, fmt.Errorf("auth: unmarshal policy: %w", err)
		}
	}
	return p.WithDefaults(), nil
}

// UpdatePolicy сохраняет политику tenant (upsert, EDR-0016 §3.2).
func (r *AuthRepository) UpdatePolicy(ctx context.Context, tenantID string, p auth.Policy) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("auth: marshal policy: %w", err)
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO tenant_settings (tenant_id, policy, updated_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (tenant_id) DO UPDATE SET policy = EXCLUDED.policy, updated_at = now()`,
		tenantID, raw); err != nil {
		return fmt.Errorf("auth: update policy: %w", err)
	}
	return nil
}

// ---- API-ключи (EDR-0016 §3.3) ----

// CreateApiKey сохраняет API-ключ.
func (r *AuthRepository) CreateApiKey(ctx context.Context, k *auth.ApiKey) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO api_keys (tenant_id, name, token_hash, scopes, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		k.TenantID, k.Name, k.TokenHash, k.Scopes, nullable(k.CreatedBy),
	).Scan(&k.ID, &k.CreatedAt)
	if err != nil {
		return fmt.Errorf("auth: create api key: %w", err)
	}
	return nil
}

// ListApiKeys возвращает ключи tenant (по убыванию created_at).
func (r *AuthRepository) ListApiKeys(ctx context.Context, tenantID string) ([]*auth.ApiKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, name, token_hash, scopes, created_by, created_at, revoked_at, last_used_at
		 FROM api_keys WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("auth: list api keys: %w", err)
	}
	defer rows.Close()
	var out []*auth.ApiKey
	for rows.Next() {
		k, err := scanApiKey(rows)
		if err != nil {
			return nil, fmt.Errorf("auth: scan api key: %w", err)
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: list api keys rows: %w", err)
	}
	return out, nil
}

// GetApiKeyByTokenHash возвращает ключ по хешу токена; ErrNotFound — нет.
func (r *AuthRepository) GetApiKeyByTokenHash(ctx context.Context, tokenHash string) (*auth.ApiKey, error) {
	k, err := scanApiKey(r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, token_hash, scopes, created_by, created_at, revoked_at, last_used_at
		 FROM api_keys WHERE token_hash = $1`, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get api key: %w", err)
	}
	return k, nil
}

// RevokeApiKey отзывает ключ (мягко: revoked_at); ErrNotFound — нет.
func (r *AuthRepository) RevokeApiKey(ctx context.Context, tenantID, keyID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL`,
		keyID, tenantID)
	if err != nil {
		return fmt.Errorf("auth: revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

// TouchApiKey обновляет last_used_at (использование ключа).
func (r *AuthRepository) TouchApiKey(ctx context.Context, keyID string) error {
	if _, err := r.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, keyID); err != nil {
		return fmt.Errorf("auth: touch api key: %w", err)
	}
	return nil
}

func scanApiKey(row pgx.Row) (*auth.ApiKey, error) {
	var k auth.ApiKey
	var createdBy *string
	var revokedAt *time.Time
	var lastUsedAt *time.Time
	if err := row.Scan(&k.ID, &k.TenantID, &k.Name, &k.TokenHash, &k.Scopes,
		&createdBy, &k.CreatedAt, &revokedAt, &lastUsedAt); err != nil {
		return nil, err
	}
	if createdBy != nil {
		k.CreatedBy = *createdBy
	}
	k.RevokedAt = revokedAt
	k.LastUsedAt = lastUsedAt
	return &k, nil
}

// ---- SSO / OAuthAccount (EDR-0017) ----

// CreateOAuthAccount сохраняет привязку внешнего identity (EDR-0017 §3.1).
// ErrOAuthExists — (provider, subject) уже привязан.
func (r *AuthRepository) CreateOAuthAccount(ctx context.Context, a *auth.OAuthAccount) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO oauth_accounts (provider, subject, user_id)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		a.Provider, a.Subject, a.UserID,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return auth.ErrOAuthExists
		}
		return fmt.Errorf("auth: create oauth account: %w", err)
	}
	return nil
}

// GetOAuthAccountByProviderSubject возвращает привязку по внешнему identity;
// ErrNotFound — нет.
func (r *AuthRepository) GetOAuthAccountByProviderSubject(ctx context.Context, provider, subject string) (*auth.OAuthAccount, error) {
	a, err := scanOAuthAccount(r.pool.QueryRow(ctx,
		`SELECT id, provider, subject, user_id, created_at
		 FROM oauth_accounts WHERE provider = $1 AND subject = $2`, provider, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get oauth account: %w", err)
	}
	return a, nil
}

// ListOAuthAccounts возвращает привязки пользователя.
func (r *AuthRepository) ListOAuthAccounts(ctx context.Context, userID string) ([]*auth.OAuthAccount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, provider, subject, user_id, created_at
		 FROM oauth_accounts WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: list oauth accounts: %w", err)
	}
	defer rows.Close()
	var out []*auth.OAuthAccount
	for rows.Next() {
		a, err := scanOAuthAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("auth: scan oauth account: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: list oauth accounts rows: %w", err)
	}
	return out, nil
}

// CreateSsoState сохраняет одноразовый OIDC-state (EDR-0017 §3.3).
func (r *AuthRepository) CreateSsoState(ctx context.Context, s *auth.SsoState) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO sso_states (state_hash, nonce, pkce_verifier, redirect, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		s.StateHash, s.Nonce, s.PKCEVerifier, s.Redirect, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("auth: create sso state: %w", err)
	}
	return nil
}

// ConsumeSsoState извлекает и удаляет одноразовый state; ErrNotFound — нет.
func (r *AuthRepository) ConsumeSsoState(ctx context.Context, stateHash string) (*auth.SsoState, error) {
	var s auth.SsoState
	err := r.pool.QueryRow(ctx,
		`DELETE FROM sso_states
		 WHERE state_hash = $1 AND expires_at > now()
		 RETURNING id, state_hash, nonce, pkce_verifier, redirect, created_at, expires_at`,
		stateHash,
	).Scan(&s.ID, &s.StateHash, &s.Nonce, &s.PKCEVerifier, &s.Redirect, &s.CreatedAt, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: consume sso state: %w", err)
	}
	return &s, nil
}

func scanOAuthAccount(row pgx.Row) (*auth.OAuthAccount, error) {
	var a auth.OAuthAccount
	if err := row.Scan(&a.ID, &a.Provider, &a.Subject, &a.UserID, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// uuidPattern — каноническая форма UUID (8-4-4-4-12 hex).
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID возвращает true, если s — валидный UUID. Используется перед
// сравнением id против UUID-колонки: невалидный ввод должен давать
// ErrNotFound, а не SQL-ошибку 22P02 (invalid input syntax for type uuid).
func isUUID(s string) bool {
	return uuidPattern.MatchString(s)
}
