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
// GetByIDForUser возвращает задание в скоупе tenant'а И владельца.
//
// SEC-004 (2026-09-26): GET /api/v1/jobs/{id} был скоуплен только по
// tenant'у (GetByID), а auth.Service.Register всегда помещает пользователя в
// единственный дефолтный tenant — значит ЛЮБОЙ зарегистрированный
// пользователь читал результат чужого асинхронного расчёта (полный
// manufacturing-пакет, BOM, раскрой и цена).
//
// SEC-001 / DB-3 (forensic 2026-09-27): фикс SEC-004 ввёл
// `AND (user_id = $3 OR user_id IS NULL)`, и это оставило ДЫРУ — задание без
// владельца читает ЛЮБОЙ пользователь того же tenant'а. NULL возникает двумя
// путями, оба доказаны на живой БД:
//
//	(а) calc_job_repo.go Create: `var userID any; if j.UserID != ""` —
//	    Bearer-аутентификация даёт userID == "" (auth.go userID возвращает
//	    "" для API-ключа), и задание сохраняется с NULL;
//	(б) FK calc_jobs_user_id_fkey ON DELETE SET NULL — при удалении
//	    пользователя его задания осиротеют.
//
// То есть заявленная в комментарии «системная» видимость на практике была
// обходом owner-скоупа. Убрана: у задания, которым не владеет конкретный
// пользователь, нет и читателя среди пользователей. Системные/админские
// запуски обслуживаются GetByID (tenant-only), который вызывает воркер
// (jobs.RunCalculate) — у него пользователя нет по определению.
func (r *CalcJobRepository) GetByIDForUser(ctx context.Context, tenantID, userID, id string) (*jobs.Job, error) {
	// Без userID чтение невозможно в принципе: нечего предъявлять.
	if userID == "" {
		return nil, jobs.ErrNotFound
	}
	row := r.pool.QueryRow(ctx,
		`SELECT `+calcJobCols+` FROM calc_jobs
		 WHERE id = $1 AND tenant_id = $2 AND user_id = $3`,
		id, tenantID, userID)
	j, err := scanCalcJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, jobs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("calc_jobs: get %s for user: %w", id, err)
	}
	return j, nil
}

// GetByID — tenant-only чтение. Используется воркером (у него нет
// пользователя) и административными вызовами. НЕ является обходом
// owner-скоупа: воркеру задание передаёт очередь, а не HTTP-клиент.
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
//
// DB-3 (forensic 2026-09-27): захват обязан быть compare-and-swap. Раньше
// UPDATE шёл без условия по статусу, поэтому при at-least-once доставке из
// очереди ДВА воркера брали одно задание (воспроизведено: 4 из 4
// конкурентных вызовов вернули nil). Последствия — двойной расчёт
// стоимости и перетирание result последним писателем.
//
// Теперь переход разрешён ТОЛЬКО из pending. `running ⇒ result IS NULL`
// остаётся инвариантом: повторный захват не обнуляет уже записанный
// результат. jobs.ErrAlreadyClaimed — сигнал «кто-то уже взял», на который
// воркер реагирует пропуском, а не ретраем.
func (r *CalcJobRepository) MarkRunning(ctx context.Context, tenantID, id string) error {
	return r.updateStatusClaim(ctx,
		`UPDATE calc_jobs
		    SET status = 'running', started_at = now(), finished_at = NULL
		  WHERE id = $1 AND tenant_id = $2 AND status = 'pending'`,
		id, tenantID)
}

// MarkSucceeded сохраняет результат (инвариант 2: succeeded ⇒ result).
func (r *CalcJobRepository) MarkSucceeded(ctx context.Context, tenantID, id string, res *stair.Result) error {
	resultB, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("calc_jobs: marshal result: %w", err)
	}
	return r.updateStatus(ctx,
		"UPDATE calc_jobs SET status = 'succeeded', result = $3, error = NULL, finished_at = now() WHERE id = $1 AND tenant_id = $2",
		id, tenantID, resultB)
}

// MarkFailed сохраняет текст ошибки (инвариант 2: failed ⇒ error).
func (r *CalcJobRepository) MarkFailed(ctx context.Context, tenantID, id, errMsg string) error {
	return r.updateStatus(ctx,
		"UPDATE calc_jobs SET status = 'failed', error = $3, finished_at = now() WHERE id = $1 AND tenant_id = $2",
		id, tenantID, errMsg)
}

// updateStatus исполняет UPDATE-статус и проверяет, что запись
// существовала в скоупе tenant'а.
func (r *CalcJobRepository) updateStatus(ctx context.Context, query string, args ...any) error {
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("calc_jobs: update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return jobs.ErrNotFound
	}
	return nil
}

// updateStatusClaim — вариант updateStatus для compare-and-swap захвата
// (DB-3). Нулевой RowsAffected означает не «нет такой записи», а «переход
// не применим»: задание уже не pending, то есть его взял другой воркер.
// Различать эти случаи обязательно — иначе воркер, получив ErrNotFound,
// отличил бы «нет такого задания» от «уже обработано» и залогировал бы
// это как потерю задания.
func (r *CalcJobRepository) updateStatusClaim(ctx context.Context, query string, args ...any) error {
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("calc_jobs: claim job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Запись могла и не существовать — уточняем, чтобы не соврать.
		var exists bool
		if e := r.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM calc_jobs WHERE id = $1 AND tenant_id = $2)`,
			args[0], args[1]).Scan(&exists); e != nil {
			return fmt.Errorf("calc_jobs: claim job: %w", e)
		}
		if exists {
			return jobs.ErrAlreadyClaimed
		}
		return jobs.ErrNotFound
	}
	return nil
}
