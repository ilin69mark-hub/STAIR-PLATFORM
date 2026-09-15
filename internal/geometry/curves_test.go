package geometry

import (
	"math"
	"testing"
)

func TestArcCreation(t *testing.T) {
	arc, err := NewArc(Point3{X: 0, Y: 0, Z: 0}, 100, Vector3{X: 0, Y: 0, Z: 1}, 0, math.Pi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arc.Radius != 100 {
		t.Fatalf("expected radius 100, got %v", arc.Radius)
	}
}

func TestArcCreationInvalidRadius(t *testing.T) {
	_, err := NewArc(Point3{}, -1, Vector3{X: 0, Y: 0, Z: 1}, 0, math.Pi)
	if err == nil {
		t.Fatal("expected error for negative radius")
	}
}

func TestArcCreationInvalidNormal(t *testing.T) {
	_, err := NewArc(Point3{}, 100, Vector3{}, 0, math.Pi)
	if err == nil {
		t.Fatal("expected error for zero normal")
	}
}

func TestArcCreationSameAngles(t *testing.T) {
	_, err := NewArc(Point3{}, 100, Vector3{X: 0, Y: 0, Z: 1}, math.Pi, math.Pi)
	if err == nil {
		t.Fatal("expected error for same angles")
	}
}

func TestArcPointAt(t *testing.T) {
	arc, _ := NewArc(Point3{X: 0, Y: 0, Z: 0}, 100, Vector3{X: 0, Y: 0, Z: 1}, 0, math.Pi)
	p0 := arc.PointAt(0)
	p1 := arc.PointAt(1)
	// Расстояние от центра до начала дуги
	d0 := p0.Distance(arc.Center)
	d1 := p1.Distance(arc.Center)
	if math.Abs(d0-100) > Precision {
		t.Fatalf("expected start distance ≈ 100, got %v", d0)
	}
	if math.Abs(d1-100) > Precision {
		t.Fatalf("expected end distance ≈ 100, got %v", d1)
	}
}

func TestArcLength(t *testing.T) {
	arc, _ := NewArc(Point3{}, 100, Vector3{X: 0, Y: 0, Z: 1}, 0, math.Pi)
	length := arc.Length()
	expected := 100 * math.Pi
	if math.Abs(length-expected) > Precision {
		t.Fatalf("expected length %v, got %v", expected, length)
	}
}

func TestArcTessellate(t *testing.T) {
	arc, _ := NewArc(Point3{}, 100, Vector3{X: 0, Y: 0, Z: 1}, 0, math.Pi)
	points := arc.Tessellate(10)
	if len(points) != 11 {
		t.Fatalf("expected 11 points, got %d", len(points))
	}
}

func TestBezierCreation(t *testing.T) {
	b := &Bezier{
		P0: Point3{X: 0, Y: 0, Z: 0},
		P1: Point3{X: 1, Y: 1, Z: 0},
		P2: Point3{X: 2, Y: 1, Z: 0},
		P3: Point3{X: 3, Y: 0, Z: 0},
	}
	if len(ValidateBezier(b)) != 0 {
		t.Fatal("expected valid bezier")
	}
}

func TestBezierPointAt(t *testing.T) {
	b := &Bezier{
		P0: Point3{X: 0, Y: 0, Z: 0},
		P1: Point3{X: 1, Y: 1, Z: 0},
		P2: Point3{X: 2, Y: 1, Z: 0},
		P3: Point3{X: 3, Y: 0, Z: 0},
	}
	p0 := b.PointAt(0)
	p1 := b.PointAt(1)
	if p0 != b.P0 {
		t.Fatalf("expected start point P0")
	}
	if p1 != b.P3 {
		t.Fatalf("expected end point P3")
	}
}

func TestBezierTessellate(t *testing.T) {
	b := &Bezier{
		P0: Point3{X: 0, Y: 0, Z: 0},
		P1: Point3{X: 1, Y: 1, Z: 0},
		P2: Point3{X: 2, Y: 1, Z: 0},
		P3: Point3{X: 3, Y: 0, Z: 0},
	}
	points := b.Tessellate(5)
	if len(points) != 6 {
		t.Fatalf("expected 6 points, got %d", len(points))
	}
}

func TestBezierApproximateLength(t *testing.T) {
	b := &Bezier{
		P0: Point3{X: 0, Y: 0, Z: 0},
		P1: Point3{X: 1, Y: 1, Z: 0},
		P2: Point3{X: 2, Y: 1, Z: 0},
		P3: Point3{X: 3, Y: 0, Z: 0},
	}
	length := b.ApproximateLength(100)
	if length <= 0 {
		t.Fatalf("expected positive length, got %v", length)
	}
}

func TestFilletCreation(t *testing.T) {
	f, err := NewFillet(Point3{X: 0, Y: 0, Z: 0}, Point3{X: 100, Y: 0, Z: 0}, 10, Vector3{X: 0, Y: 0, Z: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Radius != 10 {
		t.Fatalf("expected radius 10, got %v", f.Radius)
	}
}

func TestFilletCreationInvalidRadius(t *testing.T) {
	_, err := NewFillet(Point3{}, Point3{X: 100, Y: 0, Z: 0}, -1, Vector3{X: 0, Y: 0, Z: 1})
	if err == nil {
		t.Fatal("expected error for negative radius")
	}
}

func TestFilletTessellate(t *testing.T) {
	f, _ := NewFillet(Point3{X: 0, Y: 0, Z: 0}, Point3{X: 100, Y: 0, Z: 0}, 10, Vector3{X: 0, Y: 0, Z: 1})
	points := f.Tessellate(10)
	if len(points) != 11 {
		t.Fatalf("expected 11 points, got %d", len(points))
	}
}

func TestChamferCreation(t *testing.T) {
	c, err := NewChamfer(Point3{X: 0, Y: 0, Z: 0}, Point3{X: 100, Y: 0, Z: 0}, 5, Vector3{X: 0, Y: 0, Z: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Width != 5 {
		t.Fatalf("expected width 5, got %v", c.Width)
	}
}

func TestChamferCreationInvalidWidth(t *testing.T) {
	_, err := NewChamfer(Point3{}, Point3{X: 100, Y: 0, Z: 0}, -1, Vector3{X: 0, Y: 0, Z: 1})
	if err == nil {
		t.Fatal("expected error for negative width")
	}
}

func TestChamferTessellate(t *testing.T) {
	c, _ := NewChamfer(Point3{X: 0, Y: 0, Z: 0}, Point3{X: 100, Y: 0, Z: 0}, 5, Vector3{X: 0, Y: 0, Z: 1})
	points := c.Tessellate()
	if len(points) != 4 {
		t.Fatalf("expected 4 points, got %d", len(points))
	}
}

func TestNGonCreation(t *testing.T) {
	vertices := []Point3{
		{X: 0, Y: 0, Z: 0},
		{X: 100, Y: 0, Z: 0},
		{X: 100, Y: 100, Z: 0},
		{X: 0, Y: 100, Z: 0},
	}
	n, err := NewNGon(vertices, Vector3{X: 0, Y: 0, Z: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(n.Vertices) != 4 {
		t.Fatalf("expected 4 vertices, got %d", len(n.Vertices))
	}
}

func TestNGonCreationTooFewVertices(t *testing.T) {
	vertices := []Point3{
		{X: 0, Y: 0, Z: 0},
		{X: 100, Y: 0, Z: 0},
	}
	_, err := NewNGon(vertices, Vector3{X: 0, Y: 0, Z: 1})
	if err == nil {
		t.Fatal("expected error for too few vertices")
	}
}

func TestRegularNGon(t *testing.T) {
	n, err := RegularNGon(6, Point3{X: 0, Y: 0, Z: 0}, 100, Vector3{X: 0, Y: 0, Z: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(n.Vertices) != 6 {
		t.Fatalf("expected 6 vertices, got %d", len(n.Vertices))
	}
}

func TestRegularNGonTooFewSides(t *testing.T) {
	_, err := RegularNGon(2, Point3{}, 100, Vector3{X: 0, Y: 0, Z: 1})
	if err == nil {
		t.Fatal("expected error for too few sides")
	}
}

func TestNGonArea(t *testing.T) {
	// Квадрат 100x100 = 10000
	vertices := []Point3{
		{X: 0, Y: 0, Z: 0},
		{X: 100, Y: 0, Z: 0},
		{X: 100, Y: 100, Z: 0},
		{X: 0, Y: 100, Z: 0},
	}
	n, _ := NewNGon(vertices, Vector3{X: 0, Y: 0, Z: 1})
	area := n.Area()
	if math.Abs(area-10000) > Precision {
		t.Fatalf("expected area 10000, got %v", area)
	}
}

func TestNGonToSolid(t *testing.T) {
	vertices := []Point3{
		{X: 0, Y: 0, Z: 0},
		{X: 100, Y: 0, Z: 0},
		{X: 100, Y: 100, Z: 0},
		{X: 0, Y: 100, Z: 0},
	}
	n, _ := NewNGon(vertices, Vector3{X: 0, Y: 0, Z: 1})
	solid, err := n.ToSolid(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if solid == nil {
		t.Fatal("expected non-nil solid")
	}
}

func TestValidateArc(t *testing.T) {
	arc, _ := NewArc(Point3{}, 100, Vector3{X: 0, Y: 0, Z: 1}, 0, math.Pi)
	issues := ValidateArc(arc)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestValidateArcNil(t *testing.T) {
	issues := ValidateArc(nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}

func TestValidateBezier(t *testing.T) {
	b := &Bezier{
		P0: Point3{X: 0, Y: 0, Z: 0},
		P1: Point3{X: 1, Y: 1, Z: 0},
		P2: Point3{X: 2, Y: 1, Z: 0},
		P3: Point3{X: 3, Y: 0, Z: 0},
	}
	issues := ValidateBezier(b)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestValidateFillet(t *testing.T) {
	f, _ := NewFillet(Point3{}, Point3{X: 100, Y: 0, Z: 0}, 10, Vector3{X: 0, Y: 0, Z: 1})
	issues := ValidateFillet(f)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestValidateChamfer(t *testing.T) {
	c, _ := NewChamfer(Point3{}, Point3{X: 100, Y: 0, Z: 0}, 5, Vector3{X: 0, Y: 0, Z: 1})
	issues := ValidateChamfer(c)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestValidateNGon(t *testing.T) {
	vertices := []Point3{
		{X: 0, Y: 0, Z: 0},
		{X: 100, Y: 0, Z: 0},
		{X: 100, Y: 100, Z: 0},
		{X: 0, Y: 100, Z: 0},
	}
	n, _ := NewNGon(vertices, Vector3{X: 0, Y: 0, Z: 1})
	issues := ValidateNGon(n)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}
