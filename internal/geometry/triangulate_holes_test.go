package geometry

import (
	"math"
	"testing"
)

// ringArea — сумма знаковых площадей треугольников триангуляции.
func ringArea(tris [][3]int, pts []Point3) float64 {
	var s float64
	for _, tr := range tris {
		a, b, c := pts[tr[0]], pts[tr[1]], pts[tr[2]]
		s += triangleArea(a, b, c)
	}
	return s
}

// TriangulateWithHoles возвращает свои точки, поэтому в тестах площадь
// считается по ним, а не по входным точкам.
func triangulate(t *testing.T, outer []Point3, holes [][]Point3) (float64, int) {
	t.Helper()
	pts, tris, err := TriangulateWithHoles(outer, holes)
	if err != nil {
		t.Fatalf("TriangulateWithHoles: %v", err)
	}
	return ringArea(tris, pts), len(tris)
}

// Плоскость Z=0: все вызовы передавали z=0, поэтому параметр убран.
func rectRing(x0, x1, y0, y1 float64) []Point3 {
	return []Point3{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

func circleRing(cx, cy, r float64, n int) []Point3 {
	out := make([]Point3, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		out[i] = Point3{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a)}
	}
	return out
}

func slotRing(cx, cy, length, width, z float64) []Point3 {
	// Овал по X: длина length, ширина width.
	h := length / 2
	w := width / 2
	r := w
	cy0, cy1 := cy-h+r, cy+h-r
	arc := func(cxc, cyc, from, to float64, steps int) []Point3 {
		out := make([]Point3, 0, steps)
		for i := 0; i <= steps; i++ {
			a := from + (to-from)*float64(i)/float64(steps)
			out = append(out, Point3{X: cxc + r*math.Cos(a), Y: cyc + r*math.Sin(a), Z: z})
		}
		return out
	}
	out := make([]Point3, 0, 12)
	out = append(out, arc(cx+r, cy1, -math.Pi/2, math.Pi/2, 6)...)
	out = append(out, arc(cx-r, cy0, math.Pi/2, 3*math.Pi/2, 6)...)
	return out
}

func TestTriangulateWithHolesCoversPlateMinusHoles(t *testing.T) {
	outer := rectRing(0, 280, 0, 50)
	holes := [][]Point3{
		circleRing(24, 25, 6, 24),
		circleRing(101.333, 25, 6, 24),
		circleRing(178.667, 25, 6, 24),
		circleRing(256, 25, 6, 24),
	}
	got, n := triangulate(t, outer, holes)
	want := 280*50.0 - 4*polyArea2D(project2D(holes[0], mustNormal(outer)))
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("area = %.6f, want %.6f", got, want)
	}
	t.Logf("треугольников %d (без мостиков и дубликатов вершин)", n)
}

// polyArea2D — площадь замкнутого кольца в его СОБСТВЕННОЙ плоскости.
func polyArea2D(ring [][2]float64) float64 {
	return math.Abs(signedArea2(ring))
}

// slotRingY — овал, длинный по Y: длина вдоль Y, ширина по X. Строится
// поворотом овала из slotRing на 90° ВОКРУГ ЦЕНТРА, а не вокруг нуля.
func slotRingY(cx, cy, length, width, z float64) []Point3 {
	base := slotRing(0, 0, length, width, 0)
	out := make([]Point3, len(base))
	for i, p := range base {
		out[i] = Point3{X: cx + p.Y, Y: cy + p.X, Z: z}
	}
	return out
}

func TestTriangulateWithHolesSingleSquare(t *testing.T) {
	outer := rectRing(0, 100, 0, 100)
	holes := [][]Point3{rectRing(30, 70, 30, 70)}
	got, _ := triangulate(t, outer, holes)
	if want := 100.0*100 - 40*40; math.Abs(got-want) > 1e-6 {
		t.Fatalf("area = %.6f, want %.6f", got, want)
	}
}

func TestTriangulateWithHolesReversedOrientation(t *testing.T) {
	outer := rectRing(0, 200, 0, 80)
	// Внешний контур CW, отверстие CCW — нормализация обязана это скрыть.
	outerCW := reverse3(outer)
	holes := [][]Point3{reverse3(circleRing(50, 40, 8, 20))}
	got, _ := triangulate(t, outerCW, holes)
	want := 200*80.0 - polyArea2D(project2D(holes[0], mustNormal(outer)))
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("area = %.6f, want %.6f", got, want)
	}
}

func TestTriangulateWithHolesSlots(t *testing.T) {
	// Пятка пола с эталонной раскладкой: 4 отверстия, первое и последнее —
	// овалы 30×12, средние — круглые Ø12.
	outer := rectRing(0, 280, 0, 50)
	holes := [][]Point3{
		slotRing(24, 25, 30, 12, 0),
		circleRing(101.333, 25, 6, 24),
		circleRing(178.667, 25, 6, 24),
		slotRingY(256, 25, 30, 12, 0),
	}
	got, _ := triangulate(t, outer, holes)
	want := 280*50.0 - polyArea2D(project2D(holes[0], mustNormal(outer))) -
		polyArea2D(project2D(holes[1], mustNormal(outer))) -
		polyArea2D(project2D(holes[2], mustNormal(outer))) -
		polyArea2D(project2D(holes[3], mustNormal(outer)))
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("area = %.6f, want %.6f", got, want)
	}
}

