package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"stairplatform/internal/application/auth"
)

// TestRepoOAuthAccountCRUD — Create/Get по (provider, subject), ErrOAuthExists,
// List по user (EDR-0017 §3.1).
func TestRepoOAuthAccountCRUD(t *testing.T) {
	ar, repo := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	user := testOwnerID(t, repo, tenant)

	acct := &auth.OAuthAccount{Provider: "testidp", Subject: "sub-xyz", UserID: user}
	if err := ar.CreateOAuthAccount(ctx, acct); err != nil {
		t.Fatalf("CreateOAuthAccount: %v", err)
	}
	if acct.ID == "" {
		t.Fatal("expected generated id")
	}

	got, err := ar.GetOAuthAccountByProviderSubject(ctx, "testidp", "sub-xyz")
	if err != nil {
		t.Fatalf("GetOAuthAccount: %v", err)
	}
	if got.UserID != user {
		t.Fatalf("user = %q, want %q", got.UserID, user)
	}

	// Дубликат (provider, subject) — ErrOAuthExists.
	if err := ar.CreateOAuthAccount(ctx, &auth.OAuthAccount{Provider: "testidp", Subject: "sub-xyz", UserID: user}); !errors.Is(err, auth.ErrOAuthExists) {
		t.Fatalf("expected ErrOAuthExists, got %v", err)
	}

	// Разный subject — допустим.
	if err := ar.CreateOAuthAccount(ctx, &auth.OAuthAccount{Provider: "testidp", Subject: "sub-other", UserID: user}); err != nil {
		t.Fatalf("create second account: %v", err)
	}

	list, err := ar.ListOAuthAccounts(ctx, user)
	if err != nil {
		t.Fatalf("ListOAuthAccounts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(list))
	}

	// Неизвестная пара — ErrNotFound.
	if _, err := ar.GetOAuthAccountByProviderSubject(ctx, "testidp", "nope"); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestRepoSsoStateConsume — одноразовое использование и истечение TTL.
func TestRepoSsoStateConsume(t *testing.T) {
	ar, _ := integrationAuthRepo(t)
	ctx := context.Background()

	st := &auth.SsoState{
		StateHash:    auth.HashToken("state-raw-1"),
		Nonce:        "nonce",
		PKCEVerifier: "verifier",
		Redirect:     "/projects",
		ExpiresAt:    time.Now().UTC().Add(10 * time.Minute),
	}
	if err := ar.CreateSsoState(ctx, st); err != nil {
		t.Fatalf("CreateSsoState: %v", err)
	}

	got, err := ar.ConsumeSsoState(ctx, st.StateHash)
	if err != nil {
		t.Fatalf("ConsumeSsoState: %v", err)
	}
	if got.PKCEVerifier != "verifier" || got.Nonce != "nonce" {
		t.Fatalf("state = %+v", got)
	}

	// Повторное потребление — ErrNotFound (одноразовость).
	if _, err := ar.ConsumeSsoState(ctx, st.StateHash); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on double consume, got %v", err)
	}

	// Истёкшее состояние — ErrNotFound.
	expired := &auth.SsoState{
		StateHash:    auth.HashToken("state-expired"),
		Nonce:        "n",
		PKCEVerifier: "v",
		Redirect:     "/",
		ExpiresAt:    time.Now().UTC().Add(-time.Minute),
	}
	if err := ar.CreateSsoState(ctx, expired); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if _, err := ar.ConsumeSsoState(ctx, expired.StateHash); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for expired state, got %v", err)
	}
}
