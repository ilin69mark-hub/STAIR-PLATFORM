package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"stairplatform/internal/application/auth"
)

// roleErrAuth — admin-аутентификация с управляемой ошибкой UpdateUser.
type roleErrAuth struct {
	adminAuth
	updateErr error
}

func (r *roleErrAuth) UpdateUser(ctx context.Context, tenantID, actorID, userID string, role *auth.Role, status *auth.Status) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return nil
}

func TestListUsersAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/users", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var users []userListDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) != 1 || users[0].ID != "u-2" {
		t.Fatalf("unexpected users: %+v", users)
	}
}

func TestListUsersAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/users", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRoleAsAdmin(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2",
		`{"role":"admin"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRoleInvalidRole(t *testing.T) {
	router := NewRouter(nil, nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2",
		`{"role":"superadmin"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRoleOwnRoleForbidden(t *testing.T) {
	a := &roleErrAuth{updateErr: auth.ErrForbidden}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-admin",
		`{"role":"user"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRoleNotFound(t *testing.T) {
	a := &roleErrAuth{updateErr: auth.ErrNotFound}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-missing",
		`{"role":"admin"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRoleAsNonAdmin(t *testing.T) {
	router := NewRouter(nil, nil, testAuth{}, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2",
		`{"role":"admin"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRoleInternalError(t *testing.T) {
	a := &roleErrAuth{updateErr: errors.New("boom")}
	router := NewRouter(nil, nil, a, DefaultConfig())
	req := authedRequest(http.MethodPatch, "/api/v1/admin/users/u-2",
		`{"role":"admin"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
