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

func rectRing(x0, x1, y0, y1, z float64) []Point3 {
	return []Point3{{X: x0, Y: y0, Z: z}, {X: x1, Y: y0, Z: z}, {X: x1, Y: y1, Z: z}, {X: x0, Y: y1, Z: z}}
}

func circleRing(cx, cy, r, z float64, n int) []Point3 {
	out := make([]Point3, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		out[i] = Point3{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a), Z: z}
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
	var out []Point3
	out = append(out, arc(cx+r, cy1, -math.Pi/2, math.Pi/2, 6)...)
	out = append(out, arc(cx-r, cy0, math.Pi/2, 3*math.Pi/2, 6)...)
	return out
}

func TestTriangulateWithHolesCoversPlateMinusHoles(t *testing.T) {
	outer := rectRing(0, 280, 0, 50, 0)
	holes := [][]Point3{
		circleRing(24, 25, 6, 0, 24),
		circleRing(101.333, 25, 6, 0, 24),
		circleRing(178.667, 25, 6, 0, 24),
		circleRing(256, 25, 6, 0, 24),
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

func sumLens(rings [][]Point3) int {
	n := 0
	for _, r := range rings {
		n += len(r)
	}
	return n
}

func TestTriangulateWithHolesSingleSquare(t *testing.T) {
	outer := rectRing(0, 100, 0, 100, 0)
	holes := [][]Point3{rectRing(30, 70, 30, 70, 0)}
	got, _ := triangulate(t, outer, holes)
	if want := 100.0*100 - 40*40; math.Abs(got-want) > 1e-6 {
		t.Fatalf("area = %.6f, want %.6f", got, want)
	}
}

func TestTriangulateWithHolesReversedOrientation(t *testing.T) {
	outer := rectRing(0, 200, 0, 80, 0)
	// Внешний контур CW, отверстие CCW — нормализация обязана это скрыть.
	outerCW := reverse3(outer)
	holes := [][]Point3{reverse3(circleRing(50, 40, 8, 0, 20))}
	got, _ := triangulate(t, outerCW, holes)
	want := 200*80.0 - polyArea2D(project2D(holes[0], mustNormal(outer)))
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("area = %.6f, want %.6f", got, want)
	}
}

func TestTriangulateWithHolesSlots(t *testing.T) {
	// Пятка пола с эталонной раскладкой: 4 отверстия, первое и последнее —
	// овалы 30×12, средние — круглые Ø12.
	outer := rectRing(0, 280, 0, 50, 0)
	holes := [][]Point3{
		slotRing(24, 25, 30, 12, 0),
		circleRing(101.333, 25, 6, 0, 24),
		circleRing(178.667, 25, 6, 0, 24),
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
