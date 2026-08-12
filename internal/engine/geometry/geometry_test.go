package geometry

import (
	"math"
	"reflect"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

func mustLength(t *testing.T, mm float64) engineering.Length {
	t.Helper()
	l, err := engineering.NewLength(mm)
	if err != nil {
		t.Fatalf("NewLength(%v): %v", mm, err)
	}
	return l
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6
}

// testConfig возвращает валидную конфигурацию прямого марша (H=2700,
// n=15, h=180, b=270, W=900, T=50, st=40).
func testConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.StepThickness = mustLength(t, 40)
	return cfg
}

func TestBuildStraightFlightCount(t *testing.T) {
	model, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// 2 косоура + 15 проступей + 15 подступенков = 32 тела.
	if len(model.Solids()) != 32 {
		t.Fatalf("solids = %d, want 32", len(model.Solids()))
	}
}

func TestBuildStraightFlightNoStepsWhenZeroThickness(t *testing.T) {
	cfg := testConfig(t)
	cfg.StepThickness = mustLength(t, 0)
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// только 2 косоура.
	if len(model.Solids()) != 2 {
		t.Fatalf("solids = %d, want 2", len(model.Solids()))
	}
}

func TestBuildStraightFlightStringerBounds(t *testing.T) {
	model, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// левый косоур y∈[0,50], правый y∈[850,900] (W=900, T=50).
	var leftMinY, leftMaxY, rightMinY, rightMaxY float64
	leftMinY, rightMinY = math.Inf(1), math.Inf(1)
	leftMaxY, rightMaxY = math.Inf(-1), math.Inf(-1)
	for _, solid := range model.Solids() {
		minY, maxY := solidYBounds(solid)
		if maxY <= 50+1e-6 {
			leftMinY = math.Min(leftMinY, minY)
			leftMaxY = math.Max(leftMaxY, maxY)
		}
		if minY >= 850-1e-6 {
			rightMinY = math.Min(rightMinY, minY)
			rightMaxY = math.Max(rightMaxY, maxY)
		}
	}
	if !nearlyEqual(leftMinY, 0) || !nearlyEqual(leftMaxY, 50) {
		t.Fatalf("left stringer y-bounds = [%v,%v], want [0,50]", leftMinY, leftMaxY)
	}
	if !nearlyEqual(rightMinY, 850) || !nearlyEqual(rightMaxY, 900) {
		t.Fatalf("right stringer y-bounds = [%v,%v], want [850,900]", rightMinY, rightMaxY)
	}
}

func TestBuildStraightFlightStepPositions(t *testing.T) {
	model, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// первая проступь: z∈[180,220], x∈[0,270], y∈[50,850].
	if !pointExists(model, kerngeo.NewPoint3(270, 50, 220)) {
		t.Fatal("first tread top corner (270,50,220) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(0, 850, 220)) {
		t.Fatal("first tread top corner (0,850,220) must exist")
	}
	// первый подступенок: x∈[0,40], z∈[0,180].
	if !pointExists(model, kerngeo.NewPoint3(0, 50, 0)) {
		t.Fatal("first riser corner (0,50,0) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(40, 850, 180)) {
		t.Fatal("first riser corner (40,850,180) must exist")
	}
	// последняя проступь: z∈[2700,2740], x∈[3780,4050].
	if !pointExists(model, kerngeo.NewPoint3(4050, 850, 2740)) {
		t.Fatal("last tread top corner (4050,850,2740) must exist")
	}
}

func TestBuildStraightFlightErrors(t *testing.T) {
	cfg := testConfig(t)

	cfg.StepCount = 0
	if _, err := BuildStraightFlight(cfg); err == nil {
		t.Fatal("step count 0 must be rejected")
	}
	cfg = testConfig(t)
	cfg.StepHeight = mustLength(t, 0)
	if _, err := BuildStraightFlight(cfg); err == nil {
		t.Fatal("zero step height must be rejected")
	}
	cfg = testConfig(t)
	cfg.TreadDepth = mustLength(t, 0)
	if _, err := BuildStraightFlight(cfg); err == nil {
		t.Fatal("zero tread depth must be rejected")
	}
	cfg = testConfig(t)
	cfg.Width = mustLength(t, 50)
	cfg.StringerThickness = mustLength(t, 50)
	if _, err := BuildStraightFlight(cfg); err == nil {
		t.Fatal("width not exceeding two stringer thicknesses must be rejected")
	}
	if _, err := BuildStraightFlight(nil); err == nil {
		t.Fatal("nil config must be rejected")
	}
}

func TestBuildStraightFlightDeterminism(t *testing.T) {
	a, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("model construction must be deterministic")
	}
}

func TestToPreviewMesh(t *testing.T) {
	model, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	mesh, err := ToPreviewMesh(model)
	if err != nil {
		t.Fatal(err)
	}
	if len(mesh.Vertices) == 0 || len(mesh.Triangles) == 0 {
		t.Fatal("mesh must not be empty")
	}
	if want := expectedTriangleCount(model); len(mesh.Triangles) != want {
		t.Fatalf("triangles = %d, want %d", len(mesh.Triangles), want)
	}
	// все индексы треугольников корректны.
	for _, tr := range mesh.Triangles {
		for _, v := range tr {
			if v < 0 || v >= len(mesh.Vertices) {
				t.Fatalf("triangle %v references vertex %d out of range", tr, v)
			}
		}
	}
}

func TestToPreviewMeshDeterminism(t *testing.T) {
	model, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	a, err := ToPreviewMesh(model)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ToPreviewMesh(model)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("mesh construction must be deterministic")
	}
}

func TestToPreviewMeshNil(t *testing.T) {
	if _, err := ToPreviewMesh(nil); err == nil {
		t.Fatal("nil model must be rejected")
	}
}

// solidYBounds возвращает min/max координаты Y всех вершин тела.
func solidYBounds(s *kerngeo.Solid) (float64, float64) {
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, shell := range s.Shells() {
		for _, face := range shell.Faces() {
			for _, e := range face.Outer().Edges() {
				v1, v2 := e.Endpoints()
				for _, v := range []*kerngeo.Vertex{v1, v2} {
					y := v.Point().Y
					if y < minY {
						minY = y
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
	}
	return minY, maxY
}

// pointExists проверяет наличие вершины с точными координатами в модели.
func pointExists(model *kerngeo.Compound, want kerngeo.Point3) bool {
	for _, solid := range model.Solids() {
		for _, shell := range solid.Shells() {
			for _, face := range shell.Faces() {
				for _, e := range face.Outer().Edges() {
					v1, v2 := e.Endpoints()
					for _, v := range []*kerngeo.Vertex{v1, v2} {
						p := v.Point()
						if p == want {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// expectedTriangleCount возвращает ожидаемое число треугольников сетки:
// каждая грань с k вершинами даёт k-2 треугольников (ENG-GEO-0008).
func expectedTriangleCount(model *kerngeo.Compound) int {
	count := 0
	for _, solid := range model.Solids() {
		for _, shell := range solid.Shells() {
			for _, face := range shell.Faces() {
				count += len(face.Outer().Edges()) - 2
			}
		}
	}
	return count
}
