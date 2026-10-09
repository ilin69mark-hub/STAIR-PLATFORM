package advisor

import (
	"fmt"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

func standard() *constraint.ConstraintSet {
	return constraint.StandardProfile("STANDARD")
}

// inStraight — типовой вход советника для прямого марша.
func inStraight(h, h0 float64) Input {
	return Input{
		Flight:       engineering.FlightStraight,
		HeightMm:     h,
		TargetStepMm: h0,
		ComfortMm:    solver.DefaultComfortStep,
		ClearanceMm:  2100,
		RailingMm:    900,
		// Материал задан явно: без него он выводится из толщины косоура, и
		// после сужения стали до выпуска 3–8 мм толщина 40 мм стала означать
		// дуб — советник молча советовал бы по дубовому листу 9000×4600.
		Material:        "STEEL-S235",
		StringerThickMm: 40,
	}
}

// inNorm asserts сгодится для проверки варианта по всем нормам.
func normOK(set *constraint.ConstraintSet, s validation.Suggestion) bool {
	cand := &engineering.StairConfiguration{
		StepCount: s.StepCount, StepHeight: engineering.Length(s.StepHeightMm),
		TreadDepth: engineering.Length(s.TreadDepthMm),
		Angle:      engineering.Angle(s.AngleDeg * 3.141592653589793 / 180),
		Clearance:  2100, RailingHeight: 900, StringerThickness: 40,
	}
	return !validation.Validate(cand, set).Blocking
}

func TestAdviseBlockingFillsGuideAndSuggestions(t *testing.T) {
	set := standard()
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 230, Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveChecked(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveChecked: %v", err)
	}
	if !vr.Blocking {
		t.Fatalf("want blocking, got %v", vr.Blocking)
	}

	got := Advise(inStraight(2700, 230), set, vr)
	if !got.Blocking {
		t.Fatalf("Advise must keep Blocking")
	}
	found := false
	for _, it := range got.Issues {
		if it.Code != constraint.GEO_ANGLE {
			continue
		}
		found = true
		if it.Param == "" || it.Guide == "" {
			t.Fatalf("angle issue must carry Param and Guide")
		}
		if len(it.Suggestions) == 0 {
			t.Fatalf("angle issue must have suggestions")
		}
		if len(it.Suggestions) > MaxSuggestions {
			t.Fatalf("too many suggestions: %d", len(it.Suggestions))
		}
		for _, s := range it.Suggestions {
			if s.StepHeightMm > 200 || s.StepHeightMm < 150 ||
				s.TreadDepthMm > 320 || s.TreadDepthMm < 260 ||
				s.AngleDeg > 45 || s.AngleDeg < 30 {
				t.Fatalf("suggestion %+v out of norms", s)
			}
			if !normOK(set, s) {
				t.Fatalf("suggestion %+v must pass all norms", s)
			}
		}
	}
	if !found {
		t.Fatalf("no angle issue in result")
	}
}

func TestAdviseStepHeightSuggestion(t *testing.T) {
	set := standard()
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 120, Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveChecked(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveChecked: %v", err)
	}
	if !vr.Blocking {
		t.Fatalf("want blocking (step height 120 < 150)")
	}
	got := Advise(inStraight(2700, 120), set, vr)
	for _, it := range got.Issues {
		if it.Code != constraint.GEO_STEP_HEIGHT {
			continue
		}
		if len(it.Suggestions) == 0 {
			t.Fatalf("step-height issue must have suggestions")
		}
		for _, s := range it.Suggestions {
			if !normOK(set, s) {
				t.Fatalf("suggestion %+v must pass all norms", s)
			}
		}
	}
}

func TestAdviseNonBlockingUnchanged(t *testing.T) {
	set := standard()
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 180, Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveChecked(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveChecked: %v", err)
	}
	if vr.Blocking {
		t.Fatalf("fixture must be valid")
	}
	got := Advise(inStraight(2700, 180), set, vr)
	for _, it := range got.Issues {
		if it.Param != "" || it.Guide != "" || it.Suggestions != nil {
			t.Fatalf("non-blocking must stay untouched: %+v", it)
		}
	}
}

func TestAdviseLShapeSuggestionsCarryLowerCount(t *testing.T) {
	set := standard()
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 230, Flight: engineering.FlightLShape,
		LowerStepCount: 5, LandingWidth: 1000, Width: 900,
		Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveCheckedLShape(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveCheckedLShape: %v", err)
	}
	if !vr.Blocking {
		t.Fatalf("want blocking")
	}
	in := inStraight(2700, 230)
	in.Flight = engineering.FlightLShape
	in.LowerStepCount = 5
	in.LandingMm = 1000

	got := Advise(in, set, vr)
	found := false
	for _, it := range got.Issues {
		if len(it.Suggestions) == 0 {
			continue
		}
		found = true
		for _, s := range it.Suggestions {
			if s.LowerStepCount < 1 || s.LowerStepCount >= s.StepCount {
				t.Fatalf("L-shape suggestion %+v must have 1<=n1<n", s)
			}
			if !normOK(set, s) {
				t.Fatalf("suggestion %+v must pass all norms", s)
			}
		}
	}
	if !found {
		t.Fatalf("no suggestions for L-shape blocking")
	}
}

func TestAdviseWarningRulesGetParamGuide(t *testing.T) {
	set := standard()
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 230, Clearance: 1800, RailingHeight: 700, StringerThickness: 20,
	}
	_, vr, err := solver.SolveChecked(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveChecked: %v", err)
	}
	if !vr.Blocking {
		t.Fatalf("want blocking (step height)")
	}
	got := Advise(inStraight(2700, 230), set, vr)
	have := map[string]bool{}
	for _, it := range got.Issues {
		have[string(it.Code)] = true
		if it.Param == "" || it.Guide == "" {
			t.Fatalf("issue %s must carry Param and Guide", it.Code)
		}
		if it.Code == constraint.GEO_CLEARANCE && it.Suggestions != nil {
			t.Fatalf("clearance must not have numeric suggestions")
		}
	}
	for _, code := range []string{
		string(constraint.GEO_STEP_HEIGHT), string(constraint.GEO_CLEARANCE),
		string(constraint.GEO_STRINGER_THICKNESS), string(constraint.SAF_RAILING_HEIGHT),
	} {
		if !have[code] {
			t.Fatalf("missing issue %s", code)
		}
	}
}

