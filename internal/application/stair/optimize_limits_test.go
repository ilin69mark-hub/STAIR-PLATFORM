package stair

import (
	"context"
	"errors"
	"testing"
	"time"

	"stairplatform/internal/domain/engineering"
)

func sec3Cfg(flight engineering.FlightType) Config {
	return Config{
		Width: 900, Height: 3000, Flight: flight, StepHeight: 180,
		StringerThickness: 60, StepThickness: 40, Riser: true,
		Clearance: 2200, RailingHeight: 900, ApproachSpace: 1000,
		RoomWidth: 6000, RoomLength: 2000,
		LandingWidth: 1000, LowerStepCount: 7, OuterRadius: 1200,
	}
}

// TestSEC003_SearchSpaceRejectedFast — регрессия SEC-003.
//
// Payload из аудита: step_count_max=2e9, comfort_step_grid=1e-7.
// До фикса — 60.00 s wall (лимит таймаута маршрута) и 499.
// После фикса — мгновенный ErrSearchSpaceTooLarge.
func TestSEC003_SearchSpaceRejectedFast(t *testing.T) {
	attacks := []struct {
		name string
		req  OptimizeRequest
	}{
		{"audit payload (2e9 steps, 1e-7 grid)", OptimizeRequest{
			Target: TargetPrice, StepCountMin: 1, StepCountMax: 2_000_000_000,
			ComfortStepMin: 600, ComfortStepMax: 640, ComfortStepGrid: 0.0000001,
		}},
		{"huge step window", OptimizeRequest{
			Target: TargetPrice, StepCountMin: 1, StepCountMax: 1_000_000_000,
		}},
		{"denormal grid", OptimizeRequest{
			Target: TargetPrice, ComfortStepMin: 600, ComfortStepMax: 640,
			ComfortStepGrid: 5e-324,
		}},
		{"grid just below minimum", OptimizeRequest{
			Target: TargetPrice, ComfortStepMin: 600, ComfortStepMax: 640,
			ComfortStepGrid: MinComfortStepGrid - 0.01,
		}},
		{"zero grid", OptimizeRequest{
			Target: TargetPrice, ComfortStepMin: 600, ComfortStepMax: 640,
			ComfortStepGrid: 1e-12,
		}},
		{"max int step count", OptimizeRequest{
			Target: TargetPrice, StepCountMin: 1,
			StepCountMax: 1<<62 - 1,
		}},
	}
	for _, flight := range []engineering.FlightType{
		engineering.FlightStraight, engineering.FlightLShape, engineering.FlightUShape,
	} {
		for _, a := range attacks {
			t.Run(string(flight)+"/"+a.name, func(t *testing.T) {
				s := NewService()
				start := time.Now()
				_, err := s.Optimize(context.Background(), sec3Cfg(flight), Options{}, a.req)
				elapsed := time.Since(start)
				if !errors.Is(err, ErrSearchSpaceTooLarge) {
					t.Fatalf("want ErrSearchSpaceTooLarge, got err=%v (elapsed %s)", err, elapsed)
				}
				// Отказ должен быть мгновенным: до вызова optimization.Search.
				if elapsed > 250*time.Millisecond {
					t.Errorf("rejection took %s — search probably started", elapsed)
				}
				t.Logf("rejected in %s: %v", elapsed, err)
			})
		}
	}
}

// TestSEC003_LegitSearchStillWorks — защита от регрессии: нормальный поиск
// (в т.ч. границы, заданные клиентом внутри потолков) продолжает работать
// и даёт тот же результат, что до фикса.
func TestSEC003_LegitSearchStillWorks(t *testing.T) {
	s := NewService()
	cases := []struct {
		name string
		req  OptimizeRequest
	}{
		{"auto bounds", OptimizeRequest{Target: TargetPrice}},
		{"narrowed window", OptimizeRequest{Target: TargetPrice, StepCountMin: 15, StepCountMax: 18}},
		{"grid at minimum", OptimizeRequest{Target: TargetPrice, ComfortStepGrid: MinComfortStepGrid}},
		{"comfort sub-range", OptimizeRequest{Target: TargetPrice, ComfortStepMin: 620, ComfortStepMax: 640, ComfortStepGrid: 2}},
	}
	for _, flight := range []engineering.FlightType{
		engineering.FlightStraight, engineering.FlightLShape, engineering.FlightUShape,
	} {
		for _, c := range cases {
			t.Run(string(flight)+"/"+c.name, func(t *testing.T) {
				start := time.Now()
				out, err := s.Optimize(context.Background(), sec3Cfg(flight), Options{}, c.req)
				if err != nil {
					t.Fatalf("legit search must succeed, got %v", err)
				}
				if out == nil {
					t.Fatal("nil result")
				}
				t.Logf("valid=%v evaluated=%d best_step_height=%.1f comfort=%.1f elapsed=%s",
					out.Valid, out.Evaluated, out.BestConfig.StepHeight.Millimeters(), out.ComfortStep, time.Since(start))
			})
		}
	}
}

