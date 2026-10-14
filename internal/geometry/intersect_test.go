package geometry

import "testing"

// boxMesh — набор замкнутых коробов, по одному на каждый PartRange.
// Раскладка по телам обязательна: без неё FindPenetrations работать не на
// чем, и проверять тут нечего.
func boxMesh(boxes []PartRange, corners []Point3) *Mesh {
	var m Mesh
	for _, b := range boxes {
		c := corners[b.Solid*8 : b.Solid*8+8]
		base := len(m.Vertices)
		start := len(m.Triangles)
		m.Vertices = append(m.Vertices, c...)
		add := func(a, bb, cc int) {
			m.Triangles = append(m.Triangles, [3]int{base + a, base + bb, base + cc})
		}
		// Нижняя грань (z-), верхняя (z+), четыре боковые.
		add(0, 2, 1)
		add(0, 3, 2)
		add(4, 5, 6)
		add(4, 6, 7)
		add(0, 1, 5)
		add(0, 5, 4)
		add(1, 2, 6)
		add(1, 6, 5)
		add(2, 3, 7)
		add(2, 7, 6)
		add(3, 0, 4)
		add(3, 4, 7)
		m.PartRanges = append(m.PartRanges, PartRange{
			Solid: b.Solid, Role: b.Role, Start: start, End: len(m.Triangles),
		})
	}
	return &m
}

func unitBox(x0, y0, z0, s float64) []Point3 {
	return []Point3{
		{x0, y0, z0}, {x0 + s, y0, z0}, {x0 + s, y0 + s, z0}, {x0, y0 + s, z0},
		{x0, y0, z0 + s}, {x0 + s, y0, z0 + s}, {x0 + s, y0 + s, z0 + s}, {x0, y0 + s, z0 + s},
	}
}

// TestFindPenetrationsFindsFullContainment — куб внутри куба: габариты
// перекрываются, все 8 вершин внутреннего лежат в наружном.
func TestFindPenetrationsFindsFullContainment(t *testing.T) {
	corners := append(unitBox(0, 0, 0, 100), unitBox(40, 40, 40, 10)...)
	m := boxMesh([]PartRange{
		{Solid: 0, Role: "stringer", Start: 0, End: 12},
		{Solid: 1, Role: "tread", Start: 12, End: 24},
	}, corners)
	got := FindPenetrations(m)
	if len(got) != 1 {
		t.Fatalf("проникновений = %d, ожидалось 1: %+v", len(got), got)
	}
	p := got[0]
	// Большой куб — A, малый — B. Вершины малого лежат внутри большого,
	// поэтому заполнено PointsInA, а PointsInB остаётся пустым.
	if p.PointsInA != 8 {
		t.Errorf("PointsInA = %d, ожидалось 8 (все вершины малого куба внутри большого)", p.PointsInA)
	}
	if p.PointsInB != 0 {
		t.Errorf("PointsInB = %d, ожидалось 0 (вершины большого куба вне малого)", p.PointsInB)
	}
	if p.RoleA != "stringer" || p.RoleB != "tread" {
		t.Errorf("роли = %q/%q, ожидалось stringer/tread", p.RoleA, p.RoleB)
	}
	if d := p.Depth(); d != 10 {
		t.Errorf("Depth = %v, ожидалось 10 (сторона малого куба)", d)
	}
}

// TestFindPenetrationsIgnoresSeparatedBodies — два куба, не касающихся
// друг друга, проникновением не считаются.
func TestFindPenetrationsIgnoresSeparatedBodies(t *testing.T) {
	corners := append(unitBox(0, 0, 0, 10), unitBox(100, 100, 100, 10)...)
	m := boxMesh([]PartRange{
		{Solid: 0, Role: "tread", Start: 0, End: 12},
		{Solid: 1, Role: "riser", Start: 12, End: 24},
	}, corners)
	if got := FindPenetrations(m); len(got) != 0 {
		t.Fatalf("проникновений = %d, ожидалось 0: %+v", len(got), got)
	}
}

// TestFindPenetrationsIgnoresTouchingFaces — кубы, соприкасающиеся гранью
// (перекрытие габаритов = 0), проникновением не считаются: объёма
// пересечения нет.
func TestFindPenetrationsIgnoresTouchingFaces(t *testing.T) {
	corners := append(unitBox(0, 0, 0, 10), unitBox(10, 0, 0, 10)...)
	m := boxMesh([]PartRange{
		{Solid: 0, Role: "tread", Start: 0, End: 12},
		{Solid: 1, Role: "riser", Start: 12, End: 24},
	}, corners)
	if got := FindPenetrations(m); len(got) != 0 {
		t.Fatalf("проникновений = %d, ожидалось 0 (только касание): %+v", len(got), got)
	}
}

// TestFindPenetrationsPartialOverlap — наполовину вложенный куб: часть
// вершин внутри, часть снаружи.
func TestFindPenetrationsPartialOverlap(t *testing.T) {
	corners := append(unitBox(0, 0, 0, 100), unitBox(90, 10, 10, 50)...)
	m := boxMesh([]PartRange{
		{Solid: 0, Role: "stringer", Start: 0, End: 12},
		{Solid: 1, Role: "tread", Start: 12, End: 24},
	}, corners)
	got := FindPenetrations(m)
	if len(got) != 1 {
		t.Fatalf("проникновений = %d, ожидалось 1: %+v", len(got), got)
	}
	// Малый куб наполовину вложен: часть вершин внутри большого.
	if n := got[0].PointsInA; n == 0 || n == 8 {
		t.Errorf("PointsInA = %d, ожидалось значение между 0 и 8 (частичное пересечение)", n)
	}
}

// TestFindPenetrationsNeedsPartRanges — без раскладки по телам работать
// не на чем, и детектор обязан честно вернуть пусто, а не выдумать.
func TestFindPenetrationsNeedsPartRanges(t *testing.T) {
	m := boxMesh(nil, unitBox(0, 0, 0, 10))
	if got := FindPenetrations(m); len(got) != 0 {
		t.Fatalf("без PartRanges ожидалось 0, получено %d", len(got))
	}
	if got := FindPenetrations(nil); len(got) != 0 {
		t.Fatalf("nil-меш: ожидалось 0, получено %d", len(got))
	}
}

// TestFindPenetrationsDeterministic — два одинаковых прогона обязаны дать
// побайтово одинаковый результат (ADR-0007).
func TestFindPenetrationsDeterministic(t *testing.T) {
	corners := append(
		append(unitBox(0, 0, 0, 100), unitBox(40, 40, 40, 10)...),
		append(unitBox(50, 5, 5, 20), unitBox(0, 0, 90, 30)...)...)
	m := boxMesh([]PartRange{
		{Solid: 0, Role: "stringer", Start: 0, End: 12},
		{Solid: 1, Role: "tread", Start: 12, End: 24},
		{Solid: 2, Role: "riser", Start: 24, End: 36},
		{Solid: 3, Role: "landing", Start: 36, End: 48},
	}, corners)
	first := FindPenetrations(m)
	for run := 0; run < 3; run++ {
		again := FindPenetrations(m)
		if len(again) != len(first) {
			t.Fatalf("прогон %d: %d проникновений, в первом %d", run, len(again), len(first))
		}
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("прогон %d, элемент %d: %+v != %+v", run, i, again[i], first[i])
			}
		}
	}
	if len(first) == 0 {
		t.Fatal("тест ничего не нашёл — он не проверяет ничего")
	}
}
