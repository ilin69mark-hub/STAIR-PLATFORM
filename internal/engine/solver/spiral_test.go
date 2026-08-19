package solver

import (
	"errors"
	"math"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
)

func TestSolveSpiralNominal(t *testing.T) {
	// EDR-0007 §4: H=2700, h0=180 → n=15, h=180; δ=2π/15.
	// r = R − W = 800−500 = 300; r_walk = r + 2/3·W = 300 + 333.333.
	wantDelta := 2 * math.Pi / 15
	wantR := 300.0
	wantRWalk := wantR + (2.0/3.0)*500
	wantBIn := wantDelta * wantR
	wantBWalk := wantDelta * wantRWalk
	wantBOut := wantDelta * 800
	wantS := 2*180 + wantBWalk
	wantArc := wantDelta * 800 * 15
	wantAlpha := math.Atan(180 / wantBWalk)

	res, err := SolveSpiral(mustLength(t, 2700), mustLength(t, 180), mustLength(t, 500), mustLength(t, 800))
	if err != nil {
		t.Fatalf("SolveSpiral: %v", err)
	}

	if res.StepCount != 15 {
		t.Errorf("StepCount = %d, want 15", res.StepCount)
	}
	if !nearlyEqual(res.StepHeight.Millimeters(), 180) {
		t.Errorf("StepHeight = %v, want 180", res.StepHeight.Millimeters())
	}
	if !nearlyEqual(res.OuterRadius.Millimeters(), 800) {
		t.Errorf("OuterRadius = %v, want 800", res.OuterRadius.Millimeters())
	}
	if !nearlyEqual(res.ColumnRadius.Millimeters(), wantR) {
		t.Errorf("ColumnRadius = %v, want %v", res.ColumnRadius.Millimeters(), wantR)
	}
	if !nearlyEqual(res.WalkRadius.Millimeters(), wantRWalk) {
		t.Errorf("WalkRadius = %v, want %v", res.WalkRadius.Millimeters(), wantRWalk)
	}
	if !nearlyEqual(res.InnerTread.Millimeters(), wantBIn) {
		t.Errorf("InnerTread = %v, want %v", res.InnerTread.Millimeters(), wantBIn)
	}
	if !nearlyEqual(res.WalkTread.Millimeters(), wantBWalk) {
		t.Errorf("WalkTread = %v, want %v", res.WalkTread.Millimeters(), wantBWalk)
	}
	if !nearlyEqual(res.OuterTread.Millimeters(), wantBOut) {
		t.Errorf("OuterTread = %v, want %v", res.OuterTread.Millimeters(), wantBOut)
	}
	if !nearlyEqual(res.ComfortStep, wantS) {
		t.Errorf("ComfortStep = %v, want %v", res.ComfortStep, wantS)
	}
	if math.Abs(res.AngularTotal-FullTurnRadians) > 1e-12 {
		t.Errorf("AngularTotal = %v, want 2π", res.AngularTotal)
	}
	if math.Abs(res.AngularStep-wantDelta) > 1e-12 {
		t.Errorf("AngularStep = %v, want %v", res.AngularStep, wantDelta)
	}
	if !nearlyEqual(res.ArcLength.Millimeters(), wantArc) {
		t.Errorf("ArcLength = %v, want %v", res.ArcLength.Millimeters(), wantArc)
	}
	if !nearlyEqual(res.Angle.Radians(), wantAlpha) {
		t.Errorf("Angle = %v rad, want %v rad", res.Angle.Radians(), wantAlpha)
	}
	if math.Abs(res.Angle.Degrees()-math.Atan(180/wantBWalk)*180/math.Pi) > 0.1 {
		t.Errorf("Angle = %v deg, want ~%.2f", res.Angle.Degrees(), math.Atan(180/wantBWalk)*180/math.Pi)
	}
}

func TestSolveSpiralEdgeCases(t *testing.T) {
	cases := []struct {
		name             string
		H, h0, W, R      float64
		wantErrSubstring string
	}{
		{"zero rise", 0, 180, 500, 800, "Высота подъёма"},
		{"zero target riser", 2700, 0, 500, 800, "Высота ступени"},
		{"zero width", 2700, 180, 0, 800, "Ширина марша"},
		{"R equals W", 2700, 180, 800, 800, "Радиус спирали"},
		{"R below W", 2700, 180, 900, 800, "Радиус спирали"},
		{"tiny rise", 50, 180, 500, 800, "не содержит ступеней"},
		{"inner tread collapse", 2700, 180, 300, 400, "Проступь у колонны"},
		{"outer tread collapse", 5400, 180, 500, 1000, "Проступь у наружной кромки"},
		{"walk tread collapse", 2700, 180, 500, 3000, "Проступь по линии хода"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SolveSpiral(
				mustLength(t, tc.H),
				mustLength(t, tc.h0),
				mustLength(t, tc.W),
				mustLength(t, tc.R),
			)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var inp *InputError
			if !errors.As(err, &inp) {
				t.Fatalf("expected *InputError, got %T", err)
			}
			if !strings.Contains(err.Error(), tc.wantErrSubstring) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSubstring)
			}
		})
	}
}

