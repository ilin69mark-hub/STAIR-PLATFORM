package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"stairplatform/internal/application/auth"
)

// Регрессия CRITICAL-05 (2026-09-27): атомарность блокировки учётной записи.
//
// ДО фикса UpdateUser вызывал UpdateUserStatus, DeleteUserSessions и
// RevokeApiKeysByUser тремя независимыми вызовами, а ошибки последних двух
// глотались. Сбой любого шага оставлял промежуточное состояние: пользователь
// помечен disabled, но его сессия или Bearer-ключ живы. Свойство видел только
// тот, кто читает логи.
//
// Здесь проверяется на живой БД: DisableUser обязан закрывать оба способа
// входа разом, отличать «нет пользователя» от «есть», и быть идемпотентным.

// c005User создаёт пользователя с уникальным email.
func c005User(t *testing.T, ar *AuthRepository, tenant, email string) *auth.User {
	t.Helper()
	u := &auth.User{
		TenantID: tenant, Email: email, Name: "C005", Role: auth.RoleUser, Status: auth.StatusActive,
	}
	if err := ar.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u
}

// TestCRIT005_DisableUserClosesBothEntryPoints — блокировка закрывает и
// сессии, и API-ключи одной операцией.
func TestCRIT005_DisableUserClosesBothEntryPoints(t *testing.T) {
	ar, repo := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	u := c005User(t, ar, tenant, fmt.Sprintf("c005-both-%d@test.dev", time.Now().UnixNano()))

	sess := &auth.Session{UserID: u.ID, TokenHash: auth.HashToken("c005-sess-" + u.ID), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := ar.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	key := &auth.ApiKey{
		TenantID: tenant, Name: "c005", TokenHash: auth.HashToken("c005-key-" + u.ID),
		CreatedBy: u.ID, Scopes: []string{string(auth.PermissionUsersList)},
	}
	if err := ar.CreateApiKey(ctx, key); err != nil {
		t.Fatalf("CreateApiKey: %v", err)
	}

	// До блокировки оба способа входа живы.
	if _, err := ar.GetSessionByTokenHash(ctx, auth.HashToken("c005-sess-"+u.ID)); err != nil {
		t.Fatalf("сессия должна существовать: %v", err)
	}
	if _, err := ar.GetApiKeyByTokenHash(ctx, auth.HashToken("c005-key-"+u.ID)); err != nil {
		t.Fatalf("ключ должен существовать: %v", err)
	}

	if err := ar.DisableUser(ctx, tenant, u.ID); err != nil {
		t.Fatalf("DisableUser: %v", err)
	}

	got, err := ar.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Status != auth.StatusDisabled {
		t.Errorf("status = %s, want disabled", got.Status)
	}
	if _, err := ar.GetSessionByTokenHash(ctx, auth.HashToken("c005-sess-"+u.ID)); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("сессия должна быть удалена, got %v", err)
	}
	k, err := ar.GetApiKeyByTokenHash(ctx, auth.HashToken("c005-key-"+u.ID))
	if err != nil {
		t.Fatalf("ключ должен остаться в таблице (мягкий отзыв): %v", err)
	}
	if k.Active() {
		t.Error("ключ должен быть отозван (revoked_at выставлен)")
	}
}

// TestCRIT005_DisableUserIsIdempotent — повторная блокировка не ошибка:
// админ может нажать «заблокировать» дважды, и второй вызов обязан быть
// безопасен (иначе UI покажет ошибку при успешной операции).
func TestCRIT005_DisableUserIsIdempotent(t *testing.T) {
	ar, repo := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)
	u := c005User(t, ar, tenant, fmt.Sprintf("c005-idem-%d@test.dev", time.Now().UnixNano()))

	if err := ar.DisableUser(ctx, tenant, u.ID); err != nil {
		t.Fatalf("первый DisableUser: %v", err)
	}
	if err := ar.DisableUser(ctx, tenant, u.ID); err != nil {
		t.Fatalf("повторный DisableUser должен быть успешным: %v", err)
	}
	got, err := ar.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Status != auth.StatusDisabled {
		t.Errorf("status = %s, want disabled", got.Status)
	}
}

// TestCRIT005_DisableUserTenantScope — DisableUser не должен блокировать
// пользователя ЧУЖОГО tenant (горизонтальная эскалация через подставной
// user_id). ErrNotFound, не ErrForbidden: факт существования не раскрываем.
func TestCRIT005_DisableUserTenantScope(t *testing.T) {
	ar, repo := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)

	var other string
	if err := repo.pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`,
		"C005 other", "c005-other-"+itoaUD()).Scan(&other); err != nil {
		t.Fatalf("create other tenant: %v", err)
	}
	u := c005User(t, ar, other, fmt.Sprintf("c005-scope-%d@test.dev", time.Now().UnixNano()))

	if err := ar.DisableUser(ctx, tenant, u.ID); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("чужой tenant должен дать ErrNotFound, got %v", err)
	}
	got, err := ar.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Status != auth.StatusActive {
		t.Errorf("пользователь чужого tenant не должен блокироваться, status = %s", got.Status)
	}
}

// TestCRIT005_DisableUserRejectsEmptyIds — пустые идентификаторы не должны
// превращаться в «разблокировать всех»: операция обязана отказать.
func TestCRIT005_DisableUserRejectsEmptyIDs(t *testing.T) {
	ar, repo := integrationAuthRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, repo)

	for _, tc := range []struct{ tenant, user string }{
		{"", "u"}, {tenant, ""}, {"", ""},
	} {
		if err := ar.DisableUser(ctx, tc.tenant, tc.user); !errors.Is(err, auth.ErrNotFound) {
			t.Errorf("DisableUser(%q,%q) = %v, want ErrNotFound", tc.tenant, tc.user, err)
		}
	}
}
