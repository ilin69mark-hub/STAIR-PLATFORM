package constraint

import (
	"fmt"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// standard_frozen_test.go — S-241 п. 1: значения STANDARD v1 заморожены.
//
// Что здесь защищается. Профиль STANDARD v1 объявлен замороженным: его
// версия зафиксирована, а новые значения живут в новых версиях и профилях по
// назначению. Пока защиты не было, ПРАВИЛО СРАБОТЫВАЛО ТАК: правка порога
// не ломала ни один тест. TestStandardProfileIntegrity проверяет структуру
// (целостность, одна активная версия) и ни одного числа.
//
// Последствие было тихим и неприятным: можно было сдвинуть высоту ступени
// со 150 на 140, и весь набор остался зелёным, потому что проверялась
// согласованность, а не сами значения. Норматив — не то, что «работает».
//
// Основание для изменения здесь — новая версия профиля. Правка этого
// снимка означает сознательное решение раскрыть, а не «подогнать тест».
//
// Ожидание падает с ПОЛНЫМ перечнем расхождений, а не с первым: список из
// семи правил целиком помещается в сообщение, и не надо перезапускать
// гейт семь раз.

// frozenRule — снимок одного правила STANDARD v1.
type frozenRule struct {
	code   RuleCode
	min    float64
	max    float64
	hasMin bool
	hasMax bool
	// tolerance — часть замороженного значения наравне с границами.
	// Поначалу снимок его не содержал, и это оказалось ещё одной дырой:
	// сдвинь допуск ограждения с 1 мм до 10, и 899 мм — на миллиметр ниже
	// брака по ГОСТ 25772 — прошёл бы молча.
	tolerance float64
	severity  Severity
	basis     Basis
	doc       string
}

func standardFrozenExpectations() []frozenRule {
	return []frozenRule{
		{GEO_STEP_HEIGHT, 150, 200, true, true, 0.1, SeverityError, BasisErgonomics,
			"эргономика (антропометрия шага)"},
		{GEO_TREAD_DEPTH, 260, 320, true, true, 0.1, SeverityError, BasisErgonomics,
			"эргономика (антропометрия шага)"},
		{GEO_ANGLE, 30, 45, true, true, 0.01, SeverityError, BasisNorm,
			"ГОСТ 23120 (предельный уклон марша)"},
		{GEO_CLEARANCE, 2000, 0, true, false, 1, SeverityWarning, BasisNorm,
			"СП 59.13330 (доступность)"},
		{GEO_STRINGER_THICKNESS, 30, 0, true, false, 0.1, SeverityWarning, BasisManufacturing,
			"производство (пиломатериал по ГОСТ 8486/24454)"},
		{GEO_STRINGER_THICKNESS_SHEET, 3, 12, true, true, 0.1, SeverityWarning, BasisManufacturing,
			"производство (лазерный раскрой листа"},
		{SAF_RAILING_HEIGHT, 900, 0, true, false, 1, SeverityWarning, BasisNorm,
			"ГОСТ 25772-2025 (ограждения лестниц"},
	}
}

// TestStandardProfileValuesAreFrozen — числа STANDARD v1 не меняются молча.
func TestStandardProfileValuesAreFrozen(t *testing.T) {
	set := StandardProfile("standard")
	want := standardFrozenExpectations()
	var diffs []string

	for _, w := range want {
		versions, ok := set.Constraints[w.code]
		if !ok || len(versions) == 0 {
			diffs = append(diffs, fmt.Sprintf("%s: правила нет в профиле", w.code))
			continue
		}
		c := versions[0]
		if c.Range.Min != w.min {
			diffs = append(diffs, fmt.Sprintf("%s: min = %v, в замороженном STANDARD v1 %v",
				w.code, c.Range.Min, w.min))
		}
		if c.Range.HasMin != w.hasMin {
			diffs = append(diffs, fmt.Sprintf("%s: HasMin = %v, ожидалось %v",
				w.code, c.Range.HasMin, w.hasMin))
		}
		if c.Range.Max != w.max {
			diffs = append(diffs, fmt.Sprintf("%s: max = %v, в замороженном STANDARD v1 %v",
				w.code, c.Range.Max, w.max))
		}
		if c.Range.HasMax != w.hasMax {
			diffs = append(diffs, fmt.Sprintf("%s: HasMax = %v, ожидалось %v",
				w.code, c.Range.HasMax, w.hasMax))
		}
		if c.Range.Tolerance != w.tolerance {
			diffs = append(diffs, fmt.Sprintf("%s: допуск = %v, в замороженном STANDARD v1 %v",
				w.code, c.Range.Tolerance, w.tolerance))
		}
		if c.Severity != w.severity {
			diffs = append(diffs, fmt.Sprintf("%s: severity = %q, было %q",
				w.code, c.Severity, w.severity))
		}
		if c.Source == nil {
			diffs = append(diffs, fmt.Sprintf("%s: источника нет", w.code))
			continue
		}
		if c.Source.Basis != w.basis {
			diffs = append(diffs, fmt.Sprintf("%s: основание = %q, было %q",
				w.code, c.Source.Basis, w.basis))
		}
		// Документ сверяется по префиксу: в тексте есть уточнение в
		// скобках («… (доступность)»), и полное сравнение завязывало бы
		// тест на формулировку, а не на происхождение числа.
		if !strings.HasPrefix(c.Source.Doc, w.doc) {
			diffs = append(diffs, fmt.Sprintf("%s: документ %q, начинался с %q",
				w.code, c.Source.Doc, w.doc))
		}
	}

	// Правил ровно семь: лишнее или забытое правило меняет то, что заказ
	// пройдёт, и это должно быть видно здесь же, а не всплывать в сборе.
	if got := len(set.Constraints); got != len(want) {
		diffs = append(diffs, fmt.Sprintf("правил в профиле %d, ждали %d", got, len(want)))
	}

	if len(diffs) > 0 {
		t.Fatalf("STANDARD v1 заморожен, но его значения разошлись с эталоном.\n"+
			"Правка оправдана только новой ВЕРСИЕЙ профиля, а не молчаливой "+
			"заменой числа. Расхождения:\n  - %s",
			strings.Join(diffs, "\n  - "))
	}
}

// TestStandardProfileEscapedRules — какие правила профили усиливают до
// блокировки (S-241 п. 2).
//
// Сейчас это ровно одно ПРАВИЛО, которое в STANDARD v1 было
// предупреждением: SAF_RAILING_HEIGHT. И это не «так получилось» —
// ограждение ниже 900 мм в жилом здании брак, и оно обязано блокировать
// расчёт, а не предупреждать (S-239).
//
// Опасность в другую сторону: ничто не мешает будущему профилю начать
// блокировать что угодно. Если GEO_STRINGER_THICKNESS (основание
// «производство») станет error, заказ перестанет выдаваться из-за толщины
// косоура, которая по MFG-0005 вполне годна, и человек не поймёт почему.
//
// Инвариант «эргономика не бывает error» здесь НЕ проверяется и не может:
// GEO_STEP_HEIGHT с основанием «эргономика» — это error. Проверяется
// фактический список правил, а не догадка о нём.
func TestStandardProfileEscapedRules(t *testing.T) {
	std := StandardProfile("standard")
	allowed := map[RuleCode]bool{
		// Правила, блокирующие уже в STANDARD: профили их не трогают.
		GEO_STEP_HEIGHT: true,
		GEO_TREAD_DEPTH: true,
		GEO_ANGLE:       true,
		// Правило, которое профили УСИЛИВАЮТ намеренно (S-239).
		SAF_RAILING_HEIGHT: true,
	}

	// Что усиливают профили относительно самого STANDARD.
	escaped := map[RuleCode]bool{}
	for _, p := range Profiles() {
		set := ProfileFor(engineering.BuildingPurpose(p))
		for code, versions := range set.Constraints {
			if len(versions) == 0 {
				continue
			}
			base := std.Constraints[code]
			if len(base) == 0 {
				continue
			}
			if versions[0].Severity == SeverityError && base[0].Severity != SeverityError {
				escaped[code] = true
			}
		}
	}

	if !escaped[SAF_RAILING_HEIGHT] {
		t.Error("SAF_RAILING_HEIGHT перестал блокировать в профилях: ограждение " +
			"ниже 900 мм выпускается — это брак (S-239)")
	}
	for code := range escaped {
		if allowed[code] {
			continue
		}
		t.Errorf("профиль усилил до error правило %q, которое в STANDARD было %q "+
			"(основание «%s»). Оно теперь блокирует заказ, а не предупреждает. "+
			"Такое усиление должно быть сознательным, с обоснованием по норме",
			code, std.Constraints[code][0].Severity, std.Constraints[code][0].Source.Basis)
	}
}
