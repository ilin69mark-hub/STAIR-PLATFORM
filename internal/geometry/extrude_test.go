package geometry

import (
	"testing"
)

func collectPoints(s *Solid) []Point3 {
	seen := make(map[Point3]bool)
	var pts []Point3
	walk := func(p Point3) {
		if !seen[p] {
			seen[p] = true
			pts = append(pts, p)
		}
	}
	for _, shell := range s.Shells() {
		for _, face := range shell.Faces() {
			for _, e := range face.Outer().Edges() {
				v1, v2 := e.Endpoints()
				walk(v1.Point())
				walk(v2.Point())
			}
		}
	}
	return pts
}

func hasPoint(s *Solid, want Point3) bool {
	for _, p := range collectPoints(s) {
		if p.X == want.X && p.Y == want.Y && p.Z == want.Z {
			return true
		}
	}
	return false
}

func TestExtrudePrismStructure(t *testing.T) {
	profile := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(10, 0, 0),
		NewPoint3(10, 10, 0),
		NewPoint3(0, 10, 0),
	}
	solid, err := Extrude(profile, NewVector3(0, 0, 1), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(solid.Shells()) != 1 {
		t.Fatalf("shells = %d, want 1", len(solid.Shells()))
	}
	faces := solid.Shells()[0].Faces()
	if len(faces) != 6 {
		t.Fatalf("faces = %d, want 6", len(faces))
	}
	// 8 уникальных вершин призмы 4-гранного профиля.
	pts := collectPoints(solid)
	if len(pts) != 8 {
		t.Fatalf("vertices = %d, want 8", len(pts))
	}
	// крышки и боковые грани на ожидаемых координатах.
	if !hasPoint(solid, NewPoint3(0, 0, 5)) || !hasPoint(solid, NewPoint3(10, 10, 5)) {
		t.Fatal("top cap vertices must be at z=5")
	}
	if !hasPoint(solid, NewPoint3(0, 0, 0)) || !hasPoint(solid, NewPoint3(10, 10, 0)) {
		t.Fatal("bottom cap vertices must be at z=0")
	}
}

func TestExtrudeDeterminism(t *testing.T) {
	profile := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(4, 0, 0),
		NewPoint3(4, 0, 4),
		NewPoint3(0, 0, 4),
	}
	a, err := Extrude(profile, NewVector3(0, 1, 0), 3)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Extrude(profile, NewVector3(0, 1, 0), 3)
	if err != nil {
		t.Fatal(err)
	}
	// одинаковое число граней и совпадающие уникальные вершины.
	fa := a.Shells()[0].Faces()
	fb := b.Shells()[0].Faces()
	if len(fa) != len(fb) {
		t.Fatalf("faces differ: %d vs %d", len(fa), len(fb))
	}
	pa, pb := collectPoints(a), collectPoints(b)
	if len(pa) != len(pb) {
		t.Fatalf("vertices differ: %d vs %d", len(pa), len(pb))
	}
	for i := range pa {
		if pa[i] != pb[i] {
			t.Fatalf("vertex %d differs: %+v vs %+v", i, pa[i], pb[i])
		}
	}
}

func TestExtrudeErrors(t *testing.T) {
	ok := []Point3{NewPoint3(0, 0, 0), NewPoint3(10, 0, 0), NewPoint3(10, 10, 0)}
	dir := NewVector3(0, 0, 1)

	if _, err := Extrude(ok[:2], dir, 5); err == nil {
		t.Fatal("fewer than 3 profile points must be rejected")
	}
	collinear := []Point3{NewPoint3(0, 0, 0), NewPoint3(1, 0, 0), NewPoint3(2, 0, 0)}
	if _, err := Extrude(collinear, dir, 5); err == nil {
		t.Fatal("collinear profile must be rejected")
	}
	nonPlanar := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(10, 0, 0),
		NewPoint3(10, 10, 0),
		NewPoint3(0, 10, 5),
	}
	if _, err := Extrude(nonPlanar, dir, 5); err == nil {
		t.Fatal("non-planar profile must be rejected")
	}
	// направление в плоскости профиля (XY) вырождает призму.
	inPlane := NewVector3(1, 0, 0)
	if _, err := Extrude(ok, inPlane, 5); err == nil {
		t.Fatal("in-plane extrusion direction must be rejected")
	}
	if _, err := Extrude(ok, dir, 0); err == nil {
		t.Fatal("zero extrusion distance must be rejected")
	}
	if _, err := Extrude(ok, ZeroVector(), 5); err == nil {
		t.Fatal("zero extrusion direction must be rejected")
	}
}

func TestMeshAddTriangle(t *testing.T) {
	m := &Mesh{}
	m.Vertices = []Point3{NewPoint3(0, 0, 0), NewPoint3(1, 0, 0), NewPoint3(0, 1, 0)}
	if err := m.AddTriangle(0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if len(m.Triangles) != 1 {
		t.Fatalf("triangles = %d, want 1", len(m.Triangles))
	}
	if err := m.AddTriangle(0, 1, 5); err == nil {
		t.Fatal("out-of-range index must be rejected")
	}
}
