package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	kerngeo "stairplatform/internal/geometry"
	"stairplatform/internal/infrastructure/cad"
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
	ListTenantProjects(ctx context.Context, tenantID string) ([]*project.Project, error)
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
	ApproveConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID, comment string) (*project.ConfigurationApproval, error)
	GetConfigurationApproval(ctx context.Context, tenantID, userID, projectID, configurationID string) (*project.ConfigurationApproval, error)
	ListApprovals(ctx context.Context, tenantID, userID, projectID string) ([]*project.ConfigurationApproval, error)
	Calculate(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options) (*project.Calculation, error)
	Optimize(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options, oreq stair.OptimizeRequest) (*project.OptimizeOutcome, error)
	GetResult(ctx context.Context, tenantID, userID, projectID string) (*project.Calculation, error)
	ExportCAD(ctx context.Context, tenantID, userID, projectID string) (*kerngeo.Mesh, error)
	ListConfigurations(ctx context.Context, tenantID, userID, projectID string) ([]*project.StairConfiguration, error)
	GetConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID string) (*project.StairConfiguration, error)
	RestoreConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID string) (*project.StairConfiguration, error)
}

// ---- DTO ----

// createProjectRequest — запрос создания проекта.
type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// projectDTO — представление проекта (BC-001, EDR-0008 включает владельца).
type projectDTO struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	Status                 string    `json:"status"`
	OwnerID                string    `json:"owner_id"`
	CurrentConfigurationID string    `json:"current_configuration_id,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
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

// approvalDTO — утверждение конфигурации (EDR-0011).
type approvalDTO struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	ConfigurationID string    `json:"configuration_id"`
	ApprovedByID    string    `json:"approved_by"`
	Comment         string    `json:"comment"`
	CreatedAt       time.Time `json:"created_at"`
}

// configurationDTO — ревизия конфигурации (EDR-0012): иммутабельная версия
// конфигурации проекта с монотонным номером Revision.
// Current — признак текущей (активной) ревизии проекта.
type configurationDTO struct {
	ID                  string    `json:"id"`
	ProjectID           string    `json:"project_id"`
	Revision            int       `json:"revision"`
	WidthMM             float64   `json:"width_mm"`
	HeightMM            float64   `json:"height_mm"`
	Flight              string    `json:"flight"`
	StepHeightMM        float64   `json:"step_height_mm"`
	StringerThicknessMM float64   `json:"stringer_thickness_mm"`
	StepThicknessMM     float64   `json:"step_thickness_mm"`
	Riser               bool      `json:"riser"`
	ClearanceMM         float64   `json:"clearance_mm"`
	RailingHeightMM     float64   `json:"railing_height_mm"`
	ComfortStepMM       float64   `json:"comfort_step_mm"`
	LandingWidthMM      float64   `json:"landing_width_mm"`
	LowerStepCount      int       `json:"lower_step_count"`
	OuterRadiusMM       float64   `json:"outer_radius_mm"`
	Current             bool      `json:"current"`
	CreatedAt           time.Time `json:"created_at"`
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
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		u := authUser(r.Context())
		if u == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}
		p, err := svc.CreateProject(r.Context(), tenantID(r.Context()), u.ID, req.Name, req.Description)
		if err != nil {
			writeInputError(w, "invalid_input", err)
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
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
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
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
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
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
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
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		role, err := project.ParseProjectRole(req.Role)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_role", userInputMessage(err))
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
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", userInputMessage(err))
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
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		role, err := project.ParseProjectRole(req.Role)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_role", userInputMessage(err))
			return
		}
		err = svc.UpdateMemberRole(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("userID"), role)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Проект или участник не найдены.")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", userInputMessage(err))
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
			writeError(w, http.StatusNotFound, "not_found", "Проект или участник не найдены.")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", userInputMessage(err))
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
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		c, err := svc.AddComment(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), req.Body)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case errors.Is(err, project.ErrConflict):
			writeError(w, http.StatusUnprocessableEntity, "invalid_body", userInputMessage(err))
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", userInputMessage(err))
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
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
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
			writeError(w, http.StatusNotFound, "not_found", "Комментарий не найден.")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Нет прав на удаление комментария.")
		case err != nil:
			writeError(w, http.StatusConflict, "conflict", userInputMessage(err))
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
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		rv, err := svc.RequestReview(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), req.Comment)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case errors.Is(err, project.ErrConflict):
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", userInputMessage(err))
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
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
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
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
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
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
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
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
		writeError(w, http.StatusNotFound, "not_found", "Отзыв или проект не найден.")
	case errors.Is(err, project.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
	case errors.Is(err, project.ErrConflict):
		writeError(w, http.StatusUnprocessableEntity, "invalid_status", userInputMessage(err))
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
	default:
		writeJSON(w, http.StatusOK, toReviewDTO(rv))
	}
}

// handleApproveConfiguration — POST /api/v1/projects/{id}/configurations/{configID}/approve
// (auth+CSRF, owner). 201 — утверждено; 400 — битый JSON; 403 — нет прав;
// 404 — нет проекта/конфигурации; 422 — уже утверждено.
func handleApproveConfiguration(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req reviewCommentRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		a, err := svc.ApproveConfiguration(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("configID"), req.Comment)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Проект или конфигурация не найдены.")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case errors.Is(err, project.ErrConflict):
			writeInputError(w, "already_approved", err)
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			writeJSON(w, http.StatusCreated, toApprovalDTO(a))
		}
	}
}

// handleGetConfigurationApproval — GET /api/v1/projects/{id}/configurations/{configID}/approval
// (auth). 200 — утверждение; 403 — нет прав; 404 — не найдена/не утверждена.
func handleGetConfigurationApproval(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := svc.GetConfigurationApproval(r.Context(), tenantID(r.Context()), userID(r.Context()),
			r.PathValue("id"), r.PathValue("configID"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Конфигурация не согласована.")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			writeJSON(w, http.StatusOK, toApprovalDTO(a))
		}
	}
}

// handleListApprovals — GET /api/v1/projects/{id}/approvals (auth).
// 200 — история утверждений; 403 — нет прав; 404 — нет проекта.
func handleListApprovals(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		approvals, err := svc.ListApprovals(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			out := make([]approvalDTO, 0, len(approvals))
			for _, a := range approvals {
				out = append(out, toApprovalDTO(a))
			}
			writeJSON(w, http.StatusOK, out)
		}
	}
}

// handleListConfigurations — GET /api/v1/projects/{id}/configurations (auth).
// 200 — история ревизий по возрастанию номера; 403; 404.
func handleListConfigurations(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		configs, err := svc.ListConfigurations(r.Context(), tenantID(r.Context()), userID(r.Context()), projectID)
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			current := currentConfigID(r, svc, projectID)
			out := make([]configurationDTO, 0, len(configs))
			for _, c := range configs {
				out = append(out, toConfigurationDTO(c, current))
			}
			writeJSON(w, http.StatusOK, out)
		}
	}
}

// handleGetConfiguration — GET /api/v1/projects/{id}/configurations/{configID}
// (auth). 200 — ревизия; 403; 404.
func handleGetConfiguration(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		c, err := svc.GetConfiguration(r.Context(), tenantID(r.Context()), userID(r.Context()),
			projectID, r.PathValue("configID"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Конфигурация не найдена.")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			writeJSON(w, http.StatusOK, toConfigurationDTO(c, currentConfigID(r, svc, projectID)))
		}
	}
}

// handleRestoreConfiguration — POST /api/v1/projects/{id}/configurations/{configID}/restore
// (auth+CSRF). 200 — восстановленная ревизия; 403; 404.
func handleRestoreConfiguration(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")
		c, err := svc.RestoreConfiguration(r.Context(), tenantID(r.Context()), userID(r.Context()),
			projectID, r.PathValue("configID"))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Конфигурация не найдена.")
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
		default:
			writeJSON(w, http.StatusOK, toConfigurationDTO(c, c.ID))
		}
	}
}

// currentConfigID возвращает ID текущей ревизии проекта (EDR-0012) или "".
func currentConfigID(r *http.Request, svc ProjectService, projectID string) string {
	p, err := svc.GetProject(r.Context(), tenantID(r.Context()), userID(r.Context()), projectID)
	if err != nil {
		return ""
	}
	return p.CurrentConfigurationID
}

// handleCalculateProject — POST /api/v1/projects/{id}/calculate (auth+CSRF).
// 200 — расчёт сохранён (включая blocking-валидацию); 400 — битый JSON;
// 404 — нет проекта; 422 — невалидный вход; 500 — сбой.
func handleCalculateProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")

		var req calculateRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		cfg, err := toConfig(req)
		if err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}
		opts, err := toOptions(req)
		if err != nil {
			writeInputError(w, "invalid_rates", err)
			return
		}

		calc, err := svc.Calculate(r.Context(), tenantID(r.Context()), userID(r.Context()), projectID, cfg, opts)
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "Просмотрщик не может изменять проект")
			return
		}
		if err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}
		writeJSON(w, http.StatusOK, toCalculationDTO(calc))
	}
}

// handleOptimizeProject — POST /api/v1/projects/{id}/optimize (auth+CSRF).
// Находит оптимальную конфигурацию проекта и сохраняет её с расчётом
// (EDR-0032). 200 — итог поиска (+ сохранённый расчёт при valid:true);
// 400 — битый JSON; 404 — нет проекта; 403 — viewer; 422 — невалидный
// вход/цель; 500 — сбой.
func handleOptimizeProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := r.PathValue("id")

		var req optimizeRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		cfg, err := toConfig(req.calculateRequest)
		if err != nil {
			writeInputError(w, "invalid_input", err)
			return
		}
		opts, err := toOptions(req.calculateRequest)
		if err != nil {
			writeInputError(w, "invalid_rates", err)
			return
		}

		out, err := svc.Optimize(r.Context(), tenantID(r.Context()), userID(r.Context()), projectID, cfg, opts, toOptimizeRequest(req))
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "Проект не найден")
			return
		case errors.Is(err, project.ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden", "Просмотрщик не может изменять проект")
			return
		case err != nil:
			writeInputError(w, "invalid_input", err)
			return
		}
		writeJSON(w, http.StatusOK, toProjectOptimizeResponse(out))
	}
}

// toProjectOptimizeResponse — итог оптимизации проекта: результат поиска
// плюс ID сохранённого расчёта/конфигурации (когда найден).
func toProjectOptimizeResponse(out *project.OptimizeOutcome) projectOptimizeResponse {
	resp := projectOptimizeResponse{optimizeResponse: toOptimizeResponse(out.Result)}
	if out.Calculation != nil {
		resp.CalculationID = out.Calculation.ID
		resp.ConfigurationID = out.Calculation.ConfigurationID
		resp.Saved = true
	}
	return resp
}

type projectOptimizeResponse struct {
	optimizeResponse
	Saved           bool   `json:"saved"`
	CalculationID   string `json:"calculation_id,omitempty"`
	ConfigurationID string `json:"configuration_id,omitempty"`
}

// handleExportProject — GET /api/v1/projects/{id}/export (auth).
// Возвращает канонический экспортный документ (снапшот конвейера) как есть.
// 200 — документ; 404 — нет проекта или расчёта; 500 — сбой.
func handleExportProject(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		calc, err := svc.GetResult(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Для проекта нет расчёта")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="project-export.json"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(calc.Result)
	}
}

// handleExportCAD — GET /api/v1/projects/{id}/export/cad?format=dxf|stl|svg
// (auth, член проекта). Детерминированный экспорт сетки текущей конфигурации
// (EDR-0022). 200 — файл; 400 — неверный format; 403 — нет членства;
// 404 — нет проекта/конфигурации; 500 — сбой конвейера.
func handleExportCAD(svc ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		format, err := cad.ParseFormat(r.URL.Query().Get("format"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", userInputMessage(err))
			return
		}
		mesh, err := svc.ExportCAD(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"))
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Для проекта нет конфигурации")
			return
		}
		if errors.Is(err, project.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden", "Недостаточно прав для проекта")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		var buf bytes.Buffer
		if err := cad.Write(&buf, mesh, format); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Не удалось выполнить экспорт чертежа")
			return
		}
		w.Header().Set("Content-Type", format.MIME())
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="project-%s%s"`, r.PathValue("id"), format.Extension()))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}
}

