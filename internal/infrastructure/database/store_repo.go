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

// ---- Прайс крепежа (MNT-0001) ----
//
// Отдельные методы рядом с материальными, а не обобщение «прайс по коду»:
// единица у них разная (₽/кг против ₽/шт), и таблицы разные. Общий метод
// означал бы колонку «цена» с плавающим смыслом.

// ListFastenerPrices возвращает цены крепежа, заданные магазином (₽ за штуку).
func (r *StoreRepository) ListFastenerPrices(ctx context.Context, tenantID string) ([]store.FastenerPrice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT fastener_code, price_per_pc_rub FROM store_fastener_prices
		 WHERE tenant_id = $1 ORDER BY fastener_code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list fastener prices: %w", err)
	}
	defer rows.Close()
	var out []store.FastenerPrice
	for rows.Next() {
		var p store.FastenerPrice
		if err := rows.Scan(&p.Code, &p.PricePerPcRub); err != nil {
			return nil, fmt.Errorf("store: scan fastener price: %w", err)
		}
		p.Overridden = true
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list fastener prices rows: %w", err)
	}
	return out, nil
}

// SetFastenerPrice сохраняет цену крепежа магазина (upsert).
func (r *StoreRepository) SetFastenerPrice(ctx context.Context, tenantID, code string, price int64, updatedBy string) error {
	var by any
	if updatedBy != "" {
		by = updatedBy
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO store_fastener_prices (tenant_id, fastener_code, price_per_pc_rub, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, fastener_code) DO UPDATE
		   SET price_per_pc_rub = EXCLUDED.price_per_pc_rub,
		       updated_by = EXCLUDED.updated_by,
		       updated_at = now()`,
		tenantID, code, price, by); err != nil {
		return fmt.Errorf("store: set fastener price: %w", err)
	}
	return nil
}

// DeleteFastenerPrice возвращает крепёж к встроенной ставке.
func (r *StoreRepository) DeleteFastenerPrice(ctx context.Context, tenantID, code string) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM store_fastener_prices WHERE tenant_id = $1 AND fastener_code = $2`,
		tenantID, code); err != nil {
		return fmt.Errorf("store: delete fastener price: %w", err)
	}
	return nil
}

// ---- Свои позиции крепежа (000048) ----

// ListFastenerPositions возвращает позиции крепежа, добавленные магазином.
// Встроенный каталог сюда не входит: он один на всех и живёт в коде.
func (r *StoreRepository) ListFastenerPositions(ctx context.Context, tenantID string) ([]store.FastenerPosition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT fastener_code, name_ru, kind, diameter_mm, length_mm, base
		   FROM store_fastener_positions
		  WHERE tenant_id = $1 ORDER BY fastener_code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list fastener positions: %w", err)
	}
	defer rows.Close()
	var out []store.FastenerPosition
	for rows.Next() {
		var p store.FastenerPosition
		if err := rows.Scan(&p.Code, &p.NameRu, &p.Kind, &p.DiameterMM, &p.LengthMM, &p.Base); err != nil {
			return nil, fmt.Errorf("store: scan fastener position: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list fastener positions rows: %w", err)
	}
	return out, nil
}

