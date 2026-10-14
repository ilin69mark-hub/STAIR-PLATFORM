package constraint

import (
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// profiles_test.go — S-236: нормативные профили по назначению здания.
//
// Проверяем то, ради чего профили и делались:
//   - дефолт = одноквартирный дом (решение владельца 2026-10-13);
//   - ограждение ниже 900 БЛОКИРУЕТ расчёт, а не предупреждает (S-239);
//   - STANDARD v1 остался прежним (заморожен) — правка профилей его не
//     трогает;
//   - профиль — копия: мутация одного заказа не течёт в другой.

// TestProfileDefaultIsHouse — пустое назначение = дом.
func TestProfileDefaultIsHouse(t *testing.T) {
	set := ProfileFor("")
	if set.Name != "HOUSE" {
		t.Errorf("профиль по умолчанию %q, ожидался HOUSE", set.Name)
	}
	if got := ProfileFor("house").Name; got != set.Name {
		t.Errorf("house (%q) и пустое назначение (%q) разошлись", got, set.Name)
	}
	// Неизвестное значение молча уехать в «дом» не должно — его отбивает
	// Validate конфигурации; здесь просто проверяем Effective.
	if got := ProfileFor("склад-в-пятиэтажке").Name; got != "HOUSE" {
		t.Errorf("неизвестное назначение дало профиль %q, ждали HOUSE (дом)", got)
	}
}

// TestProfilesBlockLowRailing — в профиле по назначению ограждение ниже
// 900 мм блокирует. В STANDARD остаётся предупреждением (заморожен).
func TestProfilesBlockLowRailing(t *testing.T) {
	for _, name := range Profiles() {
		set := ProfileFor(engineering.BuildingPurpose(name))
		c, ok := set.Active(SAF_RAILING_HEIGHT)
		if !ok {
			t.Fatalf("%s: нет правила высоты ограждения", name)
		}
		if c.Severity != SeverityError {
			t.Errorf("%s: ограждение %.0f мм — %s, нужен error (S-239)",
				name, c.Range.Min, c.Severity)
		}
		// Contains=false означает «значение вне диапазона» — то есть
		// 850 мм должно её НЕ пройти, иначе брак выпустится.
		if c.Range.Contains(850) {
			t.Errorf("%s: 850 мм проходит проверку — брак выпустится", name)
		}
	}
	// STANDARD v1 не тронут.
	std := StandardProfile("STANDARD")
	if c, ok := std.Active(SAF_RAILING_HEIGHT); !ok || c.Severity != SeverityWarning {
		t.Error("STANDARD v1 перестал быть предупреждением — он заморожен")
	}
}

// TestProfileSourcesNameThePurpose — источник правила называет норматив,
// по которому выбран профиль (СП 55/54/118/56), а не только документ
// про ограждение: пользователь должен видеть, по нормам какого здания
// его считают.
func TestProfileSourcesNameThePurpose(t *testing.T) {
	for _, name := range Profiles() {
		set := ProfileFor(engineering.BuildingPurpose(name))
		c, _ := set.Active(SAF_RAILING_HEIGHT)
		if c.Advice == nil || !strings.Contains(c.Advice.Guide, purposeNormDoc(
			engineering.BuildingPurpose(name))) {
			t.Errorf("%s: подсказка не называет норму %s: %q",
				name, purposeNormDoc(engineering.BuildingPurpose(name)),
				c.Advice.Guide)
		}
	}
}

// TestProfileIsCopyPerCall — два вызова дают независимые наборы.
func TestProfileIsCopyPerCall(t *testing.T) {
	a := ProfileFor(engineering.PurposeHouse)
	ca, _ := a.Active(SAF_RAILING_HEIGHT)
	ca.Range.Min = 1
	b := ProfileFor(engineering.PurposeHouse)
	cb, ok := b.Active(SAF_RAILING_HEIGHT)
	if !ok || cb.Range.Min != 900 {
		t.Errorf("правка одного профиля повлияла на другой: min=%.0f", cb.Range.Min)
	}
}

// TestAllProfilesKeepSources — S-235 не сломан профилями: у всех правил
// профиля есть источник (профиль строится из STANDARD, но не забывает).
func TestAllProfilesKeepSources(t *testing.T) {
	for _, name := range Profiles() {
		set := ProfileFor(engineering.BuildingPurpose(name))
		if err := ValidateIntegrity(set); err != nil {
			t.Errorf("%s: нарушения целостности: %v", name, err)
		}
		for code, versions := range set.Constraints {
			for _, c := range versions {
				if c.Source.IsZero() {
					t.Errorf("%s: правило %s v%d без источника", name, code, c.Version)
				}
			}
		}
	}
}
