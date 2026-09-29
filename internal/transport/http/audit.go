package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
)

// AuditService — прикладной интерфейс аудита (EDR-0013), ожидаемый
// транспортным слоем (инверсия зависимостей, DOM-0008).
type AuditService interface {
	Record(ctx context.Context, e *audit.Event) error
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

// auditRequestID извлекает request id из контекста (ключ из middleware.go,
// тот же пакет) для записи в метаданные события.
func auditRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
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
				writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
			case errors.Is(err, project.ErrForbidden):
				writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав.")
			default:
				writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			}
			return
		}
		events, err := auditSvc.ListProjectAudit(r.Context(), tenantID(r.Context()), projectID)
		if err != nil {
			// API-002: доменные ошибки больше не превращаются в 500 —
			// статус и код определяет единый контракт (error_contract.go).
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
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
// 200 — события tenant; 403 — нет права audit.read_all.
func handleListTenantAudit(auditSvc AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if u := authUser(r.Context()); u == nil || !u.Role.HasPermission(auth.PermissionAuditReadAll) {
			writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
			return
		}
		events, err := auditSvc.ListTenantAudit(r.Context(), tenantID(r.Context()))
		if err != nil {
			// API-002: доменные ошибки больше не превращаются в 500 —
			// статус и код определяет единый контракт (error_contract.go).
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			return
		}
		out := make([]auditEventDTO, 0, len(events))
		for _, e := range events {
			out = append(out, toAuditEventDTO(e))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// auditRecordRequest — тело POST /api/v1/audit (клиентские события,
// EDR-0013 §4.2). Актор и tenant берутся из контекста, никогда из тела.
type auditRecordRequest struct {
	Action       string `json:"action"`
	ResourceType string `json:"resource_type,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

// handleRecordAudit — POST /api/v1/audit (auth). Записывает клиентское
// событие (клики, применение вариантов). 201 — записано; 400 — пустое
// действие; 401 — не аутентифицирован; 422 — действие не в allowlist
// клиентских событий или превышены лимиты полей. Ошибка журнала не ломает
// бизнес-операцию (best-effort, EDR-0013 §4.1).
//
// SEC-002 (2026-09-26): действие приходилось из тела без проверки, поэтому
// любой аутентифицированный пользователь мог записать в журнал безопасности
// строку с action="user.role_changed" (или "payment.refunded") и жёстко
// заданным result="ok" — то есть подделать доказательство в журнале
// SEC-0013. Воспроизведено: POST /api/v1/audit с
// {"action":"user.role_changed"} -> 201 и строка в БД.
//
// Теперь принимаются ТОЛЬКО audit.ClientActions() — клиентские события
// интерфейса (EDR-0013 §4.2). Всё остальное — серверная эмиссия.
// Result клиент не выбирает: он жёстко равен ResultOK, иначе подделка могла
// бы замаскировать реальные отказы под «успех».
func handleRecordAudit(auditSvc AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := userID(r.Context())
		if uid == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется аутентификация")
			return
		}
		var req auditRecordRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		if req.Action == "" {
			writeError(w, http.StatusBadRequest, "invalid_action", "Не указано действие (action)")
			return
		}
		action := audit.Action(req.Action)
		// SEC-002: allowlist клиентских событий. Отдельно отличаем
		// «неизвестное действие» от «известного, но серверного» — первое
		// похоже на опечатку клиента, второе на попытку подделки.
		switch {
		case !action.Valid():
			writeError(w, http.StatusUnprocessableEntity, "invalid_action",
				"Неизвестное действие аудита. Допустимые клиентские события: "+
					strings.Join(auditActionNames(), ", ")+".")
			return
		case !action.ClientRecordable():
			writeError(w, http.StatusForbidden, "action_not_recordable",
				"Это действие регистрируется только сервером и не может быть записано из клиента.")
			return
		}
		e := &audit.Event{
			ActorID:      uid,
			TenantID:     tenantID(r.Context()),
			Action:       action,
			ResourceType: req.ResourceType,
			ResourceID:   req.ResourceID,
			Result:       audit.ResultOK,
			Detail:       req.Detail,
			RequestID:    auditRequestID(r.Context()),
			IP:           clientIP(r),
		}
		// Второй рубеж: доменная валидация (длины полей, tenant, action).
		if err := e.Validate(); err != nil {
			if errors.Is(err, audit.ErrInvalidEvent) {
				writeError(w, http.StatusUnprocessableEntity, "invalid_audit_event",
					"Поля события аудита превышают допустимые пределы.")
				return
			}
			writeErrorWithRequestID(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		if err := auditSvc.Record(r.Context(), e); err != nil {
			if errors.Is(err, audit.ErrInvalidEvent) {
				writeError(w, http.StatusUnprocessableEntity, "invalid_audit_event",
					"Событие аудита отклонено проверкой целостности.")
				return
			}
			writeErrorWithRequestID(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
	}
}

// auditActionNames —allowlist клиентских действий строкой для сообщения
// об ошибке (не раскрывает реестр серверных действий).
func auditActionNames() []string {
	acts := audit.ClientActions()
	out := make([]string, 0, len(acts))
	for _, a := range acts {
		out = append(out, string(a))
	}
	return out
}
