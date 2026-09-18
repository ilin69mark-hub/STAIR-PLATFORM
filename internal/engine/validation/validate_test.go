package validation

import (
	"reflect"
	"strconv"
	"strings"
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

func mustAngleDeg(t *testing.T, deg float64) engineering.Angle {
	t.Helper()
	a, err := engineering.NewAngleDegrees(deg)
	if err != nil {
		t.Fatalf("NewAngleDegrees(%v): %v", deg, err)
	}
	return a
}

// validConfig — конфигурация, проходящая все правила STANDARD (EDR-0002).
func validConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(mustLength(t, 900), mustLength(t, 2700), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 280)
	cfg.Angle = mustAngleDeg(t, 35)
	cfg.Clearance = mustLength(t, 2100)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.RailingHeight = mustLength(t, 1000)
	return cfg
}

func TestValidateCompliantConfig(t *testing.T) {
	res := Validate(validConfig(t), constraint.StandardProfile("standard"))
	if len(res.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", res.Issues)
	}
	if !res.Valid || res.Blocking {
		t.Fatalf("expected Valid=true Blocking=false, got %+v", res)
	}
}

func TestValidateStepHeightError(t *testing.T) {
	cfg := validConfig(t)
	cfg.StepHeight = mustLength(t, 250) // вне 150-200
	res := Validate(cfg, constraint.StandardProfile("standard"))
	if res.Valid || !res.Blocking {
		t.Fatal("step height error must be blocking")
	}
	found := false
	for _, i := range res.Issues {
		if i.Code == constraint.GEO_STEP_HEIGHT && i.Severity == constraint.SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatal("GEO-STEP-HEIGHT error issue expected")
	}
}

func TestValidateTreadDepthError(t *testing.T) {
	cfg := validConfig(t)
	cfg.TreadDepth = mustLength(t, 200) // вне 260-320
	res := Validate(cfg, constraint.StandardProfile("standard"))
	if res.Valid || !res.Blocking {
		t.Fatal("tread depth error must be blocking")
	}
}

func TestValidateAngleError(t *testing.T) {
	cfg := validConfig(t)
	cfg.Angle = mustAngleDeg(t, 50) // вне 30-45
	res := Validate(cfg, constraint.StandardProfile("standard"))
	if res.Valid || !res.Blocking {
		t.Fatal("angle error must be blocking")
	}
}

func TestValidateClearanceWarning(t *testing.T) {
	cfg := validConfig(t)
	cfg.Clearance = mustLength(t, 1500) // < 2000, только Warning
	res := Validate(cfg, constraint.StandardProfile("standard"))
	if !res.Valid || res.Blocking {
		t.Fatal("clearance warning must not block (Valid=true, Blocking=false)")
	}
}

func TestValidateRailingWarning(t *testing.T) {
	cfg := validConfig(t)
	cfg.RailingHeight = mustLength(t, 800) // < 900, только Warning
	res := Validate(cfg, constraint.StandardProfile("standard"))
	if !res.Valid || res.Blocking {
		t.Fatal("railing height warning must not block")
	}
}

func TestValidateStringerWarning(t *testing.T) {
	cfg := validConfig(t)
	cfg.StringerThickness = mustLength(t, 10) // < 30, только Warning
	res := Validate(cfg, constraint.StandardProfile("standard"))
	if !res.Valid || res.Blocking {
		t.Fatal("stringer thickness warning must not block")
	}
}

func TestValidateBoundaryWithinTolerance(t *testing.T) {
	cfg := validConfig(t)
	cfg.StepHeight = mustLength(t, 200.05) // 200 + допуск 0.1
	res := Validate(cfg, constraint.StandardProfile("standard"))
	if len(res.Issues) != 0 {
		t.Fatalf("value within tolerance must pass, got %+v", res.Issues)
	}
}

func TestValidateDeterminism(t *testing.T) {
	cfg := validConfig(t)
	cfg.StepHeight = mustLength(t, 250)
	cfg.Clearance = mustLength(t, 1500)
	set := constraint.StandardProfile("standard")
	first := Validate(cfg, set)
	second := Validate(cfg, set)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("validation must be deterministic:\n%+v\n%+v", first, second)
	}
	// сортировка по Code и последовательная нумерация ID.
	for i := 1; i < len(first.Issues); i++ {
		if first.Issues[i].Code < first.Issues[i-1].Code {
			t.Fatalf("issues not sorted by code: %v before %v",
				first.Issues[i].Code, first.Issues[i-1].Code)
		}
	}
	for i, issue := range first.Issues {
		want := "ISSUE-" + strconv.Itoa(i+1)
		if issue.ID != want {
			t.Fatalf("issue %d id = %s, want %s", i, issue.ID, want)
		}
	}
}

func TestValidateInactiveVersionSkipped(t *testing.T) {
	cfg := validConfig(t)
	set := constraint.NewSet("cs", "test")
	// версия 2 с жёстким диапазоном активна, версия 1 с диапазоном STANDARD — нет.
	if err := set.Add(&constraint.Constraint{
		Code: constraint.GEO_STEP_HEIGHT, Category: "geometry", Version: 1, Active: false,
		Severity: constraint.SeverityError,
		Range:    constraint.Range{Min: 150, Max: 200, HasMin: true, HasMax: true, Tolerance: 0.1},
		Message:  "step height 150-200", Fix: "fix",
	}); err != nil {
		t.Fatal(err)
	}
	if err := set.Add(&constraint.Constraint{
		Code: constraint.GEO_STEP_HEIGHT, Category: "geometry", Version: 2, Active: true,
		Severity: constraint.SeverityError,
		Range:    constraint.Range{Min: 150, Max: 160, HasMin: true, HasMax: true, Tolerance: 0.1},
		Message:  "step height 150-160", Fix: "fix",
	}); err != nil {
		t.Fatal(err)
	}
	cfg.StepHeight = mustLength(t, 180) // вне активного диапазона 150-160
	res := Validate(cfg, set)
	if len(res.Issues) != 1 || res.Issues[0].Code != constraint.GEO_STEP_HEIGHT {
		t.Fatalf("expected single step-height issue, got %+v", res.Issues)
	}
}

// TestValidateAdviceFilled — Param и Guide (отрендеренный из шаблона правила)
// заполняются прямо в Validate, без прохождения через advisor: у каждого issue
// есть русская подсказка даже в сохранённом снапшоте (S-P6).
func TestValidateAdviceFilled(t *testing.T) {
	cfg := validConfig(t)
	cfg.Clearance = mustLength(t, 1500) // вне нормы ≥2000 → GEO-CLEARANCE
	res := Validate(cfg, constraint.StandardProfile("standard"))
	var cl *Issue
	for i := range res.Issues {
		if res.Issues[i].Code == constraint.GEO_CLEARANCE {
			cl = &res.Issues[i]
		}
	}
	if cl == nil {
		t.Fatal("expected GEO-CLEARANCE issue")
	}
	if cl.Param == "" {
		t.Errorf("Param пуст: %+v", cl)
	}
	if cl.Guide == "" {
		t.Errorf("Guide пуст: %+v", cl)
	}
	if cl.Fix == "" {
		t.Errorf("Fix пуст: %+v", cl)
	}
	if cl.Message == "" {
		t.Errorf("Message пуст: %+v", cl)
	}
	if !strings.Contains(cl.Guide, "1500") {
		t.Errorf("Guide должен содержать значение {value}=1500: %q", cl.Guide)
	}
}
