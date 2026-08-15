package http

import (
	"context"
	"errors"
	"net/http"

	"stairplatform/internal/application/auth"
)

// UsersService — прикладной интерфейс управления пользователями
// (EDR-0015 §3.4). Реализуется application/auth.Service (совпадает с частью
// AuthService); описан отдельно для читаемости контракта.
type UsersService interface {
	ListUsers(ctx context.Context, tenantID string) ([]*auth.User, error)
	UpdateUserRole(ctx context.Context, tenantID, actorID, userID string, role auth.Role) error
}

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

type updateUserRoleRequest struct {
	Role string `json:"role"`
}

// requireUsersPermission — проверка права для admin-эндпоинтов users
// (EDR-0015 §3.4): 403 без права.
func requireUsersPermission(w http.ResponseWriter, r *http.Request, perm auth.Permission) bool {
	u := authUser(r.Context())
	if u == nil || !u.Role.HasPermission(perm) {
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

// handleUpdateUserRole — PATCH /api/v1/admin/users/{id} (auth+admin).
// 200 — роль обновлена; 403 — нет права/смена собственной роли;
// 404 — пользователь не найден в tenant; 422 — невалидная роль.
func handleUpdateUserRole(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.PermissionUsersUpdateRole) {
			return
		}
		var req updateUserRoleRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		role, err := auth.ParseRole(req.Role)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_role", "role must be user or admin")
			return
		}
		if err := svc.UpdateUserRole(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"), role); err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeError(w, http.StatusForbidden, "forbidden", "cannot change your own role")
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
