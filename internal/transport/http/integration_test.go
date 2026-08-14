package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/database"
)

// integrationRouter собирает реальный стек: PostgreSQL (по
// STAIR_TEST_DATABASE_URL), application/project.Service и транспортный
// роутер. Без переменной окружения тест пропускается (как и в
// infrastructure/database).
func integrationRouter(t *testing.T) http.Handler {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping HTTP+DB integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, database.DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool, "../../../migrations", "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	svc := project.NewService(
		database.NewProjectRepository(pool),
		stair.NewService(),
		project.DefaultRules(),
	)
	return NewRouter(stair.NewService(), svc)
}

// TestIntegrationCriticalFlow — сквозной поток через HTTP поверх реальной
// БД: создание проекта → расчёт → чтение → экспорт. Проверяет, что
// снапшот сохраняется целиком (включая mesh) и отдаётся в /export.
func TestIntegrationCriticalFlow(t *testing.T) {
	router := integrationRouter(t)

	// 1. Создание проекта.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/projects",
		strings.NewReader(`{"name": "Интеграционный поток", "description": "HTTP+DB"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("create decode: %v", err)
	}
	if p.ID == "" || p.Status != "draft" {
		t.Fatalf("unexpected project: %+v", p)
	}

	// 2. Расчёт референса (n=15, валидный конвейер с производственным
	// пакетом и mesh).
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+p.ID+"/calculate",
		strings.NewReader(referenceJSON)))
	if rec.Code != http.StatusOK {
		t.Fatalf("calculate: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var c calculationDTO
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatalf("calculate decode: %v", err)
	}

	var snap project.Snapshot
	if err := json.Unmarshal(c.Result, &snap); err != nil {
		t.Fatalf("snapshot unmarshal: %v", err)
	}
	if !snap.Validation.Valid || snap.Validation.Blocking {
		t.Fatalf("expected valid snapshot, got %+v", snap.Validation)
	}
	if snap.ProjectID != p.ID {
		t.Fatalf("project id = %s, want %s", snap.ProjectID, p.ID)
	}
	if snap.Mesh == nil || len(snap.Mesh.Vertices) == 0 || len(snap.Mesh.Triangles) == 0 {
		t.Fatal("snapshot must persist full mesh (vertices+triangles)")
	}
	if snap.Manufacturing == nil || len(snap.Manufacturing.Parts) == 0 {
		t.Fatal("snapshot must persist manufacturing package")
	}
	if snap.Pricing == nil || snap.Pricing.FinalPrice.Major(snap.Pricing.Currency) <= 0 {
		t.Fatal("snapshot must persist pricing")
	}

	// 3. Чтение проекта по ID.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+p.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rec.Code)
	}
	var got projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("get decode: %v", err)
	}
	if got.Name != "Интеграционный поток" {
		t.Fatalf("name = %q", got.Name)
	}

	// 4. Экспортный документ — тот же снапшот с mesh.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+p.ID+"/export", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("export: expected 200, got %d", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("expected attachment content-disposition, got %q", cd)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("export must be valid JSON: %v", err)
	}
	if _, ok := doc["project_id"]; !ok {
		t.Fatal("export must contain project_id")
	}
	if _, ok := doc["mesh"]; !ok {
		t.Fatal("export must contain mesh snapshot")
	}
}
