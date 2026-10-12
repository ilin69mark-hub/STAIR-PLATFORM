package geometry

import (
	"math"
	"testing"
)

// TestArcSegmentsRespectsTolerance — главное свойство: сколько бы ArcSegments
// ни вернула, реальное отклонение обязано укладываться в допуск.
func TestArcSegmentsRespectsTolerance(t *testing.T) {
	cases := []struct {
		radius, sweep, tol float64
	}{
		{10, math.Pi / 2, 0.05},   // нос ступени
		{10, math.Pi / 2, 0.03},   // тот же нос, жёстче
		{10, math.Pi / 2, 0.3},    // грубо
		{21.2, 2 * math.Pi, 0.2},  // сечение трубки Ø42,4 целиком
		{300, math.Pi, 5.0},       // сектор поворотной ступени
		{2.0, math.Pi / 4, 0.01},  // мелкая деталь, жёсткий допуск
		{500, 2 * math.Pi, 0.5},   // большая окружность
		{1.0, math.Pi / 2, 0.001}, // мелкий радиус
	}
	for _, c := range cases {
		n := ArcSegments(c.radius, c.sweep, c.tol)
		dev := ArcChordDeviationMM(c.radius, c.sweep, n)
		if dev > c.tol*(1+1e-9) {
			t.Errorf("R=%.1f θ=%.4f tol=%.4f → n=%d, отклонение %.6f > допуск",
				c.radius, c.sweep, c.tol, n, dev)
		}
	}
}

// TestArcSegmentsMonotonic — при большем радиусе или угле нужно не меньше
// сегментов: точность не должна ухудшаться сама по себе.
func TestArcSegmentsMonotonic(t *testing.T) {
	const tol = 0.05
	prev := 0
	for _, r := range []float64{5, 10, 20, 50, 100, 250, 500} {
		n := ArcSegments(r, math.Pi/2, tol)
		if n < prev {
			t.Errorf("R=%.0f: сегментов %d меньше предыдущих %d", r, n, prev)
		}
		prev = n
	}
	prev = 0
	for _, sweep := range []float64{0.1, 0.5, 1.0, 2.0, 3.0, math.Pi} {
		n := ArcSegments(50, sweep, tol)
		if n < prev {
			t.Errorf("θ=%.2f: сегментов %d меньше предыдущих %d", sweep, n, prev)
		}
		prev = n
	}
}

// TestArcSegmentsTighterToleranceNeedsMore — более жёсткий допуск требует
// больше сегментов. Иначе допуск ничего не значит.
func TestArcSegmentsTighterToleranceNeedsMore(t *testing.T) {
	prev := 0
	for _, tol := range []float64{1.0, 0.5, 0.1, 0.05, 0.01, 0.001} {
		n := ArcSegments(20, math.Pi/2, tol)
		if n < prev {
			t.Errorf("tol=%.3f: сегментов %d меньше, чем при более грубом %d", tol, n, prev)
		}
		prev = n
	}
}

// TestArcSegmentsMinBound — короткая дуга не превращается в угол из двух
// отрезков: результат не меньше MinArcSegments.
func TestArcSegmentsMinBound(t *testing.T) {
	if got := ArcSegments(100, 0.01, 0.001); got < MinArcSegments {
		t.Errorf("n = %d, ожидалось не меньше %d", got, MinArcSegments)
	}
	if got := ArcSegments(100, 0.01, 0.001); got != MinArcSegments {
		t.Errorf("n = %d, ожидалось ровно %d", got, MinArcSegments)
	}
	if got := ArcSegmentsMin(100, 0.01, 0.001, 8); got != 8 {
		t.Errorf("с нижней границей 8: получено %d", got)
	}
}

// TestArcSegmentsDegenerateInput — вырожденные входы не паникуют и не делят
// на ноль.
func TestArcSegmentsDegenerateInput(t *testing.T) {
	cases := []struct{ r, sweep, tol float64 }{
		{0, math.Pi / 2, 0.05},
		{-10, math.Pi / 2, 0.05},
		{10, 0, 0.05},
		{10, -1, 0.05},
		{10, math.Pi / 2, 0},     // нулевой допуск → подставляется дефолт
		{10, math.Pi / 2, -1},    // отрицательный → дефолт
		{1, math.Pi / 2, 5},      // допуск больше радиуса
		{1, math.Pi / 2, 1.0001}, // чуть больше радиуса
	}
	for _, c := range cases {
		if n := ArcSegments(c.r, c.sweep, c.tol); n < MinArcSegments {
			t.Errorf("R=%v θ=%v tol=%v → n=%d, ожидалось не меньше %d",
				c.r, c.sweep, c.tol, n, MinArcSegments)
		}
	}
}

// TestArcSegmentsDeterministic — одинаковый вход даёт одинаковый выход
// (ADR-0007). Проверка особенно важна здесь: результат идёт в геометрию.
func TestArcSegmentsDeterministic(t *testing.T) {
	for i := 0; i < 50; i++ {
		a := ArcSegments(37.5, 1.23, 0.07)
		b := ArcSegments(37.5, 1.23, 0.07)
		if a != b {
			t.Fatalf("прогон %d: %d != %d", i, a, b)
		}
	}
}

// TestArcSegmentsReproducesKnownValues — опорные числа, посчитанные
// вручную, чтобы формулу нельзя было сломать «вроде работает».
func TestArcSegmentsReproducesKnownValues(t *testing.T) {
	// R=10, θ=90°, tol=0.05:
	//   ratio = 0.005, arccos(0.995) = 0.100042, знаменатель 0.200083,
	//   n = ⌈1.570796 / 0.200083⌉ = ⌈7.85⌉ = 8.
	// Проверка вручную: при 8 сегментах отклонение
	//   10·(1−cos(π/16)) = 0.04815 ≤ 0.05,
	// а при 7 уже 0.06289 > 0.05 — значит 8 минимально.
	if got := ArcSegments(10, math.Pi/2, 0.05); got != 8 {
		t.Errorf("R=10 θ=90° tol=0.05 → %d, ожидалось 8", got)
	}
	// R=10, θ=90°, tol=0.3: заметно грубее, сегментов меньше
	if coarse := ArcSegments(10, math.Pi/2, 0.3); coarse >= 8 {
		t.Errorf("R=10 θ=90° tol=0.3 → %d, ожидалось меньше 8", coarse)
	}
	// Полная окружность R=20, tol=0.1
	n := ArcSegments(20, 2*math.Pi, 0.1)
	dev := ArcChordDeviationMM(20, 2*math.Pi, n)
	if dev > 0.1 {
		t.Errorf("полная окружность: отклонение %.6f > 0.1 (n=%d)", dev, n)
	}
}