// ---- converters ----

func toProjectDTO(p *project.Project) projectDTO {
	return projectDTO{
		ID: p.ID, Name: p.Name, Description: p.Description, Status: p.Status,
		OwnerID: p.OwnerID, CurrentConfigurationID: p.CurrentConfigurationID,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
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

func toApprovalDTO(a *project.ConfigurationApproval) approvalDTO {
	return approvalDTO{
		ID: a.ID, ProjectID: a.ProjectID, ConfigurationID: a.ConfigurationID,
		ApprovedByID: a.ApprovedByID, Comment: a.Comment, CreatedAt: a.CreatedAt,
	}
}

// configurationDTOWithCurrent — DTO-конвертер ревизии с признаком текущей
// (EDR-0012): current=true, если конфигурация — active revision проекта.
func toConfigurationDTO(c *project.StairConfiguration, current string) configurationDTO {
	return configurationDTO{
		ID:                  c.ID,
		ProjectID:           c.ProjectID,
		Revision:            c.Revision,
		WidthMM:             c.WidthMM,
		HeightMM:            c.HeightMM,
		Flight:              c.Flight,
		StepHeightMM:        c.StepHeightMM,
		StringerThicknessMM: c.StringerThicknessMM,
		StepThicknessMM:     c.StepThicknessMM,
		Riser:               c.Riser,
		ClearanceMM:         c.ClearanceMM,
		RailingHeightMM:     c.RailingHeightMM,
		ComfortStepMM:       c.ComfortStepMM,
		LandingWidthMM:      c.LandingWidthMM,
		LowerStepCount:      c.LowerStepCount,
		OuterRadiusMM:       c.OuterRadiusMM,
		Current:             c.ID == current,
		CreatedAt:           c.CreatedAt,
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
