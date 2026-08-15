// Package jobs реализует фоновые задания расчёта (EDR-0035, Distrubuted
// Processing, Phase B B4): асинхронный расчёт лестницы через JobQueue
// (EDR-0020). Мета-уровень (прикладные сервисы) зависит от доменов,
// движков и инфраструктурных портов, но не от HTTP/БД/UI (ADR-0006).
//
// Поток: API ставит {job_id, tenant_id} в очередь и создаёт запись в
// calc_jobs (status=pending); воркер читает запись, выполняет конвейер
// (инъектированная CalculateFunc) и фиксирует succeeded/failed. Ретраи —
// штатная политика cmd/worker (EDR-0020 инвариант 3); расчёт детерминирован
// (ADR-0003), повторный прогон консистентен.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/queue"
)

// Status — статус фонового задания (EDR-0035 §3.2).
type Status string

// Статусы calc_jobs.
const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// ErrNotFound — запись задания не найдена (в скоупе tenant'а).
var ErrNotFound = errors.New("jobs: not found")

// ErrInvalid — некорректные входные данные задания.
var ErrInvalid = errors.New("jobs: invalid input")

// Payload — входные данные фонового расчёта (store в calc_jobs.payload).
type Payload struct {
	Config  stair.Config  `json:"config"`
	Options stair.Options `json:"options"`
}

// Job — запись фонового задания (calc_jobs, EDR-0035 §3.2).
type Job struct {
	ID         string        `json:"id"`
	TenantID   string        `json:"tenant_id"`
	UserID     string        `json:"user_id,omitempty"`
	Type       string        `json:"type"`
	Status     Status        `json:"status"`
	Payload    Payload       `json:"payload,omitempty"`
	Result     *stair.Result `json:"result,omitempty"`
	Error      string        `json:"error,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	StartedAt  time.Time     `json:"started_at,omitempty"`
	FinishedAt time.Time     `json:"finished_at,omitempty"`
}

// Repository — порт хранилища calc_jobs (инверсия зависимостей DOM-0008);
// реализуется infrastructure/database.CalculationJobRepository и фейком в
// юнит-тестах.
type Repository interface {
	// Create сохраняет новое задание (status pending). ID должен быть
	// уникальным (первичный ключ).
	Create(ctx context.Context, j *Job) error
	// GetByID возвращает задание в скоупе tenant'а; ErrNotFound — нет
	// записи / не belongs-to-tenant.
	GetByID(ctx context.Context, tenantID, id string) (*Job, error)
	// MarkRunning фиксирует «взято в работу» воркером.
	MarkRunning(ctx context.Context, tenantID, id string) error
	// MarkSucceeded сохраняет результат расчёта.
	MarkSucceeded(ctx context.Context, tenantID, id string, result *stair.Result) error
	// MarkFailed сохраняет текст ошибки.
	MarkFailed(ctx context.Context, tenantID, id string, errMsg string) error
}

// CalculateFunc — выполняемая воркером функция расчёта (обычно обёртка над
// stair.Service.Calculate). Инъектируется для тестируемости.
type CalculateFunc func(ctx context.Context, cfg stair.Config, opts stair.Options) (*stair.Result, error)

// Service — прикладной сервис фоновых заданий. Объединяет Repository и
// JobQueue; на стороне API (SubmitCalculate) очередь не nil, calc может
// быть nil; на стороне воркера (RunCalculate) — наоборот.
type Service struct {
	repo  Repository
	queue queue.JobQueue
	calc  CalculateFunc
	now   func() time.Time
}

// NewService создаёт сервис фоновых заданий.
func NewService(repo Repository, q queue.JobQueue, calc CalculateFunc) *Service {
	return &Service{repo: repo, queue: q, calc: calc, now: time.Now}
}

// SubmitCalculate — API-сторона: валидирует вход, создаёт запись pending,
// ставит задание calc.calculate в очередь и возвращает запись (её ID —
// публичный job_id). Инвариант 1 (EDR-0035 §4): запись не сирота — сбой
// enqueue помечает её failed.
func (s *Service) SubmitCalculate(ctx context.Context, tenantID, userID string, payload Payload) (*Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant required", ErrInvalid)
	}
	if s.queue == nil {
		return nil, fmt.Errorf("%w: queue not configured", ErrInvalid)
	}
	id, err := newJobID()
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	j := &Job{
		ID:        id,
		TenantID:  tenantID,
		UserID:    userID,
		Type:      queue.JobCalcCalculate,
		Status:    StatusPending,
		Payload:   payload,
		CreatedAt: now,
	}
	if err := s.repo.Create(ctx, j); err != nil {
		return nil, fmt.Errorf("jobs: create %s: %w", id, err)
	}

	queuePayload, err := json.Marshal(struct {
		JobID    string `json:"job_id"`
		TenantID string `json:"tenant_id"`
	}{JobID: id, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("jobs: marshal queue payload: %w", err)
	}
	job, err := queue.NewJob(queue.JobCalcCalculate, json.RawMessage(queuePayload))
	if err != nil {
		return nil, err
	}
	if err := s.queue.Enqueue(ctx, job); err != nil {
		if merr := s.repo.MarkFailed(ctx, tenantID, id, "enqueue failed: "+err.Error()); merr != nil {
			slog.Error("jobs: mark failed after enqueue failure", "job_id", id, "error", merr)
		}
		return nil, fmt.Errorf("jobs: enqueue %s: %w", job.Type, err)
	}
	return j, nil
}

// GetJob возвращает задание в скоупе tenant'а (GET /api/v1/jobs/{id}).
func (s *Service) GetJob(ctx context.Context, tenantID, id string) (*Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// RunCalculate — воркер-сторона (cmd/worker): loads запись по job_id,
// фиксирует running, выполняет calculateFunc и сохраняет результат/ошибку.
// Возвращаемая ошибка запускает штатный retry/бакофф очереди.
func (s *Service) RunCalculate(ctx context.Context, tenantID, id string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.calc == nil {
		return fmt.Errorf("jobs: calculator not configured")
	}
	j, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return fmt.Errorf("jobs: load %s: %w", id, err)
	}
	if err := s.repo.MarkRunning(ctx, tenantID, id); err != nil {
		return fmt.Errorf("jobs: mark running %s: %w", id, err)
	}
	res, err := s.calc(ctx, j.Payload.Config, j.Payload.Options)
	if err != nil {
		if merr := s.repo.MarkFailed(ctx, tenantID, id, err.Error()); merr != nil {
			return fmt.Errorf("jobs: mark failed %s: %w", id, merr)
		}
		return err
	}
	if err := s.repo.MarkSucceeded(ctx, tenantID, id, res); err != nil {
		return fmt.Errorf("jobs: mark succeeded %s: %w", id, err)
	}
	return nil
}

// newJobID возвращает крипто-стойкий hex ID задания (16 байт) — тот же
// формат, что queue без экспорта его генератора.
func newJobID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("jobs: job id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
