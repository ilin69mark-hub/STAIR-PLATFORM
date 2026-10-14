// Точечные выдачи: пять правил безопасности (EDR-0041 §4).
package auth_test

import (
	"strings"
	"testing"

	"stairplatform/internal/application/auth"
)

func TestRule1_CeilingCannotBeExceeded(t *testing.T) {
	t.Parallel()

	// Выдающий может только то, что имеет сам.
	ceiling := auth.NewGrantSet(
		auth.Grant{Capability: auth.CapFinanceSales, Level: auth.GrantedRead},
		auth.Grant{Capability: auth.CapTrafficFunnel, Level: auth.GrantedRead},
	)

	// Ровно то, что есть, — можно.
	if err := auth.ValidateGrantsFor(auth.NewGrantSet(
		auth.Grant{Capability: auth.CapFinanceSales, Level: auth.GrantedRead},
	), ceiling); err != nil {
		t.Errorf("выдача своего права отвергнута: %v", err)
	}

	// Права, которого у выдающего нет, — нельзя.
	err := auth.ValidateGrantsFor(auth.NewGrantSet(
		auth.Grant{Capability: auth.CapFinanceRefunds, Level: auth.GrantedAction},
	), ceiling)
	if err == nil {
		t.Fatal("выдали право, которого у выдающего нет")
	}
	if !strings.Contains(err.Error(), "у вас нет такого права") {
		t.Errorf("неожиданный текст ошибки: %v", err)
	}
}

// Потолок по уровню, а не только по наличию: видеть заказы не значит
// иметь право их менять.
func TestRule1_CeilingIsCheckedPerLevel(t *testing.T) {
	t.Parallel()

	ceiling := auth.NewGrantSet(
		auth.Grant{Capability: auth.CapFinanceSales, Level: auth.GrantedRead},
	)
	err := auth.ValidateGrantsFor(auth.NewGrantSet(
		auth.Grant{Capability: auth.CapFinanceSales, Level: auth.GrantedWrite},
	), ceiling)
	if err == nil {
		t.Fatal("выдали «менять» тому, кому выдано только «видеть»")
	}
}

func TestRule2_GrantsRightStaysWithOwner(t *testing.T) {
	t.Parallel()

	// Владелец (ceiling = nil) тоже не может выдать право выдавать.
	err := auth.ValidateGrantsFor(auth.NewGrantSet(
		auth.Grant{Capability: auth.CapStaffGrants, Level: auth.GrantedAction},
	), nil)
	if err == nil || !strings.Contains(err.Error(), "остаётся за владельцем") {
		t.Fatalf("владелец смог выдать право выдавать: %v", err)
	}

	// И не-владелец — тем более.
	ceiling := auth.OwnerCapabilities()
	if err := auth.ValidateGrantsFor(auth.NewGrantSet(
		auth.Grant{Capability: auth.CapStaffGrants, Level: auth.GrantedAction},
	), ceiling); err == nil {
		t.Fatal("выдали право выдавать не-владельцу")
	}
}

func TestRule3_WriteImpliesRead(t *testing.T) {
	t.Parallel()

	s := auth.NewGrantSet(auth.Grant{Capability: auth.CapSiteContacts, Level: auth.GrantedWrite})
	if !s.Has(auth.CapSiteContacts, auth.LevelRead) {
		t.Error("выдан write, но Has(read) = false — «менять» должно подразумевать «видеть»")
	}
	if !s.Has(auth.CapSiteContacts, auth.LevelWrite) {
		t.Error("выдан write, но Has(write) = false")
	}
	if s.Has(auth.CapSiteContacts, auth.LevelAction) {
		t.Error("write не должен давать действие")
	}
}

// Данные и действия не смешиваются: нельзя выдать «менять» метрике, у
// которой только чтение, и «действие» метрике-данным.
func TestRule3_LevelMustMatchDeclaration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cap  auth.Capability
		lvl  auth.Granted
	}{
		{"write у read-only", auth.CapAuditView, auth.GrantedWrite},
		{"write у действия", auth.CapFinanceRefunds, auth.GrantedWrite},
		{"action у данных", auth.CapSiteContacts, auth.GrantedAction},
		{"read у действия", auth.CapStaffDelete, auth.GrantedRead},
	}
	for _, tc := range cases {
		if err := auth.ValidateGrantSet(auth.NewGrantSet(
			auth.Grant{Capability: tc.cap, Level: tc.lvl},
		)); err == nil {
			t.Errorf("%s: уровень принят, хотя не соответствует декларации", tc.name)
		}
	}
}

func TestValidateGrantSetRejectsUnknownCapability(t *testing.T) {
	t.Parallel()

	if err := auth.ValidateGrantSet(auth.NewGrantSet(
		auth.Grant{Capability: "made.up", Level: auth.GrantedRead},
	)); err == nil {
		t.Fatal("принята метрика, которой нет в реестре")
	}
}

func TestRule4_OwnerKeepsEveryMetric(t *testing.T) {
	t.Parallel()

	owner := auth.OwnerCapabilities()

	// Полный набор: кроме права выдавать (оно по правилу 2 не хранится).
	if owner.Len() != len(auth.CapabilityRegistry)-1 {
		t.Errorf("у владельца %d метрик, ожидалось %d (все, кроме staff.grants)",
			owner.Len(), len(auth.CapabilityRegistry)-1)
	}
	if owner.HasAny(auth.CapStaffGrants) {
		t.Error("право выдавать не должно храниться в наборе — оно по роли")
	}

	// Урезать владельца нельзя: набор без единой метрики не проходит
	// проверку «владелец должен иметь всё».
	reduced := auth.NewGrantSet(auth.Grant{Capability: auth.CapFinanceSales, Level: auth.GrantedRead})
	if err := auth.ValidateOwnerGrants(reduced); err == nil {
		t.Fatal("владельцу оставили одну метрику — должен быть запрет")
	}

	if err := auth.ValidateOwnerGrants(owner); err != nil {
		t.Errorf("полный набор владельца отвергнут: %v", err)
	}
}

