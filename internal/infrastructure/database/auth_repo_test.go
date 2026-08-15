package database

import (
	"context"
	"os"
	"testing"

	"stairplatform/internal/application/auth"
)

func integrationAuthRepo(t *testing.T) (*AuthRepository, *ProjectRepository) {
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
	return NewAuthRepository(pool), NewProjectRepository(pool)
}

// TestAuthRepoListUsers — список пользователей tenant (EDR-0015 §3.4).
func TestAuthRepoListUsers(t *testing.T) {
	ar, repo := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	testOwnerID(t, repo, tenant)
	u2 := &auth.User{Name: "Member", Email: "member@test.dev", TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u2); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	users, err := ar.ListUsers(ctx, tenant)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users in tenant, got %d", len(users))
	}

	// Чужой tenant (пустой UUID) — пусто.
	foreign, err := ar.ListUsers(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("ListUsers foreign: %v", err)
	}
	if len(foreign) != 0 {
		t.Fatalf("expected 0 users for foreign tenant, got %d", len(foreign))
	}
}

// TestAuthRepoUpdateUserRole — смена роли и отсутствие перезаписи чужих tenant.
func TestAuthRepoUpdateUserRole(t *testing.T) {
	ar, repo := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	owner := testOwnerID(t, repo, tenant)

	if err := ar.UpdateUserRole(ctx, tenant, owner, auth.RoleAdmin); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}
	got, err := ar.GetUserByID(ctx, owner)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Role != auth.RoleAdmin {
		t.Fatalf("expected role admin, got %s", got.Role)
	}

	// Чужой tenant — ErrNotFound, роль не меняется.
	if err := ar.UpdateUserRole(ctx, "00000000-0000-0000-0000-000000000000", owner, auth.RoleUser); err != auth.ErrNotFound {
		t.Fatalf("expected ErrNotFound for foreign tenant, got %v", err)
	}
	got2, _ := ar.GetUserByID(ctx, owner)
	if got2.Role != auth.RoleAdmin {
		t.Fatalf("role must remain admin, got %s", got2.Role)
	}

	// Несуществующий пользователь — ErrNotFound.
	if err := ar.UpdateUserRole(ctx, tenant, "no-such-user", auth.RoleUser); err != auth.ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing user, got %v", err)
	}
}