func TestAdviseGuideRenderedFromRuleTemplate(t *testing.T) {
	set := standard()
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 230, Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveChecked(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveChecked: %v", err)
	}
	got := Advise(inStraight(2700, 230), set, vr)

	checked := 0
	for _, it := range got.Issues {
		if strings.Contains(it.Guide, "{") || strings.Contains(it.Guide, "}") {
			t.Fatalf("guide must have no leftover placeholders: %q", it.Guide)
		}
		rule, ok := set.Active(it.Code)
		if !ok || rule.Advice == nil {
			continue
		}
		checked++
		if it.Param != rule.Advice.Param {
			t.Fatalf("%s: param from rule expected %q, got %q", it.Code, rule.Advice.Param, it.Param)
		}
		if it.Code == constraint.GEO_ANGLE {
			wantPrefix := fmt.Sprintf("Угол наклона %.1f°", it.Value)
			if !strings.HasPrefix(it.Guide, wantPrefix) {
				t.Fatalf("%s: guide prefix %q, want %q", it.Code, it.Guide, wantPrefix)
			}
			if !strings.Contains(it.Guide, "2700") {
				t.Fatalf("%s: guide must contain height: %q", it.Code, it.Guide)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no issues with advice were checked")
	}
}

func TestAdviseInfeasibleManufacturingNoDeadEnds(t *testing.T) {
	set := standard()
	// Прямой ДЕРЕВЯННЫЙ марш H=6200: нормо-диапазон существует, но заготовка
	// косоура 9020 мм длиннее сам��го крупного листа дуба 9000 мм. Дерево
	// режется на станке и сварным стыком не собирается, поэтому вариант
	// заведомо нереализуем. Советник не должен его предлагать.
	//
	// Для стали тот же марш изготовляем: длинный косоур режется кусками со
	// сварным стыком (это проверяет TestManufactureFeasibleRespectsMaterial).
	cfg := &engineering.StairConfiguration{
		Height: 6200, StepHeight: 230, Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveChecked(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveChecked: %v", err)
	}
	if !vr.Blocking {
		t.Fatalf("want blocking, got %v", vr.Blocking)
	}
	in := inStraight(6200, 230)
	in.Material = "WOOD-OAK"
	got := Advise(in, set, vr)

	for _, it := range got.Issues {
		if len(it.Suggestions) > 0 {
			t.Fatalf("%s: must not suggest dead-end configurations, got %d suggestions",
				it.Code, len(it.Suggestions))
		}
	}
	issue := findIssue(got.Issues, constraint.MFG_SHEET)
	if issue == nil {
		t.Fatal("must add blocking MFG-SHEET issue when no variant is manufacturable")
	}
	if issue.Param == "" || !strings.Contains(issue.Guide, "9020") || !strings.Contains(issue.Guide, "9000×4600") {
		t.Fatalf("MFG-SHEET issue must explain dims: param=%q guide=%q", issue.Param, issue.Guide)
	}
	if !got.Blocking {
		t.Fatalf("Advise must keep Blocking")
	}
}

func TestAdviseFeasibleKeepsSuggestions(t *testing.T) {
	set := standard()
	// H=2700: косоур 5400×2750 влезает на лист 6000×3000 — варианты остаются.
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 230, Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveChecked(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveChecked: %v", err)
	}
	got := Advise(inStraight(2700, 230), set, vr)
	if findIssue(got.Issues, constraint.MFG_SHEET) != nil {
		t.Fatal("feasible configuration must not add MFG-SHEET issue")
	}
	found := false
	for _, it := range got.Issues {
		if it.Code == constraint.GEO_ANGLE && len(it.Suggestions) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("feasible configuration must keep suggestions")
	}
}

func TestManufactureFeasibleRespectsMaterial(t *testing.T) {
	in := inStraight(5000, 180)
	// Заготовка косоура-гребёнки: длина по склону 8660 мм, постоянная глубина
	// листа 300 мм. Сталь режется лазером по полю стола 3000×1500, причём
	// длинный косоур режется кусками со сварным стыком — 8660 мм против поля
	// 3000 мм не мешает, мешала бы только ширина.
	if !manufactureFeasible(in, [2]float64{8660, 300}) {
		t.Fatal("steel stringer 8660×300 must be feasible: длина режется кусками, ширина 300 в поле")
	}
	// Поперечный рез по ширине недопустим: зубья гребёнки отделятся от спинки.
	if manufactureFeasible(in, [2]float64{8660, 2000}) {
		t.Fatal("steel must not fit a stringer 2000 mm wide: поле стола 1500 мм вместе с отступом")
	}
	// Дуб режется на станке и сварным стыком не собирается: проверяется
	// каталог листов. Максимальный лист дуба 9000×4600.
	in.Material = "WOOD-OAK"
	if !manufactureFeasible(in, [2]float64{8660, 300}) {
		t.Fatal("oak must fit a stringer 8660×300 on sheet 9000×4600")
	}
	if manufactureFeasible(in, [2]float64{9500, 300}) {
		t.Fatal("oak must not fit a stringer 9500 mm long: largest oak sheet is 9000 mm")
	}
}

func TestManufacturingIssueMaterialGuide(t *testing.T) {
	in := inStraight(5000, 180)
	worst := [2]float64{8660, 5050}
	// Дубу подсказка реферирует крупнейший лист дуба, а не стали.
	in.Material = "WOOD-OAK"
	iss := manufacturingIssue(in, worst)
	if !strings.Contains(iss.Guide, "9000×4600") || strings.Contains(iss.Guide, "10400×6200") {
		t.Fatalf("oak guide must reference largest oak sheet, got %q", iss.Guide)
	}
	// Без выбора материала — сталь (макс. 10400×6200). Чтобы подсказка так и
	// работала, толщина косоура должна быть из СТАЛЬНОГО выпуска 3–8 мм:
	// без материала он выводится по толщине, и 40 мм после сужения стали
	// означают дуб.
	in.Material = ""
	in.StringerThickMm = 8
	iss = manufacturingIssue(in, worst)
	if !strings.Contains(iss.Guide, "3000×1500") || strings.Contains(iss.Guide, "9000×4600") {
		t.Fatalf("steel guide must reference laser field 3000×1500, got %q", iss.Guide)
	}
}

func findIssue(issues []validation.Issue, code constraint.RuleCode) *validation.Issue {
	for i := range issues {
		if issues[i].Code == code {
			return &issues[i]
		}
	}
	return nil
}

// TestAdviseUShapeWinderSuggestions проверяет, что для П-образного марша в
// режиме поворотных ступеней (CONF-TURN-KIND=winder) советник предлагает
// варианты с WinderCount ≥ 3, и эти варианты проходят все нормы.
func TestAdviseUShapeWinderSuggestions(t *testing.T) {
	set := standard()
	cfg := &engineering.StairConfiguration{
		Height: 2700, StepHeight: 230, Flight: engineering.FlightUShape,
		LowerStepCount: 5, LandingWidth: 1000, Width: 900,
		TurnKind: engineering.TurnWinder, WinderCount: 3,
		Clearance: 2100, RailingHeight: 900, StringerThickness: 40,
	}
	_, vr, err := solver.SolveCheckedUShape(cfg, set, solver.DefaultComfortStep)
	if err != nil {
		t.Fatalf("SolveCheckedUShape: %v", err)
	}
	in := inStraight(2700, 230)
	in.Flight = engineering.FlightUShape
	in.TurnKind = engineering.TurnWinder
	in.LowerStepCount = 5
	in.WinderCount = 3
	in.LandingMm = 1000

	got := Advise(in, set, vr)
	found := false
	for _, it := range got.Issues {
		for _, s := range it.Suggestions {
			if s.WinderCount < 3 {
				continue
			}
			found = true
			if !normOK(set, s) {
				t.Fatalf("winder suggestion %+v must pass all norms", s)
			}
		}
	}
	if !found {
		t.Fatal("no winder suggestions (WinderCount ≥ 3) produced for U-shape winder mode")
	}
}
