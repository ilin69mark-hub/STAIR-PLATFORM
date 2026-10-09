package geometry

import (
	"context"
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// combConfig — эталонная конфигурация прямого марша на металлокаркасе из
// чертежа freecad-out (2026-10-05): ширина 900, подъём 175, 16 ступеней,
// шаг 280, косоур 8, ступень 40, подступенок 40.
func combConfig() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             900,
		Height:            2800,
		Flight:            engineering.FlightStraight,
		StepCount:         16,
		StepHeight:        175,
		StepWidth:         280,
		TreadDepth:        280,
		StringerThickness: 8,
		StepThickness:     40,
		RiserThickness:    40,
		Riser:             true,
		Material:          "STEEL-S235",
		TreadMaterial:     "WOOD-OAK",
		Comb:              engineering.CombFrameDefaults(),
	}
}

// roleVolume — сумма объёмов тел одной роли.
func roleVolume(t *testing.T, model *kerngeo.Compound, role string) float64 {
	t.Helper()
	cache := kerngeo.NewTessellationCache()
	var sum float64
	for _, sol := range model.Solids() {
		if sol.Role() != role {
			continue
		}
		v, err := kerngeo.VolumeCached(sol, cache)
		if err != nil {
			t.Fatalf("role %s: volume: %v", role, err)
		}
		sum += v
	}
	return sum
}

func roleCount(model *kerngeo.Compound, role string) int {
	c := 0
	for _, s := range model.Solids() {
		if s.Role() == role {
			c++
		}
	}
	return c
}

// TestCombFrameVolumesMatchDrawing — объёмы деталей узла совпадают с чертежом
// FreeCAD (freecad-out/report-comb.txt, 2026-10-05). Это главная проверка
// переноса: пока расходится, геометрия не перенесена.
func TestCombFrameVolumesMatchDrawing(t *testing.T) {
	cfg := combConfig()
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatalf("BuildStraightFlight: %v", err)
	}
	// числа из отчёта FreeCAD: объём всех деталей роли
	want := map[string]float64{
		"stringer":   18473905.2,
		"tread":      173930973.4,
		"riser":      99360000.0,
		"heel":       52743.0 * 32,
		"web":        31258.0 * 30,
		"anchor":     52462.0 * 2,
		"top_anchor": 29115.0 * 2,
	}
	for role, w := range want {
		got := roleVolume(t, model, role)
		if math.Abs(got-w)/w > 0.005 {
			t.Errorf("роль %-10s объём %.1f, чертёж %.1f (Δ %+.2f %%)",
				role, got, w, (got-w)/w*100)
		}
	}
	if got := roleCount(model, "first_web"); got != 2 {
		t.Errorf("пластин 1-й ступени %d, ожидалось 2", got)
	}
	if got := len(model.Solids()); got != 102 {
		t.Errorf("тел %d, ожидалось 102 (2 косоура + 16 ступеней + 16 подступенков + 66 пяток)", got)
	}
}

// TestCombStringerProfileShape — профиль гребёнки: вершин 2n+3, верхний срез
// вертикален на X = n·b, пятка на полу, глубина спины ровно CombSpec.
func TestCombStringerProfileShape(t *testing.T) {
	n, b, h, st, spine := 16, 280.0, 175.0, 40.0, 300.0
	p, err := combStringerProfile(n, b, h, st, spine)
	if err != nil {
		t.Fatalf("combStringerProfile: %v", err)
	}
	if len(p) != 2*n+3 {
		t.Fatalf("вершин %d, ожидалось %d", len(p), 2*n+3)
	}
	topX, topZ := float64(n)*b, float64(n)*h-st
	last := p[len(p)-2]
	if math.Abs(last.X-topX) > 1e-6 || math.Abs(last.Z-topZ) < 1e-6 {
		t.Errorf("верхний срез не вертикален на X=%g: %v", topX, last)
	}
	if p[0].Z != 0 || p[len(p)-1].Z != 0 {
		t.Errorf("пятка не на полу: %v %v", p[0], p[len(p)-1])
	}
	// глубина по нормали от посадки до нижнего ребра
	l := math.Hypot(b, h)
	perp := func(x, z float64) float64 { return -x*h/l + z*b/l }
	seat := perp(0, h-st)
	// расстояние считается до НИЖНЕГО ребра — это последняя точка профиля
	back := math.Max(perp(p[len(p)-2].X, p[len(p)-2].Z), perp(p[len(p)-1].X, p[len(p)-1].Z))
	if math.Abs((seat-back)-spine) > 0.5 {
		t.Errorf("глубина спины %.2f, задано %g", seat-back, spine)
	}
}

