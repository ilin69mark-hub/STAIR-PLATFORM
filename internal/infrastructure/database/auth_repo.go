package database

import (
	"context"
	"errors"
	"fmt"

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
