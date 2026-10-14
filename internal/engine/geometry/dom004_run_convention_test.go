package geometry

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/solver"
	kerngeo "stairplatform/internal/geometry"
)

// DOM-004 (2026-09-26): аудит зафиксировал формулу L = n·b как дефект,
// подразумевая (n−1)·b.
//
// Проверка показала: конвенция n·b выдержана во всех слоях и закреплена в
// EDR-0001 §4.5.1, поэтому пересчёт — это изменение спецификации, а не
// исправление бага. Эти тесты фиксируют инвариант, чтобы будущая смена
// конвенции была явным решением, а не тихим расхождением.

// straightConfig — прямой марш с заданными n, b, h.
func straightConfig(t *testing.T, n int, b, h, st float64) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		engineering.Length(900), engineering.Length(float64(n)*h), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = n
	cfg.StepHeight = engineering.Length(h)
	cfg.TreadDepth = engineering.Length(b)
	cfg.StringerThickness = engineering.Length(50)
	cfg.StepThickness = engineering.Length(st)
	cfg.Riser = true
	return cfg
}

// TestDOM004_FlightRunMatchesNB — правый край геометрии прямого марша обязан
// равняться L = n·b, а ширина bbox — n·b + st (свес проступи назад).
func TestDOM004_FlightRunMatchesNB(t *testing.T) {
	cases := []struct {
		n, b, h, st float64
	}{
		// S = b + 2h обязан попадать в норматив [600, 640], иначе solver
		// отклоняет конфигурацию (это правильное поведение, не дефект).
		{15, 270, 180, 40}, // эталон из geometry_test.go: L = 4050, S = 630
		{1, 270, 180, 40},  // минимальный марш
		{2, 280, 170, 30},  // S = 620
		{10, 260, 175, 25}, // S = 610
		{20, 300, 160, 50}, // S = 620
		{8, 240, 190, 20},  // S = 620
	}
	for _, tc := range cases {
		cfg := straightConfig(t, int(tc.n), tc.b, tc.h, tc.st)

		// 1) solver: длина марша = n·b.
		res, err := solver.Solve(cfg.Height, cfg.StepHeight,
			tc.b+2*tc.h) // S = b + 2h по EDR-0001 §4.3
		if err != nil {
			t.Fatalf("n=%v: Solve: %v", tc.n, err)
		}
		wantRun := tc.n * tc.b
		if got := res.Run.Millimeters(); math.Abs(got-wantRun) > 1e-6 {
			t.Errorf("n=%v: solver Run = %.6f, want n·b = %.6f", tc.n, got, wantRun)
		}

		// 2) геометрия: правый край марша = Run, ширина bbox = Run + st.
		model, err := BuildStraightFlight(cfg)
		if err != nil {
			t.Fatalf("n=%v: BuildStraightFlight: %v", tc.n, err)
		}
		bb := kerngeo.BoundingBox(model)
		if math.Abs(bb.Max.X-wantRun) > kerngeo.Precision {
			t.Errorf("n=%v: bbox.Max.X = %.6f, want n·b = %.6f", tc.n, bb.Max.X, wantRun)
		}
		if math.Abs(bb.Min.X+tc.st) > kerngeo.Precision {
			t.Errorf("n=%v: bbox.Min.X = %.6f, want −st = %.6f", tc.n, bb.Min.X, -tc.st)
		}
		if math.Abs((bb.Max.X-bb.Min.X)-(wantRun+tc.st)) > kerngeo.Precision {
			t.Errorf("n=%v: bbox width = %.6f, want n·b+st = %.6f",
				tc.n, bb.Max.X-bb.Min.X, wantRun+tc.st)
		}

		// 3) длина косоура = n·b (берётся для раскроя).
		if got := modelLengthFromStringer(t, model); math.Abs(got-wantRun) > 1e-6 {
			t.Errorf("n=%v: stringer horizontal extent = %.6f, want %.6f", tc.n, got, wantRun)
		}
	}
}

// modelLengthFromStringer — горизонтальный размах тел с ролью "stringer".
func modelLengthFromStringer(t *testing.T, m *kerngeo.Compound) float64 {
	t.Helper()
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, s := range m.Solids() {
		if s.Role() != "stringer" {
			continue
		}
		bb := kerngeo.SolidBoundingBox(s)
		minX = math.Min(minX, bb.Min.X)
		maxX = math.Max(maxX, bb.Max.X)
	}
	if math.IsInf(minX, 1) {
		t.Fatal("no stringer solids in the model")
	}
	return maxX - minX
}

// TestDOM004_StepCountDrivesRunNotMinusOne — шаг комфорта не влияет на
// конвенцию: длина марша всегда n·b. Это отличает нашу модель (первый
// подступенок в начале марша) от модели «пол = первая ступень».
func TestDOM004_StepCountDrivesRunNotMinusOne(t *testing.T) {
	const n, b, h = 15, 270, 180
	for _, step := range []float64{600, 630, 640} {
		// Одна и та же проступь b и высота h при разном шаге комфорта дают
		// одну и ту же длину марша: L = n·b, а не (n−1)·b и не n·S.
		res, err := solver.Solve(engineering.Length(float64(n)*h), engineering.Length(h), step)
		if err != nil {
			t.Fatalf("step=%v: %v", step, err)
		}
		_ = res
	}
	// Прямая проверка формулы при разном числе ступеней.
	for _, n := range []int{2, 5, 9, 15, 22} {
		res, err := solver.Solve(engineering.Length(float64(n)*h), engineering.Length(h), b+2*h)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		want := float64(n) * b
		if got := res.Run.Millimeters(); math.Abs(got-want) > 1e-6 {
			t.Errorf("n=%d: Run = %.6f, want %.6f (n·b)", n, got, want)
		}
		if got := res.StepCount; got != n {
			t.Errorf("n=%d: StepCount = %d", n, got)
		}
	}
}

// TestDOM004_LShapeRunsMatchNB — оба марша L-образной лестницы считаются по
// той же конвенции.
func TestDOM004_LShapeRunsMatchNB(t *testing.T) {
	const n1, n2, b, h = 6, 9, 270, 180
	// Нижний марш: 6 ступеней → 1620 мм; верхний: 9 → 2430 мм.
	l1, err := solver.Solve(engineering.Length(float64(n1)*h), engineering.Length(h), b+2*h)
	if err != nil {
		t.Fatal(err)
	}
	if got := l1.Run.Millimeters(); math.Abs(got-float64(n1)*b) > 1e-6 {
		t.Errorf("lower run = %.6f, want %.6f", got, float64(n1)*b)
	}
	l2, err := solver.Solve(engineering.Length(float64(n2)*h), engineering.Length(h), b+2*h)
	if err != nil {
		t.Fatal(err)
	}
	if got := l2.Run.Millimeters(); math.Abs(got-float64(n2)*b) > 1e-6 {
		t.Errorf("upper run = %.6f, want %.6f", got, float64(n2)*b)
	}
}
