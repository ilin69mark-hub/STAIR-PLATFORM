// Передача владения проектом (EDR-0008, S-158).
//
// Каждое правило проверяется отдельно, потому что ошибка здесь стоит работы:
// проект без владельца нельзя ни удалить, ни передать дальше, а лишний
// владелец получает права на чужой расчёт.
package project

import (
	"context"
	"errors"
	"testing"

	"stairplatform/internal/application/stair"
)

// trFixture — проект с владельцем, редактором и наблюдателем.
type trFixture struct {
	svc      *Service
	repo     *fakeRepo
	owner    string
	editor   string
	viewer   string
	outsider string
}

func newTrFixture(t *testing.T) trFixture {
	t.Helper()
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	const tenant = "t1"
	repo.projects[key(tenant, "p1")] = &Project{ID: "p1", Name: "Проект", OwnerID: "u-owner"}

	if _, ok := repo.members["p1"]; !ok {
		repo.members["p1"] = map[string]*ProjectMember{}
	}
	add := func(userID string, role ProjectRole) {
		repo.members["p1"][userID] = &ProjectMember{ProjectID: "p1", UserID: userID, Role: role}
	}
	add("u-owner", RoleOwner)
	add("u-editor", RoleEditor)
	add("u-viewer", RoleViewer)

	return trFixture{svc: svc, repo: repo, owner: "u-owner", editor: "u-editor", viewer: "u-viewer", outsider: "u-outsider"}
}

func TestTransferOwnershipByProjectOwner(t *testing.T) {
	t.Parallel()
	f := newTrFixture(t)

	if err := f.svc.TransferOwnership(context.Background(), "t1", f.owner, "p1", f.editor, false); err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}

	// Новый владелец — владелец, прежний — редактор. Обмен ролями «наоборот»
	// оставил бы проект без владельца в projects.owner_id.
	var newOwner, oldOwner *ProjectMember
	for _, m := range f.repo.members["p1"] {
		if m.UserID == f.editor && newOwner == nil {
			newOwner = m
		}
		if m.UserID == f.owner && oldOwner == nil {
			oldOwner = m
		}
	}
	if newOwner.Role != RoleOwner {
		t.Errorf("у нового владельца роль %q, ожидалась owner", newOwner.Role)
	}
	if oldOwner.Role != RoleEditor {
		t.Errorf("у прежнего владельца роль %q, ожидалась editor (не удалён)", oldOwner.Role)
	}
	if len(f.repo.members["p1"]) != 3 {
		t.Errorf("участников %d, ожидалось 3: прежний владелец должен остаться в проекте", len(f.repo.members["p1"]))
	}
	if got := f.repo.ownerOf["p1"]; got != f.editor {
		t.Errorf("владелец проекта %q, ожидался %q", got, f.editor)
	}
}

func TestTransferOwnershipRefusesNonOwnerWithoutCapability(t *testing.T) {
	t.Parallel()
	f := newTrFixture(t)

	// Редактор проекта — не владелец, и платформенного права у него нет.
	err := f.svc.TransferOwnership(context.Background(), "t1", f.editor, "p1", f.viewer, false)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("редактор передал проект: %v, ожидался ErrForbidden", err)
	}
	if f.repo.ownerOf["p1"] != "" && f.repo.ownerOf["p1"] != f.owner {
		t.Errorf("владелец сменился без прав: %q", f.repo.ownerOf["p1"])
	}
}

func TestTransferOwnershipByCapabilityForNonMember(t *testing.T) {
	t.Parallel()
	f := newTrFixture(t)

	// Владелец компании не участник проекта, но у него staff.project_transfer:
	// забрать чужой проект ему можно, а вот чужого участника назначить
	// владельцем — нельзя (ErrNotFound от проверки членства получателя).
	err := f.svc.TransferOwnership(context.Background(), "t1", f.outsider, "p1", f.editor, true)
	if err != nil {
		t.Fatalf("передача с правом staff.project_transfer: %v", err)
	}
	if f.repo.ownerOf["p1"] != f.editor {
		t.Errorf("владелец %q, ожидался %q", f.repo.ownerOf["p1"], f.editor)
	}

	if err := f.svc.TransferOwnership(context.Background(), "t1", f.outsider, "p1", "u-не-участник", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("передача не-участнику: %v, ожидался ErrNotFound", err)
	}
}

func TestTransferOwnershipRefusesSelf(t *testing.T) {
	t.Parallel()
	f := newTrFixture(t)

	err := f.svc.TransferOwnership(context.Background(), "t1", f.owner, "p1", f.owner, false)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("передача самому себе: %v, ожидался ErrConflict", err)
	}
	if f.repo.ownerOf["p1"] == f.editor {
		t.Error("владелец сменился при передаче самому себе")
	}
}

func TestTransferOwnershipRefusesStranger(t *testing.T) {
	t.Parallel()
	f := newTrFixture(t)

	// Передать человеку, которого нет в проекте, нельзя: он появился бы в
	// проекте без приглашения.
	err := f.svc.TransferOwnership(context.Background(), "t1", f.owner, "p1", "u-чужой", false)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("передача не-участнику: %v, ожидался ErrNotFound", err)
	}
}

func TestTransferOwnershipRefusesUnknownProject(t *testing.T) {
	t.Parallel()
	f := newTrFixture(t)

	if err := f.svc.TransferOwnership(context.Background(), "t1", f.owner, "нет-такого", f.editor, false); !errors.Is(err, ErrForbidden) {
		// Владелец не состоит в несуществующем проекте: это «не участник»,
		// а без платформенного права — отказ.
		t.Fatalf("передача несуществующего проекта: %v, ожидался ErrForbidden", err)
	}
	if err := f.svc.TransferOwnership(context.Background(), "t1", f.owner, "нет-такого", f.editor, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("передача несуществующего проекта с правом: %v, ожидался ErrNotFound", err)
	}
}
