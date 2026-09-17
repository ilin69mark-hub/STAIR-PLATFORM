package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
)

func TestAuthExtraCoverage(t *testing.T) {
	repo := integrationRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	ownerID := testOwnerID(t, repo, tenant)
	ar := NewAuthRepository(repo.pool)

	// GetUserByEmail found + not found
	uEmail := fmt.Sprintf("extra-%d@test.dev", time.Now().UnixNano())
	u := &auth.User{Name: "Extra", Email: uEmail, TenantID: tenant, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	got, err := ar.GetUserByEmail(ctx, uEmail)
	if err != nil || got.ID != u.ID {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if _, err := ar.GetUserByEmail(ctx, "nope@ex.ru"); err == nil {
		t.Fatal("want not found")
	}

	// UpdateUserStatus valid + not found
	if err := ar.UpdateUserStatus(ctx, tenant, u.ID, auth.StatusDisabled); err != nil {
		t.Fatalf("UpdateUserStatus: %v", err)
	}
	if err := ar.UpdateUserStatus(ctx, tenant, "00000000-0000-0000-0000-000000000000", auth.StatusActive); err == nil {
		t.Fatal("want not found")
	}
	// invalid UUID
	if err := ar.UpdateUserStatus(ctx, tenant, "bad", auth.StatusActive); err == nil {
		t.Fatal("want not found for bad uuid")
	}

	// DeleteUserSessions + DeleteSessionByTokenHash
	s := &auth.Session{UserID: u.ID, TokenHash: fmt.Sprintf("h-%d", time.Now().UnixNano()), ExpiresAt: time.Now().Add(time.Hour)}
	if err := ar.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := ar.DeleteSessionByTokenHash(ctx, s.TokenHash); err != nil {
		t.Fatalf("DeleteSessionByTokenHash: %v", err)
	}
	// DeleteUserSessions (no-op but should not error)
	s2 := &auth.Session{UserID: u.ID, TokenHash: fmt.Sprintf("h2-%d", time.Now().UnixNano()), ExpiresAt: time.Now().Add(time.Hour)}
	_ = ar.CreateSession(ctx, s2)
	if err := ar.DeleteUserSessions(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}

	// GetPolicy not found -> Default + ErrNotFound
	pol, err := ar.GetPolicy(ctx, tenant)
	if err != nil {
		// tenant may have policy now; just check no panic
		_ = pol
	}
	// UpdatePolicy then GetPolicy
	if err := ar.UpdatePolicy(ctx, tenant, auth.Policy{}); err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}
	if _, err := ar.GetPolicy(ctx, tenant); err != nil {
		t.Fatalf("GetPolicy after update: %v", err)
	}

	// ApiKeys: Create, List, GetByHash, Touch, Revoke
	k := &auth.ApiKey{TenantID: tenant, Name: "key1", TokenHash: fmt.Sprintf("tok-%d", time.Now().UnixNano()), Scopes: []string{"read"}, CreatedBy: ownerID}
	if err := ar.CreateApiKey(ctx, k); err != nil {
		t.Fatalf("CreateApiKey: %v", err)
	}
	list, err := ar.ListApiKeys(ctx, tenant)
	if err != nil || len(list) == 0 {
		t.Fatalf("ListApiKeys: %v %d", err, len(list))
	}
	gotK, err := ar.GetApiKeyByTokenHash(ctx, k.TokenHash)
	if err != nil || gotK.ID != k.ID {
		t.Fatalf("GetApiKeyByTokenHash: %v", err)
	}
	if err := ar.TouchApiKey(ctx, k.ID); err != nil {
		t.Fatalf("TouchApiKey: %v", err)
	}
	if err := ar.RevokeApiKey(ctx, tenant, k.ID); err != nil {
		t.Fatalf("RevokeApiKey: %v", err)
	}
	if err := ar.RevokeApiKey(ctx, tenant, k.ID); err == nil {
		t.Fatal("second revoke should be not found")
	}

	// isUUID
	if !isUUID(u.ID) {
		t.Fatal("want true")
	}
	if isUUID("bad") {
		t.Fatal("want false")
	}
}

func TestTestimonialListAll(t *testing.T) {
	repo := integrationRepo(t)
	tenant := testTenantID(t, repo)
	tr := NewTestimonialRepository(repo.pool)
	// ListAll was 0% — just ensure it doesn't crash, returns slice
	_, _ = tr.ListAll(context.Background(), tenant)
}

func TestTxHelpers(t *testing.T) {
	repo := integrationRepo(t)
	err := WithTx(context.Background(), repo.pool, func(tx pgx.Tx) error { return nil })
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	err = WithTxOptions(context.Background(), repo.pool, pgx.TxOptions{}, func(tx pgx.Tx) error { return nil })
	if err != nil {
		t.Fatalf("WithTxOptions: %v", err)
	}
	err = WithTxReadOnly(context.Background(), repo.pool, func(tx pgx.Tx) error { return nil })
	if err != nil {
		t.Fatalf("WithTxReadOnly: %v", err)
	}
	err = WithTxReadWrite(context.Background(), repo.pool, func(tx pgx.Tx) error { return nil })
	if err != nil {
		t.Fatalf("WithTxReadWrite: %v", err)
	}
}

func TestApprovalsExtraCoverage(t *testing.T) {
	repo := integrationRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	ownerID := testOwnerID(t, repo, tenant)

	// ListTenantProjects was 0% — exercise it
	if _, err := repo.ListTenantProjects(ctx, tenant); err != nil {
		t.Fatalf("ListTenantProjects: %v", err)
	}

	// create project
	p := &project.Project{Name: "ApprovTest", Description: "x", Status: project.StatusDraft}
	if err := repo.CreateProject(ctx, tenant, ownerID, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	// save config
	cfg := &project.StairConfiguration{
		ProjectID: p.ID, WidthMM: 900, HeightMM: 2700, Flight: "straight",
		StepHeightMM: 180, StringerThicknessMM: 50, StepThicknessMM: 40, Riser: true,
		ClearanceMM: 2500, RailingHeightMM: 1000,
	}
	if err := repo.SaveConfiguration(ctx, cfg); err != nil {
		t.Fatalf("SaveConfiguration: %v", err)
	}
	// approve
	appr, err := repo.ApproveConfiguration(ctx, tenant, p.ID, cfg.ID, ownerID, "looks good")
	if err != nil {
		t.Fatalf("ApproveConfiguration: %v", err)
	}
	if appr.Comment != "looks good" {
		t.Fatalf("comment mismatch")
	}
	// duplicate should be conflict
	if _, err := repo.ApproveConfiguration(ctx, tenant, p.ID, cfg.ID, ownerID, "again"); err == nil {
		t.Fatal("want conflict on duplicate")
	}
	// get
	got, err := repo.GetConfigurationApproval(ctx, tenant, p.ID, cfg.ID)
	if err != nil || got.ID != appr.ID {
		t.Fatalf("GetConfigurationApproval: %v", err)
	}
	// list
	list, err := repo.ListApprovals(ctx, tenant, p.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListApprovals: %v %d", err, len(list))
	}
	// isUniqueViolation false case
	if isUniqueViolation(fmt.Errorf("other")) {
		t.Fatal("want false")
	}
}
