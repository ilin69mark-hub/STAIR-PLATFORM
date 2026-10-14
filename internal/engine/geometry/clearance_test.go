package geometry_test

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/geometry"
	"stairplatform/internal/engine/solver"
)

// clearanceConfig — прямая лестница высотой 2700 мм с заданной отметкой
// перекрытия. Верхняя ступень по DOM-004 стоит на Z = 2700.
func clearanceConfig(t *testing.T, ceilingMM float64) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(engineering.Length(900), engineering.Length(2700), engineering.FlightStraight)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	res, err := solver.Solve(cfg.Height, engineering.Length(180))
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	res.Apply(cfg)
	cfg.StringerThickness = engineering.Length(40)
	cfg.StepThickness = engineering.Length(60)
	cfg.ApproachSpace = engineering.Length(1000)
	cfg.CeilingHeight = engineering.Length(ceilingMM)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return cfg
}

func clearanceIssue(res *geometry.GenerationResult) string {
	for _, is := range res.Issues {
		if is.Code == "clearance_fit" {
			return is.Message
		}
	}
	return ""
}

// TestClearanceGeometric — просвет считается по ГОТОВОЙ МОДЕЛИ (S-168),
// а не берётся из числа, введённого пользователем. Лестница 2700 мм:
// верх ступени Z = 2700, предпоследняя ступень Z = 2520.
//
// Ступень на уровне самого перекрытия — это точка выхода на этаж, проходить
// под ней не нужно, поэтому она не считается худшей.
func TestClearanceGeometric(t *testing.T) {
	cases := []struct {
		name    string
		ceiling float64
		want    bool
	}{
		{"не задано — проверки нет", 0, false},
		{"высокое перекрытие — запас есть", 5000, false},
		{"ровно 2000 — граница допущена", 4700, false},
		{"чуть ниже нормы", 4200, true},
		{"перекрытие ниже верха лестницы", 2600, true},
		{"перекрытие на уровне выхода — худшая предпоследняя", 2700, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := geometry.Generate(context.Background(), clearanceConfig(t, tc.ceiling))
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			got := clearanceIssue(res) != ""
			if got != tc.want {
				t.Fatalf("ceiling=%.0f: clearance_fit = %v, want %v (issue: %q)",
					tc.ceiling, got, tc.want, clearanceIssue(res))
			}
		})
	}
}

// TestClearanceUsesHighestStepUnderCeiling — просвет берётся с САМОЙ ВЫСОКОЙ
// ступени, находящейся под перекрытием: чем выше ступень, тем меньше
// просвет. При перекрытии 2900 верх ступени 2700 даёт 200 мм.
func TestClearanceUsesHighestStepUnderCeiling(t *testing.T) {
	res, err := geometry.Generate(context.Background(), clearanceConfig(t, 2900))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	msg := clearanceIssue(res)
	if msg == "" {
		t.Fatal("expected clearance_fit warning at ceiling=2900")
	}
	// 2900 − 2700 = 200 мм просвета.
	if want := "200 мм"; !strings.Contains(msg, want) {
		t.Fatalf("expected %q in message, got %q", want, msg)
	}
}

// TestClearanceNotCheckedWhenCeilingAbsent — без CeilingHeight просвет не
// проверяется: перекрытие не задано, считать нечего. Поведение как у
// room_fit, который тоже не проверяется при нулевых габаритах помещения.
func TestClearanceNotCheckedWhenCeilingAbsent(t *testing.T) {
	res, err := geometry.Generate(context.Background(), clearanceConfig(t, 0))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if msg := clearanceIssue(res); msg != "" {
		t.Fatalf("unexpected clearance_fit without ceiling: %q", msg)
	}
}
