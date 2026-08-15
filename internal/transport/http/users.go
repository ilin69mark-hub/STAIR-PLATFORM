package http

import (
	"errors"
	"net/http"

	"stairplatform/internal/application/auth"
)

type userListDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	TenantID string `json:"tenant_id"`
}

func toUserListDTO(u *auth.User) userListDTO {
	return userListDTO{ID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role), Status: string(u.Status), TenantID: u.TenantID}
}

// updateUserRequest — PATCH /api/v1/admin/users/{id}: роль и/или статус
// (EDR-0016 §3.1). Оба поля опциональны; хотя бы одно задаётся.
type updateUserRequest struct {
	Role   *string `json:"role"`
	Status *string `json:"status"`
}

// requireUsersPermission — проверка права для admin-эндпоинтов users
// (EDR-0015 §3.4, EDR-0016 §3.1): 403 без права.
func requireUsersPermission(w http.ResponseWriter, r *http.Request, perm auth.Permission) bool {
	if !hasPermission(r, perm) {
		writeError(w, http.StatusForbidden, "forbidden", "admin required")
		return false
	}
	return true
}

// handleListUsers — GET /api/v1/admin/users (auth+admin). 200 — список
// пользователей tenant; 403 — нет права users.list.
func handleListUsers(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.PermissionUsersList) {
			return
		}
		users, err := svc.ListUsers(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		out := make([]userListDTO, 0, len(users))
		for _, u := range users {
			out = append(out, toUserListDTO(u))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleUpdateUser — PATCH /api/v1/admin/users/{id} (auth+admin).
// Меняет роль и/или статус (EDR-0016 §3.1). Требуется users.manage.
// 200 — обновлено; 403 — нет права/смена собственных роли/статуса;
// 404 — пользователь не найден в tenant; 422 — невалидные роль/статус.
func handleUpdateUser(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.PermissionUsersManage) {
			return
		}
		var req updateUserRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Role == nil && req.Status == nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", "role or status required")
			return
		}
		var role *auth.Role
		if req.Role != nil {
			parsed, err := auth.ParseRole(*req.Role)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, "invalid_role", "role must be user or admin")
				return
			}
			role = &parsed
		}
		var status *auth.Status
		if req.Status != nil {
			parsed, err := auth.ParseStatus(*req.Status)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, "invalid_status", "status must be active or disabled")
				return
			}
			status = &parsed
		}
		if err := svc.UpdateUser(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"), role, status); err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeError(w, http.StatusForbidden, "forbidden", "cannot change your own role or status")
			case errors.Is(err, auth.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "user not found")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleAdminOverview — GET /api/v1/admin/overview (auth+admin).
// Статистика tenant (EDR-0016 §3.1): счётчики пользователей (по ролям и
// статусам), проектов, аудит-событий и API-ключей. 200 — статистика.
func handleAdminOverview(svc AuthService, projects ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.PermissionUsersList) {
			return
		}
		tenant := tenantID(r.Context())
		users, err := svc.ListUsers(r.Context(), tenant)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		projectsList, err := projects.ListTenantProjects(r.Context(), tenant)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		keys, err := svc.ListApiKeys(r.Context(), tenant)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		var activeUsers, disabledUsers, admins int
		for _, u := range users {
			if u.Status == auth.StatusActive {
				activeUsers++
			} else {
				disabledUsers++
			}
			if u.Role == auth.RoleAdmin {
				admins++
			}
		}
		activeKeys := 0
		for _, k := range keys {
			if k.Active() {
				activeKeys++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tenant_id":       tenant,
			"users":           len(users),
			"active_users":    activeUsers,
			"disabled_users":  disabledUsers,
			"admins":          admins,
			"projects":        len(projectsList),
			"active_api_keys": activeKeys,
			"total_api_keys":  len(keys),
		})
	}
}
