package http

import (
	"context"
	"encoding/json"
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
	// SyncProject ставит задание crm.project_sync для проекта (EDR-0024).
	SyncProject(ctx context.Context, tenantID, projectID string, payload []byte) (*integrations.Delivery, error)
	// SendManufacturingOrder ставит задание mes.order_send для проекта (EDR-0025).
	SendManufacturingOrder(ctx context.Context, tenantID, projectID string, payload []byte) (*integrations.Delivery, error)
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

// crmProjectDocument — канонический документ проекта для CRM (EDR-0024 §3.3).
type crmProjectDocument struct {
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	OwnerID     string `json:"owner_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// handleProjectSync — POST /api/v1/projects/{id}/crm-sync (auth, член проекта).
// Синхронизирует метаданные проекта в CRM (EDR-0024 §3.5). 202 — событие
// поставлено в очередь; 403 — нет прав; 404 — нет проекта; 422 — нет
// CRM-эндпоинта.
func handleProjectSync(projects ProjectService, svc IntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		ctx := r.Context()
		user := userID(ctx)
		tenant := tenantID(ctx)

		p, err := projects.GetProject(ctx, tenant, user, projectID)
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no such project")
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

		doc := crmProjectDocument{ProjectID: p.ID, Name: p.Name, Description: p.Description,
			Status: p.Status, OwnerID: p.OwnerID,
			CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			UpdatedAt: p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")}
		payload, err := json.Marshal(doc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}

		d, err := svc.SyncProject(ctx, tenant, projectID, payload)
		if errors.Is(err, integrations.ErrNoEndpoint) {
			writeError(w, http.StatusUnprocessableEntity, "no_endpoint", "no crm endpoint configured")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusAccepted, toDeliveryDTO(d))
	}
}

// ---- MES order document (EDR-0025 §3.3) ----

type mesPartDoc struct {
	Number    string  `json:"number"`
	Kind      string  `json:"kind"`
	Material  string  `json:"material"`
	Thickness float64 `json:"thickness_mm"`
	Length    float64 `json:"length_mm"`
	Width     float64 `json:"width_mm"`
}

type mesBOMLineDoc struct {
	Number      int     `json:"line"`
	PartNumber  string  `json:"part_number"`
	Description string  `json:"description"`
	Material    string  `json:"material"`
	Thickness   float64 `json:"thickness_mm"`
	Quantity    int     `json:"quantity"`
	Length      float64 `json:"length_mm"`
	Width       float64 `json:"width_mm"`
}

type mesCutItemDoc struct {
	PartNumber string  `json:"part_number"`
	Material   string  `json:"material"`
	Thickness  float64 `json:"thickness_mm"`
	Length     float64 `json:"length_mm"`
	Width      float64 `json:"width_mm"`
	Quantity   int     `json:"quantity"`
}

type mesNestingDoc struct {
	Sheets      int     `json:"sheets"`
	PartCount   int     `json:"part_count"`
	Utilization float64 `json:"utilization"`
}

type mesOrderDocument struct {
	ProjectID string          `json:"project_id"`
	Parts     []mesPartDoc    `json:"parts"`
	BOM       []mesBOMLineDoc `json:"bom"`
	CutList   []mesCutItemDoc `json:"cut_list"`
	Nesting   *mesNestingDoc  `json:"nesting,omitempty"`
}

// handleOrderSend — POST /api/v1/projects/{id}/order-send (auth, член проекта).
// Передаёт производственный заказ в MES (EDR-0025 §3.5). 202 — событие
// поставлено в очередь; 403 — нет прав; 404 — нет проекта/расчёта;
// 422 — нет MES-эндпоинта (no_endpoint) или manufacturing package
// (no_manufacturing).
func handleOrderSend(projects ProjectService, svc IntegrationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		ctx := r.Context()
		user := userID(ctx)
		tenant := tenantID(ctx)

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

		var snap project.Snapshot
		if err := json.Unmarshal(calc.Result, &snap); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if snap.Manufacturing == nil {
			writeError(w, http.StatusUnprocessableEntity, "no_manufacturing", "no manufacturing package for project")
			return
		}

		doc := mesOrderDocument{ProjectID: projectID}
		for _, part := range snap.Manufacturing.Parts {
			doc.Parts = append(doc.Parts, mesPartDoc{
				Number: string(part.Number), Kind: string(part.Kind), Material: string(part.Material),
				Thickness: part.Thickness.Millimeters(), Length: part.Length.Millimeters(), Width: part.Width.Millimeters(),
			})
		}
		for _, line := range snap.Manufacturing.BOM.Lines {
			doc.BOM = append(doc.BOM, mesBOMLineDoc{
				Number: line.Number, PartNumber: string(line.PartNumber), Description: line.Description,
				Material: string(line.MaterialCode), Thickness: line.Thickness.Millimeters(),
				Quantity: int(line.Quantity), Length: line.Length.Millimeters(), Width: line.Width.Millimeters(),
			})
		}
		for _, item := range snap.Manufacturing.CutList.Items {
			doc.CutList = append(doc.CutList, mesCutItemDoc{
				PartNumber: string(item.PartNumber), Material: string(item.MaterialCode),
				Thickness: item.Thickness.Millimeters(), Length: item.Length.Millimeters(),
				Width: item.Width.Millimeters(), Quantity: int(item.Quantity),
			})
		}
		if n := snap.Manufacturing.Nesting; n != nil {
			doc.Nesting = &mesNestingDoc{Sheets: len(n.Sheets), PartCount: int(n.PartCount), Utilization: n.Utilization}
		}

		payload, err := json.Marshal(doc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}

		d, err := svc.SendManufacturingOrder(ctx, tenant, projectID, payload)
		if errors.Is(err, integrations.ErrNoEndpoint) {
			writeError(w, http.StatusUnprocessableEntity, "no_endpoint", "no mes endpoint configured")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusAccepted, toDeliveryDTO(d))
	}
}
