package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
)

// AuditService — прикладной интерфейс аудита (EDR-0013), ожидаемый
// транспортным слоем (инверсия зависимостей, DOM-0008).
type AuditService interface {
	ListProjectAudit(ctx context.Context, tenantID, projectID string) ([]*audit.Event, error)
	ListTenantAudit(ctx context.Context, tenantID string) ([]*audit.Event, error)
}

// auditEventDTO — запись аудита (EDR-0013, SEC-0013 metadata).
type auditEventDTO struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actor_id,omitempty"`
	TenantID     string    `json:"tenant_id"`
	ProjectID    string    `json:"project_id,omitempty"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type,omitempty"`
	ResourceID   string    `json:"resource_id,omitempty"`
	Result       string    `json:"result"`
	Detail       string    `json:"detail,omitempty"`
	RequestID    string    `json:"request_id,omitempty"`
	IP           string    `json:"ip,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func toAuditEventDTO(e *audit.Event) auditEventDTO {
	return auditEventDTO{
		ID:           e.ID,
		ActorID:      e.ActorID,
		TenantID:     e.TenantID,
		ProjectID:    e.ProjectID,
		Action:       string(e.Action),
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Result:       string(e.Result),
		Detail:       e.Detail,
		RequestID:    e.RequestID,
		IP:           e.IP,
		CreatedAt:    e.CreatedAt,
	}
}

// handleListProjectAudit — GET /api/v1/projects/{id}/audit (auth).
// 200 — история событий проекта (новые сверху); 403 — не-член;
// 404 — проекта нет в tenant.
func handleListProjectAudit(projects ProjectService, auditSvc AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		// Проверка членства (EDR-0013 §4.3): не-член → 404.
		if _, err := projects.GetProject(r.Context(), tenantID(r.Context()), userID(r.Context()), projectID); err != nil {
			switch {
			case errors.Is(err, project.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "project not found")
			case errors.Is(err, project.ErrForbidden):
				writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		events, err := auditSvc.ListProjectAudit(r.Context(), tenantID(r.Context()), projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		out := make([]auditEventDTO, 0, len(events))
		for _, e := range events {
			out = append(out, toAuditEventDTO(e))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleListTenantAudit — GET /api/v1/audit (auth+admin).
// 200 — события tenant; 403 — роль не admin.
func handleListTenantAudit(auditSvc AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if u := authUser(r.Context()); u == nil || u.Role != auth.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		events, err := auditSvc.ListTenantAudit(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		out := make([]auditEventDTO, 0, len(events))
		for _, e := range events {
			out = append(out, toAuditEventDTO(e))
		}
		writeJSON(w, http.StatusOK, out)
	}
}
