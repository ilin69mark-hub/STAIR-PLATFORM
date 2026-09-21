package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/auth"
)

// integrationCleanupRepo готовит AuthRepository+AuditRepository на тестовой БД.
func integrationCleanupRepo(t *testing.T) (*AuthRepository, *AuditRepository, *ProjectRepository) {
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
	pr := NewProjectRepository(pool)
	return NewAuthRepository(pool), NewAuditRepository(pool), pr
}

// TestDeleteExpiredSessions: удаляются только сессии с expires_at < before.
func TestDeleteExpiredSessions(t *testing.T) {
	ar, _, pr := integrationCleanupRepo(t)
	ctx := context.Background()

	tenant := testTenantID(t, pr)

	now := time.Now().UTC()
	u := &auth.User{Name: "Cleanup", Email: fmt.Sprintf("cleanup-%d@test.dev", time.Now().UnixNano()), TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Истёкшая сессия.
	expired := &auth.Session{UserID: u.ID, TokenHash: uniqueHash("expired"), ExpiresAt: now.Add(-time.Hour)}
	if err := ar.CreateSession(ctx, expired); err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	// Активная сессия.
	active := &auth.Session{UserID: u.ID, TokenHash: uniqueHash("active"), ExpiresAt: now.Add(24 * time.Hour)}
	if err := ar.CreateSession(ctx, active); err != nil {
		t.Fatalf("create active session: %v", err)
	}

	n, err := ar.DeleteExpiredSessions(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if n < 1 {
		t.Fatalf("deleted = %d, want >= 1", n)
	}

	// Активная остаётся.
	if _, err := ar.GetSessionByTokenHash(ctx, active.TokenHash); err != nil {
		t.Fatalf("active session should remain: %v", err)
	}
	// Истёкшая удалена.
	if _, err := ar.GetSessionByTokenHash(ctx, expired.TokenHash); err == nil {
		t.Fatalf("expired session should be deleted")
	}
}

// TestDeleteExpiredSsoStates: удаляются только истёкшие состояния.
func TestDeleteExpiredSsoStates(t *testing.T) {
	ar, _, _ := integrationCleanupRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Активное состояние создаём ПЕРВЫМ: CreateSsoState prune'ит истёкшие
	// состояния перед INSERT (S-109), поэтому expired, созданный после active,
	// доживает до DeleteExpiredSsoStates.
	active := &auth.SsoState{StateHash: uniqueHash("st-act"), Nonce: "n2", PKCEVerifier: "v2", Redirect: "/", ExpiresAt: now.Add(time.Hour)}
	if err := ar.CreateSsoState(ctx, active); err != nil {
		t.Fatalf("create active sso state: %v", err)
	}
	expired := &auth.SsoState{StateHash: uniqueHash("st-exp"), Nonce: "n1", PKCEVerifier: "v1", Redirect: "/", ExpiresAt: now.Add(-time.Minute)}
	if err := ar.CreateSsoState(ctx, expired); err != nil {
		t.Fatalf("create expired sso state: %v", err)
	}

	n, err := ar.DeleteExpiredSsoStates(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredSsoStates: %v", err)
	}
	if n < 1 {
		t.Fatalf("deleted = %d, want >= 1", n)
	}

	// Активное состояние пригодно (ConsumeSsoState находит и удаляет).
	if _, err := ar.ConsumeSsoState(ctx, active.StateHash); err != nil {
		t.Fatalf("active sso state should remain: %v", err)
	}
}

// TestDeleteAuditBefore: удаляются только события старше before. Insert
// выставляет created_at=now() (append-only), поэтому старое событие
// «задним числом» обновляется напрямую.
func TestDeleteAuditBefore(t *testing.T) {
	_, raw, pr := integrationCleanupRepo(t)
	ctx := context.Background()

	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)

	// Старое событие (задним числом) — должно удалиться.
	oldEv := &audit.Event{ActorID: owner, TenantID: tenant, Action: audit.ActionAuthLogin, Result: audit.ResultOK}
	if err := raw.Insert(ctx, oldEv); err != nil {
		t.Fatalf("insert old event: %v", err)
	}
	if _, err := pr.pool.Exec(ctx,
		`UPDATE audit_events SET created_at = now() - interval '48 hours' WHERE id = $1`, oldEv.ID); err != nil {
		t.Fatalf("backdate event: %v", err)
	}
	// Свежее событие — должно остаться.
	freshEv := &audit.Event{ActorID: owner, TenantID: tenant, Action: audit.ActionAuthLogin, Result: audit.ResultOK}
	if err := raw.Insert(ctx, freshEv); err != nil {
		t.Fatalf("insert fresh event: %v", err)
	}

	before := time.Now().UTC().Add(-24 * time.Hour)
	del, err := raw.DeleteBefore(ctx, before)
	if err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	if del < 1 {
		t.Fatalf("deleted = %d, want >= 1", del)
	}

	// Старое удалено, свежее осталось.
	var id string
	if err := pr.pool.QueryRow(ctx, `SELECT id FROM audit_events WHERE id = $1`, oldEv.ID).Scan(&id); err == nil {
		t.Fatalf("old event should be deleted")
	}
	if err := pr.pool.QueryRow(ctx, `SELECT id FROM audit_events WHERE id = $1`, freshEv.ID).Scan(&id); err != nil {
		t.Fatalf("fresh event should remain: %v", err)
	}
}

func uniqueHash(s string) string {
	return fmt.Sprintf("%s-%d", s, time.Now().UnixNano())
}
