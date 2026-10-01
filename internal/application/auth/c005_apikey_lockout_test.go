package auth

import (
	"context"
	"errors"
	"testing"
)

// Регрессия CRITICAL-05 (2026-09-27): блокировка пользователя и API-ключи.
//
// ДО фикса было две дыры, и обе были про «чёрный вход»:
//
//  1. AuthenticateApiKey проверял только revoked_at и НЕ смотрел на статус
//     владельца. Сессионная аутентификация (Authenticate) статус проверяет
//     (service.go:326), а Bearer-аутентификация — нет. В итоге disable учётной
//     записи закрывал сессии, но её же API-ключ продолжал работать с полным
//     набором scopes.
//
//  2. Блокировка шла тремя независимыми вызовами (UpdateUserStatus,
//     DeleteUserSessions, RevokeApiKeysByUser), причём ошибки последних двух
//     глотались через `_ =`. Сбой отзыва ключей давал «заблокированного»
//     пользователя с живым Bearer-ключом — заметить можно было только по логам.

func c005Owner(t *testing.T, repo *fakeRepo, id, tenant string, status Status) *User {
	t.Helper()
	u := &User{
		ID: id, TenantID: tenant, Email: id + "@example.com",
		Role: RoleUser, Status: status,
	}
	repo.byID[u.ID] = u
	repo.users[tenant+"/"+id] = u
	return u
}

// TestCRIT005_ApiKeyOfDisabledUserIsRejected — ядро дефекта №1.
func TestCRIT005_ApiKeyOfDisabledUserIsRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	owner := c005Owner(t, repo, "u-1", "t-1", StatusActive)
	_, token, _ := svc.CreateApiKey(context.Background(), "t-1", owner.ID, "CI",
		[]Permission{PermissionUsersList})

	if _, err := svc.AuthenticateApiKey(context.Background(), token); err != nil {
		t.Fatalf("активный пользователь: ключ должен работать, got %v", err)
	}

	// Блокируем учётную запись.
	owner.Status = StatusDisabled
	// Отзыв ключей мог и не произойти (сбой в БД) — AuthenticateApiKey всё
	// равно обязан отказать по статусу владельца. Именно этот случай раньше
	// и был дырой.
	_, err := svc.AuthenticateApiKey(context.Background(), token)
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("CRITICAL-05: Bearer-ключ заблокированного пользователя работает, err=%v", err)
	}
}

// TestCRIT005_DisableUserRevokesKeyAndSession — дефект №2: блокировка обязана
// закрывать оба способа входа разом.
func TestCRIT005_DisableUserRevokesKeyAndSession(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	// Регистрация создаёт и пользователя, и сессию — блокируем именно его,
	// чтобы сессия и ключ принадлежали одной учётной записи.
	owner, sessToken, err := svc.Register(context.Background(), "owner@example.com", "A", "password123")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_, apiToken, _ := svc.CreateApiKey(context.Background(), owner.TenantID, owner.ID, "CI",
		[]Permission{PermissionUsersList})

	if _, err := svc.AuthenticateApiKey(context.Background(), apiToken); err != nil {
		t.Fatalf("до блокировки ключ работает: %v", err)
	}
	if _, _, err := svc.Authenticate(context.Background(), sessToken); err != nil {
		t.Fatalf("до блокировки сессия работает: %v", err)
	}

	disabled := StatusDisabled
	if err := svc.UpdateUser(context.Background(), owner.TenantID, "admin", owner.ID, nil, &disabled); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	if _, err := svc.AuthenticateApiKey(context.Background(), apiToken); err == nil {
		t.Error("CRITICAL-05: после блокировки ключ всё ещё аутентифицирует")
	}
	// Сессия удалена, поэтому Authenticate её не находит (ErrSessionExpired).
	// ErrUserDisabled был бы ответом второго рубежа — проверки статуса в
	// Authenticate, если бы сессия по какой-то причине уцелела.
	if _, _, err := svc.Authenticate(context.Background(), sessToken); !errors.Is(err, ErrSessionExpired) && !errors.Is(err, ErrUserDisabled) {
		t.Errorf("после блокировки сессия должна отвергаться, got %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Errorf("сессии не удалены: %d осталось", len(repo.sessions))
	}
	if !repo.revokedKeyUsers[owner.TenantID+"/"+owner.ID] {
		t.Error("ключи не отозваны при блокировке")
	}
}

// TestCRIT005_DisableFailurePropagates — сбой отзыва ключей обязан дойти до
// вызывающего. Раньше ошибки глотались, и админ видел «готово» при живой
// дыре. Возврат ошибки позволяет повторить и зафиксировать отказ в аудите.
func TestCRIT005_DisableFailurePropagates(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	owner := c005Owner(t, repo, "u-1", "t-1", StatusActive)
	_, apiToken, _ := svc.CreateApiKey(context.Background(), "t-1", owner.ID, "CI",
		[]Permission{PermissionUsersList})

	boom := errors.New("db down")
	repo.disableErr = boom

	disabled := StatusDisabled
	err := svc.UpdateUser(context.Background(), owner.TenantID, "admin", owner.ID, nil, &disabled)
	if !errors.Is(err, boom) {
		t.Fatalf("сбой блокировки обязан возвращаться вызывающему, got %v", err)
	}

	// Промежуточного состояния быть не должно: сбой атомарной операции не
	// оставляет пользователя «наполовину заблокированным».
	if owner.Status != StatusActive {
		t.Errorf("после неудачной блокировки статус = %s, want active", owner.Status)
	}
	repo.disableErr = nil
	if _, err := svc.AuthenticateApiKey(context.Background(), apiToken); err != nil {
		t.Errorf("неудачная блокировка не должна ломать доступ: %v", err)
	}
}

// TestCRIT005_KeyWithoutOwnerIsRejected — created_by nullable: NULL означает
// удалённого создателя (ON DELETE SET NULL) либо ключ, созданный вне
// user-аккаунта. Проверять статус некого, доверять без проверки нельзя.
// Отказ должен быть неотличим от отозванного ключа, иначе ответ
// аутентификации сам становится источником информации о ключах.
func TestCRIT005_KeyWithoutOwnerIsRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	_, token, _ := svc.CreateApiKey(context.Background(), "t-1", "", "legacy",
		[]Permission{PermissionUsersList})

	_, err := svc.AuthenticateApiKey(context.Background(), token)
	if !errors.Is(err, ErrKeyRevoked) {
		t.Fatalf("ключ без владельца обязан отвергаться как отозванный, got %v", err)
	}
}

