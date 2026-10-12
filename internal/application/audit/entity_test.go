package audit

import "testing"

// Реестр действий журнала безопасности. ClientRecordable — то, чем
// ограничен клиентский путь записи: если действие ошибочно попадёт в
// allowlist, клиент сможет подделать запись в чужом аудите.

func TestAllActionsReturnsCopy(t *testing.T) {
	a := AllActions()
	if len(a) == 0 {
		t.Fatal("реестр действий пуст")
	}
	// Возвращается копия, а не сам срез: иначе вызывающий, отсортировав или
	// обрезав результат, молча изменил бы общий реестр на всё приложение.
	if len(a) > 0 {
		saved := a[0]
		a[0] = Action("подделка")
		if AllActions()[0] == Action("подделка") {
			t.Fatal("AllActions отдал сам срез, а не копию")
		}
		_ = saved
	}
}

func TestAllActionsHaveNoDuplicates(t *testing.T) {
	seen := map[Action]bool{}
	for _, a := range AllActions() {
		if seen[a] {
			t.Fatalf("действие %q повторяется в реестре", a)
		}
		seen[a] = true
		if !a.Valid() {
			t.Fatalf("действие %q есть в списке, но Valid() о нём не знает", a)
		}
	}
}

func TestClientActionsAreSubsetOfAll(t *testing.T) {
	all := map[Action]bool{}
	for _, a := range AllActions() {
		all[a] = true
	}
	for _, a := range ClientActions() {
		if !all[a] {
			t.Errorf("клиентское действие %q отсутствует в общем реестре", a)
		}
		if !a.ClientRecordable() {
			t.Errorf("действие %q в clientActions, но ClientRecordable() ложь", a)
		}
	}
}

func TestClientRecordableRejectsUnknown(t *testing.T) {
	// Несуществующее действие клиент записать не может: иначе в журнал
	// попадали бы произвольные строки от недоверенного источника.
	if Action("project.teleport").ClientRecordable() {
		t.Error("выдуманное действие прошло ClientRecordable")
	}
	if Action("").ClientRecordable() {
		t.Error("пустое действие прошло ClientRecordable")
	}
	if Action("project.teleport").Valid() {
		t.Error("выдуманное действие прошло Valid")
	}
}

func TestClientActionsReturnsCopy(t *testing.T) {
	a := ClientActions()
	if len(a) == 0 {
		t.Fatal("клиентский allowlist пуст")
	}
	a[0] = Action("подделка")
	if ClientActions()[0] == Action("подделка") {
		t.Fatal("ClientActions отдал сам срез, а не копию")
	}
}