func reverse3(p []Point3) []Point3 {
	out := make([]Point3, len(p))
	for i := range p {
		out[i] = p[len(p)-1-i]
	}
	return out
}

func mustNormal(p []Point3) Vector3 {
	n, _ := polygonNormal(p)
	return n
}

// TestTriangulateWithHolesSlopedFace — грань с отверстием под наклоном
// триангулируется в своей плоскости (S-212).
//
// Почему тест: проекция отбрасывает ОДНУ МИРОВУЮ ось, а отброшенная
// координата восстанавливалась константой (из первой точки внешнего
// контура). Для грани, параллельной мировой оси, это верно; для
// наклонной — нет: все точки лежат в одной плоскости, но их координаты
// по отброшенной оси разные, и грань схлопывалась. Крышка трубы под
// уклоном теряла до 20 % площади, и с ней объём поручня и ригелей.
// Тест держит регрессию на трёх ориентациях сразу.
func TestTriangulateWithHolesSlopedFace(t *testing.T) {
	want := 192.74 // π/4·(42,4²−39,4²): крышка трубы Ø42,4×1,5
	for _, c := range []struct {
		name string
		dir  Vector3
	}{
		{"ось Z", NewVector3(0, 0, 1)},
		{"уклон 33,7°", NewVector3(1620, 0, 1080)},
		{"уклон 36,1°", NewVector3(4195, 0, 3075)},
		{"горизонтальный Y", NewVector3(0, 1000, 0)},
	} {
		u, ok := c.dir.Normalized()
		if !ok {
			t.Fatalf("%s: нулевой вектор", c.name)
		}
		// Строим крышку-трубу через тот же путь, что и геометрия:
		// ExtrudeWithHoles создаёт грани с отверстиями и триангулирует их.
		s, err := ExtrudeWithHoles(
			ringPoints(u, 42.4, 0),
			// полость в той же плоскости: сдвиг на верхнюю крышку делает
			// сам ExtrudeWithHoles, передавать сдвинутую — значит отверстие
			// уедет за пределы грани и тест перестанет что-либо мерить.
			[][]Point3{ringPoints(u, 39.4, 0)},
			u, 1947)
		if err != nil {
			t.Fatalf("%s: ExtrudeWithHoles: %v", c.name, err)
		}
		cache := NewTessellationCache()
		caps := 0
		for _, sh := range s.Shells() {
			for _, f := range sh.Faces() {
				if len(f.Inner()) == 0 {
					continue
				}
				caps++
				tess := cache.FaceOf(f)
				if tess.TrisErr != nil {
					t.Fatalf("%s: триангуляция: %v", c.name, tess.TrisErr)
				}
				var area float64
				for _, tr := range tess.Tris {
					p := tess.Points
					ab := p[tr[1]].Sub(p[tr[0]])
					ac := p[tr[2]].Sub(p[tr[0]])
					area += 0.5 * ab.Cross(ac).Norm()
				}
				if area/want < 0.98 {
					t.Errorf("%s: крышка площадью %.2f, ждали %.2f (−%.1f %%)",
						c.name, area, want, (1-area/want)*100)
				}
			}
		}
		if caps != 2 {
			t.Errorf("%s: крышек с отверстиями %d, ждали 2", c.name, caps)
		}
	}
}

// ringPoints — окружность в плоскости, перпендикулярной u, с центром
// на расстоянии along от начала: крышка трубы вдоль оси u.
func ringPoints(u Vector3, dia, along float64) []Point3 {
	w, ok := perp(u)
	if !ok {
		panic("ringPoints: нулевой вектор")
	}
	t, ok := u.Cross(w).Normalized()
	if !ok {
		panic("ringPoints: вырожденный базис")
	}
	seg := ArcSegments(dia/2, 2*3.141592653589793, 0.19)
	pts := make([]Point3, seg)
	for i := range pts {
		a := 2 * 3.141592653589793 * float64(i) / float64(seg)
		off := w.Scale(dia / 2 * cosA(a)).Add(t.Scale(dia / 2 * sinA(a)))
		p := u.Scale(along).Add(off)
		pts[i] = Point3(p)
	}
	return pts
}

// perp — единичный вектор, перпендикулярный v (копия одноимённой
// геометрической функции, чтобы тест не зависел от неё).
func perp(v Vector3) (Vector3, bool) {
	ref := NewVector3(0, 0, 1)
	if math.Abs(v.Z) > 0.9 {
		ref = NewVector3(1, 0, 0)
	}
	return v.Cross(ref).Normalized()
}

func cosA(a float64) float64 { return math.Cos(a) }
func sinA(a float64) float64 { return math.Sin(a) }
