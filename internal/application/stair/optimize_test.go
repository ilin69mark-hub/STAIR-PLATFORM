package stair

import (
	"testing"

	"stairplatform/internal/domain/engineering"
)

func TestOptimizeSingleCandidate(t *testing.T) {
	// Одна точка поиска: n=15, S=630 → известный оптимум = этот же расчёт.
	cfg := referenceConfig()
	req := OptimizeRequest{
		Target:         TargetPrice,
		StepCountMin:   15,
		StepCountMax:   15,
		ComfortStepMin: 630,
		ComfortStepMax: 630,
	}
	svc := NewService()

	out, err := svc.Optimize(cfg, Options{}, req)
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if !out.Valid {
		t.Fatal("expected valid result")
	}
	if out.BestResult.Flight.StepCount != 15 {
		t.Fatalf("step count = %d, want 15", out.BestResult.Flight.StepCount)
	}
	// Цель = итоговая цена лучшего кандидата (мажорные единицы).
	want := out.BestResult.Price.FinalPrice.Major(out.BestResult.Price.Currency)
	if out.Objective != want {
		t.Fatalf("objective = %v, want %v", out.Objective, want)
	}
	// Лучший результат не содержит blocking-валидацию.
	if out.BestResult.Validation.Blocking {
		t.Fatal("best result must not be blocking")
	}
}

func TestOptimizeDeterminism(t *testing.T) {
	cfg := referenceConfig()
	svc := NewService()
	req := OptimizeRequest{Target: TargetPrice}

	first, err := svc.Optimize(cfg, Options{}, req)
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	second, err := svc.Optimize(cfg, Options{}, req)
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if first.BestResult.Flight.StepCount != second.BestResult.Flight.StepCount {
		t.Fatalf("step counts differ: %d vs %d",
			first.BestResult.Flight.StepCount, second.BestResult.Flight.StepCount)
	}
	if first.Objective != second.Objective {
		t.Fatalf("objectives differ: %v vs %v", first.Objective, second.Objective)
	}
}

func TestOptimizeValidBounds(t *testing.T) {
	// H=2700 → n ∈ [ceil(2700/200), floor(2700/150)] = [14, 18].
	cfg := referenceConfig()
	svc := NewService()

	out, err := svc.Optimize(cfg, Options{}, OptimizeRequest{Target: TargetPrice})
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if !out.Valid {
		t.Fatal("expected valid result")
	}
	n := out.BestResult.Flight.StepCount
	if n < 14 || n > 18 {
		t.Fatalf("best step count %d outside [14, 18]", n)
	}
	if out.Evaluated == 0 {
		t.Fatal("expected evaluations")
	}
	if out.BestResult == nil || out.BestResult.Price == nil {
		t.Fatal("expected full best result with price")
	}
}

func TestOptimizeNoValidCandidate(t *testing.T) {
	// n=18 → h=150, b=300, угол ≈ 26.6° < 30° (GEO-ANGLE) → все кандидаты
	// блокируются валидацией → Valid=false.
	cfg := referenceConfig()
	svc := NewService()
	req := OptimizeRequest{
		Target:       TargetPrice,
		StepCountMin: 18,
		StepCountMax: 18,
	}

	out, err := svc.Optimize(cfg, Options{}, req)
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if out.Valid {
		t.Fatal("expected no valid candidate")
	}
	if out.Evaluated == 0 {
		t.Fatal("expected evaluations to have been attempted")
	}
}

func TestOptimizeUnknownTarget(t *testing.T) {
	svc := NewService()
	if _, err := svc.Optimize(referenceConfig(), Options{}, OptimizeRequest{Target: "weight"}); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestOptimizeInvertedRange(t *testing.T) {
	svc := NewService()
	out, err := svc.Optimize(referenceConfig(), Options{}, OptimizeRequest{
		Target:       TargetCost,
		StepCountMin: 10,
		StepCountMax: 2,
	})
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if out.Valid {
		t.Fatal("expected invalid result for inverted range")
	}
}

func TestOptimizeLShape(t *testing.T) {
	cfg := Config{
		Width:             mustLengthHelper(900),
		Height:            mustLengthHelper(2700),
		Flight:            engineering.FlightLShape,
		StepHeight:        mustLengthHelper(180),
		StringerThickness: mustLengthHelper(50),
		StepThickness:     mustLengthHelper(40),
		Clearance:         mustLengthHelper(2500),
		RailingHeight:     mustLengthHelper(1000),
		LandingWidth:      mustLengthHelper(900),
	}
	svc := NewService()

	out, err := svc.Optimize(cfg, Options{}, OptimizeRequest{Target: TargetPrice})
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if !out.Valid {
		t.Fatal("expected valid result")
	}
	res := out.BestResult
	if res.LShape == nil {
		t.Fatal("expected L-shape result")
	}
	if res.LShape.LowerStepCount < 1 || res.LShape.LowerStepCount >= res.LShape.StepCount {
		t.Fatalf("lower step count %d out of [1, n-1]", res.LShape.LowerStepCount)
	}
}

func TestOptimizeSpiral(t *testing.T) {
	// W=700, H=2700, R=960: спираль проходит весь конвейер (solver →
	// geometry → manufacturing/nesting → price) в n≈16 (EDR-0007).
	cfg := Config{
		Width:             mustLengthHelper(700),
		Height:            mustLengthHelper(2700),
		Flight:            engineering.FlightSpiral,
		StepHeight:        mustLengthHelper(180),
		StringerThickness: mustLengthHelper(50),
		StepThickness:     mustLengthHelper(40),
		Clearance:         mustLengthHelper(2500),
		RailingHeight:     mustLengthHelper(1000),
		OuterRadius:       mustLengthHelper(960),
	}
	svc := NewService()

	out, err := svc.Optimize(cfg, Options{}, OptimizeRequest{Target: TargetPrice})
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if !out.Valid {
		t.Fatal("expected valid result")
	}
	if out.BestResult.Spiral == nil {
		t.Fatal("expected spiral result")
	}
}
