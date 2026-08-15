// Package audit реализует прикладной слой Audit System (Phase G,
// EDR-0013): персистентный журнал событий безопасности (SEC-0013).
// Application зависит от порта Repository (BE-0005); транспорт и БД —
// вне слоя (ADR-0006). Запись события — синхронная, best-effort:
// сбой журнала не ломает бизнес-операцию.
package audit

import (
	"context"
	"time"
)

// Action — вид события аудита (SEC-0013 Audit Events, каталог EDR-0013 §3.3).
type Action string

const (
	ActionAuthRegister      Action = "auth.register"
	ActionAuthLogin         Action = "auth.login"
	ActionAuthLogout        Action = "auth.logout"
	ActionAuthLoginDenied   Action = "auth.login_denied"
	ActionAuthzDenied       Action = "authz.denied"
	ActionUserRoleChanged   Action = "user.role_changed"
	ActionUserStatusChanged Action = "user.status_changed"
	ActionSettingsUpdated   Action = "settings.updated"
	ActionDataExported      Action = "data.exported"
	ActionApiKeyCreated     Action = "api_key.created"
	ActionApiKeyRevoked     Action = "api_key.revoked"
	ActionProjectCreated    Action = "project.created"
	ActionProjectModified   Action = "project.modified"
	ActionMemberAdded       Action = "member.added"
	ActionMemberRoleChanged Action = "member.role_changed"
	ActionMemberRemoved     Action = "member.removed"
	ActionConfigApproved    Action = "config.approved"
	ActionConfigRestored    Action = "config.restored"
	ActionReviewRequested   Action = "review.requested"
	ActionReviewSigned      Action = "review.signed"
	ActionReviewChanges     Action = "review.changes"
)

// Result — исход события.
type Result string

const (
	ResultOK     Result = "ok"
	ResultDenied Result = "denied"
	ResultFailed Result = "failed"
)

// Event — запись аудита (SEC-0013 Audit Metadata). Актор и tenant берутся
// из контекста запроса (EDR-0013 §4), никогда из тела.
type Event struct {
	ID           string
	ActorID      string
	TenantID     string
	ProjectID    string // пустая — глобальное событие
	Action       Action
	ResourceType string
	ResourceID   string
	Result       Result
	Detail       string
	RequestID    string
	IP           string
	CreatedAt    time.Time
}

// Repository — порт доступа к данным аудита (BE-0005).
type Repository interface {
	// Insert сохраняет событие (append-only).
	Insert(ctx context.Context, e *Event) error
	// ListByProject возвращает события проекта внутри tenant
	// (SEC-0005), новые сверху; ErrNotFound — проекта нет в tenant.
	ListByProject(ctx context.Context, tenantID, projectID string) ([]*Event, error)
	// ListByTenant возвращает события tenant (глобальный аудит, admin),
	// новые сверху.
	ListByTenant(ctx context.Context, tenantID string) ([]*Event, error)
}