func TestSolveSpiralComfortStepRejected(t *testing.T) {
	// H=2700 h0=180 → h=180, δ=2π/15. r_walk = R−W + 2/3W = R − W/3.
	// Для b_walk = δ·r_walk в норме S=2h+b_walk ∈ 600-640 (625.29 на R=800).
	// Нарушаем шаг комфорта, уменьшая проступь: R=700 → r=200, r_walk=533.33,
	// b_walk=223.4 → S=583.4 < 600 → ошибка (входит в "walk-line tread", но
	// суть — вне допустимого диапазона проступи).
	if _, err := SolveSpiral(
		mustLength(t, 2700), mustLength(t, 180), mustLength(t, 500), mustLength(t, 700),
	); err == nil {
		t.Error("expected walk tread out of range error")
	}
}

func TestSolveSpiralApply(t *testing.T) {
	w, _ := engineering.NewLength(500)
	h, _ := engineering.NewLength(2700)
	cfg := &engineering.StairConfiguration{
		Width:       w,
		Height:      h,
		Flight:      engineering.FlightSpiral,
		StepCount:   1,
		StepHeight:  mustLength(t, 180),
		OuterRadius: mustLength(t, 800),
	}
	res, err := SolveSpiral(h, mustLength(t, 180), w, mustLength(t, 800))
	if err != nil {
		t.Fatalf("SolveSpiral: %v", err)
	}
	res.Apply(cfg)
	if cfg.StepCount != 15 {
		t.Errorf("cfg.StepCount = %d, want 15", cfg.StepCount)
	}
	wantBWalk := (2 * math.Pi / 15) * (300 + (2.0/3.0)*500)
	if !nearlyEqual(cfg.TreadDepth.Millimeters(), wantBWalk) {
		t.Errorf("cfg.TreadDepth = %v, want walk tread %v", cfg.TreadDepth.Millimeters(), wantBWalk)
	}
}

func TestSolveCheckedSpiral(t *testing.T) {
	t.Run("valid spiral passes validation", func(t *testing.T) {
		cfg := &engineering.StairConfiguration{
			Width:       mustLength(t, 500),
			Height:      mustLength(t, 2700),
			Flight:      engineering.FlightSpiral,
			StepCount:   1,
			StepHeight:  mustLength(t, 180),
			OuterRadius: mustLength(t, 800),
		}
		res, vr, err := SolveCheckedSpiral(cfg, constraint.StandardProfile("standard"))
		if err != nil {
			t.Fatalf("SolveCheckedSpiral: %v", err)
		}
		if vr.Blocking || !vr.Valid {
			t.Fatalf("expected non-blocking valid result, got %+v", vr)
		}
		if cfg.StepCount != 15 {
			t.Fatalf("config must be applied when valid, got %+v", cfg)
		}
		if res.StepCount != 15 {
			t.Fatalf("res.StepCount = %d, want 15", res.StepCount)
		}
	})

	t.Run("blocking issue rolls back config", func(t *testing.T) {
		// Узкий набор: высота ступени должна быть в 150-160, h=180 вне.
		set := constraint.NewSet("cs", "test")
		if err := set.Add(&constraint.Constraint{
			Code: constraint.GEO_STEP_HEIGHT, Category: "geometry", Version: 1, Active: true,
			Severity: constraint.SeverityError,
			Range:    constraint.Range{Min: 150, Max: 160, HasMin: true, HasMax: true, Tolerance: 0.1},
			Message:  "step height 150-160", Fix: "fix",
		}); err != nil {
			t.Fatal(err)
		}
		cfg := &engineering.StairConfiguration{
			Width:       mustLength(t, 500),
			Height:      mustLength(t, 2700),
			Flight:      engineering.FlightSpiral,
			StepCount:   1,
			StepHeight:  mustLength(t, 180),
			OuterRadius: mustLength(t, 800),
		}
		res, vr, err := SolveCheckedSpiral(cfg, set)
		if err != nil {
			t.Fatalf("SolveCheckedSpiral: %v", err)
		}
		if !vr.Blocking {
			t.Fatalf("expected blocking result, got %+v", vr)
		}
		if cfg.StepCount != 0 || cfg.StepHeight.Millimeters() != 0 {
			t.Fatalf("config must be rolled back on blocking, got %+v", cfg)
		}
		if res.StepCount == 0 {
			t.Fatal("returned SpiralResult must still hold computed values")
		}
	})
}

func TestSolveSpiralDeterminism(t *testing.T) {
	a, err := SolveSpiral(mustLength(t, 2700), mustLength(t, 180), mustLength(t, 500), mustLength(t, 800))
	if err != nil {
		t.Fatalf("SolveSpiral: %v", err)
	}
	b, err := SolveSpiral(mustLength(t, 2700), mustLength(t, 180), mustLength(t, 500), mustLength(t, 800))
	if err != nil {
		t.Fatalf("SolveSpiral: %v", err)
	}
	if a != b {
		t.Error("spiral solver must be deterministic")
	}
}
