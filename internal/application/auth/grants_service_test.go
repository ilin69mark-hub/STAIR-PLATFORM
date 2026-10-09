// Сервис выдач: правила 4 и 5 на живом фейке (EDR-0041 §4).
package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
)

// fakeGrantsRepo — хранит наборы выдач в памяти и повторяет контракт
// БД: замена целиком + взведение grants_initialized.
type fakeGrantsRepo struct {
	sets   map[string]*GrantSet
	owners int
	fail   error
}

func newFakeGrantsRepo() *fakeGrantsRepo {
	return &fakeGrantsRepo{sets: map[string]*GrantSet{}}
}

func (f *fakeGrantsRepo) ListGrants(_ context.Context, userID string) (*GrantSet, error) {
	if s, ok := f.sets[userID]; ok && s != nil {
		return s, nil
	}
	return NewGrantSet(), nil
}

func (f *fakeGrantsRepo) ReplaceGrants(_ context.Context, _, userID string, s *GrantSet, _ string) error {
	if f.fail != nil {
		return f.fail
	}
	f.sets[userID] = s
	return nil
}

func (f *fakeGrantsRepo) SetGrantsInitialized(_ context.Context, userID string, init bool) error {
	if !init {
		delete(f.sets, userID)
	}
	return nil
}

func (f *fakeGrantsRepo) CountActiveOwners(_ context.Context, _ string) (int, error) {
	return f.owners, nil
}

// staffFixture: владелец, руководитель продаж и менеджер в одном tenant.
func staffFixture(t *testing.T) (*fakeRepo, *Service, *fakeGrantsRepo, *GrantsService) {
	t.Helper()
	repo := newFakeRepo()
	ctx := context.Background()
	gr := newFakeGrantsRepo()
	core := NewService(repo, time.Hour).WithGrants(gr)

	owner := &User{ID: "u-owner", TenantID: "t-1", Email: "owner@x.ru", Role: RoleOwner}
	admin := &User{ID: "u-admin", TenantID: "t-1", Email: "sales@x.ru", Role: RoleSales}
	man := &User{ID: "u-man", TenantID: "t-1", Email: "man@x.ru", Role: RoleUser}

	for _, u := range []*User{owner, admin, man} {
		if err := repo.CreateUser(ctx, u); err != nil {
			t.Fatalf("создание %s: %v", u.Email, err)
		}
	}
	return repo, core, gr, NewGrantsService(core)
}

func TestEffectiveGrantsFollowsFlag(t *testing.T) {
	repo, core, gr, _ := staffFixture(t)
	ctx := context.Background()

	admin, _ := repo.GetUserByEmail(ctx, "sales@x.ru")

	// Флаг сброшен — действует шаблон роли.
	g, err := core.EffectiveGrants(ctx, admin)
	if err != nil {
		t.Fatalf("effective grants: %v", err)
	}
	if !g.Has(CapFinanceSales, LevelWrite) {
		t.Error("шаблон руководителя продаж должен давать «менять заказы»")
	}

	// Флаг поднят и в таблице свой набор — шаблон роли больше не применяется.
	admin.GrantsInitialized = true
	gr.sets[admin.ID] = NewGrantSet(Grant{Capability: CapSiteSocial, Level: GrantedRead})
	g, err = core.EffectiveGrants(ctx, admin)
	if err != nil {
		t.Fatalf("effective grants: %v", err)
	}
	if g.Has(CapFinanceSales, LevelWrite) {
		t.Error("при поднятом флаге роль действовать не должна")
	}
	if !g.Has(CapSiteSocial, LevelRead) {
		t.Error("точечная выдача не применилась")
	}
}

func TestOwnerCanGrantAnythingExceptGranting(t *testing.T) {
	repo, _, _, svc := staffFixture(t)
	ctx := context.Background()
	owner, _ := repo.GetUserByEmail(ctx, "owner@x.ru")
	admin, _ := repo.GetUserByEmail(ctx, "sales@x.ru")

	next := RoleTemplate(RoleSales)
	if err := svc.SetGrants(ctx, "t-1", owner, admin.ID, next); err != nil {
		t.Fatalf("владелец не смог выдать шаблон руководителю: %v", err)
	}

	// Владелец может точечно выдать одну метрику «просто так».
	if err := svc.SetGrants(ctx, "t-1", owner, admin.ID,
		NewGrantSet(Grant{Capability: CapFinanceSales, Level: GrantedRead})); err != nil {
		t.Fatalf("владелец не смог выдать одну метрику: %v", err)
	}
}

