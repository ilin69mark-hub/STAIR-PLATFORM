package database

import (
	"context"
	"os"
	"testing"
	"time"

	"stairplatform/internal/application/auth"
)

// brokenAuthRepo возвращает AuthRepository с закрытым пулом: любая операция
// завершается ошибкой пула и покрывает error-ветки репозитория.
func brokenAuthRepo(t *testing.T) *AuthRepository {
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
	pool.Close()
	return NewAuthRepository(pool)
}

const testUUID = "00000000-0000-0000-0000-000000000000"

// TestAuthRepoErrNotFoundPaths — ветки ErrNotFound на живом пуле.
func TestAuthRepoErrNotFoundPaths(t *testing.T) {
	ar, _ := integrationAuthRepo(t)
	ctx := context.Background()

	if _, err := ar.GetUserByID(ctx, testUUID); err != auth.ErrNotFound {
		t.Fatalf("GetUserByID: want ErrNotFound, got %v", err)
	}
	if _, err := ar.GetUserByEmail(ctx, "missing-account@test.dev"); err != auth.ErrNotFound {
		t.Fatalf("GetUserByEmail: want ErrNotFound, got %v", err)
	}
	if _, err := ar.GetApiKeyByTokenHash(ctx, "missing-token-hash"); err != auth.ErrNotFound {
		t.Fatalf("GetApiKeyByTokenHash: want ErrNotFound, got %v", err)
	}
	if _, err := ar.GetOAuthAccountByProviderSubject(ctx, "google", "missing-subject"); err != auth.ErrNotFound {
		t.Fatalf("GetOAuthAccountByProviderSubject: want ErrNotFound, got %v", err)
	}
	if _, err := ar.ConsumeSsoState(ctx, "missing-state-hash"); err != auth.ErrNotFound {
		t.Fatalf("ConsumeSsoState: want ErrNotFound, got %v", err)
	}
	// Не-тенантный пользователь при смене роли/статуса.
	if err := ar.UpdateUserRole(ctx, testUUID, testUUID, auth.RoleUser); err != auth.ErrNotFound {
		t.Fatalf("UpdateUserRole: want ErrNotFound, got %v", err)
	}
	if err := ar.UpdateUserStatus(ctx, testUUID, testUUID, auth.StatusActive); err != auth.ErrNotFound {
		t.Fatalf("UpdateUserStatus: want ErrNotFound, got %v", err)
	}
	if err := ar.RevokeApiKey(ctx, testUUID, testUUID); err != auth.ErrNotFound {
		t.Fatalf("RevokeApiKey: want ErrNotFound, got %v", err)
	}
}

// TestAuthRepoBrokenPoolErrorPaths — все ветки fmt.Errorf через закрытый пул.
func TestAuthRepoBrokenPoolErrorPaths(t *testing.T) {
	ar := brokenAuthRepo(t)
	ctx := context.Background()

	u := &auth.User{Email: "dup@test.dev", Name: "Dup", TenantID: testUUID, Role: auth.RoleUser, Status: auth.StatusActive}
	if err := ar.CreateUser(ctx, u); err == nil {
		t.Fatal("CreateUser: want pool error")
	}
	if _, err := ar.GetUserByID(ctx, testUUID); err == nil {
		t.Fatal("GetUserByID: want pool error")
	}
	if _, err := ar.GetUserByEmail(ctx, "dup@test.dev"); err == nil {
		t.Fatal("GetUserByEmail: want pool error")
	}
	if _, err := ar.ListUsers(ctx, testUUID); err == nil {
		t.Fatal("ListUsers: want pool error")
	}
	if err := ar.UpdateUserRole(ctx, testUUID, testUUID, auth.RoleUser); err == nil {
		t.Fatal("UpdateUserRole: want pool error")
	}
	if err := ar.UpdateUserStatus(ctx, testUUID, testUUID, auth.StatusActive); err == nil {
		t.Fatal("UpdateUserStatus: want pool error")
	}
	if err := ar.DeleteUserSessions(ctx, testUUID); err == nil {
		t.Fatal("DeleteUserSessions: want pool error")
	}
	if err := ar.CreateSession(ctx, &auth.Session{UserID: testUUID, TokenHash: "h", ExpiresAt: time.Now().Add(time.Hour)}); err == nil {
		t.Fatal("CreateSession: want pool error")
	}
	if _, err := ar.GetSessionByTokenHash(ctx, "h"); err == nil {
		t.Fatal("GetSessionByTokenHash: want pool error")
	}
	if err := ar.DeleteSessionByTokenHash(ctx, "h"); err == nil {
		t.Fatal("DeleteSessionByTokenHash: want pool error")
	}
	if _, err := ar.GetPolicy(ctx, testUUID); err == nil {
		t.Fatal("GetPolicy: want pool error")
	}
	if err := ar.UpdatePolicy(ctx, testUUID, auth.DefaultPolicy()); err == nil {
		t.Fatal("UpdatePolicy: want pool error")
	}
	if err := ar.CreateApiKey(ctx, &auth.ApiKey{TenantID: testUUID, Name: "k", TokenHash: "h", Scopes: []string{"read"}}); err == nil {
		t.Fatal("CreateApiKey: want pool error")
	}
	if _, err := ar.ListApiKeys(ctx, testUUID); err == nil {
		t.Fatal("ListApiKeys: want pool error")
	}
	if _, err := ar.GetApiKeyByTokenHash(ctx, "h"); err == nil {
		t.Fatal("GetApiKeyByTokenHash: want pool error")
	}
	if err := ar.RevokeApiKey(ctx, testUUID, testUUID); err == nil {
		t.Fatal("RevokeApiKey: want pool error")
	}
	if err := ar.TouchApiKey(ctx, testUUID); err == nil {
		t.Fatal("TouchApiKey: want pool error")
	}
	if err := ar.CreateOAuthAccount(ctx, &auth.OAuthAccount{Provider: "google", Subject: "s", UserID: testUUID}); err == nil {
		t.Fatal("CreateOAuthAccount: want pool error")
	}
	if _, err := ar.GetOAuthAccountByProviderSubject(ctx, "google", "s"); err == nil {
		t.Fatal("GetOAuthAccountByProviderSubject: want pool error")
	}
	if _, err := ar.ListOAuthAccounts(ctx, testUUID); err == nil {
		t.Fatal("ListOAuthAccounts: want pool error")
	}
	if err := ar.CreateSsoState(ctx, &auth.SsoState{StateHash: "h", Nonce: "n", ExpiresAt: time.Now().Add(time.Hour)}); err == nil {
		t.Fatal("CreateSsoState: want pool error")
	}
	if _, err := ar.ConsumeSsoState(ctx, "h"); err == nil {
		t.Fatal("ConsumeSsoState: want pool error")
	}
	if _, err := ar.DeleteExpiredSessions(ctx, time.Now()); err == nil {
		t.Fatal("DeleteExpiredSessions: want pool error")
	}
	if _, err := ar.DeleteExpiredSsoStates(ctx); err == nil {
		t.Fatal("DeleteExpiredSsoStates: want pool error")
	}
}
