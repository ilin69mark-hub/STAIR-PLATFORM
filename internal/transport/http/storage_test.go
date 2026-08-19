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
	appstorage "stairplatform/internal/application/storage"
)

// fakeStorageService — тестовая реализация StorageService.
type fakeStorageService struct {
	objs      map[string][]byte
	saveErr   error
	loadErr   error
	deleteErr error
	saved     *appstorage.ExportRef
}

func newFakeStorageService() *fakeStorageService {
	return &fakeStorageService{objs: map[string][]byte{}}
}

func (f *fakeStorageService) SaveExport(_ context.Context, tenantID, category, filename string, data []byte, contentType string) (*appstorage.ExportRef, error) {
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	key := tenantID + "/" + category + "/" + filename
	f.objs[key] = data
	ref := &appstorage.ExportRef{Key: key, ContentType: contentType, Size: len(data)}
	f.saved = ref
	return ref, nil
}

func (f *fakeStorageService) Load(_ context.Context, tenantID, key string) ([]byte, string, error) {
	if f.loadErr != nil {
		return nil, "", f.loadErr
	}
	if !strings.HasPrefix(key, tenantID+"/") {
		return nil, "", errors.New("storage: object outside tenant scope")
	}
	data, ok := f.objs[key]
	if !ok {
		return nil, "", errors.New("storage: not found")
	}
	return data, "", nil
}

func (f *fakeStorageService) Delete(_ context.Context, tenantID, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if !strings.HasPrefix(key, tenantID+"/") {
		return errors.New("storage: object outside tenant scope")
	}
	if _, ok := f.objs[key]; !ok {
		return errors.New("storage: not found")
	}
	delete(f.objs, key)
	return nil
}

// testRouterWithStorage собирает роутер со storage-сервисом.
func testRouterWithStorage(p ProjectService, s StorageService) http.Handler {
	cfg := DefaultConfig()
	cfg.Storage = s
	return NewRouter(stair.NewService(), p, adminAuth{}, cfg)
}

func storageTestSetup() (*fakeStorageService, *fakeProjectService) {
	projects := newFakeProjectService()
	projects.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	projects.cadMesh = cadTestMesh()
	return newFakeStorageService(), projects
}

func TestStoreExportCAD(t *testing.T) {
	s, projects := storageTestSetup()
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/export/cad/store?format=dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var out exportRefDTO
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Key == "" || !strings.HasSuffix(out.Key, ".dxf") || out.Size == 0 || out.ContentType == "" {
		t.Fatalf("unexpected ref: %+v", out)
	}
	if _, ok := s.objs[out.Key]; !ok {
		t.Fatalf("object not stored under key %q", out.Key)
	}
}

func TestStoreExportCADNotFound(t *testing.T) {
	s, projects := storageTestSetup()
	delete(projects.projects, "p-1")
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/export/cad/store?format=dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStoreExportCADStorageError(t *testing.T) {
	s, projects := storageTestSetup()
	s.saveErr = errors.New("boom")
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/export/cad/store?format=dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetObject(t *testing.T) {
	s, projects := storageTestSetup()
	s.objs["t-1/cad/project-p-1.dxf"] = []byte("data")
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodGet, "/api/v1/storage/t-1/cad/project-p-1.dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "data" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestGetObjectForeignTenant(t *testing.T) {
	s, projects := storageTestSetup()
	s.objs["t-2/cad/other.dxf"] = []byte("x")
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodGet, "/api/v1/storage/t-2/cad/other.dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGetObjectNotFound(t *testing.T) {
	s, projects := storageTestSetup()
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodGet, "/api/v1/storage/t-1/cad/missing.dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteObject(t *testing.T) {
	s, projects := storageTestSetup()
	s.objs["t-1/cad/project-p-1.dxf"] = []byte("data")
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodDelete, "/api/v1/storage/t-1/cad/project-p-1.dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if _, ok := s.objs["t-1/cad/project-p-1.dxf"]; ok {
		t.Fatal("object not deleted")
	}
}

func TestDeleteObjectNotFound(t *testing.T) {
	s, projects := storageTestSetup()
	router := testRouterWithStorage(projects, s)
	req := authedRequest(http.MethodDelete, "/api/v1/storage/t-1/cad/missing.dxf", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
