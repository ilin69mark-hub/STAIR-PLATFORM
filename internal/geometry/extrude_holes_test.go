package geometry

import (
	"math"
	"testing"
)

// circleRing2D — круглое отверстие из n точек в плоскости Z=0.
func circleRing2D(cx, cy, r float64, n int) []Point3 {
	out := make([]Point3, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		out[i] = Point3{X: cx + r*math.Cos(a), Y: cy + r*math.Sin(a)}
	}
	return out
}

// slotRing2D — овал длиной length по X и шириной width по Y в плоскости Z=0.
// Полуокружности по краям, прямые участки между ними.
func slotRing2D(cx, cy, length, width float64) []Point3 {
	r := width / 2
	half := length/2 - r
	var out []Point3
	arc := func(ox float64, from, to float64) {
		for i := 0; i <= 6; i++ {
			a := from + (to-from)*float64(i)/6
			out = append(out, Point3{
				X: cx + ox + r*math.Cos(a),
				Y: cy + r*math.Sin(a),
			})
		}
	}
	arc(half, -math.Pi/2, math.Pi/2)
	arc(-half, math.Pi/2, 3*math.Pi/2)
	return out
}

// circleRingXZ — круглое отверстие в плоскости X=const (профиль в XZ).
func circleRingXZ(cx, cz, r float64, n int) []Point3 {
	out := make([]Point3, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		out[i] = Point3{X: cx + r*math.Cos(a), Y: 0, Z: cz + r*math.Sin(a)}
	}
	return out
}

// slotRingXZ — овал длиной length по Z и шириной width по X в плоскости Y=0.
func slotRingXZ(cx, cz, length, width float64) []Point3 {
	r := width / 2
	half := length/2 - r
	var out []Point3
	arc := func(oz float64, from, to float64) {
		for i := 0; i <= 6; i++ {
			a := from + (to-from)*float64(i)/6
			out = append(out, Point3{
				X: cx + r*math.Cos(a),
				Y: 0,
				Z: cz + oz + r*math.Sin(a),
			})
		}
	}
	// Верхняя полуокружность: от левого конца через верх к правому, нижняя —
	// от правого через низ к левому. Тогда первая и последняя точки лежат на
	// одном прямом участке, и кольцо замыкается им.
	arc(half, math.Pi, 0)
	arc(-half, 0, -math.Pi)
	return out
}

// ringArea2D — площадь кольца в плоскости XY.
func ringArea2D(r []Point3) float64 {
	var s float64
	for i := range r {
		j := (i + 1) % len(r)
		s += r[i].X*r[j].Y - r[j].X*r[i].Y
	}
	return math.Abs(s / 2)
}

// ringArea3 — площадь кольца в произвольной плоскости через Newell.
func ringArea3(r []Point3) float64 {
	var n Vector3
	for i := range r {
		j := (i + 1) % len(r)
		n = n.Add(Vector3(r[i]).Cross(Vector3(r[j])))
	}
	return n.Norm() / 2
}

// TestExtrudeWithHolesVolume — плита с отверстиями: объём равен
// (площадь пластины − сумма площадей отверстий) × толщина.
func TestExtrudeWithHolesVolume(t *testing.T) {
	prof := []Point3{
		{X: 0, Y: 0}, {X: 280, Y: 0}, {X: 280, Y: 50}, {X: 0, Y: 50},
	}
	holes := [][]Point3{
		slotRing2D(24, 25, 30, 12),
		circleRing2D(120, 25, 6, 32),
		circleRing2D(180, 25, 6, 32),
		slotRing2D(240, 25, 30, 12),
	}
	const th = 4.0
	solid, err := ExtrudeWithHoles(prof, holes, Vector3{Z: 1}, th)
	if err != nil {
		t.Fatalf("ExtrudeWithHoles: %v", err)
	}
	area := 280 * 50.0
	for _, h := range holes {
		area -= ringArea2D(h)
	}
	want := area * th
	got, err := Volume(solid)
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if math.Abs(got-want) > 1e-3 {
		t.Fatalf("volume = %.4f, want %.4f (разница %.6f)", got, want, got-want)
	}
}

