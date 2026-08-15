package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/audit"
)

// AuditRepository — реализация audit.Repository на PostgreSQL (BE-0005).
type AuditRepository struct {
	pool *pgxpool.Pool
}

// NewAuditRepository создаёт репозиторий аудита поверх пула.
func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

var _ audit.Repository = (*AuditRepository)(nil)

// auditCols — колонки аудит-события (порядок совпадает со scan).
const auditCols = "id, actor_id, tenant_id, project_id, action, resource_type, resource_id, result, detail, request_id, ip, created_at"

// Insert сохраняет событие аудита (append-only, EDR-0013).
func (r *AuditRepository) Insert(ctx context.Context, e *audit.Event) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO audit_events (actor_id, tenant_id, project_id, action, resource_type, resource_id, result, detail, request_id, ip)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, created_at`,
		nullable(e.ActorID), e.TenantID, nullable(e.ProjectID), e.Action,
		e.ResourceType, e.ResourceID, e.Result, e.Detail, e.RequestID, e.IP,
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

// ListByProject возвращает события проекта внутри tenant (SEC-0005):
// проект ищется с привязкой к tenant; новых — сверху.
func (r *AuditRepository) ListByProject(ctx context.Context, tenantID, projectID string) ([]*audit.Event, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+auditCols+`
		 FROM audit_events
		 WHERE project_id = $1
		   AND project_id IN (SELECT id FROM projects WHERE id = $1 AND tenant_id = $2)
		 ORDER BY created_at DESC`,
		projectID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("audit: list by project: %w", err)
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// ListByTenant возвращает события tenant (глобальный аудит, admin).
func (r *AuditRepository) ListByTenant(ctx context.Context, tenantID string) ([]*audit.Event, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+auditCols+`
		 FROM audit_events
		 WHERE tenant_id = $1
		 ORDER BY created_at DESC`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("audit: list by tenant: %w", err)
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

func scanAuditEvents(rows pgx.Rows) ([]*audit.Event, error) {
	var out []*audit.Event
	for rows.Next() {
		var e audit.Event
		var action, result string
		var actorID, projectID *string
		if err := rows.Scan(&e.ID, &actorID, &e.TenantID, &projectID, &action,
			&e.ResourceType, &e.ResourceID, &result, &e.Detail, &e.RequestID, &e.IP, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit: scan: %w", err)
		}
		e.Action = audit.Action(action)
		e.Result = audit.Result(result)
		if actorID != nil {
			e.ActorID = *actorID
		}
		if projectID != nil {
			e.ProjectID = *projectID
		}
		out = append(out, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit: rows: %w", err)
	}
	return out, nil
}

// nullable возвращает указатель на строку (для nullable-колонок).
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
