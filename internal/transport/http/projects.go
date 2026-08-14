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
	AddMemberByEmail(ctx context.Context, tenantID, actorID, projectID, email string, role project.ProjectRole) error
	UpdateMemberRole(ctx context.Context, tenantID, actorID, projectID, userID string, role project.ProjectRole) error
	RemoveMember(ctx context.Context, tenantID, actorID, projectID, userID string) error
	AddComment(ctx context.Context, tenantID, userID, projectID, body string) (*project.Comment, error)
	ListComments(ctx context.Context, tenantID, userID, projectID string) ([]*project.Comment, error)
	DeleteComment(ctx context.Context, tenantID, userID, projectID, commentID string) error
	RequestReview(ctx context.Context, tenantID, userID, projectID, comment string) (*project.ProjectReview, error)
	SignOffReview(ctx context.Context, tenantID, userID, projectID, reviewID, comment string) (*project.ProjectReview, error)
	RequestChanges(ctx context.Context, tenantID, userID, projectID, reviewID, comment string) (*project.ProjectReview, error)
	ListReviews(ctx context.Context, tenantID, userID, projectID string) ([]*project.ProjectReview, error)
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
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// commentDTO — комментарий к проекту (EDR-0009).
type commentDTO struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// commentRequest — тело запроса добавления комментария.
type commentRequest struct {
	Body string `json:"body"`
}

// reviewDTO — запись ревью проекта (EDR-0010).
type reviewDTO struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	RequesterID string     `json:"requester_id"`
	ReviewerID  string     `json:"reviewer_id"`
	Decision    string     `json:"decision"`
	Comment     string     `json:"comment"`
	CreatedAt   time.Time  `json:"created_at"`
	DecidedAt   *time.Time `json:"decided_at"`
}

// reviewCommentRequest — тело запроса создания/решения ревью (опциональный
// комментарий).
type reviewCommentRequest struct {
	Comment string `json:"comment"`
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
		var merr error
		if req.Email != "" {
			merr = svc.AddMemberByEmail(r.Context(), tenantID(r.Context()), userID(r.Context()),
				r.PathValue("id"), req.Email, role)
		} else {
			merr = svc.AddMember(r.Context(), tenantID(r.Context()), userID(r.Context()),
				r.PathValue("id"), req.UserID, role)
		}
		err = merr
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

// handleAddComment — POST /api/v1/projects/{id}/comments (auth+CSRF).
// 201 — добавлен; 400 — битый JSON; 403 — нет прав; 404 — нет проекта;
// 422 — пустое тело.
func handleAddComment(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req commentRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		c, err := svc.AddComment(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), req.Body)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
		case errors.Is(err, project.ErrConflict):
			writeError(w, http.StatusUnprocessableEntity, "invalid_body", err.Error())
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", err.Error())
		default:
			writeJSON(w, http.StatusCreated, toCommentDTO(c))
		}
	}
}

// handleListComments — GET /api/v1/projects/{id}/comments (auth).
// 200 — список; 403 — нет прав; 404 — нет проекта.
func handleListComments(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		comments, err := svc.ListComments(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
		default:
			out := make([]commentDTO, 0, len(comments))
			for _, c := range comments {
				out = append(out, toCommentDTO(c))
			}
			writeJSON(w, http.StatusOK, out)
		}
	}
}

// handleDeleteComment — DELETE /api/v1/projects/{id}/comments/{commentID}
// (auth+CSRF). 204 — удалён; 403 — нет прав; 404 — нет комментария/проекта.
func handleDeleteComment(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.DeleteComment(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("commentID"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "comment not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "no permission to delete comment")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// handleRequestReview — POST /api/v1/projects/{id}/review (auth+CSRF,
// owner/editor). 201 — запрос создан; 400 — битый JSON; 403 — нет прав;
// 404 — нет проекта; 422 — неверный переход.
func handleRequestReview(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req reviewCommentRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		rv, err := svc.RequestReview(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), req.Comment)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
		case errors.Is(err, project.ErrConflict):
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
		default:
			writeJSON(w, http.StatusCreated, toReviewDTO(rv))
		}
	}
}

// handleSignOffReview — POST /api/v1/projects/{id}/reviews/{reviewID}/sign-off
// (auth+CSRF, owner). 200 — подписано; 400 — битый JSON; 403 — нет прав
// (или self-approve); 404 — нет ревью/проекта; 422 — неверный переход.
func handleSignOffReview(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req reviewCommentRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		rv, err := svc.SignOffReview(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("reviewID"), req.Comment)
		writeReviewDecision(w, rv, err)
	}
}

// handleRequestChanges — POST /api/v1/projects/{id}/reviews/{reviewID}/changes
// (auth+CSRF, owner). 200 — возвращено на доработку; 400 — битый JSON;
// 403 — нет прав (или self-approve); 404 — нет ревью/проекта; 422 —
// неверный переход.
func handleRequestChanges(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req reviewCommentRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		rv, err := svc.RequestChanges(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("reviewID"), req.Comment)
		writeReviewDecision(w, rv, err)
	}
}

// handleListReviews — GET /api/v1/projects/{id}/reviews (auth).
// 200 — история; 403 — нет прав; 404 — нет проекта.
func handleListReviews(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviews, err := svc.ListReviews(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
		default:
			out := make([]reviewDTO, 0, len(reviews))
			for _, rv := range reviews {
				out = append(out, toReviewDTO(rv))
			}
			writeJSON(w, http.StatusOK, out)
		}
	}
}

// writeReviewDecision — общий вывод ответа решения по ревью.
func writeReviewDecision(w http.ResponseWriter, rv *project.ProjectReview, err error) {
	switch {
	case errors.Is(err, project.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "review or project not found")
	case errors.Is(err, project.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "insufficient project role")
	case errors.Is(err, project.ErrConflict):
		writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	default:
		writeJSON(w, http.StatusOK, toReviewDTO(rv))
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

func toCommentDTO(c *project.Comment) commentDTO {
	return commentDTO{
		ID: c.ID, ProjectID: c.ProjectID, AuthorID: c.AuthorID, Body: c.Body, CreatedAt: c.CreatedAt,
	}
}

func toReviewDTO(rv *project.ProjectReview) reviewDTO {
	return reviewDTO{
		ID: rv.ID, ProjectID: rv.ProjectID, RequesterID: rv.RequesterID,
		ReviewerID: rv.ReviewerID, Decision: rv.Decision, Comment: rv.Comment,
		CreatedAt: rv.CreatedAt, DecidedAt: rv.DecidedAt,
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
