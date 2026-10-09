// Удаление сотрудника: передача проектов + мягкое удаление (EDR-0041 §7).
//
// Каждое правило проверяется отдельно, потому что цена ошибки здесь разная:
// запрет удалить себя нужен, чтобы не остаться без прав; запрет удалить
// последнего владельца — чтобы у компании остался тот, кто может выдать
// права; требование живого получателя — чтобы проекты не уехали в пустоту.
package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// srv собирает сервис с фейковым репозиторием и тремя сотрудниками:
// владелец, менеджер и ещё один получатель проектов.
type delFixture struct {
	svc      *Service
	repo     *fakeRepo
	owner    *User
	manager  *User
	receiver *User
}

func newDelFixture(t *testing.T) delFixture {
	t.Helper()
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)

	mk := func(email, name string, role Role, status Status) *User {
		u := &User{
			ID: "u-" + email, TenantID: "t-1", Email: email, Name: name,
			Role: role, Status: status, PasswordHash: "x",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := repo.CreateUser(context.Background(), u); err != nil {
			t.Fatalf("CreateUser(%s): %v", email, err)
		}
		return u
	}

	owner := mk("owner@example.com", "Владелец", RoleOwner, StatusActive)
	manager := mk("manager@example.com", "Менеджер", RoleUser, StatusActive)
	receiver := mk("receiver@example.com", "Коллега", RoleUser, StatusActive)

	return delFixture{svc: svc, repo: repo, owner: owner, manager: manager, receiver: receiver}
}

func TestDeleteUserTransfersProjectsAndSoftDeletes(t *testing.T) {
	t.Parallel()
	f := newDelFixture(t)
	f.repo.ownedProjects[f.manager.ID] = 3
	f.repo.ownedProjects[f.receiver.ID] = 1

	err := f.svc.DeleteUser(context.Background(), "t-1", f.owner.ID, f.manager.ID, f.receiver.ID)
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// Проекты уехали к получателю, а не исчезли и не остались у удалённого.
	if got := f.repo.ownedProjects[f.manager.ID]; got != 0 {
		t.Errorf("у удалённого осталось %d проектов, ожидалось 0", got)
	}
	if got := f.repo.ownedProjects[f.receiver.ID]; got != 4 {
		t.Errorf("у получателя %d проектов, ожидалось 4 (1 своих + 3 переданных)", got)
	}
	if f.repo.transferFrom != f.manager.ID || f.repo.transferTo != f.receiver.ID {
		t.Errorf("передача %s→%s, ожидалось %s→%s",
			f.repo.transferFrom, f.repo.transferTo, f.manager.ID, f.receiver.ID)
	}

	// Строка остаётся (мягкое удаление), но человек помечен удалённым.
	if !f.manager.Deleted() {
		t.Error("сотрудник не помечен удалённым: ожидался deleted_at")
	}
	if f.manager.Status != StatusDisabled {
		t.Errorf("статус %q, ожидалось disabled", f.manager.Status)
	}
}

func TestDeleteUserRefusesSelf(t *testing.T) {
	t.Parallel()
	f := newDelFixture(t)
	err := f.svc.DeleteUser(context.Background(), "t-1", f.manager.ID, f.manager.ID, f.receiver.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ошибка %v, ожидался ErrForbidden", err)
	}
	if f.manager.Deleted() {
		t.Error("человек удалил сам себя — этого быть не должно")
	}
}

func TestDeleteUserRefusesLastOwner(t *testing.T) {
	t.Parallel()
	f := newDelFixture(t)

	// Владелец в компании один (второго нет) — удалить его нельзя, даже если
	// staff.delete делегирован менеджеру: компания без владельца не
	// управляется, и вернуть владельца уже нечем.
	f.repo.byID[f.manager.ID].Status = StatusActive
	if err := f.svc.DeleteUser(t.Context(), "t-1", f.manager.ID, f.owner.ID, f.receiver.ID); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("удаление последнего владельца: %v, ожидался ErrLastOwner", err)
	}
	if !f.repo.byID[f.owner.ID].Deleted() == false {
		t.Fatal("владелец удалён, хотя он последний")
	}
}

func TestDeleteUserOwnerRules(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)
	mk := func(email string, role Role) *User {
		u := &User{TenantID: "t-1", Email: email, Role: role, Status: StatusActive,
			CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := repo.CreateUser(t.Context(), u); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		return u
	}
	first := mk("first@example.com", RoleOwner)
	second := mk("second@example.com", RoleOwner)
	receiver := mk("r@example.com", RoleUser)

	// Владельцев двое: одного из них удалить можно — владелец останется.
	if err := svc.DeleteUser(t.Context(), "t-1", first.ID, second.ID, receiver.ID); err != nil {
		t.Fatalf("удаление второго владельца при двух владельцах: %v", err)
	}

	// Не-владелец удалить владельца не может — даже если staff.delete ему
	// делегирован. Владельцев теперь двое (first и third), иначе сработала бы
	// другая защита — «последнего владельца».
	third := mk("third@example.com", RoleOwner)
	if err := svc.DeleteUser(t.Context(), "t-1", receiver.ID, first.ID, third.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("не-владелец удалил владельца: %v, ожидался ErrForbidden", err)
	}
}

func TestDeleteUserRequiresActiveRecipient(t *testing.T) {
	t.Parallel()
	f := newDelFixture(t)
	f.repo.ownedProjects[f.manager.ID] = 2

	// Заблокированный получатель: проекты перейдут к тому, кто не сможет
	// ими пользоваться.
	f.repo.byID[f.receiver.ID].Status = StatusDisabled
	err := f.svc.DeleteUser(context.Background(), "t-1", f.owner.ID, f.manager.ID, f.receiver.ID)
	if !errors.Is(err, ErrInvalidRecipient) {
		t.Fatalf("ошибка %v, ожидался ErrInvalidRecipient", err)
	}
	if f.manager.Deleted() {
		t.Error("удаление состоялось с неподходящим получателем")
	}

	// Чужой тенант.
	outsider := &User{ID: "u-out", TenantID: "t-2", Email: "out@example.com", Role: RoleUser, Status: StatusActive}
	if err := f.repo.CreateUser(context.Background(), outsider); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := f.svc.DeleteUser(context.Background(), "t-1", f.owner.ID, f.manager.ID, outsider.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("получатель из чужого тенанта: %v, ожидался ErrNotFound", err)
	}
}

func TestDeleteUserIgnoresAlreadyDeleted(t *testing.T) {
	t.Parallel()
	f := newDelFixture(t)
	now := time.Now()
	f.manager.DeletedAt = &now
	if err := f.svc.DeleteUser(context.Background(), "t-1", f.owner.ID, f.manager.ID, f.receiver.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("повторное удаление: %v, ожидался ErrNotFound", err)
	}
}

func TestUserDeletionPreviewCountsWhatWillBeKept(t *testing.T) {
	t.Parallel()
	f := newDelFixture(t)
	f.repo.ownedProjects[f.manager.ID] = 3

	pv, err := f.svc.UserDeletionPreview(context.Background(), "t-1", f.manager.ID)
	if err != nil {
		t.Fatalf("UserDeletionPreview: %v", err)
	}
	if pv.Projects != 3 {
		t.Errorf("в предпросмотре %d проектов, ожидалось 3", pv.Projects)
	}
	// Чужие цифры — тоже не наш человек.
	if _, err := f.svc.UserDeletionPreview(context.Background(), "t-2", f.manager.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("предпросмотр чужого тенанта: %v, ожидался ErrNotFound", err)
	}
}
