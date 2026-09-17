package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

// adminErrAuth — admin-аутентификация с управляемой ошибкой G4-методов.
type adminErrAuth struct {
	adminAuth
	updateErr    error
	policyErr    error
	policyGetErr error
	createKeyErr error
	revokeKeyErr error
	listUsersErr error
	listKeysErr  error

	users []*auth.User
	keys  []*auth.ApiKey
}

func (a *adminErrAuth) GetPolicy(ctx context.Context, tenantID string) (auth.Policy, error) {
	if a.policyGetErr != nil {
		return auth.Policy{}, a.policyGetErr
	}
	return a.adminAuth.GetPolicy(ctx, tenantID)
}

func (a *adminErrAuth) ListUsers(ctx context.Context, tenantID string) ([]*auth.User, error) {
	if a.listUsersErr != nil {
		return nil, a.listUsersErr
	}
	if a.users != nil {
		return a.users, nil
	}
	return a.adminAuth.ListUsers(ctx, tenantID)
}

func (a *adminErrAuth) ListApiKeys(ctx context.Context, tenantID string) ([]*auth.ApiKey, error) {
	if a.listKeysErr != nil {
		return nil, a.listKeysErr
	}
	if a.keys != nil {
		return a.keys, nil
	}
	return a.adminAuth.ListApiKeys(ctx, tenantID)
}

func (a *adminErrAuth) UpdateUser(ctx context.Context, tenantID, actorID, userID string, role *auth.Role, status *auth.Status) error {
	if a.updateErr != nil {
		return a.updateErr
	}
	return nil
}

func (a *adminErrAuth) UpdatePolicy(ctx context.Context, tenantID, actorID string, p auth.Policy) error {
	if a.policyErr != nil {
		return a.policyErr
	}
	return nil
}

func (a *adminErrAuth) CreateApiKey(ctx context.Context, tenantID, actorID, name string, scopes []auth.Permission) (*auth.ApiKey, string, error) {
	if a.createKeyErr != nil {
		return nil, "", a.createKeyErr
	}
	return &auth.ApiKey{ID: "key-1", TenantID: tenantID, Name: name}, "secret-token", nil
}

func (a *adminErrAuth) RevokeApiKey(ctx context.Context, tenantID, actorID, keyID string) error {
	if a.revokeKeyErr != nil {
		return a.revokeKeyErr
	}
	return nil
}

func TestUpdateUserStatusAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2", `{"status":"disabled"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRoleAndStatusAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2", `{"role":"admin","status":"disabled"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserEmptyPayload(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2", `{}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserInvalidStatus(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2", `{"status":"banned"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserOwnForbidden(t *testing.T) {
	a := &adminErrAuth{updateErr: auth.ErrForbidden}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-admin", `{"role":"user"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	a := &adminErrAuth{updateErr: auth.ErrNotFound}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-missing", `{"status":"disabled"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2", `{"role":"admin"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- settings (EDR-0016 §3.2) ----

func TestGetSettingsAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/settings", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var p policyDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.MinPasswordLength != 8 {
		t.Fatalf("min_password_length = %d, want 8 (default)", p.MinPasswordLength)
	}
}

func TestGetSettingsAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/settings", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSettingsAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPut, "/api/v1/admin/settings",
		`{"min_password_length":12,"session_ttl_seconds":3600,"login_rate_limit_per_min":20}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSettingsInvalidPolicy(t *testing.T) {
	a := &adminErrAuth{policyErr: auth.ErrInvalidPolicy}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPut, "/api/v1/admin/settings",
		`{"min_password_length":3}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSettingsAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPut, "/api/v1/admin/settings", `{"min_password_length":8}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSettingsError(t *testing.T) {
	a := &adminErrAuth{policyGetErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/settings", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSettingsServiceError(t *testing.T) {
	a := &adminErrAuth{policyErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPut, "/api/v1/admin/settings", `{"min_password_length":8}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListUsersError(t *testing.T) {
	a := &adminErrAuth{listUsersErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/users", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateApiKeyServiceError(t *testing.T) {
	a := &adminErrAuth{createKeyErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPost, "/api/v1/admin/api-keys", `{"name":"CI","scopes":["users.list"]}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRevokeApiKeyServiceError(t *testing.T) {
	a := &adminErrAuth{revokeKeyErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodDelete, "/api/v1/admin/api-keys/key-1", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// generic ошибка сервиса → 500 (не NotFound → 404).
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- export (EDR-0016 §3.1) ----

func TestExportUsersJSON(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=users&format=json", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "member@example.com") {
		t.Fatalf("expected users in body: %s", rec.Body.String())
	}
}

func TestExportUsersCSV(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=users&format=csv", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "users-export.csv") {
		t.Fatalf("missing attachment disposition: %q", rec.Header().Get("Content-Disposition"))
	}
}

func TestExportProjects(t *testing.T) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "Экспорт", Status: "draft"}
	router := NewRouter(nil, projects, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=projects", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Экспорт") {
		t.Fatalf("expected projects in body: %s", rec.Body.String())
	}
}

func TestExportMissingScope(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExportInvalidScope(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=secrets", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExportAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=users", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExportInvalidFormat(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=users&format=xml", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExportProjectsCSV(t *testing.T) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "Экспорт", Status: "draft", CreatedAt: time.Now()}
	router := NewRouter(nil, projects, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=projects&format=csv", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "projects-export.csv") {
		t.Fatalf("missing attachment disposition: %q", rec.Header().Get("Content-Disposition"))
	}
}

func TestExportAuditJSON(t *testing.T) {
	auditSvc := &fakeAuditService{events: []*audit.Event{
		{ID: "e1", ActorID: "u-1", Action: audit.ActionAuthLogin, Result: audit.ResultOK,
			CreatedAt: time.Now()},
	}}
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig(), auditSvc)
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=audit", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "auth.login") {
		t.Fatalf("expected audit events in body: %s", rec.Body.String())
	}
}

func TestExportUsersError(t *testing.T) {
	a := &adminErrAuth{listUsersErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=users", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExportProjectsError(t *testing.T) {
	projects := newFakeProjectService()
	projects.listTenantErr = errors.New("pg down")
	router := NewRouter(nil, projects, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=projects", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExportAuditError(t *testing.T) {
	auditSvc := &fakeAuditService{tenantErr: errors.New("pg down")}
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig(), auditSvc)
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=audit", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestExportAuditCSV — toRow для аудита исполняется только в CSV (JSON кидает
// массив целиком), поэтому отдельный кейс.
func TestExportAuditCSV(t *testing.T) {
	auditSvc := &fakeAuditService{events: []*audit.Event{
		{ID: "e1", ActorID: "u-1", Action: audit.ActionAuthLogin, Result: audit.ResultOK,
			CreatedAt: time.Now()},
	}}
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig(), auditSvc)
	req := authedRequest(http.MethodGet, "/api/v1/admin/export?scope=audit&format=csv", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "auth.login") {
		t.Fatalf("expected audit row in CSV: %s", rec.Body.String())
	}
}

// failingWriter — ResponseWriter, который падает на записи (для ветки
// csv-ошибок в writeExport).
type failingWriter struct {
	http.ResponseWriter
	h http.Header
}

func (w failingWriter) Header() http.Header       { return w.h }
func (failingWriter) WriteHeader(int)             {}
func (failingWriter) Write(p []byte) (int, error) { return 0, errors.New("boom") }

func TestWriteExportCSVWriteError(t *testing.T) {
	w := failingWriter{ResponseWriter: httptest.NewRecorder(), h: http.Header{}}
	// Много длинных строк: буфер csv.Writer переполняется ещё в cw.Write,
	// flushed-ошибка ловится внутри ветки if err := cw.Write(...).
	rows := make([]int, 500)
	writeExport(w, "csv", "users", rows, func(int) []string {
		return []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"cccccccccccccccccccccccccccccccccccccccccccccccc"}
	})
	// Никакого panic — ветка cw.Write(err) просто выходит.
}

// TestListApiKeysDTOCoversMeta — DTO для отозванного/использованного ключа
// (ветки toApiKeyDTO: revoked_at, last_used_at).
func TestListApiKeysDTOCoversMeta(t *testing.T) {
	revoked := time.Now().Add(-time.Hour)
	lastUsed := time.Now().Add(-time.Minute)
	a := &adminErrAuth{keys: []*auth.ApiKey{
		{ID: "k-1", Name: "Старый", CreatedAt: time.Now(), LastUsedAt: &lastUsed},
		{ID: "k-2", Name: "Отозванный", CreatedAt: time.Now(), RevokedAt: &revoked},
	}}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/api-keys", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var keys []apiKeyDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].LastUsedAt == "" || keys[1].RevokedAt == "" {
		t.Fatalf("expected last_used_at/revoked_at in DTO: %+v", keys)
	}
}

func TestListApiKeysError(t *testing.T) {
	a := &adminErrAuth{listKeysErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/api-keys", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- api-keys (EDR-0016 §3.3) ----

func TestListApiKeysAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/api-keys", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateApiKeyAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPost, "/api/v1/admin/api-keys", `{"name":"CI","scopes":["users.list"]}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp createApiKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token != "secret-token" {
		t.Fatalf("expected token once, got %q", resp.Token)
	}
}

func TestCreateApiKeyMissingName(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPost, "/api/v1/admin/api-keys", `{"scopes":["users.list"]}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateApiKeyInvalidJSON(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPost, "/api/v1/admin/api-keys", `not-json`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateApiKeyUnknownScope(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPost, "/api/v1/admin/api-keys",
		`{"name":"CI","scopes":["users.list","killa.ll"]}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "killa.ll") {
		t.Fatalf("expected scope name in error: %s", rec.Body.String())
	}
}

func TestCreateApiKeyValidScopeWithSpaces(t *testing.T) {
	// Пробелы после запятой (frontend шлёт "users.list, data.export") —
	// тримаются до валидации.
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPost, "/api/v1/admin/api-keys",
		`{"name":"CI","scopes":["users.list", " data.export "]}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRevokeApiKeyAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodDelete, "/api/v1/admin/api-keys/key-1", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRevokeApiKeyNotFound(t *testing.T) {
	a := &adminErrAuth{revokeKeyErr: auth.ErrNotFound}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodDelete, "/api/v1/admin/api-keys/nope", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminApiKeysAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/admin/api-keys"},
		{http.MethodPost, "/api/v1/admin/api-keys"},
		{http.MethodDelete, "/api/v1/admin/api-keys/key-1"},
	} {
		req := authedRequest(tc.method, tc.path, `{"name":"CI"}`)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: expected 403, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

// ---- overview (EDR-0016 §3.1) ----

func TestAdminOverview(t *testing.T) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	router := NewRouter(stair.NewService(), projects, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/overview", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var stats map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats["users"] == nil || stats["projects"] == nil {
		t.Fatalf("expected user/project counters: %v", stats)
	}
}

func TestAdminOverviewAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/overview", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminOverviewCounters — счётчики по ролям, статусам и ключам:
// активный/отключённый пользователь, админ, действующий и отозванный ключ.
func TestAdminOverviewCounters(t *testing.T) {
	revoked := time.Now().Add(-time.Hour)
	lastUsed := time.Now().Add(-time.Minute)
	a := &adminErrAuth{
		users: []*auth.User{
			{ID: "u-1", Role: auth.RoleAdmin, Status: auth.StatusActive},
			{ID: "u-2", Role: auth.RoleUser, Status: auth.StatusActive},
			{ID: "u-3", Role: auth.RoleUser, Status: auth.StatusDisabled},
		},
		keys: []*auth.ApiKey{
			{ID: "k-1", Name: "Активный", CreatedAt: time.Now(), LastUsedAt: &lastUsed},
			{ID: "k-2", Name: "Отозванный", CreatedAt: time.Now(), RevokedAt: &revoked},
		},
	}
	router := NewRouter(nil, newFakeProjectService(), a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/overview", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		Users         int `json:"users"`
		ActiveUsers   int `json:"active_users"`
		DisabledUsers int `json:"disabled_users"`
		Admins        int `json:"admins"`
		ActiveKeys    int `json:"active_api_keys"`
		TotalKeys     int `json:"total_api_keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats.Users != 3 || stats.ActiveUsers != 2 || stats.DisabledUsers != 1 || stats.Admins != 1 {
		t.Fatalf("user counters wrong: %+v", stats)
	}
	if stats.ActiveKeys != 1 || stats.TotalKeys != 2 {
		t.Fatalf("key counters wrong: %+v", stats)
	}
}

