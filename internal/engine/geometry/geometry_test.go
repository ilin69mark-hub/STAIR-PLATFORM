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
	// 2 косоура + 15 проступей + 15 подступенков (одно полотно на ступень) = 32 тела.
	if len(model.Solids()) != 32 {
		t.Fatalf("solids = %d, want 32", len(model.Solids()))
	}
}

func TestBuildStraightFlightOpenRiser(t *testing.T) {
	cfg := testConfig(t)
	cfg.Riser = false
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// 2 косоура + 15 проступей = 17 тел; подступенки не строятся.
	if len(model.Solids()) != 17 {
		t.Fatalf("solids = %d, want 17", len(model.Solids()))
	}
	for _, s := range model.Solids() {
		if s.Role() == "riser" {
			t.Fatal("open-riser model must not contain riser solids")
		}
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
	// Косоуры внутри ширины (W=900, T=50): полосы y∈[200,250] и y∈[650,700]
	// (центры на w/4 и 3w/4); проступи и подступенки — во всю ширину [0,900].
	var mins, maxs []float64
	for _, solid := range model.Solids() {
		if solid.Role() != "stringer" {
			continue
		}
		minY, maxY := solidYBounds(solid)
		mins = append(mins, minY)
		maxs = append(maxs, maxY)
	}
	if len(mins) != 2 {
		t.Fatalf("stringer solids = %d, want 2", len(mins))
	}
	for _, m := range mins {
		if !nearlyEqual(m, 200) && !nearlyEqual(m, 650) {
			t.Fatalf("stringer min y-bounds = %v, want 200 or 650", mins)
		}
	}
	for _, m := range maxs {
		if !nearlyEqual(m, 250) && !nearlyEqual(m, 700) {
			t.Fatalf("stringer max y-bounds = %v, want 250 or 700", maxs)
		}
	}
}

func TestBuildStraightFlightStepPositions(t *testing.T) {
	model, err := BuildStraightFlight(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// первая проступь: z∈[140,180] (верх на носике, низ на седле косоура),
	// x∈[−40,270] (глубина шага + толщина подступенка, во всю ширину, на
	// сёдлах внутренних косоуров).
	if !pointExists(model, kerngeo.NewPoint3(270, 0, 180)) {
		t.Fatal("first tread top corner (270,0,180) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(-40, 900, 180)) {
		t.Fatal("first tread top corner (-40,900,180) must exist")
	}
	// первый подступенок (лицевая панель): x∈[−40,0], z∈[0,140] — стоит на
	// полу, спиной (x=0) к передней грани косоура, вплотную к свесу первой
	// проступи. Одно полотно во всю ширину y∈[0,900].
	if !pointExists(model, kerngeo.NewPoint3(-40, 0, 0)) {
		t.Fatal("first riser front-bottom corner (−40,0,0) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(-40, 900, 140)) {
		t.Fatal("first riser front-top corner (−40,900,140) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(0, 200, 0)) {
		t.Fatal("first riser back corner (0,200,0) must exist")
	}
	// второй подступенок (k=1) выдвинут: x∈[230,270], спина (270) — на
	// сбросе гребёнки, верх z=320 — под второй проступью.
	if !pointExists(model, kerngeo.NewPoint3(230, 0, 180)) {
		t.Fatal("second riser front corner (230,0,180) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(270, 900, 320)) {
		t.Fatal("second riser back corner (270,900,320) must exist")
	}
	// последняя проступь: z∈[2660,2700], x∈[3780,4050].
	if !pointExists(model, kerngeo.NewPoint3(4050, 900, 2700)) {
		t.Fatal("last tread top corner (4050,900,2700) must exist")
	}
}

func TestBuildStraightFlightRiserContract(t *testing.T) {
	cfg := testConfig(t)
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	byStep := make(map[int][]kerngeo.BBox)
	for _, solid := range model.Solids() {
		if solid.Role() != "riser" {
			continue
		}
		box := kerngeo.SolidBoundingBox(solid)
		byStep[int(math.Round(box.Max.X/b))] = append(byStep[int(math.Round(box.Max.X/b))], box)
	}
	// Для каждого шага k подступенок — одно полотно во всю ширину, выдвинутое
	// на толщину: охват x∈[k·b−st, k·b] (k=0 — лицевая панель [−st, 0] на
	// полу, перед передней гранью косоура), спина (x=k·b) на сбросе косоура.
	// По Z: низ — на нижележащей ступени (k·h), верх — впритык к низу
	// вышележащей проступи ((k+1)·h − st).
	for k := 0; k < cfg.StepCount; k++ {
		strips := byStep[k]
		if len(strips) != 1 {
			t.Fatalf("step %d risers = %d, want 1", k, len(strips))
		}
		minX := float64(k)*b - st
		maxX := float64(k) * b
		for _, box := range strips {
			if !nearlyEqual(box.Min.X, minX) {
				t.Fatalf("riser %d min x = %v, want %v (shifted forward by st)", k, box.Min.X, minX)
			}
			if !nearlyEqual(box.Max.X, maxX) {
				t.Fatalf("riser %d max x = %v, want %v (back on stringer drop)", k, box.Max.X, maxX)
			}
			if want := float64(k) * h; !nearlyEqual(box.Min.Z, want) {
				t.Fatalf("riser %d min z = %v, want %v (top of lower step)", k, box.Min.Z, want)
			}
			if want := float64(k+1)*h - st; !nearlyEqual(box.Max.Z, want) {
				t.Fatalf("riser %d max z = %v, want %v (bottom of upper tread)", k, box.Max.Z, want)
			}
		}
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
