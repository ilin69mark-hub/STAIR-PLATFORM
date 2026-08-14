package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"stairplatform/internal/application/auth"
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

// testTenantID возвращает/создаёт дефолтный tenant для интеграционных тестов.
func testTenantID(t *testing.T, repo *ProjectRepository) string {
	t.Helper()
	ctx := context.Background()
	ar := NewAuthRepository(repo.pool)
	ten, err := ar.DefaultTenant(ctx)
	if err != nil {
		t.Fatalf("DefaultTenant: %v", err)
	}
	return ten.ID
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

// testOwnerID — ID владельца в интеграционных тестах (tenant test).
func testOwnerID(t *testing.T, repo *ProjectRepository, tenantID string) string {
	t.Helper()
	ctx := context.Background()
	u := &auth.User{Name: "Owner", Email: fmt.Sprintf("owner-%d@test.dev", time.Now().UnixNano()%100000)}
	u.TenantID = tenantID
	u.Role = auth.RoleUser
	u.Status = auth.StatusActive
	ar := NewAuthRepository(repo.pool)
	if err := ar.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u.ID
}

func TestProjectRepositoryCRUD(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	p := &project.Project{Name: "Интеграционный", Description: "тест"}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected assigned UUID")
	}
	if p.OwnerID != owner {
		t.Fatalf("owner = %q, want %q", p.OwnerID, owner)
	}

	got, err := repo.GetProject(ctx, tenant, owner, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Интеграционный" {
		t.Fatalf("name = %q", got.Name)
	}

	list, err := repo.ListProjects(ctx, tenant, owner)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one project")
	}

	if _, err := repo.GetProject(ctx, tenant, owner, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, project.ErrNotFound) {
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
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	p := &project.Project{Name: "Расчёт"}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	cfg := &project.StairConfiguration{
		ProjectID: p.ID, WidthMM: 900, HeightMM: 2700, Flight: "straight",
		StepHeightMM: 180, StringerThicknessMM: 50, StepThicknessMM: 40,
		ClearanceMM: 80, RailingHeightMM: 900, ComfortStepMM: 620,
	}
	snap := sampleSnapshot(p.ID)

	calc, err := repo.SaveCalculationWithConfig(ctx, tenant, cfg, snap)
	if err != nil {
		t.Fatalf("SaveCalculationWithConfig: %v", err)
	}
	if cfg.ID == "" || calc.ID == "" {
		t.Fatal("expected assigned IDs")
	}
	if calc.ConfigurationID != cfg.ID {
		t.Fatalf("calculation not linked to config: %s vs %s", calc.ConfigurationID, cfg.ID)
	}

	latest, err := repo.GetLatestCalculation(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("GetLatestCalculation: %v", err)
	}
	if latest.Valid != true || latest.Blocking != false {
		t.Fatalf("calc flags = %v/%v", latest.Valid, latest.Blocking)
	}
	if len(latest.Result) == 0 || latest.Result[0] != '{' {
		t.Fatalf("result must be stored as JSON object, got %q", string(latest.Result))
	}

	conf, err := repo.GetLatestConfiguration(ctx, tenant, p.ID)
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
	tenant := testTenantID(t, repo)

	missing := "00000000-0000-0000-0000-000000000000"
	if _, err := repo.GetLatestConfiguration(ctx, tenant, missing); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("config: expected ErrNotFound, got %v", err)
	}
	if _, err := repo.GetLatestCalculation(ctx, tenant, missing); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("calc: expected ErrNotFound, got %v", err)
	}
}

