package auth

import (
	"testing"
)

// Реестр прав — точка, где ошибка тихо расширяет доступ. Эти функции
// используются при выдаче прав и при валидации скоупов API-ключей, поэтому
// проверяются напрямую, а не через сценарии.

func TestAllPermissionsCoverRegistry(t *testing.T) {
	perms := AllPermissions()
	if len(perms) == 0 {
		t.Fatal("реестр прав пуст")
	}
	// Дубликат в реестре означает, что метрика выводится дважды в интерфейсе
	// и дважды попадает в шаблон роли.
	seen := map[Permission]bool{}
	for _, p := range perms {
		if seen[p] {
			t.Fatalf("право %q в реестре повторяется", p)
		}
		seen[p] = true
		if !p.IsKnown() {
			t.Fatalf("право %q есть в списке, но IsKnown() о нём не знает", p)
		}
	}
}

func TestPermissionIsKnown(t *testing.T) {
	for _, p := range AllPermissions() {
		if !p.IsKnown() {
			t.Errorf("известное право %q не проходит IsKnown", p)
		}
	}
	// Несуществующее право должно отвергаться: иначе можно было бы выдать
	// что угодно и получить успешную запись в журнал.
	if Permission("users.teleport").IsKnown() {
		t.Error("выдуманное право прошло IsKnown")
	}
	if Permission("").IsKnown() {
		t.Error("пустое право прошло IsKnown")
	}
}

func TestAllCapabilitiesAndStringsAgree(t *testing.T) {
	caps := AllCapabilities()
	strs := CapabilityStrings()
	if len(caps) != len(strs) {
		t.Fatalf("метрик %d, а имён %d — расхождение", len(caps), len(strs))
	}
	// Скоупы API-ключей валидируются CapabilityStrings, а выдача идёт по
	// Capability. Если списки разойдутся, ключ можно будет выдать на метрику,
	// которую система потом не узнает.
	for i, c := range caps {
		if string(c) != strs[i] {
			t.Fatalf("на позиции %d: %q против %q", i, c, strs[i])
		}
		if !IsKnownCapability(string(c)) {
			t.Errorf("метрика %q есть в реестре, но IsKnownCapability о ней не знает", c)
		}
	}
}

func TestIsKnownCapabilityRejectsUnknown(t *testing.T) {
	if IsKnownCapability("stair.calculate") {
		t.Error("выдуманная метрика прошла IsKnownCapability")
	}
	if IsKnownCapability("") {
		t.Error("пустая метрика прошла IsKnownCapability")
	}
}

func TestGrantedLevelMapping(t *testing.T) {
	// «write» хранится как 'write' и подразумевает read; действие — 'action'.
	// Ошибка здесь тихо превращает «видеть» в «менять» или наоборот.
	cases := map[Level]Granted{
		LevelNone:   GrantedNone,
		LevelRead:   GrantedRead,
		LevelWrite:  GrantedWrite,
		LevelAction: GrantedAction,
	}
	for in, want := range cases {
		if got := GrantedLevel(in); got != want {
			t.Errorf("GrantedLevel(%q) = %q, ждали %q", in, got, want)
		}
	}
}