// TestExtrudeWithHolesArea — площадь поверхности: крышки минус отверстия плюс
// стенки отверстий (периметр × толщина).
func TestExtrudeWithHolesArea(t *testing.T) {
	prof := []Point3{
		{X: 0, Y: 0}, {X: 280, Y: 0}, {X: 280, Y: 50}, {X: 0, Y: 50},
	}
	holes := [][]Point3{
		slotRing2D(24, 25, 30, 12),
		circleRing2D(120, 25, 6, 32),
		slotRing2D(240, 25, 30, 12),
	}
	const th = 4.0
	solid, err := ExtrudeWithHoles(prof, holes, Vector3{Z: 1}, th)
	if err != nil {
		t.Fatalf("ExtrudeWithHoles: %v", err)
	}
	holeArea, perim := 0.0, 0.0
	for _, h := range holes {
		holeArea += ringArea2D(h)
		for i := range h {
			j := (i + 1) % len(h)
			perim += h[i].Distance(h[j])
		}
	}
	// две крышки с вычетом отверстий + периметр контура снаружи (боковые
	// грани) + периметры отверстий (стенки)
	want := 2*(280*50.0-holeArea) + 2*(280+50)*th + perim*th
	got, err := SurfaceArea(solid)
	if err != nil {
		t.Fatalf("SurfaceArea: %v", err)
	}
	if math.Abs(got-want) > 1e-2 {
		t.Fatalf("area = %.3f, want %.3f (разница %.6f)", got, want, got-want)
	}
}

// TestExtrudeWithHolesHorizontal — та же плита, но вытянутая по Y:
// отверстия лежат в вертикальной грани.
func TestExtrudeWithHolesHorizontal(t *testing.T) {
	// вертикальная плита: профиль в плоскости XZ, вытягивание по Y
	prof := []Point3{
		{X: 0, Z: 0}, {X: 50, Z: 0}, {X: 50, Z: 280}, {X: 0, Z: 280},
	}
	holes := [][]Point3{
		slotRingXZ(25, 24, 30, 12),
		circleRingXZ(25, 120, 6, 32),
		circleRingXZ(25, 180, 6, 32),
	}
	const th = 4.0
	solid, err := ExtrudeWithHoles(prof, holes, Vector3{Y: 1}, th)
	if err != nil {
		t.Fatalf("ExtrudeWithHoles: %v", err)
	}
	area := 50 * 280.0
	for _, h := range holes {
		area -= ringArea3(h)
	}
	want := area * th
	got, err := Volume(solid)
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if math.Abs(got-want) > 1e-3 {
		t.Fatalf("volume = %.4f, want %.4f", got, want)
	}
}

// TestExtrudeWithHolesInvalid — отверстие вне плоскости профиля отвергается.
func TestExtrudeWithHolesInvalid(t *testing.T) {
	prof := []Point3{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100},
	}
	// отверстие наклонено к плоскости профиля
	hole := []Point3{{X: 10, Y: 10, Z: 0}, {X: 20, Y: 10, Z: 3}, {X: 15, Y: 20, Z: 6}}
	if _, err := ExtrudeWithHoles(prof, [][]Point3{hole}, Vector3{Z: 1}, 4); err == nil {
		t.Fatal("ожидалась ошибка на отверстии не в плоскости профиля")
	}
}

// TestFaceWithHolesMesh — меш грани с отверстиями строится и не пустой.
func TestFaceWithHolesMesh(t *testing.T) {
	solid, err := ExtrudeWithHoles(
		[]Point3{{X: 0, Y: 0}, {X: 200, Y: 0}, {X: 200, Y: 50}, {X: 0, Y: 50}},
		[][]Point3{circleRing2D(100, 25, 6, 24)}, Vector3{Z: 1}, 4)
	if err != nil {
		t.Fatalf("ExtrudeWithHoles: %v", err)
	}
	cache := NewTessellationCache()
	face := solid.Shells()[0].Faces()[0]
	tess := cache.FaceOf(face)
	if tess.TrisErr != nil {
		t.Fatalf("FaceOf: %v", tess.TrisErr)
	}
	if len(tess.Tris) == 0 {
		t.Fatal("нет треугольников")
	}
	sum := 0.0
	for _, tr := range tess.Tris {
		sum += triangleArea(tess.Points[tr[0]], tess.Points[tr[1]], tess.Points[tr[2]])
	}
	want := 200*50.0 - ringArea2D(circleRing2D(100, 25, 6, 24))
	if math.Abs(sum-want) > 1e-6 {
		t.Fatalf("площадь грани = %.4f, want %.4f", sum, want)
	}
}
