package http

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
)

// ---- политики безопасности (EDR-0016 §3.2) ----

// policyDTO — представление политики в API (snake_case).
type policyDTO struct {
	MinPasswordLength    int  `json:"min_password_length"`
	RequireNumber        bool `json:"require_number"`
	RequireUpper         bool `json:"require_upper"`
	SessionTTLSeconds    int  `json:"session_ttl_seconds"`
	LoginRateLimitPerMin int  `json:"login_rate_limit_per_min"`
}

func toPolicyDTO(p auth.Policy) policyDTO {
	return policyDTO{
		MinPasswordLength:    p.MinPasswordLength,
		RequireNumber:        p.RequireNumber,
		RequireUpper:         p.RequireUpper,
		SessionTTLSeconds:    p.SessionTTLSeconds,
		LoginRateLimitPerMin: p.LoginRateLimitPerMin,
	}
}

// handleGetSettings — GET /api/v1/admin/settings (auth+admin).
// 200 — политика безопасности tenant; 403 — нет права settings.read.
func handleGetSettings(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionSettingsRead) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		p, err := svc.GetPolicy(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, toPolicyDTO(p))
	}
}

// handleUpdateSettings — PUT /api/v1/admin/settings (auth+admin).
// 200 — политика обновлена; 403 — нет права settings.write;
// 422 — невалидные значения.
func handleUpdateSettings(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionSettingsWrite) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		var req policyDTO
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		p := auth.Policy{
			MinPasswordLength:    req.MinPasswordLength,
			RequireNumber:        req.RequireNumber,
			RequireUpper:         req.RequireUpper,
			SessionTTLSeconds:    req.SessionTTLSeconds,
			LoginRateLimitPerMin: req.LoginRateLimitPerMin,
		}
		if err := svc.UpdatePolicy(r.Context(), tenantID(r.Context()), userID(r.Context()), p); err != nil {
			if errors.Is(err, auth.ErrInvalidPolicy) {
				writeError(w, http.StatusUnprocessableEntity, "invalid_policy", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, toPolicyDTO(p))
	}
}

// ---- экспорт данных tenant (EDR-0016 §3.1, data.export) ----

// handleExport — GET /api/v1/admin/export?scope=users|projects|audit
// (auth+admin). 200 — данные tenant в JSON/CSV (format=json|csv);
// 403 — нет права data.export; 422 — невалидные scope/format.
func handleExport(svc AuthService, projects ProjectService, audits AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionDataExport) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		scope := r.URL.Query().Get("scope")
		format := r.URL.Query().Get("format")
		if scope == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_scope", "scope required: users, projects, audit")
			return
		}
		if format == "" {
			format = "json"
		}
		if format != "json" && format != "csv" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_format", "format must be json or csv")
			return
		}
		tenant := tenantID(r.Context())
		switch scope {
		case "users":
			users, err := svc.ListUsers(r.Context(), tenant)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
			writeExport(w, format, "users", users, func(u *auth.User) []string {
				return []string{u.ID, u.Email, u.Name, string(u.Role), string(u.Status), u.TenantID}
			})
		case "projects":
			list, err := projects.ListTenantProjects(r.Context(), tenant)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
			writeExport(w, format, "projects", list, func(p *project.Project) []string {
				return []string{p.ID, p.Name, p.Description, p.Status, p.OwnerID, p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")}
			})
		case "audit":
			events, err := audits.ListTenantAudit(r.Context(), tenant)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
			writeExport(w, format, "audit", events, func(e *audit.Event) []string {
				return []string{e.ID, e.ActorID, string(e.Action), string(e.Result), e.Detail, e.IP, e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")}
			})
		default:
			writeError(w, http.StatusUnprocessableEntity, "invalid_scope", "scope must be users, projects or audit")
		}
	}
}

// writeExport пишет экспорт в формате json|csv с Content-Disposition.
func writeExport[T any](w http.ResponseWriter, format, name string, rows []T, toRow func(T) []string) {
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-export.csv"`, name))
		cw := csv.NewWriter(w)
		for _, row := range rows {
			if err := cw.Write(toRow(row)); err != nil {
				return
			}
		}
		cw.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-export.json"`, name))
	_ = json.NewEncoder(w).Encode(rows)
}

// ---- API-ключи (EDR-0016 §3.3) ----

// apiKeyDTO — представление ключа (без token_hash).
type apiKeyDTO struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	CreatedBy  string   `json:"created_by,omitempty"`
	CreatedAt  string   `json:"created_at"`
	RevokedAt  string   `json:"revoked_at,omitempty"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
}

func toApiKeyDTO(k *auth.ApiKey) apiKeyDTO {
	dto := apiKeyDTO{
		ID:        k.ID,
		Name:      k.Name,
		Scopes:    k.Scopes,
		CreatedBy: k.CreatedBy,
		CreatedAt: k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if k.RevokedAt != nil {
		dto.RevokedAt = k.RevokedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if k.LastUsedAt != nil {
		dto.LastUsedAt = k.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return dto
}

type createApiKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type createApiKeyResponse struct {
	apiKeyDTO
	Token string `json:"token,omitempty"`
}

// handleListApiKeys — GET /api/v1/admin/api-keys (auth+admin).
// 200 — список ключей tenant; 403 — нет права api_keys.manage.
func handleListApiKeys(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionApiKeysManage) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		keys, err := svc.ListApiKeys(r.Context(), tenantID(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		out := make([]apiKeyDTO, 0, len(keys))
		for _, k := range keys {
			out = append(out, toApiKeyDTO(k))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleCreateApiKey — POST /api/v1/admin/api-keys (auth+admin).
// 201 — ключ создан (открытый токен отдаётся один раз в поле token);
// 403 — нет права api_keys.manage; 422 — пустое имя/невалидный scope.
func handleCreateApiKey(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionApiKeysManage) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		var req createApiKeyRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_input", "name required")
			return
		}
		var scopes []auth.Permission
		for _, s := range req.Scopes {
			scopes = append(scopes, auth.Permission(s))
		}
		key, token, err := svc.CreateApiKey(r.Context(), tenantID(r.Context()), userID(r.Context()), req.Name, scopes)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		resp := createApiKeyResponse{apiKeyDTO: toApiKeyDTO(key), Token: token}
		writeJSON(w, http.StatusCreated, resp)
	}
}

// handleRevokeApiKey — DELETE /api/v1/admin/api-keys/{id} (auth+admin).
// 200 — ключ отозван; 403 — нет права api_keys.manage; 404 — ключ вне tenant.
func handleRevokeApiKey(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasPermission(r, auth.PermissionApiKeysManage) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		if err := svc.RevokeApiKey(r.Context(), tenantID(r.Context()), userID(r.Context()), r.PathValue("id")); err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "api key not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
