package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/analytics"
)

// AnalyticsRepository — реализация analytics.Repository на PostgreSQL
// (BE-0005). Агрегации read-only по операционным таблицам (EDR-0028 §3.3);
// tenant-скоуп через tenant_id/проекты (SEC-0005).
type AnalyticsRepository struct {
	pool *pgxpool.Pool
}

// NewAnalyticsRepository создаёт репозиторий аналитики поверх пула.
func NewAnalyticsRepository(pool *pgxpool.Pool) *AnalyticsRepository {
	return &AnalyticsRepository{pool: pool}
}

var _ analytics.Repository = (*AnalyticsRepository)(nil)

// UsageTotals возвращает итоговые счётчики tenant за окно [from, to]
// (EDR-0028 §3.3): набор COUNT-подзапросов в одной строке.
func (r *AnalyticsRepository) UsageTotals(ctx context.Context, tenantID string, from, to time.Time) (analytics.UsageTotals, error) {
	var t analytics.UsageTotals
	err := r.pool.QueryRow(ctx,
		`SELECT
			(SELECT COUNT(*) FROM users WHERE tenant_id = $1) AS users,
			(SELECT COUNT(DISTINCT actor_id) FROM audit_events
			  WHERE tenant_id = $1 AND action = 'auth.login' AND result = 'ok'
			    AND created_at BETWEEN $2 AND $3) AS active_users,
			(SELECT COUNT(*) FROM projects WHERE tenant_id = $1) AS projects,
			(SELECT COUNT(*) FROM calculations c JOIN projects p ON p.id = c.project_id
			  WHERE p.tenant_id = $1 AND c.created_at BETWEEN $2 AND $3) AS calculations,
			(SELECT COUNT(*) FROM audit_events
			  WHERE tenant_id = $1 AND action = 'auth.login' AND result = 'ok'
			    AND created_at BETWEEN $2 AND $3) AS logins,
			(SELECT COUNT(*) FROM audit_events
			  WHERE tenant_id = $1 AND action = 'data.exported'
			    AND created_at BETWEEN $2 AND $3) AS exports,
			(SELECT COUNT(*) FROM payment_intents
			  WHERE tenant_id = $1 AND status = 'paid'
			    AND created_at BETWEEN $2 AND $3) AS payments`,
		tenantID, from, to,
	).Scan(&t.Users, &t.ActiveUsers, &t.Projects, &t.Calculations, &t.Logins, &t.Exports, &t.Payments)
	if err != nil {
		return analytics.UsageTotals{}, fmt.Errorf("analytics: usage totals: %w", err)
	}
	return t, nil
}

