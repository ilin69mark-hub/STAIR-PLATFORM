package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"stairplatform/internal/application/integrations"
	"stairplatform/internal/application/jobs"
	"stairplatform/internal/infrastructure/database"
	infintegrations "stairplatform/internal/infrastructure/integrations"
	"stairplatform/internal/infrastructure/queue"
)

// registry — реестр обработчиков заданий по типу (EDR-0020 §3.4).
type registry struct {
	authRepo         *database.AuthRepository
	auditRepo        *database.AuditRepository
	integrationsRepo integrations.Repository
	webhook          webhookSender
	integrationsSvc  *integrations.Service
	jobsSvc          *jobs.Service
	retentionDays    int
}

// webhookSender — минимальный порт для доставки webhook (EDR-0023 §3.1),
// реализуется infrastructure/integrations.Client; стубится в тестах.
type webhookSender interface {
	Send(ctx context.Context, url, secret string, payload []byte) error
}

// newRegistry создаёт реестр с обработчиками очистки, доставки webhook и
// асинхронных расчётов (EDR-0035).
func newRegistry(authRepo *database.AuthRepository, auditRepo *database.AuditRepository,
	integrationsRepo integrations.Repository, jobsSvc *jobs.Service, retentionDays int) *registry {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	return &registry{
		authRepo:         authRepo,
		auditRepo:        auditRepo,
		integrationsRepo: integrationsRepo,
		webhook:          infintegrations.NewClient(0),
		integrationsSvc:  integrations.NewService(integrationsRepo, nil),
		jobsSvc:          jobsSvc,
		retentionDays:    retentionDays,
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
	case queue.JobQuoteSend, queue.JobProjectSync, queue.JobOrderSend:
		return r.deliverEvent(ctx, job)
	case queue.JobCalcCalculate:
		return r.calcCalculate(ctx, job)
	default:
		return fmt.Errorf("worker: unknown job type %q", job.Type)
	}
}

// calcCalculate выполняет асинхронный расчёт (EDR-0035): payload задания
// несёт {job_id, tenant_id}; вход и результат — в calc_jobs.
func (r *registry) calcCalculate(ctx context.Context, job queue.Job) error {
	if r.jobsSvc == nil {
		return fmt.Errorf("worker: jobs service not configured")
	}
	var p struct {
		JobID    string `json:"job_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return fmt.Errorf("worker: calc payload: %w", err)
	}
	if p.JobID == "" || p.TenantID == "" {
		return fmt.Errorf("worker: calc payload: missing job_id/tenant_id")
	}
	if err := r.jobsSvc.RunCalculate(ctx, p.TenantID, p.JobID); err != nil {
		return fmt.Errorf("worker: run calc %s: %w", p.JobID, err)
	}
	return nil
}

// overrideWebhook подменяет клиент webhook в тестах.
func (r *registry) overrideWebhook(s webhookSender) { r.webhook = s }

// deliverEvent доставляет событие внешней системе (EDR-0023 §3.4, EDR-0024
// §3.4): читает событие доставки и эндпоинт, отправляет webhook с
// HMAC-подписью, отмечает delivered; при ошибке — failed/DLQ по политике
// воркера. Общий для erp.quote_send и crm.project_sync.
func (r *registry) deliverEvent(ctx context.Context, job queue.Job) error {
	var p struct {
		EventID    string `json:"event_id"`
		EndpointID string `json:"endpoint_id"`
		TenantID   string `json:"tenant_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return fmt.Errorf("worker: deliver payload: %w", err)
	}
	d, err := r.integrationsRepo.GetDelivery(ctx, p.TenantID, p.EventID)
	if err != nil {
		return fmt.Errorf("worker: get delivery: %w", err)
	}
	ep, err := r.integrationsRepo.GetEndpoint(ctx, p.TenantID, p.EndpointID)
	if err != nil {
		return fmt.Errorf("worker: get endpoint: %w", err)
	}

	if err := r.webhook.Send(ctx, ep.URL, ep.SecretEnc, d.Payload); err != nil {
		if serr := r.integrationsSvc.MarkFailed(ctx, p.TenantID, p.EventID, d.Attempts+1, err.Error()); serr != nil {
			slog.Error("worker: mark delivery failed", "event_id", p.EventID, "error", serr)
		}
		return err
	}
	if err := r.integrationsSvc.MarkDelivered(ctx, p.TenantID, p.EventID); err != nil {
		return fmt.Errorf("worker: mark delivered: %w", err)
	}
	return nil
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
