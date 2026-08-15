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

	"stairplatform/internal/application/integrations"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

// fakeIntegrationService — тестовая реализация IntegrationService.
type fakeIntegrationService struct {
	endpoints   []*integrations.Endpoint
	registerErr error
	listErr     error
	sendErr     error
	sent        *integrations.Delivery
}

func newFakeIntegrationService() *fakeIntegrationService {
	return &fakeIntegrationService{}
}

func (f *fakeIntegrationService) RegisterEndpoint(_ context.Context, _, name, kind, url, secret string) (*integrations.Endpoint, error) {
	if f.registerErr != nil {
		return nil, f.registerErr
	}
	e := &integrations.Endpoint{ID: "ep-1", Name: name, Kind: integrations.Kind(kind), URL: url, SecretEnc: secret}
	f.endpoints = append(f.endpoints, e)
	return e, nil
}
func (f *fakeIntegrationService) ListEndpoints(_ context.Context, _ string) ([]*integrations.Endpoint, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.endpoints, nil
}
func (f *fakeIntegrationService) GetEndpoint(_ context.Context, _, id string) (*integrations.Endpoint, error) {
	for _, e := range f.endpoints {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, integrations.ErrNotFound
}
func (f *fakeIntegrationService) DeleteEndpoint(_ context.Context, _, id string) error {
	for i, e := range f.endpoints {
		if e.ID == id {
			f.endpoints = append(f.endpoints[:i], f.endpoints[i+1:]...)
			return nil
		}
	}
	return integrations.ErrNotFound
}
func (f *fakeIntegrationService) SendQuote(_ context.Context, _, projectID string, payload []byte) (*integrations.Delivery, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	d := &integrations.Delivery{ID: "del-1", EndpointID: "ep-1", ProjectID: projectID,
		EventType: integrations.EventTypeQuoteSend, Status: integrations.StatusPending, Payload: payload}
	f.sent = d
	return d, nil
}

func (f *fakeIntegrationService) SyncProject(_ context.Context, _, projectID string, payload []byte) (*integrations.Delivery, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	d := &integrations.Delivery{ID: "del-2", EndpointID: "ep-crm", ProjectID: projectID,
		EventType: integrations.EventTypeProjectSync, Status: integrations.StatusPending, Payload: payload}
	f.sent = d
	return d, nil
}

// testRouterWithIntegrations собирает роутер с integration-сервисом.
func testRouterWithIntegrations(p ProjectService, i IntegrationService) http.Handler {
	cfg := DefaultConfig()
	cfg.Integrations = i
	return NewRouter(stair.NewService(), p, adminAuth{}, cfg)
}

func integrationsTestSetup() (*fakeIntegrationService, *fakeProjectService) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	projects.calc = &project.Calculation{ID: "c-1", ProjectID: "p-1", Result: []byte(`{"project_id":"p-1"}`)}
	return newFakeIntegrationService(), projects
}

func TestListEndpointsAdmin(t *testing.T) {
	i, _ := integrationsTestSetup()
	i.endpoints = []*integrations.Endpoint{
		{ID: "ep-1", Name: "ERP", Kind: integrations.KindERP, URL: "https://erp.example.com"},
	}
	router := testRouterWithIntegrations(nil, i)
	req := authedRequest(http.MethodGet, "/api/v1/integrations/endpoints", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []endpointDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 || list[0].Name != "ERP" {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestCreateEndpointAdmin(t *testing.T) {
	i, _ := integrationsTestSetup()
	router := testRouterWithIntegrations(nil, i)
	body := `{"name":"ERP Kanban","kind":"erp","url":"https://erp.acme.com/hook","secret":"s3cr"}`
	req := authedRequest(http.MethodPost, "/api/v1/integrations/endpoints", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var ep endpointDTO
	if err := json.NewDecoder(rec.Body).Decode(&ep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ep.Name != "ERP Kanban" || ep.Kind != "erp" {
		t.Fatalf("unexpected endpoint: %+v", ep)
	}
}

func TestCreateEndpointAdminInvalid(t *testing.T) {
	i, _ := integrationsTestSetup()
	i.registerErr = integrations.ErrInvalid
	router := testRouterWithIntegrations(nil, i)
	req := authedRequest(http.MethodPost, "/api/v1/integrations/endpoints", `{"name":"","kind":"erp"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestDeleteEndpointAdmin(t *testing.T) {
	i, _ := integrationsTestSetup()
	i.endpoints = []*integrations.Endpoint{{ID: "ep-1", Name: "ERP", Kind: integrations.KindERP}}
	router := testRouterWithIntegrations(nil, i)
	req := authedRequest(http.MethodDelete, "/api/v1/integrations/endpoints/ep-1", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestQuoteSend(t *testing.T) {
	i, projects := integrationsTestSetup()
	d := &integrations.Delivery{ID: "del-1", EndpointID: "ep-1", EventType: integrations.EventTypeQuoteSend, Status: integrations.StatusPending}
	i.sent = d
	router := testRouterWithIntegrations(projects, i)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/quote-send", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var out deliveryDTO
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != "del-1" || out.Status != "pending" {
		t.Fatalf("unexpected delivery DTO: %+v", out)
	}
}

func TestQuoteSendNoCalculation(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.getErr = errors.New("no calc") // GetResult вернёт ошибку → 500
	router := testRouterWithIntegrations(projects, i)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/quote-send", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestQuoteSendNoEndpoint(t *testing.T) {
	i, projects := integrationsTestSetup()
	i.sendErr = integrations.ErrNoEndpoint
	router := testRouterWithIntegrations(projects, i)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/quote-send", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no_endpoint") {
		t.Fatalf("expected error code no_endpoint: %s", rec.Body.String())
	}
}

func TestProjectSync(t *testing.T) {
	i, projects := integrationsTestSetup()
	projects.projects["p-1"].CreatedAt = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	projects.projects["p-1"].UpdatedAt = time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	router := testRouterWithIntegrations(projects, i)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/crm-sync", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var out deliveryDTO
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != "del-2" || out.EventType != "crm.project_sync" || out.ProjectID != "p-1" {
		t.Fatalf("unexpected delivery DTO: %+v", out)
	}
	if i.sent == nil || len(i.sent.Payload) == 0 {
		t.Fatal("expected CRM payload sent to service")
	}
	var doc crmProjectDocument
	if err := json.Unmarshal(i.sent.Payload, &doc); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if doc.Name != "А" || doc.Status != "draft" || doc.ProjectID != "p-1" ||
		doc.CreatedAt != "2026-08-01T12:00:00Z" || doc.UpdatedAt != "2026-08-02T12:00:00Z" {
		t.Fatalf("unexpected CRM document: %+v", doc)
	}
}

func TestProjectSyncNotFound(t *testing.T) {
	i, projects := integrationsTestSetup()
	delete(projects.projects, "p-1")
	router := testRouterWithIntegrations(projects, i)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/crm-sync", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProjectSyncNoEndpoint(t *testing.T) {
	i, projects := integrationsTestSetup()
	i.sendErr = integrations.ErrNoEndpoint
	router := testRouterWithIntegrations(projects, i)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/crm-sync", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no_endpoint") {
		t.Fatalf("expected error code no_endpoint: %s", rec.Body.String())
	}
}