// UsageSeries возвращает непрерывный ряд счётчиков по бакетам гранулярности
// g (EDR-0028 §3.3). Генератор buckets даёт непрерывность (пустые бакеты
// заполняются нулями через LEFT JOIN + COALESCE).
func (r *AnalyticsRepository) UsageSeries(ctx context.Context, tenantID string, from, to time.Time, g analytics.Granularity) ([]analytics.UsagePoint, error) {
	rows, err := r.pool.Query(ctx,
		`WITH buckets AS (
			SELECT generate_series(
				date_trunc($2, $3::timestamptz),
				date_trunc($2, $4::timestamptz),
				$5::interval
			) AS bucket
		),
		logins AS (
			SELECT date_trunc($2, created_at) AS bucket, COUNT(*) AS n
			FROM audit_events
			WHERE tenant_id = $1 AND action = 'auth.login' AND result = 'ok'
			  AND created_at BETWEEN $3 AND $4
			GROUP BY 1
		),
		active AS (
			SELECT date_trunc($2, created_at) AS bucket, COUNT(DISTINCT actor_id) AS n
			FROM audit_events
			WHERE tenant_id = $1 AND action = 'auth.login' AND result = 'ok'
			  AND created_at BETWEEN $3 AND $4
			GROUP BY 1
		),
		created_projects AS (
			SELECT date_trunc($2, created_at) AS bucket, COUNT(*) AS n
			FROM projects
			WHERE tenant_id = $1 AND created_at BETWEEN $3 AND $4
			GROUP BY 1
		),
		calcs AS (
			SELECT date_trunc($2, c.created_at) AS bucket, COUNT(*) AS n
			FROM calculations c JOIN public.projects p ON p.id = c.project_id
			WHERE p.tenant_id = $1 AND c.created_at BETWEEN $3 AND $4
			GROUP BY 1
		),
		exports AS (
			SELECT date_trunc($2, created_at) AS bucket, COUNT(*) AS n
			FROM audit_events
			WHERE tenant_id = $1 AND action = 'data.exported'
			  AND created_at BETWEEN $3 AND $4
			GROUP BY 1
		),
		payments AS (
			SELECT date_trunc($2, created_at) AS bucket, COUNT(*) AS n
			FROM payment_intents
			WHERE tenant_id = $1 AND status = 'paid'
			  AND created_at BETWEEN $3 AND $4
			GROUP BY 1
		)
		SELECT b.bucket,
			COALESCE(login.n, 0), COALESCE(act.n, 0), COALESCE(proj.n, 0),
			COALESCE(calc.n, 0), COALESCE(exp.n, 0), COALESCE(pay.n, 0)
		FROM buckets b
		LEFT JOIN logins          login ON login.bucket = b.bucket
		LEFT JOIN active          act   ON act.bucket   = b.bucket
		LEFT JOIN created_projects proj ON proj.bucket  = b.bucket
		LEFT JOIN calcs           calc ON calc.bucket  = b.bucket
		LEFT JOIN exports         exp  ON exp.bucket   = b.bucket
		LEFT JOIN payments        pay  ON pay.bucket   = b.bucket
		ORDER BY b.bucket`,
		tenantID, string(g), from, to, g.Interval())
	if err != nil {
		return nil, fmt.Errorf("analytics: usage series: %w", err)
	}
	defer rows.Close()

	out := make([]analytics.UsagePoint, 0)
	for rows.Next() {
		var p analytics.UsagePoint
		if err := rows.Scan(&p.Bucket, &p.Logins, &p.ActiveUsers, &p.ProjectsCreated,
			&p.Calculations, &p.Exports, &p.Payments); err != nil {
			return nil, fmt.Errorf("analytics: usage series scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: usage series rows: %w", err)
	}
	return out, nil
}

// ProjectTotals возвращает агрегаты по проектам tenant за окно [from, to]
// (EDR-0029 §3.3): подзапросы в одной строке + распределение по статусам.
func (r *AnalyticsRepository) ProjectTotals(ctx context.Context, tenantID string, from, to time.Time) (analytics.ProjectTotals, error) {
	var t analytics.ProjectTotals
	err := r.pool.QueryRow(ctx,
		`SELECT
			(SELECT COUNT(*) FROM projects WHERE tenant_id = $1) AS projects,
			(SELECT COUNT(*) FROM projects WHERE tenant_id = $1 AND created_at BETWEEN $2 AND $3) AS created,
			(SELECT COUNT(DISTINCT p.id) FROM projects p JOIN calculations c ON c.project_id = p.id
			  WHERE p.tenant_id = $1) AS with_calc,
			(SELECT COUNT(*) FROM (
			  SELECT DISTINCT ON (c.project_id) c.project_id, c.valid
			  FROM calculations c JOIN projects p ON p.id = c.project_id
			  WHERE p.tenant_id = $1
			  ORDER BY c.project_id, c.created_at DESC
			) latest WHERE latest.valid) AS valid_projects,
			(SELECT COUNT(*) FROM stair_configurations sc JOIN projects p ON p.id = sc.project_id
			  WHERE p.tenant_id = $1) AS configurations,
			(SELECT COUNT(*) FROM calculations c JOIN projects p ON p.id = c.project_id
			  WHERE p.tenant_id = $1) AS calculations,
			(SELECT COUNT(*) FROM project_comments pc JOIN projects p ON p.id = pc.project_id
			  WHERE p.tenant_id = $1) AS comments`,
		tenantID, from, to,
	).Scan(&t.Projects, &t.ProjectsCreated, &t.ProjectsWithCalculation,
		&t.ValidProjects, &t.Configurations, &t.Calculations, &t.Comments)
	if err != nil {
		return analytics.ProjectTotals{}, fmt.Errorf("analytics: project totals: %w", err)
	}

	t.ByStatus = make(map[string]int)
	statusRows, err := r.pool.Query(ctx,
		`SELECT status, COUNT(*) FROM projects WHERE tenant_id = $1 GROUP BY status`,
		tenantID)
	if err != nil {
		return analytics.ProjectTotals{}, fmt.Errorf("analytics: project totals by_status: %w", err)
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var st string
		var n int
		if err := statusRows.Scan(&st, &n); err != nil {
			return analytics.ProjectTotals{}, fmt.Errorf("analytics: project totals by_status scan: %w", err)
		}
		t.ByStatus[st] = n
	}
	if err := statusRows.Err(); err != nil {
		return analytics.ProjectTotals{}, fmt.Errorf("analytics: project totals by_status rows: %w", err)
	}
	return t, nil
}

// ProjectList возвращает сводку по каждому проекту tenant (EDR-0029 §3.3).
// Агрегаты — подзапросы на строку; сортировка по updated_at DESC.
func (r *AnalyticsRepository) ProjectList(ctx context.Context, tenantID string) ([]analytics.ProjectRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.name, p.status, COALESCE(u.email, ''),
			p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM stair_configurations sc WHERE sc.project_id = p.id) AS configs,
			(SELECT COUNT(*) FROM calculations c WHERE c.project_id = p.id) AS calcs,
			(SELECT c.valid FROM calculations c WHERE c.project_id = p.id
			  ORDER BY c.created_at DESC LIMIT 1) AS latest_valid,
			(SELECT COUNT(*) FROM project_comments pc WHERE pc.project_id = p.id) AS comments,
			(SELECT COUNT(*) FROM project_members pm WHERE pm.project_id = p.id) AS members
		FROM projects p
		LEFT JOIN users u ON u.id = p.owner_id
		WHERE p.tenant_id = $1
		ORDER BY p.updated_at DESC`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("analytics: project list: %w", err)
	}
	defer rows.Close()

	out := make([]analytics.ProjectRow, 0)
	for rows.Next() {
		var r analytics.ProjectRow
		var latestValid *bool
		if err := rows.Scan(&r.ID, &r.Name, &r.Status, &r.OwnerEmail,
			&r.CreatedAt, &r.UpdatedAt, &r.Configurations, &r.Calculations,
			&latestValid, &r.Comments, &r.Members); err != nil {
			return nil, fmt.Errorf("analytics: project list scan: %w", err)
		}
		r.LatestCalculationValid = latestValid
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: project list rows: %w", err)
	}
	return out, nil
}
