package advisor

import (
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

func TestExtraAdvise_NoAdviceRule(t *testing.T) {
	set := constraint.NewSet("no-advice", "rules without advice")
	rule := &constraint.Constraint{
		Code:     constraint.GEO_STEP_HEIGHT,
		Severity: constraint.SeverityError,
		Version:  1,
		Active:   true,
		Range:    constraint.Range{Min: 150, Max: 200, HasMin: true, HasMax: true},
		Message:  "m",
		Fix:      "f",
	}
	if err := set.Add(rule); err != nil {
		t.Fatal(err)
	}
	if err := set.Activate(constraint.GEO_STEP_HEIGHT, 1); err != nil {
		t.Fatal(err)
	}
	vr := validation.Result{
		Blocking: true,
		Issues:   []validation.Issue{{Code: constraint.GEO_STEP_HEIGHT, Severity: constraint.SeverityError}},
	}
	got := Advise(inStraight(2700, 230), set, vr)
	if got.Blocking != true || len(got.Issues) != 1 {
		t.Fatalf("issue must be left untouched: blocking=%v issues=%d", got.Blocking, len(got.Issues))
	}
	if got.Issues[0].Param != "" {
		t.Fatalf("rule without Advice must not set Param")
	}
}

func TestExtraAdvise_UnknownRule(t *testing.T) {
	set := constraint.NewSet("empty", "no rules")
	vr := validation.Result{
		Blocking: true,
		Issues:   []validation.Issue{{Code: constraint.GEO_TREAD_DEPTH, Severity: constraint.SeverityError}},
	}
	got := Advise(inStraight(2700, 230), set, vr)
	if got.Blocking != true || len(got.Issues) != 1 {
		t.Fatalf("unknown rule must be skipped: blocked=%v issues=%d", got.Blocking, len(got.Issues))
	}
}

func TestExtraHasIssue(t *testing.T) {
	if !hasIssue([]validation.Issue{{Code: constraint.GEO_STEP_HEIGHT}}, constraint.GEO_STEP_HEIGHT) {
		t.Fatal("expected true")
	}
	if hasIssue([]validation.Issue{{Code: constraint.GEO_STEP_HEIGHT}}, constraint.GEO_TREAD_DEPTH) {
		t.Fatal("expected false")
	}
}

func TestExtraManufacturingIssue_MaterialFallback(t *testing.T) {
	// Неизвестный материал + несуществующая толщина → автоназначение STEEL-S235.
	it := manufacturingIssue(Input{Material: dommfg.MaterialCode("UNKNOWN"), StringerThickMm: 1, HeightMm: 5000}, [2]float64{9000, 3000})
	if it.Code != constraint.MFG_SHEET {
		t.Fatalf("code = %s", it.Code)
	}
	if !strings.Contains(it.Guide, "9000×3000") || !strings.Contains(it.Guide, "10400×6200") {
		t.Fatalf("guide must use fallback material sheet: %q", it.Guide)
	}
}

func TestExtraSpiralInfeasibleIssue(t *testing.T) {
	in := inSpiral(6000, 190, 900, 1300)
	it := spiralInfeasibleIssue(in)
	if it.Code != constraint.GEO_SPIRAL_TREAD || it.Severity != constraint.SeverityError {
		t.Fatalf("code/severity = %s/%s", it.Code, it.Severity)
	}
	if !strings.Contains(it.Guide, "6000") {
		t.Fatalf("guide must mention height: %q", it.Guide)
	}
}

func TestExtraGeometry_BadInputs(t *testing.T) {
	set := standard()

	if out, inf, _ := geometry(Input{}, set); out != nil || inf {
		t.Fatalf("zero height must return nil,false: %v %v", out, inf)
	}
	if out, inf, _ := geometry(inStraight(2700, 230), constraint.NewSet("x", "x")); out != nil || inf {
		t.Fatalf("set without GEO_STEP_HEIGHT must return nil,false: %v %v", out, inf)
	}
	// nStart = max(ceil(100/200)-2, 2) → mathMax false-branch (return b).
	out, inf, _ := geometry(Input{Flight: engineering.FlightStraight, HeightMm: 100, ComfortMm: 630}, set)
	if inf || len(out) != 0 {
		t.Fatalf("small height: %v, %v", out, inf)
	}
}

func TestExtraGeometry_SolveErrors(t *testing.T) {
	set := standard()

	in := inStraight(2700, 230)
	in.ComfortMm = 500
	if out, inf, _ := geometry(in, set); inf || len(out) != 0 {
		t.Fatalf("straight bad comfort: %v, %v", out, inf)
	}

	lin := inStraight(2700, 230)
	lin.Flight = engineering.FlightLShape
	lin.LowerStepCount = 6
	lin.LandingMm = 0
	if out, inf, _ := geometry(lin, set); inf || len(out) != 0 {
		t.Fatalf("L-shape zero landing: %v, %v", out, inf)
	}
	lin.LandingMm = 1000
	lin.ComfortMm = 500
	if out, inf, _ := geometry(lin, set); inf || len(out) != 0 {
		t.Fatalf("L-shape bad comfort: %v, %v", out, inf)
	}

	w := inStraight(2700, 230)
	w.Flight = engineering.FlightUShape
	w.TurnKind = engineering.TurnWinder
	w.LowerStepCount = 6
	w.WinderCount = 3
	w.LandingMm = 0
	if out, inf, _ := geometry(w, set); inf || len(out) != 0 {
		t.Fatalf("winder zero landing: %v, %v", out, inf)
	}
	w.LandingMm = 1000
	w.ComfortMm = 500
	if out, inf, _ := geometry(w, set); inf || len(out) != 0 {
		t.Fatalf("winder bad comfort: %v, %v", out, inf)
	}
}

func TestExtraClampF(t *testing.T) {
	if clampF(5, 10, 20) != 10 {
		t.Fatal("below lo")
	}
	if clampF(25, 10, 20) != 20 {
		t.Fatal("above hi")
	}
	if clampF(15, 10, 20) != 15 {
		t.Fatal("inside range")
	}
}

func TestExtraManufactureFeasible(t *testing.T) {
	in := inSpiral(6000, 190, 900, 1300)
	if !manufactureFeasible(in, [2]float64{9000, 3000}) {
		t.Fatal("spiral must always be feasible")
	}
	bad := inStraight(2700, 230)
	bad.Material = dommfg.MaterialCode("UNKNOWN")
	bad.StringerThickMm = 1
	if !manufactureFeasible(bad, [2]float64{9000, 3000}) {
		t.Fatal("unknown material must be treated as feasible")
	}
}

func TestExtraLowerCandidates(t *testing.T) {
	in := inStraight(2700, 230)
	in.Flight = engineering.FlightLShape

	in.LowerStepCount = 0
	if got := lowerCandidates(in, 5); len(got) != 4 {
		t.Fatalf("invalid orig -> round(n/2): %v", got)
	}
	in.LowerStepCount = 0
	if got := lowerCandidates(in, 0); len(got) != 0 {
		t.Fatalf("n=0 must yield empty: %v", got)
	}
	in.LowerStepCount = 1
	if got := lowerCandidates(in, 5); len(got) != 3 {
		t.Fatalf("orig=1 must drop 0 and negative: %v", got)
	}
	in.LowerStepCount = 3
	if got := lowerCandidates(in, 5); len(got) != 4 {
		t.Fatalf("valid orig must probe around: %v", got)
	}
}

func TestExtraWinderCandidates(t *testing.T) {
	in := inStraight(2700, 230)
	in.Flight = engineering.FlightUShape
	in.TurnKind = engineering.TurnWinder
	in.WinderCount = 0
	if got := winderCandidates(in, 8, 3); len(got) != 2 {
		t.Fatalf("orig<3 must reset to 3: %v", got)
	}
}

func TestExtraSpiralGeometry_ZeroWidth(t *testing.T) {
	in := inSpiral(6000, 190, 0, 0)
	out, inf, _ := spiralGeometry(in, standard(), 30, 40)
	if out != nil || inf {
		t.Fatalf("zero width must return nil,false: %v, %v", out, inf)
	}
}

func TestExtraSpreadInts(t *testing.T) {
	if got := spreadInts(5, 3, 2); got != nil {
		t.Fatalf("end<start must return nil: %v", got)
	}
	if got := spreadInts(1, 3, 5); len(got) != 3 {
		t.Fatalf("n<=k must return full range: %v", got)
	}
	if got := spreadInts(1, 10, 3); len(got) != 3 {
		t.Fatalf("spread must return k items: %v", got)
	}
}
