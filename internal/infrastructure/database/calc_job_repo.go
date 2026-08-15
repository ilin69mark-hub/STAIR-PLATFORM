package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/stair"
)

// CalcJobRepository — реализация порта jobs.Repository на PostgreSQL
// (EDR-0035 §3.2) поверх пула pgx. Хранит статус, входные данные
// (payload JSONB) и результат (result JSONB) фонового расчёта в calc_jobs.
type CalcJobRepository struct {
	pool *pgxpool.Pool
}

// NewCalcJobRepository создаёт репозиторий фоновых заданий.
func NewCalcJobRepository(pool *pgxpool.Pool) *CalcJobRepository {
	return &CalcJobRepository{pool: pool}
}

var _ jobs.Repository = (*CalcJobRepository)(nil)

const calcJobCols = `id, tenant_id, user_id, type, status, payload, result, error, created_at, started_at, finished_at`

func scanCalcJob(row pgx.Row) (*jobs.Job, error) {
	var j jobs.Job
	var userID *string
	var resultB, payloadB []byte
	var errorText *string
	var startedAt, finishedAt *time.Time
	if err := row.Scan(&j.ID, &j.TenantID, &userID, &j.Type, &j.Status, &payloadB, &resultB,
		&errorText, &j.CreatedAt, &startedAt, &finishedAt); err != nil {
		return nil, err
	}
	if userID != nil {
		j.UserID = *userID
	}
	if err := json.Unmarshal(payloadB, &j.Payload); err != nil {
		return nil, fmt.Errorf("calc_jobs: decode payload: %w", err)
	}
	if len(resultB) > 0 {
		var res stair.Result
		if err := json.Unmarshal(resultB, &res); err != nil {
			return nil, fmt.Errorf("calc_jobs: decode result: %w", err)
		}
		j.Result = &res
	}
	if errorText != nil {
		j.Error = *errorText
	}
	if startedAt != nil {
		j.StartedAt = *startedAt
	}
	if finishedAt != nil {
		j.FinishedAt = *finishedAt
	}
	return &j, nil
}

// Create сохраняет новое задание (status pending, EDR-0035 инвариант 1).
func (r *CalcJobRepository) Create(ctx context.Context, j *jobs.Job) error {
	payload, err := json.Marshal(j.Payload)
	if err != nil {
		return fmt.Errorf("calc_jobs: marshal payload: %w", err)
	}
	var userID any
	if j.UserID != "" {
		userID = j.UserID
	}
	err = r.pool.QueryRow(ctx,
		`INSERT INTO calc_jobs (id, tenant_id, user_id, type, status, payload)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at`,
		j.ID, j.TenantID, userID, j.Type, string(j.Status), payload,
	).Scan(&j.CreatedAt)
	if err != nil {
		return fmt.Errorf("calc_jobs: create: %w", err)
	}
	return nil
}

// GetByID возвращает задание в скоупе tenant'а; jobs.ErrNotFound — нет
// записи или чужая tenant'ом (инвариант 3).
func (r *CalcJobRepository) GetByID(ctx context.Context, tenantID, id string) (*jobs.Job, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+calcJobCols+` FROM calc_jobs WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	j, err := scanCalcJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, jobs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("calc_jobs: get %s: %w", id, err)
	}
	return j, nil
}

// MarkRunning фиксирует «взято в работу» воркером.
func (r *CalcJobRepository) MarkRunning(ctx context.Context, tenantID, id string) error {
	return r.updateStatus(ctx, tenantID, id,
		"UPDATE calc_jobs SET status = 'running', started_at = now(), finished_at = NULL WHERE id = $1 AND tenant_id = $2",
		id, tenantID)
}

// MarkSucceeded сохраняет результат (инвариант 2: succeeded ⇒ result).
func (r *CalcJobRepository) MarkSucceeded(ctx context.Context, tenantID, id string, res *stair.Result) error {
	resultB, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("calc_jobs: marshal result: %w", err)
	}
	return r.updateStatus(ctx, tenantID, id,
		"UPDATE calc_jobs SET status = 'succeeded', result = $3, error = NULL, finished_at = now() WHERE id = $1 AND tenant_id = $2",
		id, tenantID, resultB)
}

// MarkFailed сохраняет текст ошибки (инвариант 2: failed ⇒ error).
func (r *CalcJobRepository) MarkFailed(ctx context.Context, tenantID, id, errMsg string) error {
	return r.updateStatus(ctx, tenantID, id,
		"UPDATE calc_jobs SET status = 'failed', error = $3, finished_at = now() WHERE id = $1 AND tenant_id = $2",
		id, tenantID, errMsg)
}

// updateStatus исполняет UPDATE-статус и проверяет, что запись
// существовала в скоупе tenant'а.
func (r *CalcJobRepository) updateStatus(ctx context.Context, tenantID, id, query string, args ...any) error {
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("calc_jobs: update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return jobs.ErrNotFound
	}
	return nil
}
