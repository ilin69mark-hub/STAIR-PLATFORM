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
	u := &auth.User{Name: "Owner", Email: fmt.Sprintf("owner-%d@test.dev", time.Now().UnixNano())}
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

	p := &project.Project{Name: "Интеграционный", Description: "тест", Status: project.StatusDraft}
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

	p := &project.Project{Name: "Расчёт", Status: project.StatusDraft}
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
	u2 := &auth.User{Name: "Editor", Email: fmt.Sprintf("ed-%d@test.dev", time.Now().UnixNano()),
		TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	p := &project.Project{Name: "Совместный", Status: project.StatusDraft}
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

// TestProjectRepositoryHasConfigAccess (S-132c): член проекта имеет доступ
// к его конфигурации; не-член — нет.
func TestProjectRepositoryHasConfigAccess(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	ar := NewAuthRepository(repo.pool)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	u2 := &auth.User{Name: "Member", Email: fmt.Sprintf("mem-%d@test.dev", time.Now().UnixNano()),
		TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	p := &project.Project{Name: "S-132c доступ", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	cfg := &project.StairConfiguration{
		ProjectID: p.ID, WidthMM: 900, HeightMM: 2700, Flight: "straight",
		StepHeightMM: 180, StringerThicknessMM: 50, StepThicknessMM: 40,
		ClearanceMM: 80, RailingHeightMM: 900, ComfortStepMM: 620,
	}
	if _, err := repo.SaveCalculationWithConfig(ctx, tenant, cfg, sampleSnapshot(p.ID)); err != nil {
		t.Fatalf("SaveCalculationWithConfig: %v", err)
	}

	// Владелец — автоматический член проекта.
	ok, err := repo.HasConfigAccess(ctx, owner, cfg.ID)
	if err != nil {
		t.Fatalf("HasConfigAccess(owner): %v", err)
	}
	if !ok {
		t.Fatal("owner should have config access")
	}

	// Не-член — нет.
	ok, err = repo.HasConfigAccess(ctx, u2.ID, cfg.ID)
	if err != nil {
		t.Fatalf("HasConfigAccess(outsider): %v", err)
	}
	if ok {
		t.Fatal("non-member must not have config access")
	}

	// После добавления в проект — доступ есть.
	if err := repo.AddMember(ctx, tenant, p.ID, &project.ProjectMember{ProjectID: p.ID, UserID: u2.ID, Role: project.RoleEditor}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	ok, err = repo.HasConfigAccess(ctx, u2.ID, cfg.ID)
	if err != nil {
		t.Fatalf("HasConfigAccess(member): %v", err)
	}
	if !ok {
		t.Fatal("member should have config access")
	}

	// Несуществующая конфигурация — false без ошибки.
	ok, err = repo.HasConfigAccess(ctx, owner, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("HasConfigAccess(missing): %v", err)
	}
	if ok {
		t.Fatal("missing config must not grant access")
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

	p := &project.Project{Name: "С защитой владельца", Status: project.StatusDraft}
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

	email := fmt.Sprintf("invitee-%d@test.dev", time.Now().UnixNano())
	u2 := &auth.User{Name: "Invitee", Email: email, TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	p := &project.Project{Name: "Приглашение по email", Status: project.StatusDraft}
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

// TestProjectRepositoryComments — CRUD комментариев (EDR-0009):
// добавление, список по времени, удаление автором/владельцем.
func TestProjectRepositoryComments(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	ar := NewAuthRepository(repo.pool)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	// Второй пользователь для комментария.
	u2 := &auth.User{Name: "Editor", Email: fmt.Sprintf("cmt-%d@test.dev", time.Now().UnixNano()),
		TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	p := &project.Project{Name: "С комментариями", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Член для возможности удаления «чужим».
	if err := repo.AddMember(ctx, tenant, p.ID, &project.ProjectMember{ProjectID: p.ID, UserID: u2.ID, Role: project.RoleEditor}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	c1, err := repo.AddComment(ctx, tenant, p.ID, &project.Comment{ProjectID: p.ID, AuthorID: owner, Body: "первый"})
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	c2, err := repo.AddComment(ctx, tenant, p.ID, &project.Comment{ProjectID: p.ID, AuthorID: u2.ID, Body: "второй"})
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if c1.ID == "" || c2.ID == "" {
		t.Fatal("expected assigned comment IDs")
	}

	list, err := repo.ListComments(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(list) != 2 || list[0].Body != "первый" || list[1].Body != "второй" {
		t.Fatalf("list = %+v", list)
	}

	// Чужой (не автор, не владелец) удалить не может → ErrNotFound.
	if err := repo.DeleteComment(ctx, tenant, p.ID, c1.ID, u2.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-author delete, got %v", err)
	}
	// Автор удаляет свой.
	if err := repo.DeleteComment(ctx, tenant, p.ID, c2.ID, u2.ID); err != nil {
		t.Fatalf("author delete: %v", err)
	}
	// Владелец удаляет чужой.
	if err := repo.DeleteComment(ctx, tenant, p.ID, c1.ID, owner); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	// Повторное удаление — ErrNotFound.
	if err := repo.DeleteComment(ctx, tenant, p.ID, c1.ID, owner); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing comment, got %v", err)
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

	p := &project.Project{Name: "Самостоятельные сохранения", Status: project.StatusDraft}
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

// TestProjectRepositoryReviews (EDR-0010): атомарный переход статуса +
// строка ревью; request из draft → in_review, sign-off → approved;
// self-approve запрещён; чужой tenant — ErrNotFound.
func TestProjectRepositoryReviews(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	ar := NewAuthRepository(repo.pool)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	u2 := &auth.User{Name: "Editor", Email: fmt.Sprintf("rv-%d@test.dev", time.Now().UnixNano()),
		TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	p := &project.Project{Name: "С ревью", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := repo.AddMember(ctx, tenant, p.ID, &project.ProjectMember{ProjectID: p.ID, UserID: u2.ID, Role: project.RoleEditor}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// Editor запрашивает ревью.
	rv, err := repo.RequestReview(ctx, tenant, p.ID, u2.ID, "проверьте")
	if err != nil {
		t.Fatalf("RequestReview: %v", err)
	}
	if rv.ID == "" || rv.Decision != project.ReviewRequested || rv.ReviewerID != "" || rv.DecidedAt != nil {
		t.Fatalf("review = %+v", rv)
	}
	got, err := repo.GetProject(ctx, tenant, owner, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Status != project.StatusInReview {
		t.Fatalf("status = %q, want in_review", got.Status)
	}

	// Self-approve запрещён (автор = подписант).
	if _, err := repo.DecideReview(ctx, tenant, p.ID, rv.ID, u2.ID, project.ReviewApproved, ""); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("self-approve: want ErrForbidden, got %v", err)
	}

	// Owner подписывает.
	done, err := repo.DecideReview(ctx, tenant, p.ID, rv.ID, owner, project.ReviewApproved, "ок")
	if err != nil {
		t.Fatalf("DecideReview: %v", err)
	}
	if done.Decision != project.ReviewApproved || done.ReviewerID != owner || done.DecidedAt == nil {
		t.Fatalf("signed = %+v", done)
	}
	got, err = repo.GetProject(ctx, tenant, owner, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Status != project.StatusApproved {
		t.Fatalf("status = %q, want approved", got.Status)
	}

	// Повторное решение по уже решённому ревью — ErrNotFound.
	if _, err := repo.DecideReview(ctx, tenant, p.ID, rv.ID, owner, project.ReviewChangesRequest, ""); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("double decide: want ErrNotFound, got %v", err)
	}

	// История.
	list, err := repo.ListReviews(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("ListReviews: %v", err)
	}
	if len(list) != 1 || list[0].Decision != project.ReviewApproved {
		t.Fatalf("list = %+v", list)
	}

	// Чужой tenant не видит.
	foreign := &project.Project{Name: "Чужой", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, foreign); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := repo.RequestReview(ctx, "00000000-0000-0000-0000-000000000000", foreign.ID, owner, "x"); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign tenant request: want ErrNotFound, got %v", err)
	}
}

// testConfigAt возвращает конфигурацию проекта с заданной шириной.
func testConfigAt(projectID string, width float64) *project.StairConfiguration {
	return &project.StairConfiguration{
		ProjectID: projectID, WidthMM: width, HeightMM: 2700, Flight: "straight",
		StepHeightMM: 180, StringerThicknessMM: 50, StepThicknessMM: 40,
		ClearanceMM: 80, RailingHeightMM: 900, ComfortStepMM: 620,
	}
}

// TestProjectRepositoryVersioning (EDR-0012): ревизии монотонны и
// иммутабельны; текущая ревизия управляется расчётом и restore.
func TestProjectRepositoryVersioning(t *testing.T) {
	if os.Getenv("STAIR_TEST_DATABASE_URL") == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := integrationRepo(t)
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	p := &project.Project{Name: "Версии", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	calc1, err := repo.SaveCalculationWithConfig(ctx, tenant, testConfigAt(p.ID, 900), sampleSnapshot(p.ID))
	if err != nil {
		t.Fatalf("save #1: %v", err)
	}
	calc2, err := repo.SaveCalculationWithConfig(ctx, tenant, testConfigAt(p.ID, 1000), sampleSnapshot(p.ID))
	if err != nil {
		t.Fatalf("save #2: %v", err)
	}
	if calc1.ConfigurationID == calc2.ConfigurationID {
		t.Fatal("expected distinct revisions")
	}

	list, err := repo.ListConfigurations(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("ListConfigurations: %v", err)
	}
	if len(list) != 2 || list[0].Revision != 1 || list[1].Revision != 2 {
		t.Fatalf("revisions = %+v", list)
	}
	if list[0].ID != calc1.ConfigurationID || list[1].ID != calc2.ConfigurationID {
		t.Fatalf("revision order: %+v", list)
	}

	got, err := repo.GetConfigurationByID(ctx, tenant, p.ID, calc1.ConfigurationID)
	if err != nil {
		t.Fatalf("GetConfigurationByID: %v", err)
	}
	if got.Revision != 1 || got.WidthMM != 900 {
		t.Fatalf("config = %+v", got)
	}

	// Текущая — последняя.
	cur, err := repo.GetLatestConfiguration(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("GetLatestConfiguration: %v", err)
	}
	if cur.ID != calc2.ConfigurationID {
		t.Fatalf("current = %s, want %s", cur.ID, calc2.ConfigurationID)
	}

	// Restore rev1 → становится текущей.
	if err := repo.RestoreConfiguration(ctx, tenant, p.ID, calc1.ConfigurationID); err != nil {
		t.Fatalf("RestoreConfiguration: %v", err)
	}
	cur, err = repo.GetLatestConfiguration(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("GetLatestConfiguration after restore: %v", err)
	}
	if cur.ID != calc1.ConfigurationID {
		t.Fatalf("current after restore = %s, want %s", cur.ID, calc1.ConfigurationID)
	}

	// Чужой tenant не видит ревизии.
	if _, err := repo.ListConfigurations(ctx, "00000000-0000-0000-0000-000000000000", p.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign list: want ErrNotFound, got %v", err)
	}
	if err := repo.RestoreConfiguration(ctx, "00000000-0000-0000-0000-000000000000", p.ID, calc1.ConfigurationID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign restore: want ErrNotFound, got %v", err)
	}
	if _, err := repo.GetConfigurationByID(ctx, "00000000-0000-0000-0000-000000000000", p.ID, calc1.ConfigurationID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign get: want ErrNotFound, got %v", err)
	}
}
