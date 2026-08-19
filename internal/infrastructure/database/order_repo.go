package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/order"
)

// OrderRepository — реализация порта order.Repository на PostgreSQL
// (клиентские заказы, Store). contact хранится JSONB.
type OrderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository создаёт репозиторий заказов.
func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

// nullableBytes возвращает nil для пустого JSONB (консультации без цены).
func nullableBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}

var _ order.Repository = (*OrderRepository)(nil)

// scanOrder читает строку заказа; project_id и user_id — nullable
// (консультации без пользователя), price_json тоже nullable.
func scanOrder(row pgx.Row) (*order.Order, error) {
	var o order.Order
	var userID *string
	var projectID *string
	var priceJSON []byte
	if err := row.Scan(&o.ID, &o.TenantID, &userID, &o.Kind, &o.Status, &o.Contact,
		&o.ConfigJSON, &priceJSON, &projectID, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	if userID != nil {
		o.UserID = *userID
	}
	if priceJSON != nil {
		o.PriceJSON = priceJSON
	}
	if projectID != nil {
		o.ProjectID = *projectID
	}
	return &o, nil
}

// Create сохраняет новый заказ.
func (r *OrderRepository) Create(ctx context.Context, o *order.Order) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO orders (tenant_id, user_id, kind, status, contact, config_json, price_json)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		o.TenantID, nullable(o.UserID), string(o.Kind), string(o.Status),
		o.Contact, o.ConfigJSON, nullableBytes(o.PriceJSON),
	).Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return fmt.Errorf("order: create: %w", err)
	}
	return nil
}

// Get возвращает заказ по ID в tenant.
func (r *OrderRepository) Get(ctx context.Context, tenantID, id string) (*order.Order, error) {
	o, err := scanOrder(r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, kind, status, contact, config_json, price_json, project_id, created_at, updated_at
		 FROM orders WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, order.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("order: get: %w", err)
	}
	return o, nil
}

// ListByUser возвращает заказы пользователя (убывание created_at).
func (r *OrderRepository) ListByUser(ctx context.Context, tenantID, userID string) ([]*order.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, kind, status, contact, config_json, price_json, project_id, created_at, updated_at
		 FROM orders WHERE tenant_id = $1 AND user_id = $2 ORDER BY created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("order: list by user: %w", err)
	}
	defer rows.Close()
	return r.scanOrders(rows)
}

// ListAll возвращает все заказы tenant (убывание created_at).
func (r *OrderRepository) ListAll(ctx context.Context, tenantID string) ([]*order.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, kind, status, contact, config_json, price_json, project_id, created_at, updated_at
		 FROM orders WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("order: list all: %w", err)
	}
	defer rows.Close()
	return r.scanOrders(rows)
}

func (r *OrderRepository) scanOrders(rows pgx.Rows) ([]*order.Order, error) {
	var out []*order.Order
	for rows.Next() {
		var o order.Order
		var userID *string
		var projectID *string
		var priceJSON []byte
		if err := rows.Scan(&o.ID, &o.TenantID, &userID, &o.Kind, &o.Status, &o.Contact,
			&o.ConfigJSON, &priceJSON, &projectID, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("order: scan: %w", err)
		}
		if userID != nil {
			o.UserID = *userID
		}
		if priceJSON != nil {
			o.PriceJSON = priceJSON
		}
		if projectID != nil {
			o.ProjectID = *projectID
		}
		out = append(out, &o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("order: rows: %w", err)
	}
	return out, nil
}

// UpdateStatus меняет статус заказа (tenant-скоуп).
func (r *OrderRepository) UpdateStatus(ctx context.Context, tenantID, id string, s order.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE orders SET status = $1, updated_at = now() WHERE tenant_id = $2 AND id = $3`,
		s, tenantID, id)
	if err != nil {
		return fmt.Errorf("order: update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return order.ErrNotFound
	}
	return nil
}