func TestAdminOverviewListUsersError(t *testing.T) {
	a := &adminErrAuth{listUsersErr: errors.New("pg down")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/overview", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminOverviewProjectsError(t *testing.T) {
	projects := newFakeProjectService()
	projects.listTenantErr = errors.New("pg down")
	router := NewRouter(nil, projects, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/overview", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminOverviewKeysError(t *testing.T) {
	a := &adminErrAuth{listKeysErr: errors.New("pg down")}
	// проекты обязательны: иначе сработает panic-recovery, а не ветка ListApiKeys.
	router := NewRouter(nil, newFakeProjectService(), a, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/overview", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateSettingsAudited — PUT в POST/пути с аудитом: пишется event settings.updated.
func TestUpdateSettingsAudited(t *testing.T) {
	auditSvc := &fakeAuditService{}
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig(), auditSvc)
	req := authedRequest(http.MethodPut, "/api/v1/admin/settings", `{"min_password_length":8}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSettingsInvalidJSON(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPut, "/api/v1/admin/settings", `not-json`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserInvalidJSON(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2", `not-json`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Bearer API-ключ (EDR-0016 §7) ----

func TestRequireAuthAcceptsApiKey(t *testing.T) {
	router := NewRouter(stair.NewService(), newFakeProjectService(), newFakeAuth(), DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid api key, got %d", rec.Code)
	}
}

func TestRequireAuthRejectsInvalidApiKey(t *testing.T) {
	a := newFakeAuth()
	router := NewRouter(stair.NewService(), nil, a, DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid api key, got %d", rec.Code)
	}
}

// ---- Bearer по scopes ключа (hasPermission) ----

func TestAdminEndpointAllowedByApiKeyScope(t *testing.T) {
	// fakeAuth.AuthenticateApiKey возвращает ключ с правом users.list.
	router := NewRouter(nil, nil, newFakeAuth(), DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminEndpointDeniedByApiKeyWithoutScope(t *testing.T) {
	// testAuth.AuthenticateApiKey возвращает ключ только с users.list —
	// data.export отсутствует → 403.
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/export?scope=users", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for key without data.export scope, got %d", rec.Code)
	}
}
