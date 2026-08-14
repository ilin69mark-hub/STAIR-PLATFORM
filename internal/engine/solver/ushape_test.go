package solver

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
)

// Эталонный пример EDR-0006: H=2700, h0=180, S=630 → n=15, h=180,
// b=270. n1=6 → n2=9, H1=1080, H2=1620, L1=1620, L2=2430 — идентично
// L-образному (EDR-0005 §7); П-образный отличается только геометрией.
func TestSolveUShapeFormulas(t *testing.T) {
	res, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 6, mustLength(t, 1000))
	if err != nil {
		t.Fatalf("solve error: %v", err)
	}
	if res.StepCount != 15 {
		t.Fatalf("n = %d, want 15", res.StepCount)
	}
	if res.LowerStepCount != 6 || res.UpperStepCount != 9 {
		t.Fatalf("split = %d/%d, want 6/9", res.LowerStepCount, res.UpperStepCount)
	}
	if !nearlyEqual(res.StepHeight.Millimeters(), 180) {
		t.Fatalf("h = %v, want 180", res.StepHeight.Millimeters())
	}
	if !nearlyEqual(res.TreadDepth.Millimeters(), 270) {
		t.Fatalf("b = %v, want 270", res.TreadDepth.Millimeters())
	}
	if !nearlyEqual(res.LowerHeight.Millimeters(), 1080) {
		t.Fatalf("H1 = %v, want 1080", res.LowerHeight.Millimeters())
	}
	if !nearlyEqual(res.UpperHeight.Millimeters(), 1620) {
		t.Fatalf("H2 = %v, want 1620", res.UpperHeight.Millimeters())
	}
	if !nearlyEqual(res.LowerRun.Millimeters(), 1620) {
		t.Fatalf("L1 = %v, want 1620", res.LowerRun.Millimeters())
	}
	if !nearlyEqual(res.UpperRun.Millimeters(), 2430) {
		t.Fatalf("L2 = %v, want 2430", res.UpperRun.Millimeters())
	}
	// R1 = sqrt(L1^2 + H1^2), R2 = sqrt(L2^2 + H2^2).
	if !nearlyEqual(res.LowerStringer.Millimeters(), math.Sqrt(1620*1620+1080*1080)) {
		t.Fatalf("R1 = %v", res.LowerStringer.Millimeters())
	}
	if !nearlyEqual(res.UpperStringer.Millimeters(), math.Sqrt(2430*2430+1620*1620)) {
		t.Fatalf("R2 = %v", res.UpperStringer.Millimeters())
	}
	if !nearlyEqual(res.Angle.Radians(), math.Atan(180/270.0)) {
		t.Fatalf("alpha = %v", res.Angle.Radians())
	}
	if !nearlyEqual(res.LandingWidth.Millimeters(), 1000) {
		t.Fatalf("Wp = %v, want 1000", res.LandingWidth.Millimeters())
	}
}

func TestSolveUShapeComfortStepBounds(t *testing.T) {
	if _, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 6, mustLength(t, 1000), 599); err == nil {
		t.Fatal("comfort step below 600 must be rejected")
	}
	if _, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 6, mustLength(t, 1000), 641); err == nil {
		t.Fatal("comfort step above 640 must be rejected")
	}
}

func TestSolveUShapeEdgeCases(t *testing.T) {
	// n1 = 0 или n1 = n → ошибка разбивки.
	if _, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 0, mustLength(t, 1000)); err == nil {
		t.Fatal("n1 = 0 must error")
	}
	if _, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 15, mustLength(t, 1000)); err == nil {
		t.Fatal("n1 = n must error")
	}
	if _, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 16, mustLength(t, 1000)); err == nil {
		t.Fatal("n1 > n must error")
	}
	// Wp <= 0 → ошибка.
	if _, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 6, mustLength(t, 0)); err == nil {
		t.Fatal("Wp <= 0 must error")
	}
	// H слишком мало → n < 1.
	if _, err := SolveUShape(mustLength(t, 80), mustLength(t, 180), 1, mustLength(t, 1000)); err == nil {
		t.Fatal("rise without flight must error")
	}
	// b <= 0.
	if _, err := SolveUShape(mustLength(t, 700), mustLength(t, 350), 1, mustLength(t, 1000), 600); err == nil {
		t.Fatal("non-positive tread depth must error")
	}
}

func TestSolveUShapeDeterminism(t *testing.T) {
	a, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 6, mustLength(t, 1000))
	if err != nil {
		t.Fatal(err)
	}
	b, err := SolveUShape(mustLength(t, 2700), mustLength(t, 180), 6, mustLength(t, 1000))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("solve must be deterministic:\n%+v\n%+v", a, b)
	}
}

func TestSolveUShapeApplyWritesConfig(t *testing.T) {
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightUShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 180)
	cfg.LowerStepCount = 6
	cfg.LandingWidth = mustLength(t, 1000)
	res, err := SolveUShape(cfg.Height, cfg.StepHeight, cfg.LowerStepCount, cfg.LandingWidth)
	if err != nil {
		t.Fatal(err)
	}
	res.Apply(cfg)
	if cfg.StepCount != 15 || cfg.LowerStepCount != 6 {
		t.Fatalf("config not applied: %+v", cfg)
	}
	if cfg.Length.Millimeters() != 1620 {
		t.Fatalf("length = %v, want 1620", cfg.Length.Millimeters())
	}
	if cfg.LandingWidth.Millimeters() != 1000 {
		t.Fatalf("landing width not applied: %v", cfg.LandingWidth.Millimeters())
	}
}

func TestSolveCheckedUShapeAppliesWhenValid(t *testing.T) {
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightUShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 180)
	cfg.LowerStepCount = 6
	cfg.LandingWidth = mustLength(t, 1000)
	_, vr, err := SolveCheckedUShape(cfg, constraint.StandardProfile("standard"))
	if err != nil {
		t.Fatal(err)
	}
	if vr.Blocking || !vr.Valid {
		t.Fatalf("expected non-blocking valid result, got %+v", vr)
	}
	if cfg.StepCount != 15 {
		t.Fatalf("config must be applied when valid, got %+v", cfg)
	}
}

func TestSolveCheckedUShapeLandingWidthError(t *testing.T) {
	// Wp < W → ошибка (EDR-0006 §7).
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightUShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 180)
	cfg.LowerStepCount = 6
	cfg.LandingWidth = mustLength(t, 500) // < 900
	if _, _, err := SolveCheckedUShape(cfg, constraint.StandardProfile("standard")); err == nil {
		t.Fatal("Wp < W must error")
	}
}

func TestSolveCheckedUShapeBlockingRollback(t *testing.T) {
	// h0=250: n=11, h=245.45, b=630-490.9=139 < 260 → Error (GEO-TREAD-DEPTH).
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightUShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 250)
	cfg.LowerStepCount = 5
	cfg.LandingWidth = mustLength(t, 1000)
	res, vr, err := SolveCheckedUShape(cfg, constraint.StandardProfile("standard"))
	if err != nil {
		t.Fatalf("solve error: %v", err)
	}
	if !vr.Blocking {
		t.Fatalf("expected blocking result, got %+v", vr)
	}
	if cfg.StepCount != 0 || cfg.Length.Millimeters() != 0 {
		t.Fatalf("config must be rolled back on blocking, got %+v", cfg)
	}
	if res.StepCount == 0 {
		t.Fatal("returned UShapeResult must still hold computed values")
	}
}
