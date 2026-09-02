package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

// fakeAuditService — тестовая реализация AuditService.
type fakeAuditService struct {
	events     []*audit.Event
	projectErr error
	tenantErr  error
}

func (f *fakeAuditService) ListProjectAudit(_ context.Context, _, _ string) ([]*audit.Event, error) {
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	return f.events, nil
}

func (f *fakeAuditService) ListTenantAudit(_ context.Context, _ string) ([]*audit.Event, error) {
	if f.tenantErr != nil {
		return nil, f.tenantErr
	}
	return f.events, nil
}

func (f *fakeAuditService) Record(_ context.Context, _ *audit.Event) error {
	return nil
}

// testRouterWithAudit собирает роутер с fake-проектами и fake-аудитом.
func testRouterWithAudit(p ProjectService, a AuditService) http.Handler {
	return NewRouter(stair.NewService(), p, testAuth{}, DefaultConfig(), a)
}

func TestListProjectAudit(t *testing.T) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "Аудит", Status: "draft"}
	auditSvc := &fakeAuditService{events: []*audit.Event{
		{ID: "e1", TenantID: "t-1", ProjectID: "p-1", Action: audit.ActionProjectCreated, Result: audit.ResultOK},
	}}
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/audit", "")
	rec := httptest.NewRecorder()
	testRouterWithAudit(projects, auditSvc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "project.created") {
		t.Fatalf("missing action in body: %s", rec.Body.String())
	}
}

func TestListProjectAuditNotFound(t *testing.T) {
	projects := newFakeProjectService()
	auditSvc := &fakeAuditService{}
	req := authedRequest(http.MethodGet, "/api/v1/projects/missing/audit", "")
	rec := httptest.NewRecorder()
	testRouterWithAudit(projects, auditSvc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// testRouterWithAuditNoAuth — глобальный аудит без аудита (nil) не регистрируется.
func TestListTenantAuditRequiresAdmin(t *testing.T) {
	auditSvc := &fakeAuditService{}
	req := authedRequest(http.MethodGet, "/api/v1/audit", "")
	rec := httptest.NewRecorder()
	testRouterWithAudit(nil, auditSvc).ServeHTTP(rec, req)

	// testAuth возвращает роль user → 403 (admin required).
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d: %s", rec.Code, rec.Body.String())
	}
}

// testRouterWithAuditRoutes: маршруты аудита не регистрируются без AuditService.
func TestAuditRoutesAbsentWithoutService(t *testing.T) {
	router := testRouterWithProjects(newFakeProjectService())
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/audit", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without audit service, got %d", rec.Code)
	}
}

// adminAuth — аутентификация с ролью admin (для глобального аудита).
type adminAuth struct{ testAuth }

func (adminAuth) Authenticate(ctx context.Context, token string) (*auth.User, string, error) {
	return &auth.User{ID: "u-admin", Email: "admin@example.com", Role: auth.RoleAdmin, TenantID: "t-1"}, "", nil
}

func TestListTenantAuditAsAdmin(t *testing.T) {
	auditSvc := &fakeAuditService{events: []*audit.Event{
		{ID: "e1", TenantID: "t-1", Action: audit.ActionAuthLogin, Result: audit.ResultOK},
	}}
	router := NewRouter(stair.NewService(), nil, adminAuth{}, DefaultConfig(), auditSvc)
	req := authedRequest(http.MethodGet, "/api/v1/audit", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "auth.login") {
		t.Fatalf("missing action in body: %s", rec.Body.String())
	}
}
