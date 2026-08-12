package geometry

import (
	"math"
	"testing"
)

func triArea(p [][2]float64, tr [3]int) float64 {
	a, b, c := p[tr[0]], p[tr[1]], p[tr[2]]
	return math.Abs(0.5 * ((b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])))
}

func TestTriangulateTriangle(t *testing.T) {
	tris, err := Triangulate([]Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(10, 0, 0),
		NewPoint3(0, 10, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tris) != 1 {
		t.Fatalf("triangles = %d, want 1", len(tris))
	}
	// вершины (0,0),(10,0),(0,10) образуют CCW-контур; итоговый
	// треугольник возвращается в порядке обхода (2,0,1).
	if tris[0] != [3]int{2, 0, 1} {
		t.Fatalf("triangle = %v, want [2 0 1]", tris[0])
	}
}

func TestTriangulateQuad(t *testing.T) {
	quad := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(10, 0, 0),
		NewPoint3(10, 10, 0),
		NewPoint3(0, 10, 0),
	}
	tris, err := Triangulate(quad)
	if err != nil {
		t.Fatal(err)
	}
	if len(tris) != 2 {
		t.Fatalf("triangles = %d, want 2", len(tris))
	}
	// обратный порядок (по часовой) даёт тот же результат.
	rev := []Point3{quad[0], quad[3], quad[2], quad[1]}
	trisRev, err := Triangulate(rev)
	if err != nil {
		t.Fatal(err)
	}
	if len(trisRev) != 2 {
		t.Fatalf("reversed triangles = %d, want 2", len(trisRev))
	}
}

func TestTriangulateConcavePolygon(t *testing.T) {
	// L-образный полигон в плоскости XY, площадь 7.
	poly := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(4, 0, 0),
		NewPoint3(4, 1, 0),
		NewPoint3(1, 1, 0),
		NewPoint3(1, 4, 0),
		NewPoint3(0, 4, 0),
	}
	tris, err := Triangulate(poly)
	if err != nil {
		t.Fatal(err)
	}
	if len(tris) != 4 {
		t.Fatalf("triangles = %d, want 4", len(tris))
	}
	pts := [][2]float64{{0, 0}, {4, 0}, {4, 1}, {1, 1}, {1, 4}, {0, 4}}
	var sum float64
	for _, tr := range tris {
		sum += triArea(pts, tr)
	}
	if math.Abs(sum-7) > 1e-6 {
		t.Fatalf("triangulated area = %v, want 7", sum)
	}
}

func TestTriangulateSawtoothCap(t *testing.T) {
	// профиль косоура: n=2, b=270, h=180, heel=50 — строго простой
	// пилообразный полигон в плоскости XZ.
	profile := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(0, 0, 180),
		NewPoint3(270, 0, 180),
		NewPoint3(270, 0, 360),
		NewPoint3(540, 0, 360),
		NewPoint3(540, 0, 310),
		NewPoint3(0, 0, -50),
	}
	tris, err := Triangulate(profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(tris) != 5 {
		t.Fatalf("triangles = %d, want 5", len(tris))
	}
	// суммарная площадь должна совпасть с площадью полигона (75600).
	pts := [][2]float64{{0, 0}, {0, 180}, {270, 180}, {270, 360}, {540, 360}, {540, 310}, {0, -50}}
	var sum float64
	for _, tr := range tris {
		sum += triArea(pts, tr)
	}
	if math.Abs(sum-75600) > 1e-6 {
		t.Fatalf("triangulated area = %v, want 75600", sum)
	}
}

func TestTriangulateInXZPlane(t *testing.T) {
	// прямоугольник в плоскости XZ (y=const) — проверка проекции.
	tris, err := Triangulate([]Point3{
		NewPoint3(0, 5, 0),
		NewPoint3(10, 5, 0),
		NewPoint3(10, 5, 10),
		NewPoint3(0, 5, 10),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tris) != 2 {
		t.Fatalf("triangles = %d, want 2", len(tris))
	}
}

func TestTriangulateErrors(t *testing.T) {
	if _, err := Triangulate([]Point3{NewPoint3(0, 0, 0), NewPoint3(1, 0, 0)}); err == nil {
		t.Fatal("fewer than 3 points must be rejected")
	}
	collinear := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(1, 0, 0),
		NewPoint3(2, 0, 0),
		NewPoint3(3, 0, 0),
	}
	if _, err := Triangulate(collinear); err == nil {
		t.Fatal("collinear polygon must be rejected")
	}
}

func TestTriangulateDeterminism(t *testing.T) {
	poly := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(4, 0, 0),
		NewPoint3(4, 1, 0),
		NewPoint3(1, 1, 0),
		NewPoint3(1, 4, 0),
		NewPoint3(0, 4, 0),
	}
	a, err := Triangulate(poly)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Triangulate(poly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("triangle %d differs: %v vs %v", i, a[i], b[i])
		}
	}
}
