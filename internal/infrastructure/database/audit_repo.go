package database

import (
	"context"
	"fmt"
	"time"

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
//
// SEC-002: перед SQL вызывается e.Validate() — репозиторий является вторым
// рубежом после allowlist в транспорте. Так неизвестное action/result или
// пустой tenant не попадут в журнал даже если новый вызывающий забудет
// про доменную проверку.
func (r *AuditRepository) Insert(ctx context.Context, e *audit.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	// AUDIT-002 (2026-09-27): страховка для прямых вызовов репозитория
	// (тесты, будущий код в обход Service.Record). Validate() теперь
	// допускает «исход не задан», а CHECK audit_events_result_check — нет,
	// поэтому без приведения пустой результат падал бы технической ошибкой
	// 23514 вместо записи события. Основная нормализация — в
	// Service.Record; эта повторяет её, чтобы репозиторий был безопасен сам
	// по себе.
	e.Result = e.Result.OrOK()
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
//
// DOM-006 (2026-09-26): добавлен жёсткий LIMIT. Раньше выборка шла без
// ограничения, то есть GET /audit возвращал весь журнал tenant'а одним
// ответом (и ResponseCache складывал такой body в кэш на 5 минут).
func (r *AuditRepository) ListByProject(ctx context.Context, tenantID, projectID string, limit int) ([]*audit.Event, error) {
	if limit <= 0 || limit > audit.MaxListLimit {
		limit = audit.MaxListLimit
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+auditCols+`
		 FROM audit_events
		 WHERE project_id = $1
		   AND project_id IN (SELECT id FROM projects WHERE id = $1 AND tenant_id = $2)
		 ORDER BY created_at DESC
		 LIMIT $3`,
		projectID, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("audit: list by project: %w", err)
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// ListByTenant возвращает события tenant (глобальный аудит, admin).
// DOM-006: жёсткий LIMIT — журнал tenant'а растёт неограниченно.
func (r *AuditRepository) ListByTenant(ctx context.Context, tenantID string, limit int) ([]*audit.Event, error) {
	if limit <= 0 || limit > audit.MaxListLimit {
		limit = audit.MaxListLimit
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+auditCols+`
		 FROM audit_events
		 WHERE tenant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("audit: list by tenant: %w", err)
	}
	defer rows.Close()
	return scanAuditEvents(rows)
}

// DeleteBefore удаляет аудит-события старше before (retention, EDR-0020
// §3.5) и возвращает число удалённых.
func (r *AuditRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM audit_events WHERE created_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("audit: delete before: %w", err)
	}
	return tag.RowsAffected(), nil
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
