package http

import (
	"errors"
	"fmt"
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

// grantItem — одна метрика в наборе прав: имя и уровень.
// Уровень: read («видеть»), write («менять»), action («можно»).
// Пустой уровень означает «не выдано» — так снимают выдачу, не удаляя строку.
type grantItem struct {
	Capability string `json:"capability"`
	Level      string `json:"level"`
}

// grantsRequest — набор прав сотрудника целиком (PUT). Набор заменяется
// полностью: снять выдачу — значит прислать список без этой метрики.
type grantsRequest struct {
	Grants []grantItem `json:"grants"`
}

type grantsResponse struct {
	Grants []grantItem `json:"grants"`
	// source — откуда взялись действующие права: «grants» (точечные) или
	// «role» (шаблон роли). Фронт показывает это подсказкой, иначе
	// «ничего не выдано» выглядит как «у него нет доступа».
	Source string `json:"source"`
}

// requireUsersPermission — проверка права для admin-эндпоинтов users
// (EDR-0015 §3.4, EDR-0016 §3.1): 403 без права.
func requireUsersPermission(w http.ResponseWriter, r *http.Request, cap auth.Capability, level auth.Level) bool {
	if !hasCapability(r, cap, level) {
		writeError(w, http.StatusForbidden, "forbidden", "Требуются права администратора")
		return false
	}
	return true
}

// handleListUsers — GET /api/v1/admin/users (auth+admin). 200 — список
// пользователей tenant; 403 — нет права users.list.
func handleListUsers(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.CapStaffView, auth.LevelRead) {
			return
		}
		users, err := svc.ListUsers(r.Context(), tenantID(r.Context()))
		if err != nil {
			// API-002: доменные ошибки больше не превращаются в 500 —
			// статус и код определяет единый контракт (error_contract.go).
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
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
		if !requireUsersPermission(w, r, auth.CapStaffRole, auth.LevelAction) {
			return
		}
		var req updateUserRequest
		// DOM-007: строгий разбор — {"sttus":"disabled"} раньше молча
		// терял status, менял только role и отвечал 200, а пользователя НЕ
		// отключал (сессии живы).
		if err := decodeJSONStrict(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", jsonErrorMessage(err))
			return
		}
		if req.Role == nil && req.Status == nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", "Укажите роль или статус")
			return
		}
		var role *auth.Role
		if req.Role != nil {
			parsed, err := auth.ParseRole(*req.Role)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, "invalid_role", "Роль должна быть user или admin.")
				return
			}
			role = &parsed
		}
		var status *auth.Status
		if req.Status != nil {
			parsed, err := auth.ParseStatus(*req.Status)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, "invalid_status", "Статус должен быть active или disabled.")
				return
			}
			status = &parsed
		}
		if err := svc.UpdateUser(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"), role, status); err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeError(w, http.StatusForbidden, "forbidden", "Нельзя менять собственную роль или статус.")
			case errors.Is(err, auth.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "Пользователь не найден.")
			default:
				writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleUserDeletionPreview — GET /api/v1/admin/users/{id}/deletion-preview.
//
// Считает заранее, что произойдёт при удалении: сколько проектов уедет,
// сколько расчётов, комментариев и заказов останется в истории. Без этого
// цифры удаление выглядит как «человек исчезнет» — а на самом деле уезжает
// работа, и её надо показать до нажатия, а не после.
func handleUserDeletionPreview(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.CapStaffDelete, auth.LevelAction) {
			return
		}
		pv, err := svc.UserDeletionPreview(r.Context(), tenantID(r.Context()), r.PathValue("id"))
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "Сотрудник не найден.")
				return
			}
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			return
		}
		writeJSON(w, http.StatusOK, pv)
	}
}

