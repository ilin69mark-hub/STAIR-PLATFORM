package http

import (
	"context"
	"errors"
	"net/http"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/integrations"
	"stairplatform/internal/application/project"
)

// IntegrationService — прикладной интерфейс интеграций (EDR-0023 §3.4),
// ожидаемый транспортным слоем.
type IntegrationService interface {
	RegisterEndpoint(ctx context.Context, tenantID, name, kind, url, secret string) (*integrations.Endpoint, error)
	ListEndpoints(ctx context.Context, tenantID string) ([]*integrations.Endpoint, error)
	GetEndpoint(ctx context.Context, tenantID, id string) (*integrations.Endpoint, error)
	DeleteEndpoint(ctx context.Context, tenantID, id string) error
	SendQuote(ctx context.Context, tenantID, projectID string, payload []byte) (*integrations.Delivery, error)
}

// ---- DTO ----

type endpointDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	URL     string `json:"url"`
	Created string `json:"created_at"`
}

func toEndpointDTO(e *integrations.Endpoint) endpointDTO {
	return endpointDTO{ID: e.ID, Name: e.Name, Kind: string(e.Kind), URL: e.URL,
		Created: e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")}
}

type createEndpointRequest struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

type deliveryDTO struct {
	ID         string `json:"id"`
	EndpointID string `json:"endpoint_id"`
	ProjectID  string `json:"project_id,omitempty"`
	EventType  string `json:"event_type"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

func toDeliveryDTO(d *integrations.Delivery) deliveryDTO {
	return deliveryDTO{ID: d.ID, EndpointID: d.EndpointID, ProjectID: d.ProjectID,
		EventType: d.EventType, Status: string(d.Status), CreatedAt: d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")}
}

// ---- handlers ----

// handleListEndpoints — GET /api/v1/integrations/endpoints (auth+admin).
// 200 — список эндпоинтов tenant; 403 — нет права integrations.manage.
func handleListEndpoints(svc IntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionIntegrationsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "integrations.manage required")
			return
		}
		eps, err := svc.ListEndpoints(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		out := make([]endpointDTO, 0, len(eps))
		for _, e := range eps {
			out = append(out, toEndpointDTO(e))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleCreateEndpoint — POST /api/v1/integrations/endpoints (auth+admin).
// 201 — создан; 400 — невалидный JSON; 403 — нет права;
// 422 — невалидные данные эндпоинта.
func handleCreateEndpoint(svc IntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionIntegrationsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "integrations.manage required")
			return
		}
		var req createEndpointRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		ep, err := svc.RegisterEndpoint(r.Context(), tenantID(r.Context()), req.Name, req.Kind, req.URL, req.Secret)
		if errors.Is(err, integrations.ErrInvalid) {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusCreated, toEndpointDTO(ep))
	}
}

// handleDeleteEndpoint — DELETE /api/v1/integrations/endpoints/{id}.
// 204 — удалено; 403 — нет права; 404 — нет эндпоинта.
func handleDeleteEndpoint(svc IntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionIntegrationsManage) {
			writeError(w, http.StatusForbidden, "forbidden", "integrations.manage required")
			return
		}
		err := svc.DeleteEndpoint(r.Context(), tenantID(r.Context()), r.PathValue("id"))
		if errors.Is(err, integrations.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleQuoteSend — POST /api/v1/projects/{id}/quote-send (auth, член проекта
// с актуальным расчётом). Берёт результат (Snapshot) и ставит erp.quote_send
// в очередь (EDR-0023 §3.4). 202 — поставлено в очередь;
// 403 — нет прав; 404 — нет проекта/расчёта; 422 — нет ERP-эндпоинта.
func handleQuoteSend(projects ProjectService, svc IntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		ctx := r.Context()
		user := userID(ctx)
		tenant := tenantID(ctx)

		// Актуальный результат: требует членства и актуального расчёта.
		calc, err := projects.GetResult(ctx, tenant, user, projectID)
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no calculation for project")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}

		d, err := svc.SendQuote(ctx, tenant, projectID, calc.Result)
		if errors.Is(err, integrations.ErrNoEndpoint) {
			writeError(w, http.StatusUnprocessableEntity, "no_endpoint", "no erp endpoint configured")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusAccepted, toDeliveryDTO(d))
	}
}
