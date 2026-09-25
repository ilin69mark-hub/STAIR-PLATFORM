package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"stairplatform/internal/application/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StoreRepository — настройки магазина и прайс материалов (миграция 000028).
type StoreRepository struct {
	pool *pgxpool.Pool
}

var _ store.Repository = (*StoreRepository)(nil)

// NewStoreRepository создаёт репозиторий настроек магазина.
func NewStoreRepository(pool *pgxpool.Pool) *StoreRepository {
	return &StoreRepository{pool: pool}
}

// GetSettings возвращает настройки tenant; store.ErrNotFound — если их нет
// (магазин работает на дефолтах).
func (r *StoreRepository) GetSettings(ctx context.Context, tenantID string) (store.Settings, error) {
	var raw []byte
	var updatedBy *string
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT settings, COALESCE(updated_by::text, ''), updated_at
		   FROM store_settings WHERE tenant_id = $1`,
		tenantID,
	).Scan(&raw, &updatedBy, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Settings{}, store.ErrNotFound
	}
	if err != nil {
		return store.Settings{}, fmt.Errorf("store: get settings: %w", err)
	}
	var s store.Settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return store.Settings{}, fmt.Errorf("store: unmarshal settings: %w", err)
		}
	}
	if updatedBy != nil {
		s.UpdatedBy = *updatedBy
	}
	// updated_at из колонки — источник истины: JSON-тело может нести часы
	// клиента записи, а показывать админке надо фактическое время БД.
	s.UpdatedAt = updatedAt
	return s, nil
}

// SaveSettings сохраняет настройки магазина (upsert).
func (r *StoreRepository) SaveSettings(ctx context.Context, tenantID string, s store.Settings, updatedBy string) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("store: marshal settings: %w", err)
	}
	var by any
	if updatedBy != "" {
		by = updatedBy
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO store_settings (tenant_id, settings, updated_by, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (tenant_id) DO UPDATE
		   SET settings = EXCLUDED.settings,
		       updated_by = EXCLUDED.updated_by,
		       updated_at = now()`,
		tenantID, raw, by); err != nil {
		return fmt.Errorf("store: save settings: %w", err)
	}
	return nil
}

// ListMaterialPrices возвращает цены, заданные магазином.
func (r *StoreRepository) ListMaterialPrices(ctx context.Context, tenantID string) ([]store.MaterialPrice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT material_code, price_per_kg_rub FROM store_rates
		 WHERE tenant_id = $1 ORDER BY material_code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list material prices: %w", err)
	}
	defer rows.Close()
	var out []store.MaterialPrice
	for rows.Next() {
		var p store.MaterialPrice
		if err := rows.Scan(&p.Code, &p.PricePerKgRub); err != nil {
			return nil, fmt.Errorf("store: scan material price: %w", err)
		}
		p.Overridden = true
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list material prices rows: %w", err)
	}
	return out, nil
}

// SetMaterialPrice сохраняет цену материала магазина (upsert).
func (r *StoreRepository) SetMaterialPrice(ctx context.Context, tenantID, code string, price int64, updatedBy string) error {
	var by any
	if updatedBy != "" {
		by = updatedBy
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO store_rates (tenant_id, material_code, price_per_kg_rub, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, material_code) DO UPDATE
		   SET price_per_kg_rub = EXCLUDED.price_per_kg_rub,
		       updated_by = EXCLUDED.updated_by,
		       updated_at = now()`,
		tenantID, code, price, by); err != nil {
		return fmt.Errorf("store: set material price: %w", err)
	}
	return nil
}

// DeleteMaterialPrice возвращает материал к встроенной ставке движка.
func (r *StoreRepository) DeleteMaterialPrice(ctx context.Context, tenantID, code string) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM store_rates WHERE tenant_id = $1 AND material_code = $2`, tenantID, code); err != nil {
		return fmt.Errorf("store: delete material price: %w", err)
	}
	return nil
}
