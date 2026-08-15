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
