package geometry

import "testing"

// TestWeldRemovesExactDuplicates — сварка сливает вершины, совпадающие
// побитово, и не трогает различающиеся (S-175).
func TestWeldRemovesExactDuplicates(t *testing.T) {
	m := &Mesh{
		Vertices: []Point3{
			{X: 0, Y: 0, Z: 0},
			{X: 1, Y: 0, Z: 0},
			{X: 0, Y: 1, Z: 0},
			{X: 0, Y: 0, Z: 0}, // дубль 0
			{X: 1, Y: 0, Z: 0}, // дубль 1
			{X: 0, Y: 1, Z: 0}, // дубль 2
		},
		Triangles: [][3]int{{0, 1, 2}, {3, 4, 5}},
	}
	w, n := m.Weld()
	if n != 3 {
		t.Fatalf("welded = %d, want 3 (6 вершин → 3)", n)
	}
	if len(w.Vertices) != 3 {
		t.Fatalf("vertices = %d, want 3", len(w.Vertices))
	}
	if len(w.Triangles) != 2 {
		t.Fatalf("triangles = %d, want 2", len(w.Triangles))
	}
	// После сварки оба треугольника обязаны ссылаться на одни и те же вершины.
	if w.Triangles[0] != w.Triangles[1] {
		t.Fatalf("triangles not remapped: %v vs %v", w.Triangles[0], w.Triangles[1])
	}
}

// TestWeldKeepsDistinctPoints — точки, отличающиеся на единицу в младшем
// разряде, НЕ сливаются: сварка строго точная, без допуска.
func TestWeldKeepsDistinctPoints(t *testing.T) {
	delta := 1e-9
	m := &Mesh{
		Vertices: []Point3{
			{X: 0, Y: 0, Z: 0},
			{X: delta, Y: 0, Z: 0},
			{X: 0, Y: 1, Z: 0},
		},
		Triangles: [][3]int{{0, 1, 2}},
	}
	w, n := m.Weld()
	if n != 0 {
		t.Fatalf("welded = %d, want 0: различающиеся точки сливать нельзя", n)
	}
	if w != m {
		t.Fatal("при нулевом слиянии должен возвращаться исходный меш, а не копия")
	}
	if len(w.Vertices) != 3 {
		t.Fatalf("vertices = %d, want 3", len(w.Vertices))
	}
}

// TestWeldPreservesPartRanges — PartRanges описывают диапазоны по
// ТРЕУГОЛЬНИКАМ, поэтому сварка индексов вершин их не трогает.
func TestWeldPreservesPartRanges(t *testing.T) {
	m := &Mesh{
		Vertices:  []Point3{{X: 0}, {X: 1}, {X: 0, Y: 1}, {X: 0}, {X: 1}, {X: 0, Y: 1}},
		Triangles: [][3]int{{0, 1, 2}, {3, 4, 5}},
		PartRanges: []PartRange{
			{Solid: 0, Role: "tread", Start: 0, End: 1},
			{Solid: 1, Role: "stringer", Start: 1, End: 2},
		},
	}
	w, _ := m.Weld()
	if len(w.PartRanges) != len(m.PartRanges) {
		t.Fatalf("PartRanges = %d, want %d", len(w.PartRanges), len(m.PartRanges))
	}
	for i := range m.PartRanges {
		if w.PartRanges[i] != m.PartRanges[i] {
			t.Fatalf("PartRange[%d] = %+v, want %+v", i, w.PartRanges[i], m.PartRanges[i])
		}
	}
}

// TestWeldDropsUV — UV относятся к граням, а не к точкам, поэтому после
// сварки их быть не должно: иначе текстура натянулась бы по UV первой
// грани на все остальные.
func TestWeldDropsUV(t *testing.T) {
	m := &Mesh{
		Vertices:  []Point3{{X: 0}, {X: 1}, {X: 0, Y: 1}, {X: 0}},
		Triangles: [][3]int{{0, 1, 2}, {0, 1, 3}},
		UV:        []Point2{{U: 0}, {U: 1}, {V: 1}, {U: 0}},
	}
	w, n := m.Weld()
	if n == 0 {
		t.Fatal("ожидалось слияние")
	}
	if len(w.UV) != 0 {
		t.Fatalf("UV = %d, want 0 после сварки", len(w.UV))
	}
}

// TestWeldEmptyMesh — пустой и nil-меш не паникуют.
func TestWeldEmptyMesh(t *testing.T) {
	if _, n := (*Mesh)(nil).Weld(); n != 0 {
		t.Fatalf("nil mesh welded = %d, want 0", n)
	}
	if _, n := (&Mesh{}).Weld(); n != 0 {
		t.Fatalf("empty mesh welded = %d, want 0", n)
	}
}
