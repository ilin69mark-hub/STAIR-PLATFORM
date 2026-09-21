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
		Flight:          engineering.FlightStraight,
		HeightMm:        h,
		TargetStepMm:    h0,
		ComfortMm:       solver.DefaultComfortStep,
		ClearanceMm:     2100,
		RailingMm:       900,
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
	// Прямой марш H=6200: нормо-диапазон существует, но косоур шириной
	// H−st=6200 (+ рез 3 мм) не влезет на самый большой лист (10400×6200).
	// Советник не предлагает заведомо нереализуемые варианты.
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
	got := Advise(inStraight(6200, 230), set, vr)

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
	if issue.Param == "" || !strings.Contains(issue.Guide, "9020") || !strings.Contains(issue.Guide, "10400×6200") {
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
	worst := [2]float64{8660, 5050}
	// Без материала — сталь по толщине: косоур 8660×5050 влезает на 10400×6200.
	if !manufactureFeasible(in, worst) {
		t.Fatal("steel (default) must fit stringer on 10400×6200")
	}
	// Дуб: макс. лист 9000×4600 — ширина 5050 не влезает.
	in.Material = "WOOD-OAK"
	if manufactureFeasible(in, worst) {
		t.Fatal("oak must not fit 8660×5050 stringer")
	}
	// Алюминий: макс. лист 9000×4600 — ширина 5050 не влезает.
	in.Material = "ALUM-5083"
	if manufactureFeasible(in, worst) {
		t.Fatal("aluminum must not fit 8660×5050 stringer")
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
	// Без выбора материала — сталь (макс. 10400×6200).
	in.Material = ""
	iss = manufacturingIssue(in, worst)
	if !strings.Contains(iss.Guide, "10400×6200") {
		t.Fatalf("steel guide must reference 10400×6200, got %q", iss.Guide)
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

// inSpiral — типовой вход советника для спирального марша (EDR-0007).
// Высота (6000 мм) и целевая ступень (190 мм) фиксированы: все спиральные
// кейсы тестируются на них; варьируются ширина и радиус.
func inSpiral(w, r float64) Input {
	return Input{
		Flight:          engineering.FlightSpiral,
		HeightMm:        6000,
		TargetStepMm:    190,
		ComfortMm:       solver.DefaultComfortStep,
		WidthMm:         w,
		OuterRadiusMm:   r,
		ClearanceMm:     2300,
		RailingMm:       1100,
		StringerThickMm: 40,
	}
}

// spiralOK проверяет, что вариант винтовой лестницы решается без ошибок
// (проходит все инварианты EDR-0007, проверяемые в solveSpiral).
func spiralOK(s validation.Suggestion) bool {
	_, err := solver.SolveSpiral(
		engineering.Length(s.StepHeightMm*float64(s.StepCount)),
		engineering.Length(s.StepHeightMm),
		engineering.Length(s.WidthMm),
		engineering.Length(s.OuterRadiusMm),
	)
	return err == nil
}

func TestAdviseSpiralSuggestionsCarryRadiusAndWidth(t *testing.T) {
	set := standard()
	// Блокирующий результат — как приложенческий слой строит его из
	// solver.InputError спирали (см. internal/application/stair/errors.go).
	vr := validation.Result{Valid: false, Blocking: true, Issues: []validation.Issue{{
		ID: "ISSUE-INPUT", Code: constraint.GEO_SPIRAL_TREAD,
		Severity: constraint.SeverityError, Element: "configuration",
		Message: "Проступь по линии хода вне диапазона 260–320 мм",
		Value:   543, Min: 260, Max: 320, HasMin: true, HasMax: true,
	}}}

	got := Advise(inSpiral(1000, 3100), set, vr)
	issue := findIssue(got.Issues, constraint.GEO_SPIRAL_TREAD)
	if issue == nil {
		t.Fatalf("missing GEO_SPIRAL_TREAD issue: %+v", got.Issues)
	}
	if issue.Param == "" || issue.Guide == "" {
		t.Fatalf("spiral issue must carry Param and Guide")
	}
	if strings.Contains(issue.Guide, "{") || strings.Contains(issue.Guide, "}") {
		t.Fatalf("spiral guide must have no leftover placeholders: %q", issue.Guide)
	}
	if len(issue.Suggestions) == 0 {
		t.Fatalf("spiral blocking must have suggestions")
	}
	if len(issue.Suggestions) > MaxSuggestions {
		t.Fatalf("too many suggestions: %d", len(issue.Suggestions))
	}
	for _, s := range issue.Suggestions {
		if s.OuterRadiusMm <= 0 || s.WidthMm <= 0 {
			t.Fatalf("spiral suggestion %+v must carry radius and width", s)
		}
		if s.StepHeightMm > 200 || s.StepHeightMm < 150 {
			t.Fatalf("spiral suggestion %+v: step height out of norms", s)
		}
		if !spiralOK(s) {
			t.Fatalf("spiral suggestion %+v must solve without errors", s)
		}
	}
}

func TestAdviseSpiralReducesInfeasibleWidth(t *testing.T) {
	set := standard()
	// Ширина 3000 мм при высоте 6000 мм несовместима ни с каким радиусом:
	// проступь у колонны < 100 мм. Советник должен предложить варианты с
	// уменьшенной шириной.
	vr := validation.Result{Valid: false, Blocking: true, Issues: []validation.Issue{{
		ID: "ISSUE-INPUT", Code: constraint.GEO_SPIRAL_TREAD,
		Severity: constraint.SeverityError, Element: "configuration",
		Message: "Проступь у колонны меньше 100 мм",
		Value:   20, Min: 100, HasMin: true,
	}}}

	got := Advise(inSpiral(3000, 3100), set, vr)
	issue := findIssue(got.Issues, constraint.GEO_SPIRAL_TREAD)
	if issue == nil {
		t.Fatalf("missing GEO_SPIRAL_TREAD issue: %+v", got.Issues)
	}
	if len(issue.Suggestions) == 0 {
		t.Fatalf("must suggest width-reduced variants, got none")
	}
	for _, s := range issue.Suggestions {
		if s.WidthMm >= 3000 {
			t.Fatalf("suggestion %+v must reduce the infeasible width", s)
		}
		if !spiralOK(s) {
			t.Fatalf("suggestion %+v must solve without errors", s)
		}
	}
}

func TestAdviseSpiralGuideFromRuleTemplate(t *testing.T) {
	set := standard()
	vr := validation.Result{Valid: false, Blocking: true, Issues: []validation.Issue{{
		ID: "ISSUE-INPUT", Code: constraint.GEO_SPIRAL_TREAD,
		Severity: constraint.SeverityError, Element: "configuration",
		Message: "Проступь у колонны меньше 100 мм",
		Value:   20, Min: 100, HasMin: true,
	}}}
	got := Advise(inSpiral(3000, 3100), set, vr)
	issue := findIssue(got.Issues, constraint.GEO_SPIRAL_TREAD)
	if issue == nil {
		t.Fatalf("missing GEO_SPIRAL_TREAD issue")
	}
	rule, ok := set.Active(constraint.GEO_SPIRAL_TREAD)
	if !ok || rule.Advice == nil {
		t.Fatalf("GEO_SPIRAL_TREAD must carry advice")
	}
	if issue.Param != rule.Advice.Param {
		t.Fatalf("param from rule expected %q, got %q", rule.Advice.Param, issue.Param)
	}
	if !strings.Contains(issue.Guide, "20") {
		t.Fatalf("spiral guide must mention the violated value: %q", issue.Guide)
	}
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
