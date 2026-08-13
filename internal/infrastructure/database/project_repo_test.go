package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"stairplatform/internal/application/project"
	"stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/validation"
)

func integrationRepo(t *testing.T) *ProjectRepository {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Гарантируем применённые миграции (и текущую версию 000002).
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return NewProjectRepository(pool)
}

// sampleSnapshot возвращает тестовый снапшот с минимальным содержимым.
func sampleSnapshot(projectID string) project.Snapshot {
	return project.Snapshot{
		ProjectID: projectID,
		Validation: validation.Result{
			Valid:    true,
			Blocking: false,
		},
		Manufacturing: &manufacturing.ManufacturingPackage{},
		Pricing:       &pricing.PriceBreakdown{},
	}
}

func TestProjectRepositoryCRUD(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)

	p := &project.Project{Name: "Интеграционный", Description: "тест"}
	if err := repo.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected assigned UUID")
	}

	got, err := repo.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Интеграционный" {
		t.Fatalf("name = %q", got.Name)
	}

	list, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one project")
	}

	if _, err := repo.GetProject(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectRepositorySaveCalculationWithConfig(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)

	p := &project.Project{Name: "Расчёт"}
	if err := repo.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	cfg := &project.StairConfiguration{
		ProjectID: p.ID, WidthMM: 900, HeightMM: 2700, Flight: "straight",
		StepHeightMM: 180, StringerThicknessMM: 50, StepThicknessMM: 40,
		ClearanceMM: 80, RailingHeightMM: 900, ComfortStepMM: 620,
	}
	snap := sampleSnapshot(p.ID)

	calc, err := repo.SaveCalculationWithConfig(ctx, cfg, snap)
	if err != nil {
		t.Fatalf("SaveCalculationWithConfig: %v", err)
	}
	if cfg.ID == "" || calc.ID == "" {
		t.Fatal("expected assigned IDs")
	}
	if calc.ConfigurationID != cfg.ID {
		t.Fatalf("calculation not linked to config: %s vs %s", calc.ConfigurationID, cfg.ID)
	}

	latest, err := repo.GetLatestCalculation(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetLatestCalculation: %v", err)
	}
	if latest.Valid != true || latest.Blocking != false {
		t.Fatalf("calc flags = %v/%v", latest.Valid, latest.Blocking)
	}
	if len(latest.Result) == 0 || latest.Result[0] != '{' {
		t.Fatalf("result must be stored as JSON object, got %q", string(latest.Result))
	}

	conf, err := repo.GetLatestConfiguration(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetLatestConfiguration: %v", err)
	}
	if conf.WidthMM != 900 || conf.HeightMM != 2700 {
		t.Fatalf("config = %+v", conf)
	}
}

func TestProjectRepositoryNotFound(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)

	missing := "00000000-0000-0000-0000-000000000000"
	if _, err := repo.GetLatestConfiguration(ctx, missing); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("config: expected ErrNotFound, got %v", err)
	}
	if _, err := repo.GetLatestCalculation(ctx, missing); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("calc: expected ErrNotFound, got %v", err)
	}
}
