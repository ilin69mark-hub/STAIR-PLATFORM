package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

// ProjectService — прикладной интерфейс управления проектами (BC-001,
// Phase C/EDR-0008), ожидаемый транспортным слоем (инверсия зависимостей,
// DOM-0008). Все методы скоупированы по tenant (SEC-0005): tenantID и
// userID берутся из аутентифицированного контекста запроса. userID — инициатор;
// операции с членами проверяют роль (owner — управление, owner/editor — редактирование).
type ProjectService interface {
	CreateProject(ctx context.Context, tenantID, ownerID, name, description string) (*project.Project, error)
	GetProject(ctx context.Context, tenantID, userID, id string) (*project.Project, error)
	ListProjects(ctx context.Context, tenantID, userID string) ([]*project.Project, error)
	ListMembers(ctx context.Context, tenantID, userID, projectID string) ([]*project.ProjectMember, error)
	AddMember(ctx context.Context, tenantID, actorID, projectID, userID string, role project.ProjectRole) error
	UpdateMemberRole(ctx context.Context, tenantID, actorID, projectID, userID string, role project.ProjectRole) error
	RemoveMember(ctx context.Context, tenantID, actorID, projectID, userID string) error
	Calculate(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options) (*project.Calculation, error)
	GetResult(ctx context.Context, tenantID, userID, projectID string) (*project.Calculation, error)
}

// ---- DTO ----

// createProjectRequest — запрос создания проекта.
type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// projectDTO — представление проекта (BC-001, EDR-0008 включает владельца).
type projectDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// memberDTO — участник проекта (EDR-0008).
type memberDTO struct {
	ProjectID string    `json:"project_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// memberRequest — тело запроса управления членом (add/update role).
type memberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// calculationDTO — сохранённый расчёт проекта: метаданные + снапшот.
type calculationDTO struct {
	ProjectID       string          `json:"project_id"`
	CalculationID   string          `json:"calculation_id"`
	ConfigurationID string          `json:"configuration_id"`
	Valid           bool            `json:"valid"`
	Blocking        bool            `json:"blocking"`
	CreatedAt       time.Time       `json:"created_at"`
	Result          json.RawMessage `json:"result"`
}

// ---- handlers ----

// handleCreateProject — POST /api/v1/projects (auth+CSRF).
// 201 — создан; 400 — битый JSON; 422 — невалидный вход; 500 — сбой.
func handleCreateProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createProjectRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		u := authUser(r.Context())
		if u == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		p, err := svc.CreateProject(r.Context(), tenantID(r.Context()), u.ID, req.Name, req.Description)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, toProjectDTO(p))
	}
}

// handleGetProject — GET /api/v1/projects/{id}.
// 200 — найден; 403 — нет прав; 404 — нет проекта.
func handleGetProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := svc.GetProject(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
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
		writeJSON(w, http.StatusOK, toProjectDTO(p))
	}
}

// handleListProjects — GET /api/v1/projects (auth). Возвращает проекты,
// членом которых является вызывающий (EDR-0008).
func handleListProjects(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.ListProjects(r.Context(), tenantID(r.Context()), userID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		out := make([]projectDTO, 0, len(list))
		for _, p := range list {
			out = append(out, toProjectDTO(p))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleListMembers — GET /api/v1/projects/{id}/members (auth).
// 200 — список; 404 — нет проекта; 403 — нет прав.
func handleListMembers(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		members, err := svc.ListMembers(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
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
		out := make([]memberDTO, 0, len(members))
		for _, m := range members {
			out = append(out, toMemberDTO(m))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleAddMember — POST /api/v1/projects/{id}/members (auth+CSRF, owner).
// 201 — добавлен; 400 — битый JSON; 403 — нет прав; 404 — нет проекта;
// 422 — невалидный вход.
func handleAddMember(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req memberRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		role, err := project.ParseProjectRole(req.Role)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_role", err.Error())
			return
		}
		err = svc.AddMember(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), req.UserID, role)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", err.Error())
		default:
			writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
		}
	}
}

// handleUpdateMemberRole — PATCH /api/v1/projects/{id}/members/{userID}
// (auth+CSRF, owner). 200 — обновлено; 403 — нет прав; 404 — нет проекта;
// 422 — невалидная роль.
func handleUpdateMemberRole(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req memberRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		role, err := project.ParseProjectRole(req.Role)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_role", err.Error())
			return
		}
		err = svc.UpdateMemberRole(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("userID"), role)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "project or member not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", err.Error())
		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		}
	}
}

// handleRemoveMember — DELETE /api/v1/projects/{id}/members/{userID}
// (auth+CSRF, owner). 204 — удалён; 403 — нет прав; 404 — нет проекта.
func handleRemoveMember(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.RemoveMember(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("userID"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "project or member not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// handleCalculateProject — POST /api/v1/projects/{id}/calculate (auth+CSRF).
// 200 — расчёт сохранён (включая blocking-валидацию); 400 — битый JSON;
// 404 — нет проекта; 422 — невалидный вход; 500 — сбой.
func handleCalculateProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")

		var req calculateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		cfg, err := toConfig(req)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		opts, err := toOptions(req)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_rates", err.Error())
			return
		}

		calc, err := svc.Calculate(r.Context(), tenantID(r.Context()), userID(r.Context()), projectID, cfg, opts)
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "viewer cannot modify project")
			return
		}
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toCalculationDTO(calc))
	}
}

// handleExportProject — GET /api/v1/projects/{id}/export (auth).
// Возвращает канонический экспортный документ (снапшот конвейера) как есть.
// 200 — документ; 404 — нет проекта или расчёта; 500 — сбой.
func handleExportProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		calc, err := svc.GetResult(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="project-export.json"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(calc.Result)
	}
}

// ---- converters ----

func toProjectDTO(p *project.Project) projectDTO {
	return projectDTO{
		ID: p.ID, Name: p.Name, Description: p.Description, Status: p.Status,
		OwnerID: p.OwnerID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toMemberDTO(m *project.ProjectMember) memberDTO {
	return memberDTO{
		ProjectID: m.ProjectID, UserID: m.UserID, Role: string(m.Role), CreatedAt: m.CreatedAt,
	}
}

func toCalculationDTO(c *project.Calculation) calculationDTO {
	return calculationDTO{
		ProjectID:       c.ProjectID,
		CalculationID:   c.ID,
		ConfigurationID: c.ConfigurationID,
		Valid:           c.Valid,
		Blocking:        c.Blocking,
		CreatedAt:       c.CreatedAt,
		Result:          c.Result,
	}
}
