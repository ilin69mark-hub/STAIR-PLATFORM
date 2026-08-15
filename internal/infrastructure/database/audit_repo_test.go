package database

import (
	"context"
	"os"
	"testing"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/project"
)

func integrationAuditRepo(t *testing.T) (*AuditRepository, *ProjectRepository) {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return NewAuditRepository(pool), NewProjectRepository(pool)
}

// TestAuditInsertAndList — вставка события и чтение по проекту/tenant.
func TestAuditInsertAndList(t *testing.T) {
	ar, repo := integrationAuditRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	p := &project.Project{Name: "Аудит", Description: "тест", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	e1 := &audit.Event{ActorID: owner, TenantID: tenant, ProjectID: p.ID, Action: audit.ActionProjectCreated, Result: audit.ResultOK, IP: "127.0.0.1"}
	e2 := &audit.Event{ActorID: owner, TenantID: tenant, ProjectID: p.ID, Action: audit.ActionProjectModified, Result: audit.ResultOK}
	for _, e := range []*audit.Event{e1, e2} {
		if err := ar.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	byProject, err := ar.ListByProject(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(byProject) != 2 {
		t.Fatalf("expected 2 project events, got %d", len(byProject))
	}
	if byProject[0].Action != audit.ActionProjectModified {
		t.Fatalf("expected newest first (ProjectModified), got %q", byProject[0].Action)
	}

	byTenant, err := ar.ListByTenant(ctx, tenant)
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	if len(byTenant) != 2 {
		t.Fatalf("expected 2 tenant events, got %d", len(byTenant))
	}
}

// TestAuditListForeignTenant — проект чужого tenant не виден (SEC-0005):
// ListByProject возвращает пусто, если проект не принадлежит tenant.
func TestAuditListForeignTenant(t *testing.T) {
	ar, repo := integrationAuditRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	p := &project.Project{Name: "Аудит чужой", Description: "тест", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := ar.Insert(ctx, &audit.Event{TenantID: tenant, ProjectID: p.ID, Action: audit.ActionProjectCreated}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Чужой tenant (не существует) не видит события проекта.
	byProject, err := ar.ListByProject(ctx, "00000000-0000-0000-0000-000000000000", p.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(byProject) != 0 {
		t.Fatalf("expected 0 events for foreign tenant, got %d", len(byProject))
	}
}
