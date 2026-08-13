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

// ProjectService — прикладной интерфейс управления проектами (BC-001),
// ожидаемый транспортным слоем (инверсия зависимостей, DOM-0008).
type ProjectService interface {
	CreateProject(ctx context.Context, name, description string) (*project.Project, error)
	GetProject(ctx context.Context, id string) (*project.Project, error)
	ListProjects(ctx context.Context) ([]*project.Project, error)
	Calculate(ctx context.Context, projectID string, cfg stair.Config, opts stair.Options) (*project.Calculation, error)
	GetResult(ctx context.Context, projectID string) (*project.Calculation, error)
}

// ---- DTO ----

// createProjectRequest — запрос создания проекта.
type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// projectDTO — представление проекта (BC-001).
type projectDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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

// handleCreateProject — POST /api/v1/projects.
// 201 — создан; 400 — битый JSON; 422 — невалидный вход; 500 — сбой.
func handleCreateProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		p, err := svc.CreateProject(r.Context(), req.Name, req.Description)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, toProjectDTO(p))
	}
}

// handleGetProject — GET /api/v1/projects/{id}.
// 200 — найден; 404 — нет проекта.
func handleGetProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := svc.GetProject(r.Context(), r.PathValue("id"))
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, toProjectDTO(p))
	}
}

// handleListProjects — GET /api/v1/projects.
func handleListProjects(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.ListProjects(r.Context())
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

// handleCalculateProject — POST /api/v1/projects/{id}/calculate.
// 200 — расчёт сохранён (включая blocking-валидацию); 400 — битый JSON;
// 404 — нет проекта; 422 — невалидный вход; 500 — сбой.
func handleCalculateProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")

		var req calculateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

		calc, err := svc.Calculate(r.Context(), projectID, cfg, opts)
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toCalculationDTO(calc))
	}
}

// handleExportProject — GET /api/v1/projects/{id}/export.
// Возвращает канонический экспортный документ (снапшот конвейера) как есть.
// 200 — документ; 404 — нет проекта или расчёта; 500 — сбой.
func handleExportProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		calc, err := svc.GetResult(r.Context(), r.PathValue("id"))
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no calculation for project")
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
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
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
