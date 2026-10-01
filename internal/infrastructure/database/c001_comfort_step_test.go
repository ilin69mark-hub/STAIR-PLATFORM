package database

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/application/project"
	"stairplatform/internal/engine/validation"
)

// Регрессия CRITICAL-01 (2026-09-26): миграция 000031 ввела
// stair_configurations_positive_geometry с `> 0` по ВСЕМ геометрическим
// колонкам, включая три, где 0 — легальное доменное значение:
//
//	comfort_step_mm   = 0 → «взять solver.DefaultComfortStep»
//	clearance_mm      = 0 → «нет ограничения по просвету»
//	railing_height_mm = 0 → перил нет (RailingNone)
//
// Последствия были не теоретические:
//   - `make seed` падал с 23514 на штатной демо-конфигурации
//     (migrations/seeds/000001_demo_data.up.sql, clearance_mm = 0);
//   - существующий DB-тест TestConcurrentConfigurationRevision падал;
//   - обычный HTTP-запрос без необязательных полей доезжал до ошибки БД,
//     потому что project.Service.Calculate сохраняет конфигурацию независимо
//     от результата валидации (application/project/service.go:295).
//
// Тесты требуют STAIR_TEST_DATABASE_URL; без него БД-интеграция скипается.

// geometryConfig — конфигурация, взятая из реального пути сохранения: все
// обязательные поля заполнены, необязательные (comfort step) оставлены нулями.
func geometryConfig(projectID string) *project.StairConfiguration {
	return &project.StairConfiguration{
		ProjectID:           projectID,
		WidthMM:             900,
		HeightMM:            2700,
		Flight:              "straight",
		StepHeightMM:        180,
		StringerThicknessMM: 50,
		StepThicknessMM:     40,
		ClearanceMM:         0,
		RailingHeightMM:     0,
		ComfortStepMM:       0,
	}
}

func newGeometryTestProject(t *testing.T, ctx context.Context, pr *ProjectRepository, tenantID, name string) *project.Project {
	t.Helper()
	owner := testOwnerID(t, pr, tenantID)
	p := &project.Project{Name: name, Description: "t", Status: project.StatusDraft}
	if err := pr.CreateProject(ctx, tenantID, owner, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	return p
}

// TestSaveConfigWithZeroOptionalGeometrySucceeds — конфигурация без шага
// комфорта, без просвета и без перил обязана сохраняться и переживать
// round-trip. До фикса падала на stair_configurations_positive_geometry.
func TestSaveConfigWithZeroOptionalGeometrySucceeds(t *testing.T) {
	repo := integrationAI(t)
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenant := testTenantID(t, pr)
	p := newGeometryTestProject(t, ctx, pr, tenant, "Zero optional geometry")

	cfg := geometryConfig(p.ID)
	if _, err := pr.SaveCalculationWithConfig(ctx, tenant, cfg,
		project.Snapshot{Validation: validation.Result{Valid: true}}); err != nil {
		t.Fatalf("save config with zero comfort/clearance/railing must succeed, got: %v", err)
	}

	got, err := pr.GetConfigurationByID(ctx, tenant, p.ID, cfg.ID)
	if err != nil {
		t.Fatalf("read back config: %v", err)
	}
	if got.ComfortStepMM != 0 || got.ClearanceMM != 0 || got.RailingHeightMM != 0 {
		t.Errorf("optional geometry round-trip = comfort %v / clearance %v / railing %v, want all 0",
			got.ComfortStepMM, got.ClearanceMM, got.RailingHeightMM)
	}
	if got.Revision < 1 {
		t.Errorf("revision = %d, want >= 1", got.Revision)
	}
}

// TestDemoSeedConfigIsAccepted — ровно та конфигурация, которой заведена
// штатная демо-фикстура (clearance_mm = 0, outer_radius_mm = 0, room_* = 0).
// `make seed` падал на ней; CHECK обязан её принимать.
func TestDemoSeedConfigIsAccepted(t *testing.T) {
	repo := integrationAI(t)
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenant := testTenantID(t, pr)
	p := newGeometryTestProject(t, ctx, pr, tenant, "Demo seed shape")

	cfg := geometryConfig(p.ID)
	cfg.OuterRadiusMM = 0
	cfg.RoomWidthMM = 0
	cfg.RoomLengthMM = 0
	if _, err := pr.SaveCalculationWithConfig(ctx, tenant, cfg,
		project.Snapshot{Validation: validation.Result{Valid: true}}); err != nil {
		t.Fatalf("demo-seed-shaped config must be accepted by the geometry CHECK, got: %v", err)
	}
}

// TestConfigNegativeGeometryRejected — ослабление трёх колонок до >= 0 не
// должно было ослабить остальную геометрию: отрицательные обязательные
// размеры по-прежнему отвергаются БД.
func TestConfigNegativeGeometryRejected(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*project.StairConfiguration)
	}{
		{"width", func(c *project.StairConfiguration) { c.WidthMM = -900 }},
		{"height", func(c *project.StairConfiguration) { c.HeightMM = -2700 }},
		{"step_height", func(c *project.StairConfiguration) { c.StepHeightMM = -180 }},
		{"stringer_thickness", func(c *project.StairConfiguration) { c.StringerThicknessMM = -50 }},
		{"step_thickness", func(c *project.StairConfiguration) { c.StepThicknessMM = -40 }},
		{"clearance", func(c *project.StairConfiguration) { c.ClearanceMM = -1 }},
		{"railing_height", func(c *project.StairConfiguration) { c.RailingHeightMM = -1 }},
		{"comfort_step", func(c *project.StairConfiguration) { c.ComfortStepMM = -1 }},
	}

	repo := integrationAI(t)
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenant := testTenantID(t, pr)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newGeometryTestProject(t, ctx, pr, tenant, "Negative "+tc.name)
			cfg := geometryConfig(p.ID)
			tc.apply(cfg)
			_, err := pr.SaveCalculationWithConfig(ctx, tenant, cfg,
				project.Snapshot{Validation: validation.Result{Valid: true}})
			if err == nil {
				t.Fatalf("negative %s must be rejected by stair_configurations_positive_geometry", tc.name)
			}
			if !strings.Contains(err.Error(), "positive_geometry") &&
				!strings.Contains(err.Error(), "check constraint") {
				t.Errorf("want check-violation error, got: %v", err)
			}
		})
	}
}
