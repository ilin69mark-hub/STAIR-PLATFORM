package constraint

import (
	"strings"
	"testing"
)

// source_test.go — S-235: происхождение каждого числа обязательно.
//
// Проверяем две вещи, которые разваливаются по-разному:
//  1. Правило без источника не добавляется — иначе «норма» без ссылки
//     снова появится в коде и снова разойдётся (класс S-180/S-206/S-212).
//  2. Все правила стандартного профиля сказали, ОТКУДА взято их число, и
//     честно разделили норму (basis=norm), эргономику и производство:
//     выдавать эргономику за ГОСТ — та же ложь, что и молчание.

// TestAddRejectsRuleWithoutSource — правило без источника не принимается.
func TestAddRejectsRuleWithoutSource(t *testing.T) {
	for _, c := range []struct {
		name string
		c    *Constraint
	}{
		{"нет поля", &Constraint{Code: "A", Category: "g", Version: 1}},
		{"пустой документ", &Constraint{Code: "A", Category: "g", Version: 1,
			Source: &Source{Basis: BasisNorm}}},
		{"неизвестное основание", &Constraint{Code: "A", Category: "g", Version: 1,
			Source: &Source{Doc: "ГОСТ 1", Basis: "на глаз"}}},
	} {
		set := NewSet("cs", "test")
		if err := set.Add(c.c); err == nil {
			t.Errorf("%s: правило без внятного источника принято — молчание недопустимо (S-235)",
				c.name)
		}
	}
}

// TestStandardProfileRulesHaveSource — у каждого правила STANDARD есть
// документ и основание. Пункт нормы может быть пустым (значит «уточняется»),
// документ — нет: без него проверить число нечем.
func TestStandardProfileRulesHaveSource(t *testing.T) {
	set := StandardProfile("test")
	if len(set.Constraints) == 0 {
		t.Fatal("профиль STANDARD пуст")
	}
	total := 0
	for code, versions := range set.Constraints {
		for _, c := range versions {
			total++
			if c.Source.IsZero() {
				t.Errorf("%s v%d: нет источника", code, c.Version)
				continue
			}
			switch c.Source.Basis {
			case BasisNorm, BasisErgonomics, BasisManufacturing:
			default:
				t.Errorf("%s v%d: неизвестное основание %q", code, c.Version, c.Source.Basis)
			}
			if c.Source.String() == "" {
				t.Errorf("%s v%d: источник не выводится строкой", code, c.Version)
			}
		}
	}
	// Профиль заморожен (решение владельца 2026-10-13): набор правил
	// не должен молча уменьшиться, если кто-то уберёт правило при чистке.
	if total < 7 {
		t.Errorf("в STANDARD %d правил, ожидалось не меньше 7 — правило потерялось", total)
	}
}

// TestNormRulesHaveClauseHint — у правил с basis=norm, как правило,
// назван документ; если пункта нет, обязан быть комментарий «уточняется».
// Иначе «ГОСТ 25772» без пункта — это ссылка, которой нельзя проверить.
func TestNormRulesHaveClauseHint(t *testing.T) {
	set := StandardProfile("test")
	for code, versions := range set.Constraints {
		for _, c := range versions {
			if c.Source == nil || c.Source.Basis != BasisNorm {
				continue
			}
			if c.Source.Clause == "" && c.Source.Note == "" {
				t.Errorf("%s v%d: норма без пункта и без пояснения — «%s»",
					code, c.Version, c.Source.Doc)
			}
		}
	}
}

// TestIssueCarriesSource — нарушение доезжает до пользователя с
// источником: пустой строки быть не может, иначе S-235 дырявый
// (правило объясняется, а пользователь об этом не знает).
func TestIssueCarriesSource(t *testing.T) {
	set := StandardProfile("test")
	c, ok := set.Active(GEO_STEP_HEIGHT)
	if !ok {
		t.Fatal("в профиле нет активной GEO-STEP-HEIGHT")
	}
	if c.Source.IsZero() {
		t.Fatal("источник пуст")
	}
	if !strings.Contains(c.Source.String(), string(BasisErgonomics)) {
		t.Errorf("строка источника %q не показывает основание (эргономика, не норма)",
			c.Source.String())
	}
}
