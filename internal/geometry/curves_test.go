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

func u3(x, y, z float64) Vector3 { return Vector3{X: x, Y: y, Z: z} }

// Регрессия: Fillet.Tessellate игнорировал переданный Radius (брал половину
// длины ребра) и строил дугу в плоскости, ПЕРПЕНДИКУЛЯРНОЙ ребру. Тест раньше
// проверял только количество точек, поэтому дефект и жил. Теперь проверяем
// геометрию: центр дуги, радиус и то, что дуга касается ребра.
func TestFilletTessellateUsesGivenRadius(t *testing.T) {
	f, err := NewFillet(
		Point3{X: 0, Y: 0, Z: 0},
		Point3{X: 100, Y: 0, Z: 0},
		10,
		Vector3{X: 0, Y: 0, Z: 1},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	points := f.Tessellate(10)
	if len(points) != 11 {
		t.Fatalf("expected 11 points, got %d", len(points))
	}

	// Центр: середина ребра (50,0,0), смещённая на радиус по нормали.
	center := Point3{X: 50, Y: 0, Z: 10}
	for i, p := range points {
		if d := p.Distance(center); math.Abs(d-10) > 1e-6 {
			t.Fatalf("point %d: distance to center = %v, want 10", i, d)
		}
	}
	// Концы дуги — на ребре (z=0) и на его зеркале (z=20): полуокружность
	// радиусом 10, а не 50 (как было раньше).
	if z := points[0].Z; math.Abs(z) > 1e-6 {
		t.Fatalf("arc start should touch the edge (z=0), got z=%v", z)
	}
	if z := points[len(points)-1].Z; math.Abs(z-20) > 1e-6 {
		t.Fatalf("arc end should mirror across the edge (z=20), got z=%v", z)
	}
	// Дуга лежит в плоскости, содержащей ребро: все точки имеют y=0.
	for i, p := range points {
		if math.Abs(p.Y) > 1e-6 {
			t.Fatalf("point %d: y = %v, arc plane must contain the edge (y=0)", i, p.Y)
		}
	}
}

func TestFilletTessellateRadiusTooBigFallsBackToEdge(t *testing.T) {
	// Полуокружность радиусом 60 не помещается на ребро длиной 100.
	f, _ := NewFillet(
		Point3{X: 0, Y: 0, Z: 0},
		Point3{X: 100, Y: 0, Z: 0},
		60,
		Vector3{X: 0, Y: 0, Z: 1},
	)
	points := f.Tessellate(10)
	if len(points) != 2 || points[0].X != 0 || points[1].X != 100 {
		t.Fatalf("expected the bare edge as fallback, got %v", points)
	}
}

// RoundCorner — скругление угла профиля (нос проступи). Проверяем геометрию:
// все точки на заданном расстоянии от центра, дуга касается обоих рёбер, и
// профиль с дугой короче ломаной.
func TestRoundCornerGeometry(t *testing.T) {
	// Угол 90°: лучи из угла в минус-X и в плюс-Z.
	corner := Point3{X: 0, Y: 0, Z: 0}
	prev := Point3{X: -50, Y: 0, Z: 0}
	next := Point3{X: 0, Y: 0, Z: 50}
	const radius = 10.0

	arc, err := RoundCorner(prev, corner, next, radius, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Концы не дублируются: точек на сегмент минус один.
	if len(arc) != 7 {
		t.Fatalf("expected 7 interior points, got %d", len(arc))
	}

	// Для угла 90° центр дуги — на биссектрисе на расстоянии r/sin(45°).
	centerDist := radius / math.Sin(math.Pi/4)
	center := Point3{X: -centerDist * math.Cos(math.Pi/4), Y: 0, Z: centerDist * math.Sin(math.Pi/4)}
	for i, p := range arc {
		if d := p.Distance(center); math.Abs(d-radius) > 1e-6 {
			t.Fatalf("point %d: distance to center = %v, want %v", i, d, radius)
		}
	}

	// Касание доказываем точками касания: они лежат на окружности (значит дуга
	// к ним касается), лежат на своих рёбрах и на расстоянии r/tan(α/2) от угла.
	// Сами точки касания функция не возвращает — они остаются в профиле.
	tangent := radius / math.Tan(math.Pi/4)
	t1 := corner.Add(u3(-1, 0, 0).Scale(tangent))
	t2 := corner.Add(u3(0, 0, 1).Scale(tangent))
	for i, tp := range []Point3{t1, t2} {
		if d := tp.Distance(center); math.Abs(d-radius) > 1e-6 {
			t.Fatalf("tangent point %d must lie on the arc: distance = %v, want %v", i, d, radius)
		}
	}

	// Дуга должна снимать материал: её точки лежат ВНУТРИ острого угла
	// (треугольника prev-corner-next), то есть профиль становится короче.
	for i, p := range arc {
		inside := p.X < 0+1e-9 && p.Z > 0-1e-9 && (p.Z-p.X)/50 < 1-1e-9
		if !inside {
			t.Fatalf("arc point %d %v is outside the sharp corner: rounding must cut material", i, p)
		}
	}
	// Путь по дуге короче пути через острый угол: дуга касается рёбер там, где
	// угол срезан. Для угла 90° это четверть окружности πR/2 ≈ 15.71 против
	// двух касательных по 10 (в сумме 20).
	arcChain := append([]Point3{t1}, arc...)
	arcChain = append(arcChain, t2)
	var arcLen float64
	for i := 1; i < len(arcChain); i++ {
		arcLen += arcChain[i-1].Distance(arcChain[i])
	}
	sharp := t1.Distance(corner) + corner.Distance(t2)
	if arcLen >= sharp {
		t.Fatalf("rounded path (%v) must be shorter than the sharp corner (%v)", arcLen, sharp)
	}
	// И сходится к четверти окружности с точностью до огрубления сегментов.
	if math.Abs(arcLen-radius*math.Pi/2) > radius*0.1 {
		t.Fatalf("rounded path = %v, want about a quarter circle (%v)", arcLen, radius*math.Pi/2)
	}
}

func TestRoundCornerRejectsRadiusThatDoesNotFit(t *testing.T) {
	corner := Point3{}
	prev := Point3{X: -5, Y: 0, Z: 0} // короткое ребро
	next := Point3{X: 0, Y: 0, Z: 50} // длинное
	if _, err := RoundCorner(prev, corner, next, 10, 8); err == nil {
		t.Fatal("expected error: radius 10 does not fit an edge of length 5")
	}
}

func TestRoundCornerRejectsStraightAngle(t *testing.T) {
	corner := Point3{}
	// Коллинеарные лучи — угла нет, скруглять нечего.
	if _, err := RoundCorner(Point3{X: -50}, corner, Point3{X: -100}, 10, 8); err == nil {
		t.Fatal("expected error for collinear edges")
	}
}

// Регрессия: NGon.Area считал площадь в XY, поэтому для профиля в плоскости XZ
// (вертикальная грань, наклонный косоур) возвращал произвольное число.
func TestNGonAreaInProfilePlane(t *testing.T) {
	// Квадрат 100×50 в плоскости XZ, нормаль (0,-1,0).
	n, err := NewNGon(
		[]Point3{
			{X: 0, Y: 0, Z: 0},
			{X: 100, Y: 0, Z: 0},
			{X: 100, Y: 0, Z: 50},
			{X: 0, Y: 0, Z: 50},
		},
		Vector3{X: 0, Y: -1, Z: 0},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if area := n.Area(); math.Abs(area-5000) > 1e-6 {
		t.Fatalf("area = %v, want 5000", area)
	}

	// Тот же квадрат в XY — контроль, что базис не сломан.
	flat, _ := NewNGon(
		[]Point3{
			{X: 0, Y: 0, Z: 0},
			{X: 100, Y: 0, Z: 0},
			{X: 100, Y: 50, Z: 0},
			{X: 0, Y: 50, Z: 0},
		},
		Vector3{X: 0, Y: 0, Z: 1},
	)
	if area := flat.Area(); math.Abs(area-5000) > 1e-6 {
		t.Fatalf("area = %v, want 5000", area)
	}
}
