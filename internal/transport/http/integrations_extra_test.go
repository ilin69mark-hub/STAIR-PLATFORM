package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

func integrationRouterWithAuth(p ProjectService, i IntegrationService, a AuthService) http.Handler {
	cfg := DefaultConfig()
	cfg.Integrations = i
	return NewRouter(stair.NewService(), p, a, cfg)
}

type fakeIntegrationDeleteError struct {
	fakeIntegrationService
}

func (f *fakeIntegrationDeleteError) DeleteEndpoint(_ context.Context, _, _ string) error {
	return errors.New("boom")
}

func TestListEndpointsForbidden(t *testing.T) {
	router := integrationRouterWithAuth(nil, newFakeIntegrationService(), testAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/integrations/endpoints", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListEndpointsInternalError(t *testing.T) {
	i := newFakeIntegrationService()
	i.listErr = errors.New("boom")
	router := integrationRouterWithAuth(nil, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/integrations/endpoints", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListEndpointsEmpty(t *testing.T) {
	i := newFakeIntegrationService()
	router := integrationRouterWithAuth(nil, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/integrations/endpoints", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []endpointDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestCreateEndpointForbidden(t *testing.T) {
	router := integrationRouterWithAuth(nil, newFakeIntegrationService(), testAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/integrations/endpoints",
		`{"name":"x","kind":"erp"}`))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateEndpointBadJSON(t *testing.T) {
	i := newFakeIntegrationService()
	router := integrationRouterWithAuth(nil, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/integrations/endpoints", `{bad`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateEndpointInternalError(t *testing.T) {
	i := newFakeIntegrationService()
	i.registerErr = errors.New("boom")
	router := integrationRouterWithAuth(nil, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/integrations/endpoints",
		`{"name":"x","kind":"erp"}`))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteEndpointForbidden(t *testing.T) {
	router := integrationRouterWithAuth(nil, newFakeIntegrationService(), testAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/integrations/endpoints/ep-1", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteEndpointNotFound(t *testing.T) {
	i := newFakeIntegrationService()
	router := integrationRouterWithAuth(nil, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/integrations/endpoints/missing", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteEndpointInternalError(t *testing.T) {
	router := integrationRouterWithAuth(nil, &fakeIntegrationDeleteError{}, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodDelete, "/api/v1/integrations/endpoints/ep-1", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestQuoteSendNotFound(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getErr = project.ErrNotFound
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/quote-send", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestQuoteSendForbidden(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getErr = project.ErrForbidden
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/quote-send", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestQuoteSendInternalError(t *testing.T) {
	i, projects := integrationsTestSetup()
	i.sendErr = errors.New("boom")
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/quote-send", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectSyncForbidden(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getForbidden = true
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/crm-sync", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectSyncInternalError(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getProjectErr = errors.New("boom")
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/crm-sync", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectSyncSendInternalError(t *testing.T) {
	i, projects := integrationsTestSetup()
	i.sendErr = errors.New("boom")
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/crm-sync", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrderSendNotFound(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getErr = project.ErrNotFound
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/order-send", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrderSendForbidden(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getErr = project.ErrForbidden
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/order-send", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrderSendGetResultError(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getErr = errors.New("boom")
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/order-send", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrderSendBadResultJSON(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.calc = &project.Calculation{ID: "c-1", ProjectID: "p-1", Result: []byte(`not json`)}
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/order-send", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrderSendInternalError(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.calc = &project.Calculation{ID: "c-1", ProjectID: "p-1", Result: mesSnapshotResult(t)}
	i.sendErr = errors.New("boom")
	router := integrationRouterWithAuth(projects, i, adminAuth{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/projects/p-1/order-send", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "internal") {
		t.Fatalf("expected internal error body: %s", rec.Body.String())
	}
}
