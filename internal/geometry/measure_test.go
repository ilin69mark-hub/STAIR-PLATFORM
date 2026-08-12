package geometry

import (
	"reflect"
	"testing"
)

func boxSolid() *Solid {
	profile := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(10, 0, 0),
		NewPoint3(10, 10, 0),
		NewPoint3(0, 10, 0),
	}
	solid, err := Extrude(profile, NewVector3(0, 0, 1), 5)
	if err != nil {
		panic(err)
	}
	return solid
}

// stringerLikeSolid — пилообразный профиль (n=2, heel=50) в плоскости XZ,
// вытянутый по Y на 50: объём = площадь профиля × 50.
func stringerLikeSolid() *Solid {
	n := 2
	b, h := 270.0, 180.0
	pts := []Point3{NewPoint3(0, 0, 0)}
	for k := 0; k < n; k++ {
		pts = append(pts,
			NewPoint3(float64(k)*b, 0, float64(k+1)*h),
			NewPoint3(float64(k+1)*b, 0, float64(k+1)*h),
		)
	}
	pts = append(pts,
		NewPoint3(float64(n)*b, 0, float64(n)*h-50),
		NewPoint3(0, 0, -50),
	)
	solid, err := Extrude(pts, NewVector3(0, 1, 0), 50)
	if err != nil {
		panic(err)
	}
	return solid
}

func TestVolumeBox(t *testing.T) {
	vol, err := Volume(boxSolid())
	if err != nil {
		t.Fatal(err)
	}
	if !nearlyEqual(vol, 500) {
		t.Fatalf("volume = %v, want 500", vol)
	}
}

func TestVolumeStringer(t *testing.T) {
	vol, err := Volume(stringerLikeSolid())
	if err != nil {
		t.Fatal(err)
	}
	// площадь пилы n=2 с heel=50: 2 × 270 × 140 = 75600 (см. triangulate_test).
	if !nearlyEqual(vol, 75600*50) {
		t.Fatalf("volume = %v, want %v", vol, 75600*50)
	}
}

func TestVolumeErrors(t *testing.T) {
	if _, err := Volume(NewSolid()); err == nil {
		t.Fatal("solid without shells must error")
	}
}

func TestSurfaceAreaBox(t *testing.T) {
	area, err := SurfaceArea(boxSolid())
	if err != nil {
		t.Fatal(err)
	}
	// 2*(10*10) + 4*(10*5) = 400.
	if !nearlyEqual(area, 400) {
		t.Fatalf("area = %v, want 400", area)
	}
}

func TestSurfaceAreaStringer(t *testing.T) {
	area, err := SurfaceArea(stringerLikeSolid())
	if err != nil {
		t.Fatal(err)
	}
	if area <= 0 {
		t.Fatalf("area = %v, want positive", area)
	}
}

func TestSolidCount(t *testing.T) {
	if got := SolidCount(nil); got != 0 {
		t.Fatalf("nil model: count = %d, want 0", got)
	}
	if got := SolidCount(NewCompound()); got != 0 {
		t.Fatalf("empty compound: count = %d, want 0", got)
	}
	model := NewCompound(boxSolid(), stringerLikeSolid())
	if got := SolidCount(model); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
}

func TestBoundingBox(t *testing.T) {
	if !BoundingBox(nil).IsEmpty() {
		t.Fatal("nil model must give empty bbox")
	}
	model := NewCompound(boxSolid())
	bb := BoundingBox(model)
	if !nearlyEqual(bb.Min.X, 0) || !nearlyEqual(bb.Min.Y, 0) || !nearlyEqual(bb.Min.Z, 0) {
		t.Fatalf("min = %+v, want origin", bb.Min)
	}
	if !nearlyEqual(bb.Max.X, 10) || !nearlyEqual(bb.Max.Y, 10) || !nearlyEqual(bb.Max.Z, 5) {
		t.Fatalf("max = %+v, want (10,10,5)", bb.Max)
	}
	if !bb.Contains(NewPoint3(5, 5, 2.5)) {
		t.Fatal("center must be inside bbox")
	}
}

func TestMeasureDeterminism(t *testing.T) {
	a := stringerLikeSolid()
	b := stringerLikeSolid()
	va, _ := Volume(a)
	vb, _ := Volume(b)
	if va != vb {
		t.Fatalf("volumes differ: %v vs %v", va, vb)
	}
	sa, _ := SurfaceArea(a)
	sb, _ := SurfaceArea(b)
	if sa != sb {
		t.Fatalf("areas differ: %v vs %v", sa, sb)
	}
	// устойчивость к порядку граней проверяем через DeepEqual модели.
	if !reflect.DeepEqual(a, b) {
		t.Fatal("identical inputs must produce identical solids")
	}
}
