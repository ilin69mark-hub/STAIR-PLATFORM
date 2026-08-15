package optimization

import (
	"context"
	"testing"
)

func TestSearchPicksMinimumValid(t *testing.T) {
	// Цель = n; валидны только чётные n. Минимум — n=2.
	eval := func(c Candidate) (bool, Objective) {
		if c.StepCount%2 != 0 {
			return false, 0
		}
		return true, Objective(c.StepCount)
	}
	got := Search(context.Background(), eval, Options{StepCountMin: 1, StepCountMax: 10})
	if !got.Valid {
		t.Fatal("expected valid result")
	}
	if got.Best.StepCount != 2 {
		t.Fatalf("best step count = %d, want 2", got.Best.StepCount)
	}
	if got.Value != 2 {
		t.Fatalf("value = %v, want 2", got.Value)
	}
	if got.Evaluated != 10 {
		t.Fatalf("evaluated = %d, want 10", got.Evaluated)
	}
}

func TestSearchMaximize(t *testing.T) {
	eval := func(c Candidate) (bool, Objective) {
		return true, Objective(c.StepCount)
	}
	got := Search(context.Background(), eval, Options{StepCountMin: 1, StepCountMax: 5, Maximize: true})
	if got.Best.StepCount != 5 {
		t.Fatalf("best = %d, want 5", got.Best.StepCount)
	}
}

func TestSearchTieFirstWins(t *testing.T) {
	// Все значения равны — выигрывает первый встреченный (n=1).
	eval := func(c Candidate) (bool, Objective) {
		return true, 42
	}
	got := Search(context.Background(), eval, Options{StepCountMin: 1, StepCountMax: 5})
	if got.Best.StepCount != 1 {
		t.Fatalf("best = %d, want 1 (first on tie)", got.Best.StepCount)
	}
}

func TestSearchNoValid(t *testing.T) {
	eval := func(c Candidate) (bool, Objective) {
		return false, 0
	}
	got := Search(context.Background(), eval, Options{StepCountMin: 1, StepCountMax: 5})
	if got.Valid {
		t.Fatal("expected invalid result")
	}
	if got.Evaluated != 5 {
		t.Fatalf("evaluated = %d, want 5", got.Evaluated)
	}
}

func TestSearchInvertedRange(t *testing.T) {
	got := Search(context.Background(), func(c Candidate) (bool, Objective) { return true, 0 },
		Options{StepCountMin: 5, StepCountMax: 1})
	if got.Valid {
		t.Fatal("expected invalid for inverted range")
	}
}

func TestSearchNilEvaluator(t *testing.T) {
	if got := Search(context.Background(), nil, Options{StepCountMin: 1, StepCountMax: 5}); got.Valid {
		t.Fatal("expected invalid for nil evaluator")
	}
}

func TestSearchCancelled(t *testing.T) {
	// Отменённый контекст: поиск не выполняет оценок и помечает Cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eval := func(c Candidate) (bool, Objective) { return true, 1 }
	got := Search(ctx, eval, Options{StepCountMin: 1, StepCountMax: 5})
	if !got.Cancelled {
		t.Fatal("expected Cancelled for cancelled context")
	}
	if got.Evaluated != 0 {
		t.Fatalf("evaluated = %d, want 0", got.Evaluated)
	}
}

func TestSearchCancelledMidway(t *testing.T) {
	// Отмена после нескольких оценок: накопленный лучший сохраняется,
	// а результат помечается Cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := 0
	eval := func(c Candidate) (bool, Objective) {
		count++
		if count == 3 {
			cancel()
		}
		return true, Objective(c.StepCount)
	}
	got := Search(ctx, eval, Options{StepCountMin: 1, StepCountMax: 100, ComfortMin: 0, ComfortMax: 0})
	if !got.Cancelled {
		t.Fatal("expected Cancelled for mid-search cancellation")
	}
	if count != 3 {
		t.Fatalf("evaluations = %d, want 3", count)
	}
	if !got.Valid {
		t.Fatal("expected best preserved from before cancellation")
	}
	if got.Best.StepCount != 1 {
		t.Fatalf("best = %d, want 1", got.Best.StepCount)
	}
}

func TestSearchComfortGridPoints(t *testing.T) {
	var steps []float64
	eval := func(c Candidate) (bool, Objective) {
		steps = append(steps, c.ComfortStep)
		return true, Objective(c.ComfortStep)
	}
	// n вырожден (одна точка), сетка 600..604 с шагом 2 → 600,602,604.
	Search(context.Background(), eval, Options{
		StepCountMin: 1, StepCountMax: 1,
		LowerStepMin: 1, LowerStepMax: 1,
		ComfortMin: 600, ComfortMax: 604, ComfortStep: 2,
	})
	want := []float64{600, 602, 604}
	if len(steps) != len(want) {
		t.Fatalf("points = %v, want %v", steps, want)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("points = %v, want %v", steps, want)
		}
	}
}

func TestSearchDegenerateComfort(t *testing.T) {
	// max <= min → одна точка сетки, но пробег по n1.
	count := 0
	eval := func(c Candidate) (bool, Objective) {
		count++
		return true, Objective(c.LowerStepCount)
	}
	got := Search(context.Background(), eval, Options{
		StepCountMin: 1, StepCountMax: 1,
		LowerStepMin: 1, LowerStepMax: 3,
		ComfortMin: 0, ComfortMax: 0,
	})
	if count != 3 {
		t.Fatalf("evaluated = %d, want 3", count)
	}
	if got.Best.LowerStepCount != 1 {
		t.Fatalf("best lower = %d, want 1", got.Best.LowerStepCount)
	}
}

func TestComfortPointsGrid(t *testing.T) {
	// Сетка от 600 с шагом 1: 600, 601 (601.5 не кратен шагу).
	pts := comfortPoints(600, 601.5, 1)
	if len(pts) != 2 || pts[0] != 600 || pts[1] != 601 {
		t.Fatalf("pts = %v, want [600 601]", pts)
	}
}
