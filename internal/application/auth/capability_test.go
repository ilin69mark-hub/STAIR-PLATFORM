// Реестр метрик: форма и согласованность деклараций (EDR-0041 §3.1).
package auth_test

import (
	"testing"

	"stairplatform/internal/application/auth"
)

func TestCapabilityRegistryShape(t *testing.T) {
	t.Parallel()

	// 36 метрик из плана минус две лишние: project.view и project.edit.
	// Доступ к проекту даётся не метрикой, а личным членством в проекте
	// (project_members): метрика была бы второй правдой о доступе, и её
	// пришлось бы выдавать вместе с каждой ролью в проекте.
	if got := len(auth.CapabilityRegistry); got != 34 {
		t.Fatalf("метрик в реестре: %d, ожидалось 34", got)
	}

	seen := make(map[auth.Capability]bool, len(auth.CapabilityRegistry))
	blocks := make(map[string]bool)
	for _, s := range auth.CapabilityRegistry {
		if seen[s.Capability] {
			t.Errorf("дубль метрики %q", s.Capability)
		}
		seen[s.Capability] = true

		if s.Title == "" {
			t.Errorf("метрика %q без названия — её нечем показать в матрице", s.Capability)
		}
		if s.Block == "" {
			t.Errorf("метрика %q без блока", s.Capability)
		}
		blocks[s.Block] = true
		if len(s.Levels) == 0 {
			t.Errorf("метрика %q без уровней", s.Capability)
		}
	}

	// Каждый блок реестра обязан быть объявлен в списке заголовков, иначе
	// матрица прав не сгруппирует строки, а плитки экрана — не соберутся.
	for _, b := range auth.CapabilityBlocks {
		if !blocks[b.ID] {
			t.Errorf("блок %q объявлен в заголовках, но в нём нет ни одной метрики", b.ID)
		}
		delete(blocks, b.ID)
	}
	for b := range blocks {
		t.Errorf("метрики блока %q есть, а заголовка блока нет", b)
	}
}

func TestActionCapabilitiesHaveOnlyActionLevel(t *testing.T) {
	t.Parallel()

	for _, s := range auth.CapabilityRegistry {
		isAction := len(s.Levels) == 1 && s.Levels[0] == auth.LevelAction
		if isAction != auth.IsAction(s.Capability) {
			t.Errorf("метрика %q: IsAction=%v, а объявлено %v", s.Capability, isAction, s.Levels)
		}
		// Не action: либо только чтение, либо чтение с правкой изменения.
		// Смешивать action с read/write нельзя — это разные сущности.
		if !isAction {
			ok := len(s.Levels) == 1 && s.Levels[0] == auth.LevelRead
			if len(s.Levels) == 2 {
				ok = s.Levels[0] == auth.LevelRead && s.Levels[1] == auth.LevelWrite
			}
			if !ok {
				t.Errorf("метрика %q не action, но уровни %v — ожидались [read] либо [read, write]", s.Capability, s.Levels)
			}
		}
	}
}

func TestAllowsLevelFollowsDeclaration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cap   auth.Capability
		level auth.Level
		want  bool
	}{
		{auth.CapSiteContacts, auth.LevelRead, true},
		{auth.CapSiteContacts, auth.LevelWrite, true},
		{auth.CapSiteContacts, auth.LevelAction, false},
		{auth.CapAuditView, auth.LevelRead, true},
		{auth.CapAuditView, auth.LevelWrite, false},
		{auth.CapFinanceRefunds, auth.LevelAction, true},
		{auth.CapFinanceRefunds, auth.LevelRead, false},
		{auth.CapFinanceRefunds, auth.LevelWrite, false},
		{auth.Capability("nonexistent.cap"), auth.LevelRead, false},
	}
	for _, tc := range cases {
		if got := auth.AllowsLevel(tc.cap, tc.level); got != tc.want {
			t.Errorf("AllowsLevel(%q, %q) = %v, ожидалось %v", tc.cap, tc.level, got, tc.want)
		}
	}
}

// write подразумевает read; action — самостоятелен и не даёт ни того, ни
// другого. Это ядро правила «нельзя выдать лишнее уровнем выше».
func TestSatisfies(t *testing.T) {
	t.Parallel()

	cases := []struct {
		granted auth.Granted
		want    auth.Level
		satisfy bool
	}{
		{auth.GrantedNone, auth.LevelRead, false},
		{auth.GrantedNone, auth.LevelWrite, false},
		{auth.GrantedNone, auth.LevelAction, false},
		{auth.GrantedRead, auth.LevelRead, true},
		{auth.GrantedRead, auth.LevelWrite, false},
		{auth.GrantedRead, auth.LevelAction, false},
		{auth.GrantedWrite, auth.LevelRead, true},
		{auth.GrantedWrite, auth.LevelWrite, true},
		{auth.GrantedWrite, auth.LevelAction, false},
		{auth.GrantedAction, auth.LevelAction, true},
		{auth.GrantedAction, auth.LevelRead, false},
		{auth.GrantedAction, auth.LevelWrite, false},
	}
	for _, tc := range cases {
		if got := auth.Satisfies(tc.granted, tc.want); got != tc.satisfy {
			t.Errorf("Satisfies(%q, %q) = %v, ожидалось %v", tc.granted, tc.want, got, tc.satisfy)
		}
	}
}

func TestParseLevelAcceptsEmptyAsNone(t *testing.T) {
	t.Parallel()

	// Пустой набор выдач — валидное состояние («не дал ничего»), поэтому
	// пустая строка не должна быть ошибкой разбора.
	for _, s := range []string{"", "none", "read", "write", "action"} {
		if _, err := auth.ParseLevel(s); err != nil {
			t.Errorf("ParseLevel(%q) вернул ошибку: %v", s, err)
		}
	}
	if _, err := auth.ParseLevel("root"); err == nil {
		t.Error("ParseLevel(\"root\") ожидался отказ, а прошёл")
	}
}