// CreateFastenerPosition добавляет позицию крепежа и её цену.
//
// Цена пишется в store_fastener_prices — туда же, где цена встроенной
// позиции. Отдельной таблицы цен для своих позиций нет намеренно: цена
// позиции должна жить в одном месте независимо от того, встроенная она или
// добавлена.
//
// Обе записи идут в одной транзакции. Позиция без цены попала бы в смету по
// нулю — ровно тот дефект, который S-179 закрывал у ограждения.
func (r *StoreRepository) CreateFastenerPosition(ctx context.Context, tenantID string,
	p store.FastenerPosition, pricePerPcRub int64, updatedBy string) error {
	var by any
	if updatedBy != "" {
		by = updatedBy
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin fastener position: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO store_fastener_positions
		   (tenant_id, fastener_code, name_ru, kind, diameter_mm, length_mm, base,
		    created_by, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
		tenantID, p.Code, p.NameRu, p.Kind, p.DiameterMM, p.LengthMM, p.Base, by); err != nil {
		return fmt.Errorf("store: create fastener position: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO store_fastener_prices (tenant_id, fastener_code, price_per_pc_rub, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, fastener_code) DO UPDATE
		   SET price_per_pc_rub = EXCLUDED.price_per_pc_rub,
		       updated_by = EXCLUDED.updated_by,
		       updated_at = now()`,
		tenantID, p.Code, pricePerPcRub, by); err != nil {
		return fmt.Errorf("store: set fastener position price: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit fastener position: %w", err)
	}
	return nil
}

// DeleteFastenerPosition удаляет свою позицию и её цену.
//
// Цена удаляется вместе с позицией, а не остаётся сиротой: иначе код мог бы
// снова занять ту же строку прайса, и новая позиция молча получила бы цену
// удалённой.
func (r *StoreRepository) DeleteFastenerPosition(ctx context.Context, tenantID, code string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin delete fastener position: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM store_fastener_positions WHERE tenant_id = $1 AND fastener_code = $2`,
		tenantID, code); err != nil {
		return fmt.Errorf("store: delete fastener position: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM store_fastener_prices WHERE tenant_id = $1 AND fastener_code = $2`,
		tenantID, code); err != nil {
		return fmt.Errorf("store: delete fastener position price: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit delete fastener position: %w", err)
	}
	return nil
}

// ---- Прайс герметика (CONF-SEALANT) ----
//
// Методы рядом с крепёжными, а не обобщение «прайс по коду»: единица одна
// (₽/штука), но ТАБЛИЦЫ разные и смысл цены разный (дюбель/глухарь против
// картриджа герметика). Общий метод с флагом вида заставил бы вызывающий
// помнить, что именно он правит.

// ListSealantPrices возвращает цены герметика, заданные магазином (₽ за картридж).
func (r *StoreRepository) ListSealantPrices(ctx context.Context, tenantID string) ([]store.SealantPrice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT sealant_code, price_per_cartridge_rub FROM store_sealant_prices
		 WHERE tenant_id = $1 ORDER BY sealant_code`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list sealant prices: %w", err)
	}
	defer rows.Close()
	var out []store.SealantPrice
	for rows.Next() {
		var p store.SealantPrice
		if err := rows.Scan(&p.Code, &p.PricePerCartridgeRub); err != nil {
			return nil, fmt.Errorf("store: scan sealant price: %w", err)
		}
		p.Overridden = true
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list sealant prices rows: %w", err)
	}
	return out, nil
}

// SetSealantPrice сохраняет цену герметика магазина (upsert).
func (r *StoreRepository) SetSealantPrice(ctx context.Context, tenantID, code string, price int64, updatedBy string) error {
	var by any
	if updatedBy != "" {
		by = updatedBy
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO store_sealant_prices (tenant_id, sealant_code, price_per_cartridge_rub, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, sealant_code) DO UPDATE
		   SET price_per_cartridge_rub = EXCLUDED.price_per_cartridge_rub,
		       updated_by = EXCLUDED.updated_by,
		       updated_at = now()`,
		tenantID, code, price, by); err != nil {
		return fmt.Errorf("store: set sealant price: %w", err)
	}
	return nil
}

// DeleteSealantPrice возвращает герметик к встроенной ставке.
func (r *StoreRepository) DeleteSealantPrice(ctx context.Context, tenantID, code string) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM store_sealant_prices WHERE tenant_id = $1 AND sealant_code = $2`,
		tenantID, code); err != nil {
		return fmt.Errorf("store: delete sealant price: %w", err)
	}
	return nil
}

// ListPurchasedPrices возвращает закупочные цены покупных деталей ограждения
// (₽ за штуку по ВИДУ детали).
//
// Ключ — вид детали, а не код позиции: у каждого экземпляра спецификации свой
// номер (PST-01, PST-02, …), и прайс по нему построить нельзя.
func (r *StoreRepository) ListPurchasedPrices(ctx context.Context, tenantID string) ([]store.PurchasedPrice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT part_kind, price_per_pc_rub FROM store_purchased_prices
		 WHERE tenant_id = $1 ORDER BY part_kind`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list purchased prices: %w", err)
	}
	defer rows.Close()
	var out []store.PurchasedPrice
	for rows.Next() {
		var p store.PurchasedPrice
		if err := rows.Scan(&p.Kind, &p.PricePerPcRub); err != nil {
			return nil, fmt.Errorf("store: scan purchased price: %w", err)
		}
		p.Overridden = true
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list purchased prices rows: %w", err)
	}
	return out, nil
}

// SetPurchasedPrice сохраняет закупочную цену магазина (upsert).
func (r *StoreRepository) SetPurchasedPrice(ctx context.Context, tenantID, kind string, price int64, updatedBy string) error {
	var by any
	if updatedBy != "" {
		by = updatedBy
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO store_purchased_prices (tenant_id, part_kind, price_per_pc_rub, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (tenant_id, part_kind) DO UPDATE
		   SET price_per_pc_rub = EXCLUDED.price_per_pc_rub,
		       updated_by = EXCLUDED.updated_by,
		       updated_at = now()`,
		tenantID, kind, price, by); err != nil {
		return fmt.Errorf("store: set purchased price: %w", err)
	}
	return nil
}

// DeletePurchasedPrice возвращает деталь ограждения к встроенной ставке.
func (r *StoreRepository) DeletePurchasedPrice(ctx context.Context, tenantID, kind string) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM store_purchased_prices WHERE tenant_id = $1 AND part_kind = $2`,
		tenantID, kind); err != nil {
		return fmt.Errorf("store: delete purchased price: %w", err)
	}
	return nil
}