// TestHoleRowFormula — ряд отверстий по формуле: поля 2d, шаг «через одно».
func TestHoleRowFormula(t *testing.T) {
	m, pitch, xs := HoleRow(270, 10, 2, 3, 2, false)
	if m != 20 {
		t.Errorf("поле от края %g, ожидалось 20", m)
	}
	if len(xs) < 2 {
		t.Fatalf("отверстий %d, ожидалось минимум 2", len(xs))
	}
	if math.Abs(xs[0]-20) > 1e-9 || math.Abs(xs[len(xs)-1]-(270-20)) > 1e-9 {
		t.Errorf("крайние отверстия %.2f и %.2f — поля должны быть симметричны 20",
			xs[0], xs[len(xs)-1])
	}
	for i := 1; i < len(xs); i++ {
		gap := xs[i] - xs[i-1]
		if math.Abs(gap-pitch) > 1e-9 {
			t.Errorf("промежуток %d = %.3f, ожидалось %.3f (равномерный ряд)", i, gap, pitch)
		}
		if gap < 3*10-1e-9 {
			t.Errorf("промежуток %.1f меньше минимальных 3·D = 30", gap)
		}
	}
	// центральное отверстие добавляется один раз и не ломает порядок
	_, _, withCenter := HoleRow(161, 10, 2, 3, 2, true)
	for i := 1; i < len(withCenter); i++ {
		if withCenter[i] < withCenter[i-1] {
			t.Errorf("координаты не отсортированы: %v", withCenter)
			break
		}
	}
}

// TestAnchorHolesFirstLastAreSlots — в пятке пола и пятке выхода первое
// отверстие — продольный овал, последнее — поперечный, между ними круглые.
func TestAnchorHolesFirstLastAreSlots(t *testing.T) {
	sp := engineering.CombFrameDefaults()
	xs := []float64{24, 120, 216}
	holes := anchorHoles(sp, xs, kerngeo.NewPoint3(0, 25, 2),
		kerngeo.NewVector3(1, 0, 0), kerngeo.NewVector3(0, 0, 1))
	if len(holes) != len(xs) {
		t.Fatalf("отверстий %d, ожидалось %d", len(holes), len(xs))
	}
	// кольца лежат в плоскости XZ: размах по X и по Z
	spanX := func(r []kerngeo.Point3) float64 {
		lo, hi := r[0].X, r[0].X
		for _, p := range r {
			lo, hi = math.Min(lo, p.X), math.Max(hi, p.X)
		}
		return hi - lo
	}
	spanZ := func(r []kerngeo.Point3) float64 {
		lo, hi := r[0].Z, r[0].Z
		for _, p := range r {
			lo, hi = math.Min(lo, p.Z), math.Max(hi, p.Z)
		}
		return hi - lo
	}
	// первое — овал 30 вдоль детали (X) и 12 поперёк (Z)
	if a, c := spanX(holes[0]), spanZ(holes[0]); a != 30 || c != 12 {
		t.Errorf("первое отверстие %.1f×%.1f, ожидался продольный овал 30×12", a, c)
	}
	// последнее — овал 30 поперёк детали (Z) и 12 вдоль (X)
	last := holes[len(holes)-1]
	if a, c := spanX(last), spanZ(last); a != 12 || c != 30 {
		t.Errorf("последнее отверстие %.1f×%.1f, ожидался поперечный овал 12×30", a, c)
	}
	for _, h := range holes[1 : len(holes)-1] {
		if len(h) != sp.HoleSegments {
			t.Errorf("среднее отверстие из %d точек, ожидалось круглое (%d)",
				len(h), sp.HoleSegments)
		}
	}
}

// ringSpan — размах кольца по Y.
func ringSpan(r []kerngeo.Point3) float64 {
	lo, hi := r[0].Y, r[0].Y
	for _, p := range r {
		lo, hi = math.Min(lo, p.Y), math.Max(hi, p.Y)
	}
	return hi - lo
}

// alongSpan — размах кольца по X.
func alongSpan(r []kerngeo.Point3) float64 {
	lo, hi := r[0].X, r[0].X
	for _, p := range r {
		lo, hi = math.Min(lo, p.X), math.Max(hi, p.X)
	}
	return hi - lo
}

// TestGenerateCombFrameBuilds — конвейер Generate строит модель узла целиком.
func TestGenerateCombFrameBuilds(t *testing.T) {
	res, err := Generate(context.Background(), combConfig())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.Model == nil || len(res.Model.Solids()) < 100 {
		t.Fatalf("тел %d, ожидалось 102", len(res.Model.Solids()))
	}
	if res.Measurement.Volume <= 0 {
		t.Fatalf("объём %.1f", res.Measurement.Volume)
	}
}

// roleBoxes — габариты тел одной роли. Нужен там, где проверяют ПОЛОЖЕНИЕ
// детали, а не объём: объём у ступени с фаской и без одинаков с любой стороны.
func roleBoxes(model *kerngeo.Compound, role string) []kerngeo.BBox {
	var out []kerngeo.BBox
	for _, s := range model.Solids() {
		if s.Role() == role {
			out = append(out, kerngeo.BoundingBox(kerngeo.NewCompound(s)))
		}
	}
	return out
}
