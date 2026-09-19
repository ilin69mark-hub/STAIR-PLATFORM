package stair

import (
	"fmt"
	"strings"
	"testing"

	"stairplatform/internal/engine/constraint"
)

// TestConfigInputErrorLowerStep — GEO-LOWER-STEP из доменной валидации
// ("lower step count must be in [1, N]") несёт конкретный допустимый
// диапазон в Guide, а не общий текст без значений.
func TestConfigInputErrorLowerStep(t *testing.T) {
	inp := configInputError(fmt.Errorf("stair: lower step count must be in [1, 14] for L"))
	if inp == nil {
		t.Fatal("expected InputError for lower step count message")
	}
	if inp.Code != constraint.GEO_LOWER_STEP {
		t.Fatalf("code = %q, want %q", inp.Code, constraint.GEO_LOWER_STEP)
	}
	if !strings.Contains(inp.Guide, "от 1 до 14") {
		t.Fatalf("guide must carry the concrete range, got %q", inp.Guide)
	}
	if !strings.Contains(inp.Guide, "всего ступеней 15") {
		t.Fatalf("guide must carry the total step count, got %q", inp.Guide)
	}

	// Сообщение без распознаваемого диапазона — безопасный общий текст.
	inp = configInputError(fmt.Errorf("stair: lower step count must be in range"))
	if inp == nil {
		t.Fatal("expected InputError for lower step count message")
	}
	if inp.Code != constraint.GEO_LOWER_STEP {
		t.Fatalf("code = %q, want %q", inp.Code, constraint.GEO_LOWER_STEP)
	}
	if !strings.Contains(inp.Guide, "допустимом диапазоне") {
		t.Fatalf("guide must fall back to generic text, got %q", inp.Guide)
	}
}

// TestLowerStepMaxFromError — извлечение верхней границы из сообщения.
func TestLowerStepMaxFromError(t *testing.T) {
	cases := []struct {
		msg  string
		want int
	}{
		{"stair: lower step count must be in [1, 14] for L", 14},
		{"stair: lower step count must be in [1, 7] for U", 7},
		{"geometry: stair: lower step count must be in [1, 9] for L", 9},
		{"stair: lower step count must be in range", 0},
		{"stair: lower step count must be negative", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := lowerStepMaxFromError(c.msg); got != c.want {
			t.Errorf("lowerStepMaxFromError(%q) = %d, want %d", c.msg, got, c.want)
		}
	}
}

// TestInputIssueLowerStep — ошибка движка с подсказкой проходит в
// блокирующий результат с кодом GEO-LOWER-STEP.
func TestInputIssueLowerStep(t *testing.T) {
	res, ok := inputIssue(fmt.Errorf("geometry: stair: lower step count must be in [1, 14] for L"))
	if !ok {
		t.Fatal("expected input issue to be recognized")
	}
	if !res.Blocking || res.Valid {
		t.Fatalf("expected blocking invalid result, got %+v", res)
	}
	if len(res.Issues) == 0 {
		t.Fatal("expected at least one issue")
	}
	it := res.Issues[0]
	if it.Code != constraint.GEO_LOWER_STEP {
		t.Fatalf("code = %q, want %q", it.Code, constraint.GEO_LOWER_STEP)
	}
	if it.Param != "Нижних ступеней" {
		t.Fatalf("param = %q, want %q", it.Param, "Нижних ступеней")
	}
	if !strings.Contains(it.Guide, "от 1 до 14") {
		t.Fatalf("guide must carry the concrete range, got %q", it.Guide)
	}
}
