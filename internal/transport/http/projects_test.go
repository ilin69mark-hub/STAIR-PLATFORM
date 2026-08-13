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

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
)

// fakeProjectService — тестовая реализация ProjectService.
type fakeProjectService struct {
	projects     map[string]*project.Project
	calc         *project.Calculation
	createErr    error
	calculateErr error
	getErr       error
}

func newFakeProjectService() *fakeProjectService {
	return &fakeProjectService{projects: map[string]*project.Project{}}
}

func (f *fakeProjectService) CreateProject(ctx context.Context, name, description string) (*project.Project, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	p := &project.Project{ID: "p-1", Name: name, Description: description,
		Status: "draft", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.projects[p.ID] = p
	return p, nil
}

func (f *fakeProjectService) GetProject(ctx context.Context, id string) (*project.Project, error) {
	p, ok := f.projects[id]
	if !ok {
		return nil, project.ErrNotFound
	}
	return p, nil
}

func (f *fakeProjectService) ListProjects(ctx context.Context) ([]*project.Project, error) {
	out := make([]*project.Project, 0, len(f.projects))
	for _, p := range f.projects {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeProjectService) Calculate(ctx context.Context, projectID string, cfg stair.Config, opts stair.Options) (*project.Calculation, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.calculateErr != nil {
		return nil, f.calculateErr
	}
	return &project.Calculation{
		ID: "c-1", ProjectID: projectID, ConfigurationID: "cfg-1",
		Valid: true, Result: []byte(`{"project_id":"` + projectID + `"}`),
		CreatedAt: time.Now(),
	}, nil
}

func (f *fakeProjectService) GetResult(ctx context.Context, projectID string) (*project.Calculation, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.calc == nil {
		return nil, project.ErrNotFound
	}
	return f.calc, nil
}

func testRouterWithProjects(p ProjectService) http.Handler {
	return NewRouter(stair.NewService(), p)
}

func TestCreateProject(t *testing.T) {
	svc := newFakeProjectService()
	body := `{"name": "Лестница", "description": "описание"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "Лестница" || p.Description != "описание" || p.Status != "draft" {
		t.Fatalf("unexpected project: %+v", p)
	}
}

func TestCreateProjectInvalidJSON(t *testing.T) {
	svc := newFakeProjectService()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader("{bad"))
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetProject(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-42"] = &project.Project{ID: "p-42", Name: "А", Status: "draft"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p-42", nil)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "А" {
		t.Fatalf("name = %q", p.Name)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p-404", nil)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListProjects(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var list []projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
}

func TestCalculateProject(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p-1/calculate",
		strings.NewReader(referenceJSON))
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var c calculationDTO
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.CalculationID != "c-1" || c.ConfigurationID != "cfg-1" || !c.Valid {
		t.Fatalf("unexpected calculation: %+v", c)
	}
	if len(c.Result) == 0 {
		t.Fatal("expected result snapshot")
	}
}

func TestCalculateProjectNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p-404/calculate",
		strings.NewReader(referenceJSON))
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCalculateProjectInvalidJSON(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p-1/calculate",
		strings.NewReader("{bad"))
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestExportProject(t *testing.T) {
	svc := newFakeProjectService()
	svc.calc = &project.Calculation{
		ID: "c-1", ProjectID: "p-1",
		Result: []byte(`{"project_id":"p-1","validation":{"valid":true}}`),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p-1/export", nil)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("export must be valid JSON: %v", err)
	}
	if doc["project_id"] != "p-1" {
		t.Fatalf("project_id = %v", doc["project_id"])
	}
}

func TestExportProjectNoCalculation(t *testing.T) {
	svc := newFakeProjectService()
	svc.getErr = errors.New("boom")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p-1/export", nil)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