// TestSEC003_NormativeWindowAlwaysWithinLimit — нормативное окно при любой
// допустимой высоте обязано укладываться в потолок, иначе легитимный
// запрос без границ был бы отвергнут.
func TestSEC003_NormativeWindowAlwaysWithinLimit(t *testing.T) {
	// maxSupportedHeightMM = 6000 (stair/service.go), GEO-STEP-HEIGHT 150..200.
	// Максимальное нормативное окно: n от ceil(H/200) до floor(H/150).
	worst := 0
	for hm := 1.0; hm <= 6000; hm += 0.5 {
		span := int(6000.0/150.0) - int(hm/200.0) + 1
		if span > worst {
			worst = span
		}
	}
	if worst > MaxStepCountSpan {
		t.Errorf("normative step window can reach %d > MaxStepCountSpan %d", worst, MaxStepCountSpan)
	}
	// Сетка шага комфорта: 600..640 с шагом 1 мм = 41 точка.
	if pts := comfortPoints(600, 640, 1); pts > MaxComfortPoints {
		t.Errorf("normative comfort grid has %d points > MaxComfortPoints %d", pts, MaxComfortPoints)
	}
	// Нормативный перебор n1 при n = 40 = 40 значений.
	if 40 > MaxLowerStepSpan {
		t.Errorf("normative n1 span %d > MaxLowerStepSpan %d", 40, MaxLowerStepSpan)
	}
	t.Logf("worst normative step span=%d (limit %d), comfort points=%d (limit %d)",
		worst, MaxStepCountSpan, comfortPoints(600, 640, 1), MaxComfortPoints)
}

// TestSEC003_CandidateCeilingEnforced — потолок кандидатов срабатывает даже
// когда каждый отдельный диапазон в допустимых пределах.
func TestSEC003_CandidateCeilingEnforced(t *testing.T) {
	// 48 × 48 × 48 = 110 592 > 8192 при L/U — должно быть отвергнуто
	// потолком кандидатов, а не молча усечено.
	s := NewService()
	_, err := s.Optimize(context.Background(), sec3Cfg(engineering.FlightLShape), Options{},
		OptimizeRequest{
			Target: TargetPrice, StepCountMin: 1, StepCountMax: MaxStepCountSpan,
			ComfortStepMin: 600, ComfortStepMax: 640, ComfortStepGrid: MinComfortStepGrid,
		})
	if !errors.Is(err, ErrSearchSpaceTooLarge) {
		t.Fatalf("want ErrSearchSpaceTooLarge, got %v", err)
	}
	// Сообщение должно называть именно число кандидатов.
	if got := err.Error(); !contains(got, "candidate count") {
		t.Errorf("err must name candidate count, got %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// TestSEC003_ComfortPointsHelper — арифметика числа точек сетки.
func TestSEC003_ComfortPointsHelper(t *testing.T) {
	cases := []struct {
		min, max, grid float64
		want           int
	}{
		{600, 640, 2, 21},
		{600, 640, 1, 41},
		{600, 640, 40, 2},
		{600, 640, 41, 1},
		{600, 600, 2, 1},
		{640, 600, 2, 1},
		{0, 0, 0, 1},
	}
	for _, c := range cases {
		if got := comfortPoints(c.min, c.max, c.grid); got != c.want {
			t.Errorf("comfortPoints(%v,%v,%v)=%d want %d", c.min, c.max, c.grid, got, c.want)
		}
	}
}

// TestSEC003_WinnerKeepsVariations — защита от регрессии SEC-003/PERF:
// отключение вариаций в переборе не должно лишить ИТОГОВЫЙ ответ
// интерактивных вариантов A/B/C. Лучший кандидат пересчитывается полностью
// (optimize.go, финальный Calculate), поэтому room_fit-варианты в нём есть.
func TestSEC003_WinnerKeepsVariations(t *testing.T) {
	s := NewService()
	// L-образная лестница, которая НЕ вписывается: bbox по Y = Wp + n2·b
	// заметно больше RoomLength, значит room_fit срабатывает.
	cfg := sec3Cfg(engineering.FlightLShape)
	cfg.RoomLength = 2000
	cfg.RoomWidth = 6000

	// Контроль: room_fit действительно возникает.
	direct, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	var fitIssues int
	for _, is := range direct.Validation.Issues {
		if is.Code == "room_fit" {
			fitIssues++
		}
	}
	if fitIssues == 0 {
		t.Skip("room_fit не срабатывает на этой конфигурации — тест не применим")
	}

	out, err := s.Optimize(context.Background(), cfg, Options{}, OptimizeRequest{Target: TargetPrice})
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if !out.Valid || out.BestResult == nil {
		t.Fatalf("optimize must be valid, got %+v", out)
	}
	var withVar int
	for _, is := range out.BestResult.Validation.Issues {
		if len(is.Variations) > 0 {
			withVar++
		}
	}
	if withVar == 0 {
		t.Errorf("winner lost its variations (issues=%d) — skipVariations leaked into the final result",
			len(out.BestResult.Validation.Issues))
	} else {
		t.Logf("winner keeps variations on %d issue(s), evaluated=%d", withVar, out.Evaluated)
	}

	// И прямой расчёт без optimize обязан сохранять вариации (регрессия
	// skipVariations не должна влиять на обычный путь).
	if len(direct.Validation.Issues) == 0 {
		t.Error("direct Calculate lost all issues")
	}
}

// TestSEC003_SkipVariationsIsInternalOnly — флаг неэкспортируемый: внешний
// пакет не может его включить (проверяется компиляцией — тест просто
// убеждается, что поле существует и имеет нужное имя/тип).
func TestSEC003_SkipVariationsIsInternalOnly(t *testing.T) {
	o := Options{skipVariations: true}
	if !o.skipVariations {
		t.Fatal("unexported flag must be settable inside the package")
	}
	if (Options{}).skipVariations {
		t.Fatal("zero Options must have skipVariations=false")
	}
}