// handleDeleteUser — DELETE /api/v1/admin/users/{id}?transfer_to=<uuid>.
//
// Кому передать проекты, передаётся параметром запроса, а не телом: тело у
// DELETE некоторые прокси выбрасывают, и запрос молча превратился бы в
// удаление без получателя проектов.
//
// Требуется staff.delete. 403 — нет права/себя/владельца при не-владельце/
// последнего владельца; 404 — нет сотрудника или получателя; 422 — получатель
// неактивен.
func handleDeleteUser(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.CapStaffDelete, auth.LevelAction) {
			return
		}
		transferTo := r.URL.Query().Get("transfer_to")
		if transferTo == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input",
				"Укажите, кому передать проекты сотрудника (transfer_to).")
			return
		}
		err := svc.DeleteUser(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id"), transferTo)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrLastOwner):
				writeError(w, http.StatusConflict, "last_owner",
					"Нельзя удалить последнего владельца компании.")
			case errors.Is(err, auth.ErrInvalidRecipient):
				writeError(w, http.StatusUnprocessableEntity, "invalid_recipient",
					"Проекты можно передать только активному сотруднику вашей компании.")
			case errors.Is(err, auth.ErrForbidden):
				writeError(w, http.StatusForbidden, "forbidden",
					"Нельзя удалить себя или владельца компании.")
			case errors.Is(err, auth.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found",
					"Сотрудник не найден или уже удалён.")
			default:
				writeServiceError(w, r, err, "Внутренняя ошибка сервера")
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
		if !requireUsersPermission(w, r, auth.CapStaffView, auth.LevelRead) {
			return
		}
		tenant := tenantID(r.Context())
		users, err := svc.ListUsers(r.Context(), tenant)
		if err != nil {
			// API-002: доменные ошибки больше не превращаются в 500 —
			// статус и код определяет единый контракт (error_contract.go).
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			return
		}
		projectsList, err := projects.ListTenantProjects(r.Context(), tenant)
		if err != nil {
			// API-002: доменные ошибки больше не превращаются в 500 —
			// статус и код определяет единый контракт (error_contract.go).
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
			return
		}
		keys, err := svc.ListApiKeys(r.Context(), tenant)
		if err != nil {
			// API-002: доменные ошибки больше не превращаются в 500 —
			// статус и код определяет единый контракт (error_contract.go).
			writeServiceError(w, r, err, "Внутренняя ошибка сервера")
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

// ---- точечные выдачи прав (EDR-0041) ----

// handleGetUserGrants — GET /api/v1/admin/users/{id}/grants.
// Кто может смотреть: тот, у кого есть метрика «Сотрудники» на чтение.
// Сама выдача чужих прав видна только владельцу (staff.grants) — иначе
// любой, кто видит сотрудников, видел бы и их полномочия.
func handleGetUserGrants(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireUsersPermission(w, r, auth.CapStaffView, auth.LevelRead) {
			return
		}
		id := r.PathValue("id")
		set, err := svc.GetUserGrants(r.Context(), tenantID(r.Context()), id)
		if err != nil {
			writeServiceError(w, r, err, "Не удалось прочитать права сотрудника")
			return
		}
		writeJSON(w, http.StatusOK, grantsResponse{Grants: grantItemsOf(set), Source: grantsSource(r, id)})
	}
}

// handleSetUserGrants — PUT /api/v1/admin/users/{id}/grants.
// Заменяет набор целиком. Требуется staff.grants (выдавать права может
// только владелец) и потолок выдачи: нельзя выдать больше, чем есть у себя.
// Отказы: 403 — нет права или попытка выдать чужое; 404 — нет сотрудника;
// 422 — метрика неизвестна или уровень ей не соответствует.
func handleSetUserGrants(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Выдавать права может только владелец — и проверяется это ПО РОЛИ,
		// а не наличием метрики: метрика staff.grants намеренно не хранится
		// ни у кого (EDR-0041 §4.2), иначе цепочку «выдал → выдал» нельзя
		// было бы остановить, и роль owner проверялась бы обычным гейтом.
		actor := authUser(r.Context())
		if actor == nil || !actor.Role.IsOwner() {
			writeError(w, http.StatusForbidden, "forbidden",
				"Выдавать права сотрудникам может только владелец компании")
			return
		}
		var req grantsRequest
		if err := decodeJSONStrict(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", jsonErrorMessage(err))
			return
		}
		set, err := grantSetFrom(req.Grants)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_grant", err.Error())
			return
		}
		id := r.PathValue("id")
		if err := svc.SetUserGrants(r.Context(), tenantID(r.Context()), actor, id, set); err != nil {
			switch {
			case errors.Is(err, auth.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "Сотрудник не найден")
			case errors.Is(err, auth.ErrGrantCeiling):
				writeError(w, http.StatusForbidden, "forbidden",
					"Нельзя выдать право, которого нет у вас: передать можно только то, что есть у вас самих.")
			case errors.Is(err, auth.ErrGrantOwnerProtected):
				writeError(w, http.StatusForbidden, "forbidden",
					"У владельца нельзя снимать метрики: иначе не останется того, кто может выдать права.")
			case errors.Is(err, auth.ErrForbidden):
				writeError(w, http.StatusForbidden, "forbidden", "Нельзя выдать права себе или без права выдавать")
			default:
				writeServiceError(w, r, err, "Не удалось сохранить права сотрудника")
			}
			return
		}
		writeJSON(w, http.StatusOK, grantsResponse{Grants: grantItemsOf(set), Source: "grants"})
	}
}

// grantSetFrom собирает GrantSet из DTO. Неизвестная метрика или уровень —
// ошибка: иначе опечатка в интерфейсе молча пропала бы, и сотрудник остался
// бы без доступа, которого владелец думал, что выдал.
func grantSetFrom(items []grantItem) (*auth.GrantSet, error) {
	pairs := make([]auth.Grant, 0, len(items))
	for _, it := range items {
		level, err := auth.ParseLevel(it.Level)
		if err != nil {
			return nil, fmt.Errorf("неизвестный уровень %q у метрики %q", it.Level, it.Capability)
		}
		if level == auth.LevelNone {
			continue
		}
		if !auth.AllowsLevel(auth.Capability(it.Capability), level) {
			return nil, fmt.Errorf("метрике %q нельзя выдать уровень %q", it.Capability, it.Level)
		}
		pairs = append(pairs, auth.Grant{
			Capability: auth.Capability(it.Capability),
			Level:      auth.GrantedLevel(level),
		})
	}
	return auth.NewGrantSet(pairs...), nil
}

func grantItemsOf(s *auth.GrantSet) []grantItem {
	out := []grantItem{}
	for _, g := range s.Pairs() {
		out = append(out, grantItem{Capability: string(g.Capability), Level: string(g.Level)})
	}
	return out
}

// grantsSource — откуда действуют права: «grants» или шаблон роли.
func grantsSource(r *http.Request, userID string) string {
	if u := authUser(r.Context()); u != nil && u.ID != userID {
		return ""
	}
	if u := authUser(r.Context()); u != nil && u.GrantsInitialized {
		return "grants"
	}
	return "role"
}
