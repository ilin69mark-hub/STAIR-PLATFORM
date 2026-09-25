package stair

import (
	"strings"
	"testing"
)

func TestMaterialThicknessFromError(t *testing.T) {
	cases := []struct {
		msg  string
		want float64
	}{
		{"manufacturing: no material supports thickness 70 mm", 70},
		{"thickness 40.5 mm of косоура", 40.5},
		{"no thickness here", 0},
		{"thickness 70", 70},
		{"thickness bad mm", 0},
	}
	for _, c := range cases {
		if got := materialThicknessFromError(c.msg); got != c.want {
			t.Fatalf("msg %q got %v want %v", c.msg, got, c.want)
		}
	}
}

func TestMaterialCodeFromError(t *testing.T) {
	if got := materialCodeFromError(`material "WOOD-OAK" does not support thickness`); got != "WOOD-OAK" {
		t.Fatalf("got %q", got)
	}
	if got := materialCodeFromError(`no material`); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
	if got := materialCodeFromError(`material "WOOD-OAK does not`); got != "WOOD-OAK does not" {
		t.Fatalf("got %q", got)
	}
}

func TestMaterialPartFromError(t *testing.T) {
	if got := materialPartFromError(`thickness 70 mm of косоура`); got != "косоура" {
		t.Fatalf("got %q", got)
	}
	if got := materialPartFromError(`no mm of`); got != "деталь" {
		t.Fatalf("want деталь, got %q", got)
	}
}

func TestLowerStepMaxExtra(t *testing.T) {
	if got := lowerStepMaxFromError(`must be in [1, 7] for L`); got != 7 {
		t.Fatalf("got %d", got)
	}
	if got := lowerStepMaxFromError(`no range`); got != 0 {
		t.Fatalf("want 0, got %d", got)
	}
	if got := lowerStepMaxFromError(`[1, bad]`); got != 0 {
		t.Fatalf("want 0 for bad, got %d", got)
	}
}

func TestInputIssueThickness(t *testing.T) {
	msg := "manufacturing: no material supports thickness 70 mm"
	res, ok := inputIssue(errString(msg))
	if !ok || len(res.Issues) == 0 {
		t.Fatalf("want input issue, ok=%v res=%+v", ok, res)
	}
}

func TestConfigInputErrorBranches(t *testing.T) {
	branches := []string{
		"supported maximum 6000",
		"supported maximum 5000",
		"width must be positive",
		"height must be positive",
		"flight type is required",
		"stringer thickness must not be negative",
		"step thickness must not be negative",
		"clearance must not be negative",
		"railing height must not be negative",
		"landing width must not be negative",
		"lower step count must not be negative",
		"outer radius must exceed the stair width",
		"landing width must be at least the flight width",
		"lower step count must be in [1, 5]",
		"width must exceed two stringer thicknesses",
		`material "STEEL" not found in catalog`,
		`material "WOOD-OAK" does not support thickness 70 mm of косоура`,
		`width 5000 exceeds maximum 3000 for material "WOOD-OAK"`,
		`rise height 6000 exceeds maximum 4550 for material "WOOD-OAK"`,
		"invalid railing sides",
		"invalid turn direction",
		"invalid spiral direction",
	}
	for _, m := range branches {
		if got := configInputError(errString(m)); got == nil {
			t.Fatalf("want InputError for %q", m)
		}
	}
	// API-001 (forensic 2026-09-24): раньше неизвестная ошибка доменной
	// валидации возвращалась nil → «прочая» ошибка → 500. Теперь она
	// становится обычной входной ошибкой (блокирующий результат валидации).
	got := configInputError(errString("approach space must be within 1000-1200 mm for straight"))
	if got == nil {
		t.Fatal("want generic InputError for unmapped validation error")
	}
	if !strings.Contains(got.Message, "1000") {
		t.Fatalf("generic InputError must keep the original text, got %q", got.Message)
	}
	if configInputError(errString("some internal failure")) == nil {
		t.Fatal("generic fallback must cover unknown errors too")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