func TestSalesAdminCannotGrantWhatHeDoesNotHave(t *testing.T) {
	repo, _, _, svc := staffFixture(t)
	ctx := context.Background()
	admin, _ := repo.GetUserByEmail(ctx, "sales@x.ru")
	man, _ := repo.GetUserByEmail(ctx, "man@x.ru")

	// Руководитель продаж может передать то, что имеет сам: заказы на чтение.
	if err := svc.SetGrants(ctx, "t-1", admin, man.ID,
		NewGrantSet(Grant{Capability: CapFinanceSales, Level: GrantedRead})); err != nil {
		t.Fatalf("передача своего права отвергнута: %v", err)
	}

	// А возврат платежа — не может: у него самого нет.
	err := svc.SetGrants(ctx, "t-1", admin, man.ID,
		NewGrantSet(Grant{Capability: CapFinanceRefunds, Level: GrantedAction}))
	if !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("ожидался ErrGrantCeiling, получено %v", err)
	}

	// И не может повысить уровень: цены материалов он видит, а права менять
	// у него нет — значит и выдать «менять» он не может. Сама метрика запись
	// допускает, отказ приходит именно от потолка, а не от реестра.
	err = svc.SetGrants(ctx, "t-1", admin, man.ID,
		NewGrantSet(Grant{Capability: CapPriceMaterials, Level: GrantedWrite}))
	if !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("выдача уровня выше своего не отвергнута: %v", err)
	}
}

func TestOwnerGrantsCannotBeReduced(t *testing.T) {
	repo, _, _, svc := staffFixture(t)
	ctx := context.Background()
	owner, _ := repo.GetUserByEmail(ctx, "owner@x.ru")

	err := svc.SetGrants(ctx, "t-1", owner, owner.ID,
		RoleTemplate(RoleOwner))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("владелец смог выдать права себе (правило 5), получено %v", err)
	}
}

// Владельца нельзя урезать другим человеком — даже если тот сам имеет
// полный набор (например, второй владелец хочет ограничить первого).
func TestSecondOwnerCannotReduceAnotherOwner(t *testing.T) {
	repo, _, _, svc := staffFixture(t)
	ctx := context.Background()
	owner, _ := repo.GetUserByEmail(ctx, "owner@x.ru")
	second := &User{ID: "u-owner2", TenantID: "t-1", Email: "owner2@x.ru", Role: RoleOwner}
	if err := repo.CreateUser(ctx, second); err != nil {
		t.Fatalf("создание второго владельца: %v", err)
	}

	err := svc.SetGrants(ctx, "t-1", second, owner.ID,
		NewGrantSet(Grant{Capability: CapFinanceSales, Level: GrantedRead}))
	if !errors.Is(err, ErrGrantOwnerProtected) {
		t.Fatalf("урезание владельца не отвергнуто: %v", err)
	}
}

func TestSelfGrantsRejectedForEveryone(t *testing.T) {
	repo, _, _, svc := staffFixture(t)
	ctx := context.Background()

	for _, email := range []string{"owner@x.ru", "sales@x.ru", "man@x.ru"} {
		u, _ := repo.GetUserByEmail(ctx, email)
		if err := svc.SetGrants(ctx, "t-1", u, u.ID, RoleTemplate(RoleSales)); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: выдать права себе не отвергнуто (%v)", email, err)
		}
	}
}

func TestGrantsRepoFailurePropagates(t *testing.T) {
	repo, _, gr, svc := staffFixture(t)
	ctx := context.Background()
	owner, _ := repo.GetUserByEmail(ctx, "owner@x.ru")
	admin, _ := repo.GetUserByEmail(ctx, "sales@x.ru")

	gr.fail = errors.New("db down")
	if err := svc.SetGrants(ctx, "t-1", owner, admin.ID,
		NewGrantSet(Grant{Capability: CapFinanceSales, Level: GrantedRead})); err == nil {
		t.Fatal("сбой репозитория проглочен")
	}
}

