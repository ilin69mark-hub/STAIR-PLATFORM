package solver

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
)

func mustLength(t *testing.T, mm float64) engineering.Length {
	t.Helper()
	l, err := engineering.NewLength(mm)
	if err != nil {
		t.Fatalf("NewLength(%v): %v", mm, err)
	}
	return l
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6
}

// H=2700, h0=180, S=630:
// n=15, h=180, b=630-360=270, L=4050, R=sqrt(4050^2+2700^2)=4869.324...
func TestSolveFormulas(t *testing.T) {
	res, err := Solve(mustLength(t, 2700), mustLength(t, 180))
	if err != nil {
		t.Fatalf("solve error: %v", err)
	}
	if res.StepCount != 15 {
		t.Fatalf("n = %d, want 15", res.StepCount)
	}
	if !nearlyEqual(res.StepHeight.Millimeters(), 180) {
		t.Fatalf("h = %v, want 180", res.StepHeight.Millimeters())
	}
	if !nearlyEqual(res.TreadDepth.Millimeters(), 270) {
		t.Fatalf("b = %v, want 270", res.TreadDepth.Millimeters())
	}
	if !nearlyEqual(res.Run.Millimeters(), 4050) {
		t.Fatalf("L = %v, want 4050", res.Run.Millimeters())
	}
	if !nearlyEqual(res.Stringer.Millimeters(), math.Sqrt(4050*4050+2700*2700)) {
		t.Fatalf("R = %v", res.Stringer.Millimeters())
	}
	if !nearlyEqual(res.Angle.Radians(), math.Atan(180/270.0)) {
		t.Fatalf("alpha = %v", res.Angle.Radians())
	}
}

func TestSolveCustomComfortStep(t *testing.T) {
	// S=600: b = 600 - 2*180 = 240.
	res, err := Solve(mustLength(t, 2700), mustLength(t, 180), 600)
	if err != nil {
		t.Fatalf("solve error: %v", err)
	}
	if !nearlyEqual(res.TreadDepth.Millimeters(), 240) {
		t.Fatalf("b = %v, want 240", res.TreadDepth.Millimeters())
	}
}

func TestSolveComfortStepBounds(t *testing.T) {
	if _, err := Solve(mustLength(t, 2700), mustLength(t, 180), 599); err == nil {
		t.Fatal("comfort step below 600 must be rejected")
	}
	if _, err := Solve(mustLength(t, 2700), mustLength(t, 180), 641); err == nil {
		t.Fatal("comfort step above 640 must be rejected")
	}
}

func TestSolveNoFlight(t *testing.T) {
	// H < h0/2: n = round(80/180) = round(0.444) = 0 < 1.
	if _, err := Solve(mustLength(t, 80), mustLength(t, 180)); err == nil {
		t.Fatal("rise without flight (n<1) must error")
	}
}

func TestSolveInvalidTread(t *testing.T) {
	// S=600, h=350: b = 600 - 700 = -100.
	if _, err := Solve(mustLength(t, 700), mustLength(t, 350), 600); err == nil {
		t.Fatal("non-positive tread depth must error")
	}
}

func TestSolveDeterminism(t *testing.T) {
	a, err := Solve(mustLength(t, 2700), mustLength(t, 180))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Solve(mustLength(t, 2700), mustLength(t, 180))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("solve must be deterministic:\n%+v\n%+v", a, b)
	}
}

func TestApplyWritesConfig(t *testing.T) {
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 180)
	res, err := Solve(cfg.Height, cfg.StepHeight)
	if err != nil {
		t.Fatal(err)
	}
	res.Apply(cfg)
	if cfg.StepCount != 15 || cfg.Length.Millimeters() != 4050 {
		t.Fatalf("config not applied: %+v", cfg)
	}
	if cfg.StringerLength.Millimeters() <= 0 {
		t.Fatal("stringer length must be written")
	}
}

func TestSolveCheckedBlockingRollback(t *testing.T) {
	// h0=250: n=11, h=245.45, b=630-490.9=139 < 260 → Error (GEO-TREAD-DEPTH).
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 250)
	res, vr, err := SolveChecked(cfg, constraint.StandardProfile("standard"))
	if err != nil {
		t.Fatalf("solve error: %v", err)
	}
	if !vr.Blocking {
		t.Fatalf("expected blocking result, got %+v", vr)
	}
	// результат не применён — конфигурация в исходном состоянии.
	if cfg.StepCount != 0 || cfg.Length.Millimeters() != 0 || cfg.StringerLength.Millimeters() != 0 {
		t.Fatalf("config must be rolled back on blocking, got %+v", cfg)
	}
	if res.StepCount == 0 {
		t.Fatal("returned FlightResult must still hold computed values")
	}
}

func TestSolveCheckedAppliesWhenValid(t *testing.T) {
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 180)
	_, vr, err := SolveChecked(cfg, constraint.StandardProfile("standard"))
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
