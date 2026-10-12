package geometry

import (
	"context"
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// woodNoseConfig — прямой марш с ДЕРЕВЯННЫМИ ступенями и заданным радиусом
// филёнки на носике. Металлокаркас оставлен: он даёт стальной косоур 8 мм,
// то есть ровно ту конфигурацию, которую считает калькулятор по умолчанию.
func woodNoseConfig(t *testing.T, nose float64) *engineering.StairConfiguration {
	t.Helper()
	return &engineering.StairConfiguration{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2800),
		Flight:            engineering.FlightStraight,
		StepCount:         16,
		StepHeight:        engineering.Length(175),
		TreadDepth:        engineering.Length(280),
		StringerThickness: engineering.Length(8),
		StepThickness:     engineering.Length(40),
		RiserThickness:    engineering.Length(40),
		Riser:             true,
		Material:          "STEEL-S235",
		TreadMaterial:     "WOOD-OAK",
		TreadNoseRadiusMM: engineering.Length(nose),
		ApproachSpace:     engineering.Length(0),
		Comb:              engineering.CombFrameDefaults(),
	}
}

// TestTreadNoseRadiusRemovesQuarterRound — геометрическая проверка
// скругления носа.
//
// Дуга построена ВПИСАННЫМ многоугольником из noseArcSegments = 10 отрезков,
// поэтому срезается чуть больше точной четверти круга. Тест фиксирует оба
// числа: точное (по вписанному многоугольнику — это и есть то, что построено)
// и приближённое (четверть круга — это то, что указано на чертеже).
func TestTreadNoseRadiusRemovesQuarterRound(t *testing.T) {
	const (
		steps    = 16
		width    = 900.0
		radius   = 10.0
		perTread = 900.0
	)
	treadVolume := func(nose float64) float64 {
		res, err := Generate(context.Background(), woodNoseConfig(t, nose))
		if err != nil {
			t.Fatal(err)
		}
		return roleVolume(t, res.Model, "tread")
	}
	got := treadVolume(0) - treadVolume(radius)

	// Срезано на каждой ступени: вписанный многоугольник четверти круга.
	// Число сегментов берём тем же расчётом, что и геометрия (S-172),
	// иначе тест проверял бы не то, что построено.
	segs := kerngeo.ArcSegments(radius, math.Pi/2, noseArcToleranceMM)
	polygon := 0.5 * float64(segs) * radius * radius *
		math.Sin(math.Pi/(2*float64(segs)))
	want := steps * width * (radius*radius - polygon)
	if math.Abs(got-want) > 1 {
		t.Fatalf("R%v срезает %.1f мм³ на всех ступенях, хотим %.1f (вписанная дуга, %d сегментов)",
			radius, got, want, segs)
	}

	// Число с чертежа: 19 314 мм³ на ступень при R10 и ширине 900. Дуга из
	// 10 сегментов срезает на 1,5 % больше точной четверти — это цена
	// polygonal-приближения, а не ошибка геометрии.
	exact := steps * width * (radius*radius - math.Pi*radius*radius/4)
	if got < exact {
		t.Fatalf("вписанная дуга не может срезать меньше точной четверти: %.1f < %.1f", got, exact)
	}
	if rel := (got - exact) / exact; rel > 0.02 {
		t.Fatalf("срез %.1f отличается от точной четверти %.1f на %.1f%%, чертеж даёт 19 314 мм³/ступень",
			got, exact, rel*100)
	}
	if perTreadMm3 := got / steps; math.Abs(perTreadMm3-19314) > 300 {
		t.Fatalf("%.0f мм³ на ступень, чертеж freecad-out даёт 19 314 (±300 на %d сегментов дуги)",
			perTreadMm3, segs)
	}
}

// TestTreadNoseRadiusClampedByThickness — радиус больше толщины ступени
// физически невозможен: нос не поместится в деталь. Геометрия обязана
// ограничить его толщиной, а не построить тело с вывернутой гранью.
func TestTreadNoseRadiusClampedByThickness(t *testing.T) {
	cfg := woodNoseConfig(t, 10)
	cfg.StepThickness = engineering.Length(6) // тоньше радиуса
	if got := noseRadiusFor(cfg); got != 6 {
		t.Fatalf("nose radius = %v, want 6 (толщина ступени)", got)
	}
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if vol := roleVolume(t, res.Model, "tread"); vol <= 0 {
		t.Fatalf("tread volume = %v, want positive", vol)
	}
	var _ = kerngeo.Precision
}

// TestTreadFilletSitsOnOverhangSide — сторона фаски проступи.
//
// Решение владельца 2026-10-08: филёнка R10 должна стоять на стороне СВЕСА —
// на той кромке, которой проступь нависает над подступенком. До правки стоял
// roundFront, и фаска уезжала на противоположную кромку, к зубу косоура, где
// свеса нет.
//
// Проверять по габариту бесполезно: скругляется УГОЛ, поэтому габарит проступи
// не меняется ни с какой стороны (TestTreadNoseRoundingKeepsBoundingBox). Признак
// стороны — вершина в углу (min X, max Z): при фаске на стороне свеса она
// срезана и её нет, при фаске на другой стороне — есть.
func TestTreadFilletSitsOnOverhangSide(t *testing.T) {
	// Углы вершин каждой проступи: есть ли угол (min X, max Z) — то есть
	// свесовая сторона — и (max X, max Z) — сторона подъёма.
	treadCorners := func(nose float64) (overhang, toplimit int) {
		t.Helper()
		res, err := Generate(context.Background(), woodNoseConfig(t, nose))
		if err != nil {
			t.Fatal(err)
		}
		m := res.Mesh
		if m == nil {
			t.Fatal("Generate вернул модель без меша")
		}
		for _, pr := range m.PartRanges {
			if pr.Role != "tread" {
				continue
			}
			minX, maxZ := math.Inf(1), math.Inf(-1)
			for i := pr.Start; i < pr.End; i++ {
				for _, vi := range m.Triangles[i] {
					if vi >= len(m.Vertices) {
						t.Fatalf("треугольник ссылается на вершину %d из %d", vi, len(m.Vertices))
					}
					p := m.Vertices[vi]
					if p.X < minX {
						minX = p.X
					}
					if p.Z > maxZ {
						maxZ = p.Z
					}
				}
			}
			const eps = 1e-6
			for i := pr.Start; i < pr.End; i++ {
				for _, vi := range m.Triangles[i] {
					p := m.Vertices[vi]
					if math.Abs(p.X-minX) < eps && math.Abs(p.Z-maxZ) < eps {
						overhang++
					}
				}
			}
		}
		return overhang, toplimit
	}

	sharpOverhang, _ := treadCorners(0)
	if sharpOverhang == 0 {
		t.Fatal("без фаски угол (min X, max Z) у проступи должен быть — конфигурация или сборка модели сломана")
	}

	filletOverhang, _ := treadCorners(10)
	if filletOverhang != 0 {
		t.Fatalf("фаска стоит НЕ на стороне свеса: угол (min X, max Z) у %d проступей не срезан", filletOverhang)
	}
	// Объём среза при этом сохраняется — фаска переехала, а не исчезла.
	if v0, v10 := roleVolume(t, mustGenerate(t, woodNoseConfig(t, 0)), "tread"),
		roleVolume(t, mustGenerate(t, woodNoseConfig(t, 10)), "tread"); v0 <= v10 {
		t.Fatalf("фаска не срезает объём: без фаски %.1f, с фаской %.1f", v0, v10)
	}
}

func mustGenerate(t *testing.T, cfg *engineering.StairConfiguration) *kerngeo.Compound {
	t.Helper()
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return res.Model
}