// TestProjectRepositoryMembers — CRUD участников (EDR-0008) и защита owner:
// единственный владелец на проект, нельзя менять/удалять роль владельца.
func TestProjectRepositoryMembers(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	ar := NewAuthRepository(repo.pool)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	// Второй пользователь того же tenant.
	u2 := &auth.User{Name: "Editor", Email: fmt.Sprintf("ed-%d@test.dev", time.Now().UnixNano()%100000),
		TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	p := &project.Project{Name: "Совместный"}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// owner автоматически в составе.
	members, err := repo.ListMembers(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 || members[0].Role != project.RoleOwner {
		t.Fatalf("expected single owner member, got %+v", members)
	}

	// Добавить участника.
	if err := repo.AddMember(ctx, tenant, p.ID, &project.ProjectMember{ProjectID: p.ID, UserID: u2.ID, Role: project.RoleEditor}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	m, err := repo.GetMember(ctx, tenant, p.ID, u2.ID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if m.Role != project.RoleEditor {
		t.Fatalf("role = %q", m.Role)
	}

	// Обновить роль.
	if err := repo.UpdateMemberRole(ctx, tenant, p.ID, u2.ID, project.RoleViewer); err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}
	m, _ = repo.GetMember(ctx, tenant, p.ID, u2.ID)
	if m.Role != project.RoleViewer {
		t.Fatalf("role after update = %q", m.Role)
	}

	// Удалить участника.
	if err := repo.RemoveMember(ctx, tenant, p.ID, u2.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if _, err := repo.GetMember(ctx, tenant, p.ID, u2.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after removal, got %v", err)
	}
}

// TestProjectRepositoryCannotTouchOwner — защита владельца от изменения/удаления.
func TestProjectRepositoryCannotTouchOwner(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	p := &project.Project{Name: "С защитой владельца"}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := repo.UpdateMemberRole(ctx, tenant, p.ID, owner, project.RoleViewer); err == nil {
		t.Fatal("expected error changing owner role")
	}
	if err := repo.RemoveMember(ctx, tenant, p.ID, owner); err == nil {
		t.Fatal("expected error removing owner")
	}
}

// TestProjectRepositoryAddMemberByEmail — приглашение по email (C2):
// резолв в пользователя того же tenant; неизвестный email → ErrNotFound.
func TestProjectRepositoryAddMemberByEmail(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	ar := NewAuthRepository(repo.pool)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	email := fmt.Sprintf("invitee-%d@test.dev", time.Now().UnixNano()%100000)
	u2 := &auth.User{Name: "Invitee", Email: email, TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	p := &project.Project{Name: "Приглашение по email"}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := repo.AddMemberByEmail(ctx, tenant, p.ID, email, project.RoleEditor); err != nil {
		t.Fatalf("AddMemberByEmail: %v", err)
	}
	m, err := repo.GetMember(ctx, tenant, p.ID, u2.ID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if m.Role != project.RoleEditor {
		t.Fatalf("role = %q", m.Role)
	}
	if err := repo.AddMemberByEmail(ctx, tenant, p.ID, "nobody@nowhere.test", project.RoleViewer); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown email, got %v", err)
	}
}

func TestProjectRepositoryStandaloneConfigAndCalculation(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	p := &project.Project{Name: "Самостоятельные сохранения"}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Отдельное сохранение конфигурации (без расчёта).
	cfg := &project.StairConfiguration{
		ProjectID: p.ID, WidthMM: 1000, HeightMM: 2800, Flight: "straight",
		StepHeightMM: 175, StringerThicknessMM: 60, StepThicknessMM: 40,
		ClearanceMM: 90, RailingHeightMM: 950, ComfortStepMM: 0,
	}
	if err := repo.SaveConfiguration(ctx, cfg); err != nil {
		t.Fatalf("SaveConfiguration: %v", err)
	}
	if cfg.ID == "" {
		t.Fatal("expected assigned config ID")
	}

	// Отдельное сохранение расчёта (связан с конфигурацией).
	calc := &project.Calculation{
		ProjectID: p.ID, ConfigurationID: cfg.ID,
		Valid: true, Blocking: false, Result: []byte(`{"ok":true}`),
	}
	if err := repo.SaveCalculation(ctx, calc); err != nil {
		t.Fatalf("SaveCalculation: %v", err)
	}
	if calc.ID == "" {
		t.Fatal("expected assigned calculation ID")
	}

	got, err := repo.GetLatestCalculation(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("GetLatestCalculation: %v", err)
	}
	if got.ConfigurationID != cfg.ID || !got.Valid {
		t.Fatalf("unexpected calculation: %+v", got)
	}
}