// Репозиторий выдач — обязательная связь. Без неё EffectiveGrants должен
// сказать об этом внятно, а не упасть: раньше здесь был nil-указатель, и
// выдачу прав нашла только ручная проверка на стенде.
func TestGrantsRepoIsRequired(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	ctx := context.Background()
	core := NewService(repo, time.Hour) // без WithGrants

	owner := &User{ID: "u-owner", TenantID: "t-1", Email: "owner@x.ru", Role: RoleOwner}
	if err := repo.CreateUser(ctx, owner); err != nil {
		t.Fatalf("создание владельца: %v", err)
	}

	target := &User{ID: "u-1", TenantID: "t-1", Email: "a@x.ru", Role: RoleUser, GrantsInitialized: true}
	if err := repo.CreateUser(ctx, target); err != nil {
		t.Fatalf("создание сотрудника: %v", err)
	}

	// Флаг поднят, а репозитория нет — читать права нечем.
	if _, err := core.EffectiveGrants(ctx, target); !errors.Is(err, ErrGrantsNotConfigured) {
		t.Errorf("EffectiveGrants вернул %v, ожидался ErrGrantsNotConfigured", err)
	}

	// И запись должна отказать, а не упасть.
	err := NewGrantsService(core).SetGrants(ctx, "t-1", owner, target.ID, NewGrantSet())
	if !errors.Is(err, ErrGrantsNotConfigured) {
		t.Errorf("SetGrants вернул %v, ожидался ErrGrantsNotConfigured", err)
	}
}

// Каждая выдача обязана попасть в журнал безопасности: иначе «кто и кому
// что выдал» не восстановить, а именно это журнал и нужен для разбора
// инцидента. Регрессия была реальной — сервис выдач создавался без аудита
// и писал в никуда.
func TestGrantChangeIsAudited(t *testing.T) {
	repo := newFakeRepo()
	ctx := context.Background()
	rec := &auditRecorder{}
	core := NewService(repo, time.Hour, audit.NewService(rec)).WithGrants(newFakeGrantsRepo())
	svc := NewGrantsService(core)

	owner := &User{ID: "u-owner", TenantID: "t-1", Email: "owner@x.ru", Role: RoleOwner}
	emp := &User{ID: "u-emp", TenantID: "t-1", Email: "emp@x.ru", Role: RoleUser}
	for _, u := range []*User{owner, emp} {
		if err := repo.CreateUser(ctx, u); err != nil {
			t.Fatalf("создание %s: %v", u.Email, err)
		}
	}

	if err := svc.SetGrants(ctx, "t-1", owner, emp.ID, NewGrantSet(
		Grant{Capability: CapFinanceSales, Level: GrantedRead},
	)); err != nil {
		t.Fatalf("выдача: %v", err)
	}

	if len(rec.events) == 0 {
		t.Fatal("выдача прав не записана в журнал аудита")
	}
	last := rec.events[len(rec.events)-1]
	if last.Action != audit.ActionUserGrantsChanged {
		t.Errorf("последнее событие = %q, ожидалось %q", last.Action, audit.ActionUserGrantsChanged)
	}
	if last.Result != audit.ResultOK {
		t.Errorf("результат = %q, ожидался ok", last.Result)
	}
	if !strings.Contains(last.Detail, "emp@x.ru") {
		t.Errorf("в записи нет сотрудника: %q", last.Detail)
	}
}

// Отказ тоже должен остаться в журнале — по нему видно попытки выдать
// лишнее.
func TestDeniedGrantIsAudited(t *testing.T) {
	repo := newFakeRepo()
	ctx := context.Background()
	rec := &auditRecorder{}
	core := NewService(repo, time.Hour, audit.NewService(rec)).WithGrants(newFakeGrantsRepo())
	svc := NewGrantsService(core)

	sales := &User{ID: "u-sales", TenantID: "t-1", Email: "sales@x.ru", Role: RoleSales}
	emp := &User{ID: "u-emp", TenantID: "t-1", Email: "emp@x.ru", Role: RoleUser}
	for _, u := range []*User{sales, emp} {
		if err := repo.CreateUser(ctx, u); err != nil {
			t.Fatalf("создание %s: %v", u.Email, err)
		}
	}

	if err := svc.SetGrants(ctx, "t-1", sales, emp.ID, NewGrantSet(
		Grant{Capability: CapFinanceRefunds, Level: GrantedAction},
	)); !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("ожидался ErrGrantCeiling, получено %v", err)
	}

	found := false
	for _, e := range rec.events {
		if e.Action == audit.ActionAuthzDenied && e.Result == audit.ResultDenied {
			found = true
		}
	}
	if !found {
		t.Error("отказ в выдаче не записан как authz.denied")
	}
}
