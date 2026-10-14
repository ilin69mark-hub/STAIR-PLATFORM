package validation_test

import (
	"testing"

	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// report_test.go — S-240: развёрнутый отчёт по уровням.
//
// Главное, что тут проверяется, — не «есть ли уровни», а что отчёт НЕ
// позволяет выдать непроверенное за проверенное. Все гейты ниже — про
// это: unknown не превращается в passed, not_performed не превращается в
// «валидно», а отсутствие причины считается ошибкой.

// TestReportHasThreeLevels — уровни есть, и их ровно три.
func TestReportHasThreeLevels(t *testing.T) {
	r := validation.BuildReport(validation.Result{Valid: true}, false, []string{"нет схемы"})
	for _, code := range []validation.LevelCode{
		validation.LevelGeometry, validation.LevelCodes, validation.LevelCalc,
	} {
		if r.Level(code) == nil {
			t.Errorf("уровня %q нет", code)
		}
	}
	if len(r.Levels) != 3 {
		t.Errorf("уровней %d, ждали 3", len(r.Levels))
	}
}

// TestCalcLevelNotPerformedWithReasons — прочность НЕ выполнена, и
// причины названы. Ключевой гейт: пустой список при not_performed читался
// бы как забытый модуль, а перечисление даёт понять, чего не хватает.
func TestCalcLevelNotPerformedWithReasons(t *testing.T) {
	reasons := []string{"нет расчётной схемы площадки"}
	r := validation.BuildReport(validation.Result{Valid: true}, false, reasons)

	calc := r.Level(validation.LevelCalc)
	if calc.Status != validation.LevelNotPerformed {
		t.Fatalf("статус расчёта %q, ждали not_performed", calc.Status)
	}
	if len(calc.Reasons) == 0 {
		t.Fatal("уровень не выполнен, но причины не названы — это выглядит как забытый модуль")
	}
	if calc.Reasons[0] != reasons[0] {
		t.Errorf("причина %q, ожидалась %q", calc.Reasons[0], reasons[0])
	}
	if calc.Summary == "" {
		t.Error("уровень без пояснения: человек не поймёт, что означает «не выполнено»")
	}
}

// TestNotPerformedDoesNotMakeProjectValidOrInvalid — непроверенная
// прочность тянет итог в partial, но НЕ делает проект валидным и НЕ
// делает невалидным. Оба варианта были бы враньём: «валидно» скрывает
// непроверенное, «невалидно» наказывает за отсутствие расчёта.
func TestNotPerformedDoesNotMakeProjectValidOrInvalid(t *testing.T) {
	r := validation.BuildReport(validation.Result{Valid: true}, false, []string{"нет схемы"})
	if got := r.Overall(); got != validation.LevelPartial {
		t.Errorf("общий итог %q, ждали partial: проект посчитан, но не всё", got)
	}
	if !r.Valid() {
		t.Error("Valid()=false при отсутствии нарушений: непроверенная прочность " +
			"не должна делать проект невалидным")
	}

	// А вот настоящее нарушение должно давать и failed, и Valid()=false.
	vr := validation.Result{Valid: false, Issues: []validation.Issue{{
		Code:     constraint.GEO_STEP_HEIGHT,
		Severity: constraint.SeverityError,
		Message:  "высота ступени вне диапазона",
		Source:   "документ=СП 55.13330; пункт=п. 5; основание=норма",
	}}}
	r2 := validation.BuildReport(vr, false, []string{"нет схемы"})
	if r2.Overall() != validation.LevelFailed {
		t.Errorf("общий итог %q, при блокирующем нарушении ждали failed", r2.Overall())
	}
	if r2.Valid() {
		t.Error("Valid()=true при блокирующем нарушении")
	}
}

// TestReportCarriesNormSource — развёрнутая проверка доносит документ,
// пункт и основание. Без этого отчёт доказывает, что кто-то когда-то
// написал правило, но не по какому документу.
func TestReportCarriesNormSource(t *testing.T) {
	vr := validation.Result{Valid: false, Issues: []validation.Issue{{
		Code:     constraint.SAF_RAILING_HEIGHT,
		Severity: constraint.SeverityError,
		Message:  "высота ограждения должна быть не менее 900 мм",
		Value:    850, Min: 900, HasMin: true,
		Source: "СП 55.13330, п. 5.1 [норма]",
		Fix:    "увеличьте высоту ограждения",
	}}}
	lvl := validation.BuildReport(vr, false, nil).Level(validation.LevelCodes)
	if len(lvl.Checks) != 1 {
		t.Fatalf("проверок %d, ждали 1", len(lvl.Checks))
	}
	c := lvl.Checks[0]
	if c.Doc != "СП 55.13330" {
		t.Errorf("документ %q, ожидался СП 55.13330", c.Doc)
	}
	if c.Clause != "п. 5.1" {
		t.Errorf("пункт %q, ожидался п. 5.1", c.Clause)
	}
	if c.Origin() != "норма" {
		t.Errorf("основание %q, ожидалось норма", c.Origin())
	}
	if c.Status != validation.CheckFailed {
		t.Errorf("вердикт %q при SeverityError, ждали failed", c.Status)
	}
	if !c.HasMin || c.Min != 900 || c.Value != 850 {
		t.Errorf("границы потеряны: value=%v min=%v hasMin=%v", c.Value, c.Min, c.HasMin)
	}
	if c.Fix == "" {
		t.Error("потеряно что делать: без Fix человек не знает, куда нажать")
	}
}

// TestUnknownBasisStaysUnknown — основание, которого нет, остаётся
// неизвестным. Догадка «наверное, норма» в отчёте опаснее пустоты.
func TestUnknownBasisStaysUnknown(t *testing.T) {
	vr := validation.Result{Valid: true, Issues: []validation.Issue{{
		Code: constraint.GEO_STEP_HEIGHT, Severity: constraint.SeverityWarning,
		Message: "подсказка", Source: "СП 55.13330",
	}}}
	lvl := validation.BuildReport(vr, false, nil).Level(validation.LevelCodes)
	if got := lvl.Checks[0].Origin(); got != "не указано" {
		t.Errorf("основание %q, ожидалось «не указано»", got)
	}
}

// TestCheckWithoutSourceIsNotCertification — проверка без источника
// остаётся проверкой, но не «сертификатом»: её статус не может быть
// passed в отчёте, который читают как подтверждение соответствия.
func TestCheckWithoutSourceIsNotCertification(t *testing.T) {
	vr := validation.Result{Valid: true, Issues: []validation.Issue{{
		Code: constraint.GEO_STEP_HEIGHT, Severity: constraint.SeverityWarning,
		Message: "предупреждение без источника",
	}}}
	lvl := validation.BuildReport(vr, false, nil).Level(validation.LevelCodes)
	if lvl.Checks[0].Doc != "" || lvl.Checks[0].Clause != "" {
		t.Error("документ и пункт выдуманы для проверки без источника")
	}
}

// TestSortedChecksStable — порядок проверок в отчёте не зависит от порядка
// обхода движка. Иначе отчёт прыгал бы между двумя вызовами одного и того
// же расчёта, и его нельзя было бы ни показать, ни сравнить.
func TestSortedChecksStable(t *testing.T) {
	vr := validation.Result{Valid: true, Issues: []validation.Issue{
		{Code: constraint.GEO_TREAD_DEPTH, Severity: constraint.SeverityWarning, Message: "b"},
		{Code: constraint.SAF_RAILING_HEIGHT, Severity: constraint.SeverityWarning, Message: "c"},
		{Code: constraint.GEO_STEP_HEIGHT, Severity: constraint.SeverityWarning, Message: "a"},
	}}
	lvl := validation.BuildReport(vr, false, nil).Level(validation.LevelCodes)
	got := lvl.SortedChecks()
	for i := 1; i < len(got); i++ {
		if got[i-1].Code > got[i].Code {
			t.Fatalf("порядок не отсортирован: %q перед %q", got[i-1].Code, got[i].Code)
		}
	}
	// И исходный срез не должен пострадать от сортировки.
	if lvl.Checks[0].Code == got[0].Code && len(got) > 1 {
		t.Error("SortedChecks отсортировал исходный срез на месте — это меняет отчёт на стороне")
	}
}

// TestInvalidWithoutIssuesIsVisible — невалидный результат без единого
// замечания означает, что описание причины потерялось выше по конвейеру.
// Прятать это за «геометрия посчитана» нельзя: человек увидит зелёный
// уровень при красном заказе.
func TestInvalidWithoutIssuesIsVisible(t *testing.T) {
	r := validation.BuildReport(validation.Result{Valid: false}, false, nil)
	geo := r.Level(validation.LevelGeometry)
	if geo.Status != validation.LevelNotPerformed {
		t.Errorf("статус геометрии %q, ждали not_performed: потерянная причина "+
			"не должна выглядеть как успех", geo.Status)
	}
	if len(geo.Reasons) == 0 {
		t.Error("причина потери не названа")
	}
}
