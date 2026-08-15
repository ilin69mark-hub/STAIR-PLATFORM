package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/integrations"
)

// IntegrationRepository — реализация порта integrations.Repository на
// PostgreSQL (EDR-0023 §3.3) поверх пула pgx.
type IntegrationRepository struct {
	pool *pgxpool.Pool
}

// NewIntegrationRepository создаёт репозиторий интеграций.
func NewIntegrationRepository(pool *pgxpool.Pool) *IntegrationRepository {
	return &IntegrationRepository{pool: pool}
}

var _ integrations.Repository = (*IntegrationRepository)(nil)

// CreateEndpoint сохраняет новый webhook-эндпоинт.
func (r *IntegrationRepository) CreateEndpoint(ctx context.Context, e *integrations.Endpoint) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO integration_endpoints (tenant_id, name, kind, url, secret_enc)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		e.TenantID, e.Name, string(e.Kind), e.URL, e.SecretEnc,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("integrations: create endpoint: %w", err)
	}
	return nil
}

const endpointCols = `id, tenant_id, name, kind, url, secret_enc, created_at, updated_at`

func scanEndpoint(row pgx.Row) (*integrations.Endpoint, error) {
	var e integrations.Endpoint
	var kind string
	if err := row.Scan(&e.ID, &e.TenantID, &e.Name, &kind, &e.URL, &e.SecretEnc, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	e.Kind = integrations.Kind(kind)
	return &e, nil
}

// ListEndpoints возвращает эндпоинты tenant в порядке создания.
func (r *IntegrationRepository) ListEndpoints(ctx context.Context, tenantID string) ([]*integrations.Endpoint, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+endpointCols+` FROM integration_endpoints WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("integrations: list endpoints: %w", err)
	}
	defer rows.Close()
	var out []*integrations.Endpoint
	for rows.Next() {
		e, err := scanEndpoint(rows)
		if err != nil {
			return nil, fmt.Errorf("integrations: scan endpoint: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("integrations: list endpoints rows: %w", err)
	}
	return out, nil
}

// GetEndpoint возвращает эндпоинт по ID внутри tenant.
func (r *IntegrationRepository) GetEndpoint(ctx context.Context, tenantID, id string) (*integrations.Endpoint, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+endpointCols+` FROM integration_endpoints WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	e, err := scanEndpoint(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, integrations.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("integrations: get endpoint: %w", err)
	}
	return e, nil
}

// DeleteEndpoint удаляет эндпоинт (каскадно события).
func (r *IntegrationRepository) DeleteEndpoint(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM integration_endpoints WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("integrations: delete endpoint: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return integrations.ErrNotFound
	}
	return nil
}

// FindEndpointByKind возвращает единственный активный эндпоинт kind.
func (r *IntegrationRepository) FindEndpointByKind(ctx context.Context, tenantID string, kind integrations.Kind) (*integrations.Endpoint, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+endpointCols+` FROM integration_endpoints
		 WHERE tenant_id = $1 AND kind = $2
		 ORDER BY created_at LIMIT 1`, tenantID, string(kind))
	e, err := scanEndpoint(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, integrations.ErrNoEndpoint
	}
	if err != nil {
		return nil, fmt.Errorf("integrations: find endpoint by kind: %w", err)
	}
	return e, nil
}

var deliveryCols = `id, tenant_id, endpoint_id, project_id, event_type, payload, status, attempts, last_error, created_at, delivered_at`

func scanDelivery(row pgx.Row) (*integrations.Delivery, error) {
	var d integrations.Delivery
	var status string
	var projectID *string
	var lastErr *string
	if err := row.Scan(&d.ID, &d.TenantID, &d.EndpointID, &projectID, &d.EventType, &d.Payload,
		&status, &d.Attempts, &lastErr, &d.CreatedAt, &d.DeliveredAt); err != nil {
		return nil, err
	}
	if projectID != nil {
		d.ProjectID = *projectID
	}
	d.Status = integrations.Status(status)
	if lastErr != nil {
		d.LastError = *lastErr
	}
	return &d, nil
}

// CreateDelivery создаёт событие доставки со статусом pending.
func (r *IntegrationRepository) CreateDelivery(ctx context.Context, d *integrations.Delivery) error {
	var projectID any
	if d.ProjectID != "" {
		projectID = d.ProjectID
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO integration_events (tenant_id, endpoint_id, project_id, event_type, payload, status)
		 VALUES ($1, $2, $3, $4, $5, 'pending')
		 RETURNING id, created_at`,
		d.TenantID, d.EndpointID, projectID, d.EventType, d.Payload,
	).Scan(&d.ID, &d.CreatedAt)
	if err != nil {
		return fmt.Errorf("integrations: create delivery: %w", err)
	}
	d.Status = integrations.StatusPending
	return nil
}

// UpdateDeliveryStatus обновляет статус события доставки.
func (r *IntegrationRepository) UpdateDeliveryStatus(ctx context.Context, tenantID, id string, s integrations.Status, attempts int, lastErr string, deliveredAt *time.Time) error {
	var le any
	if lastErr != "" {
		le = lastErr
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE integration_events
		 SET status = $1, attempts = $2, last_error = $3, delivered_at = $4
		 WHERE tenant_id = $5 AND id = $6`,
		string(s), attempts, le, deliveredAt, tenantID, id)
	if err != nil {
		return fmt.Errorf("integrations: update delivery: %w", err)
	}
	return nil
}

// GetDelivery возвращает событие доставки.
func (r *IntegrationRepository) GetDelivery(ctx context.Context, tenantID, id string) (*integrations.Delivery, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+deliveryCols+` FROM integration_events WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	d, err := scanDelivery(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, integrations.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("integrations: get delivery: %w", err)
	}
	return d, nil
}