// TestCRIT005_KeyOfDeletedOwnerIsRejected — владелец удалён: GetUserByID
// возвращает ErrNotFound, доступ не даём. Сбой БД — тоже не повод пропустить.
func TestCRIT005_KeyOfDeletedOwnerIsRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	owner := c005Owner(t, repo, "u-1", "t-1", StatusActive)
	_, token, _ := svc.CreateApiKey(context.Background(), "t-1", owner.ID, "CI",
		[]Permission{PermissionUsersList})
	if _, err := svc.AuthenticateApiKey(context.Background(), token); err != nil {
		t.Fatalf("до удаления владельца: %v", err)
	}

	delete(repo.byID, owner.ID) // владелец удалён
	_, err := svc.AuthenticateApiKey(context.Background(), token)
	if !errors.Is(err, ErrKeyRevoked) {
		t.Fatalf("ключ удалённого владельца обязан отвергаться, got %v", err)
	}
}

// TestCRIT005_ReactivationRestoresAccess — разблокировка возвращает доступ:
// иначе «временно заблокировать» без возможности вернуть пользователя в
// работу было бы односторонней дверью.
func TestCRIT005_ReactivationRestoresAccess(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, 0)
	owner := c005Owner(t, repo, "u-1", "t-1", StatusActive)
	_, token, _ := svc.CreateApiKey(context.Background(), "t-1", owner.ID, "CI",
		[]Permission{PermissionUsersList})

	disabled := StatusDisabled
	if err := svc.UpdateUser(context.Background(), owner.TenantID, "admin", owner.ID, nil, &disabled); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := svc.AuthenticateApiKey(context.Background(), token); err == nil {
		t.Fatal("после блокировки ключ не должен работать")
	}

	// Ключ отозван вместе с блокировкой, поэтому разблокировка возвращает
	// доступ к сессиям, но ключ надо выпустить заново — и это ожидаемо.
	active := StatusActive
	if err := svc.UpdateUser(context.Background(), owner.TenantID, "admin", owner.ID, nil, &active); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if owner.Status != StatusActive {
		t.Fatalf("статус после разблокировки = %s, want active", owner.Status)
	}
	_, err := svc.AuthenticateApiKey(context.Background(), token)
	if !errors.Is(err, ErrKeyRevoked) {
		t.Errorf("отозванный ключ не должен «ожить» вместе с разблокировкой, got %v", err)
	}
}