func TestRule5_OwnerHasNoGrantsRightInTable(t *testing.T) {
	t.Parallel()

	// Правило 5 в этой модели выражено структурно: право выдавать не
	// хранится ни у кого, поэтому его нельзя выдать даже себе.
	if auth.OwnerCapabilities().HasAny(auth.CapStaffGrants) {
		t.Error("право выдавать прав должно быть в наборе")
	}
	for _, role := range []auth.Role{auth.RoleUser, auth.RoleSales} {
		if auth.RoleTemplate(role).HasAny(auth.CapStaffGrants) {
			t.Errorf("шаблон роли %q содержит право выдавать", role)
		}
	}
}

func TestRoleTemplatesMatchIntent(t *testing.T) {
	t.Parallel()

	// Владелец и служебная admin дают полный набор: сузить admin значило бы
	// молча отнять доступ у действующих администраторов.
	for _, role := range []auth.Role{auth.RoleOwner, auth.RoleAdmin} {
		if got := auth.RoleTemplate(role).Len(); got != len(auth.CapabilityRegistry)-1 {
			t.Errorf("роль %q: метрик %d, ожидался полный набор (%d)", role, got, len(auth.CapabilityRegistry)-1)
		}
	}

	sales := auth.RoleTemplate(auth.RoleSales)
	mustHave := []auth.Capability{
		auth.CapFinanceSales, auth.CapTrafficFunnel, auth.CapStaffView,
	}
	for _, c := range mustHave {
		if !sales.HasAny(c) {
			t.Errorf("у руководителя продаж нет метрики %q", c)
		}
	}
	// Руководитель продаж не должен получать финансовые и security-метрики.
	mustNotHave := []auth.Capability{
		auth.CapFinanceRefunds, auth.CapSiteContacts, auth.CapSecurityPolicy,
		auth.CapAuditView, auth.CapSecurityKeys, auth.CapStaffDelete,
	}
	for _, c := range mustNotHave {
		if sales.HasAny(c) {
			t.Errorf("у руководителя продаж не должно быть метрики %q", c)
		}
	}
	// Заказы он ведёт, значит статус менять может.
	if !sales.Has(auth.CapFinanceSales, auth.LevelWrite) {
		t.Error("руководитель продаж должен уметь менять статус заказа")
	}

	manager := auth.RoleTemplate(auth.RoleUser)
	if !manager.Has(auth.CapFinanceSales, auth.LevelRead) {
		t.Error("менеджер должен видеть заказы")
	}
	if manager.Has(auth.CapFinanceSales, auth.LevelWrite) {
		t.Error("менеджер не должен менять статус заказа")
	}
	if manager.Len() > 3 {
		t.Errorf("у менеджера %d метрик — шаблон слишком широкий", manager.Len())
	}
}

func TestGrantSetBlocksGrouping(t *testing.T) {
	t.Parallel()

	s := auth.NewGrantSet(
		auth.Grant{Capability: auth.CapSiteContacts, Level: auth.GrantedWrite},
		auth.Grant{Capability: auth.CapFinanceSales, Level: auth.GrantedRead},
		auth.Grant{Capability: auth.CapStaffCreate, Level: auth.GrantedAction},
	)
	blocks := s.CapabilitiesInBlocks()
	if len(blocks[auth.BlockSite]) != 1 {
		t.Errorf("в блоке сайта %d метрик, ожидалась 1", len(blocks[auth.BlockSite]))
	}
	if len(blocks[auth.BlockFinance]) != 1 || len(blocks[auth.BlockStaff]) != 1 {
		t.Errorf("разбивка по блокам неверна: %+v", blocks)
	}
	if _, ok := blocks[auth.BlockTraffic]; ok {
		t.Error("пустой блок попал в разбивку")
	}
}

func TestGrantSetRoundTrip(t *testing.T) {
	t.Parallel()

	caps := []auth.Capability{auth.CapSiteSocial, auth.CapSiteSEO}
	levels := []auth.Granted{auth.GrantedWrite, auth.GrantedRead}
	s, err := auth.GrantSetFromPairs(caps, levels)
	if err != nil {
		t.Fatalf("сборка набора: %v", err)
	}
	if !s.Has(auth.CapSiteSocial, auth.LevelWrite) || !s.Has(auth.CapSiteSEO, auth.LevelRead) {
		t.Error("уровни не восстановились")
	}
	if _, err := auth.GrantSetFromPairs(caps, levels[:1]); err == nil {
		t.Error("набор с разным числом метрик и уровней принят")
	}
}

// Отчёт, который нельзя менять, не должен выдавать «менять» — ни в шаблоне,
// ни владельцу. Иначе матрица покажет переключатель, которого нет.
func TestTemplatesDoNotGrantWriteOnReadOnlyMetrics(t *testing.T) {
	t.Parallel()

	for _, role := range []auth.Role{auth.RoleOwner, auth.RoleAdmin, auth.RoleSales, auth.RoleUser} {
		set := auth.RoleTemplate(role)
		for _, spec := range auth.CapabilityRegistry {
			if auth.IsAction(spec.Capability) {
				continue
			}
			if !auth.AllowsLevel(spec.Capability, auth.LevelWrite) && set.Has(spec.Capability, auth.LevelWrite) {
				t.Errorf("роль %q выдаёт «менять» метрику %q, которая только для чтения",
					role, spec.Capability)
			}
		}
	}
}
