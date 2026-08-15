package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"stairplatform/internal/infrastructure/database"
	"stairplatform/internal/infrastructure/queue"
)

// registry — реестр обработчиков заданий по типу (EDR-0020 §3.4).
type registry struct {
	authRepo      *database.AuthRepository
	auditRepo     *database.AuditRepository
	retentionDays int
}

// newRegistry создаёт реестр с обработчиками очистки.
func newRegistry(authRepo *database.AuthRepository, auditRepo *database.AuditRepository, retentionDays int) *registry {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	return &registry{
		authRepo:      authRepo,
		auditRepo:     auditRepo,
		retentionDays: retentionDays,
	}
}

// Handle исполняет задание по типу. Неизвестный тип — ошибка.
func (r *registry) Handle(ctx context.Context, job queue.Job) error {
	switch job.Type {
	case queue.JobCleanupSessions:
		return r.cleanupSessions(ctx)
	case queue.JobCleanupSsoStates:
		return r.cleanupSsoStates(ctx)
	case queue.JobCleanupAudit:
		return r.cleanupAudit(ctx)
	default:
		return fmt.Errorf("worker: unknown job type %q", job.Type)
	}
}

// cleanupSessions удаляет истёкшие к текущему моменту сессии
// (EDR-0020 §3.3).
func (r *registry) cleanupSessions(ctx context.Context) error {
	n, err := r.authRepo.DeleteExpiredSessions(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("cleanup sessions: %w", err)
	}
	slog.Info("worker: cleanup sessions", "deleted", n)
	return nil
}

// cleanupSsoStates удаляет истёкшие одноразовые OIDC-состояния.
func (r *registry) cleanupSsoStates(ctx context.Context) error {
	n, err := r.authRepo.DeleteExpiredSsoStates(ctx)
	if err != nil {
		return fmt.Errorf("cleanup sso states: %w", err)
	}
	slog.Info("worker: cleanup sso states", "deleted", n)
	return nil
}

// cleanupAudit удаляет аудит-события старше retentionDays (EDR-0020 §3.3).
func (r *registry) cleanupAudit(ctx context.Context) error {
	before := time.Now().UTC().Add(-time.Duration(r.retentionDays) * 24 * time.Hour)
	n, err := r.auditRepo.DeleteBefore(ctx, before)
	if err != nil {
		return fmt.Errorf("cleanup audit: %w", err)
	}
	slog.Info("worker: cleanup audit", "deleted", n, "before", before.Format(time.RFC3339))
	return nil
}
