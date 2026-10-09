// Репозиторий точечных выдач прав (EDR-0041 §5).
//
// Отдельный файл от AuthRepository: выдачи — самостоятельная сущность с
// собственными правилами, и смешивать её с входом/сессиями незачем.
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"stairplatform/internal/application/auth"
)

// ListGrants возвращает набор выдач сотрудника. Отсутствие строк — законное
// состояние («не дал ничего»), а не ошибка.
func (r *AuthRepository) ListGrants(ctx context.Context, userID string) (*auth.GrantSet, error) {
	if !isUUID(userID) {
		return auth.NewGrantSet(), nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT capability, level FROM user_grants WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: list grants: %w", err)
	}
	defer rows.Close()

	pairs := []auth.Grant{}
	for rows.Next() {
		var c auth.Capability
		var l auth.Granted
		if err := rows.Scan(&c, &l); err != nil {
			return nil, fmt.Errorf("auth: scan grant: %w", err)
		}
		pairs = append(pairs, auth.Grant{Capability: c, Level: l})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: list grants rows: %w", err)
	}
	s := auth.NewGrantSet(pairs...)
	return s, nil
}

// ReplaceGrants атомарно заменяет набор выдач и взводит grants_initialized.
//
// Замена целиком, а не дописывание: иначе снять выдачу было бы невозможно —
// строку надо было бы удалять, а это другая операция с другим аудитом. Одна
// транзакция означает, что читатель никогда не увидит «наполовину
// выданное» состояние.
func (r *AuthRepository) ReplaceGrants(
	ctx context.Context, tenantID, userID string, s *auth.GrantSet, grantedBy string,
) error {
	if !isUUID(userID) {
		return auth.ErrNotFound
	}
	pairs := s.Pairs()

	return WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var owner bool
		// tenant_id проверяем здесь: смена набора не должна работать для
		// сотрудника чужого tenant (SEC-0005).
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (
			   SELECT 1 FROM users
			   WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			 )`, userID, tenantID).Scan(&owner); err != nil {
			return fmt.Errorf("auth: grants owner check: %w", err)
		}
		if !owner {
			return auth.ErrNotFound
		}

		if _, err := tx.Exec(ctx,
			`DELETE FROM user_grants WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("auth: clear grants: %w", err)
		}
		for _, g := range pairs {
			if g.Level == auth.GrantedNone {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO user_grants (user_id, capability, level, granted_by)
				 VALUES ($1, $2, $3, $4)`,
				userID, g.Capability, string(g.Level), nullIfEmptyUUID(grantedBy)); err != nil {
				return fmt.Errorf("auth: insert grant %s: %w", g.Capability, err)
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET grants_initialized = TRUE, updated_at = now() WHERE id = $1`,
			userID); err != nil {
			return fmt.Errorf("auth: set grants_initialized: %w", err)
		}
		return nil
	})
}

// SetGrantsInitialized снимает или ставит флаг «права заданы вручную».
// Возврат к роли — осознанное действие, поэтому метод не трогает сами строки.
func (r *AuthRepository) SetGrantsInitialized(ctx context.Context, userID string, initialized bool) error {
	if !isUUID(userID) {
		return auth.ErrNotFound
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET grants_initialized = $2, updated_at = now() WHERE id = $1`,
		userID, initialized)
	if err != nil {
		return fmt.Errorf("auth: set grants_initialized: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

// CountActiveOwners считает неудалённых владельцев tenant. Ноль означает,
// что снимать права с последнего владельца нельзя: иначе восстановить доступ
// к выдаче прав будет уже некому.
func (r *AuthRepository) CountActiveOwners(ctx context.Context, tenantID string) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM users
		 WHERE tenant_id = $1 AND role = 'owner' AND deleted_at IS NULL`, tenantID).Scan(&n); err != nil {
		return 0, fmt.Errorf("auth: count owners: %w", err)
	}
	return n, nil
}

// nullIfEmptyUUID: granted_by — uuid, а выдающим может быть системный
// вызов без конкретного пользователя. Пустую строку в uuid нельзя.
func nullIfEmptyUUID(s string) any {
	if s == "" || !isUUID(s) {
		return nil
	}
	return s
}

// ensure the grants repository satisfies the domain port.
var _ auth.GrantsRepository = (*AuthRepository)(nil)
