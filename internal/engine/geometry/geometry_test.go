package geometry

import (
	"context"
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

// --- Скругление носа ступени (фаска-филёнка) ------------------------------
//
// Фаска — не только вид: скругление снимает материал, поэтому объём детали
// обязан уменьшиться, и из этого же объёма потом считается масса и цена.
// Габариты при этом НЕ меняются: скругление идёт по уже существующей толщине
// проступи, покупатель платит за обработку, а не за лишний миллиметр.

// withNose копирует конфиг с заданным радиусом скругления носа.
func withNose(t *testing.T, cfg *engineering.StairConfiguration, r float64) *engineering.StairConfiguration {
	t.Helper()
	cp := *cfg
	cp.TreadNoseRadiusMM = engineering.Length(r)
	return &cp
}

// firstTreadIndex — индекс первой проступи в модели прямого марша: тела
// строятся слотами «косоуры → проступи → подступенки», косоуров всегда два.
func firstTreadIndex() int { return 2 }

func buildFlight(t *testing.T, cfg *engineering.StairConfiguration) *kerngeo.Compound {
	t.Helper()
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func modelVolume(t *testing.T, model *kerngeo.Compound) float64 {
	t.Helper()
	total := 0.0
	for _, solid := range model.Solids() {
		v, err := kerngeo.Volume(solid)
		if err != nil {
			t.Fatalf("volume of solid: %v", err)
		}
		total += v
	}
	return total
}

func TestTreadNoseRoundingReducesVolume(t *testing.T) {
	cfg := testConfig(t)
	vSharp := modelVolume(t, buildFlight(t, cfg))
	vRounded := modelVolume(t, buildFlight(t, withNose(t, cfg, 8)))

	if vRounded >= vSharp {
		t.Fatalf("rounded volume %v must be less than sharp %v", vRounded, vSharp)
	}
	// Снятый объём — 15 проступей × (R² − πR²/4) × ширина марша. Для R=8,
	// w=900, n=15 это ≈ 145 000 мм³: проверяем порядок, чтобы «скругление» не
	// съело полпроступи и не оказалось пустым.
	removed := vSharp - vRounded
	wantApprox := 15 * (64 - math.Pi*64/4) * 900
	if math.Abs(removed-wantApprox)/wantApprox > 0.05 {
		t.Fatalf("removed %v mm³, want about %v mm³ (5%% tolerance)", removed, wantApprox)
	}
}

func TestTreadNoseRoundingKeepsBoundingBox(t *testing.T) {
	cfg := testConfig(t)
	bs := kerngeo.BoundingBox(buildFlight(t, cfg))
	br := kerngeo.BoundingBox(buildFlight(t, withNose(t, cfg, 8)))
	if bs.Min.Sub(br.Min).Norm() > 1e-6 || bs.Max.Sub(br.Max).Norm() > 1e-6 {
		t.Fatalf("bounding box must not change: %v/%v vs %v/%v",
			bs.Min, bs.Max, br.Min, br.Max)
	}
}

func TestTreadNoseRoundingIsManifoldAndValid(t *testing.T) {
	model := buildFlight(t, withNose(t, testConfig(t), 8))
	// Скруглённая проступь обязана быть корректным телом: иначе
	// manufacturing/engine.go откажется считать («invalid geometry») и фаска
	// не просто не покажется, а сломает весь расчёт.
	for _, is := range kerngeo.Validate(model.Solids()[firstTreadIndex()]) {
		if is.Severity == kerngeo.SeverityError {
			t.Fatalf("chamfered tread is invalid: %s: %s", is.Code, is.Message)
		}
	}
}

func TestTreadNoseRadiusClampedToThickness(t *testing.T) {
	// Радиус больше толщины ступени — нос не поместился бы в деталь.
	cfg := testConfig(t)
	cfg.StepThickness = mustLength(t, 20)
	model := buildFlight(t, withNose(t, cfg, 40))
	for _, is := range kerngeo.Validate(model.Solids()[firstTreadIndex()]) {
		if is.Severity == kerngeo.SeverityError {
			t.Fatalf("radius above thickness must be clamped, got invalid solid: %s: %s", is.Code, is.Message)
		}
	}
}

func TestTreadWithoutNoseRadiusIsUnchanged(t *testing.T) {
	// Радиус 0 (металл) — тело должно совпасть с прежним: фаска не должна
	// менять геометрию там, где её не просят.
	cfg := testConfig(t)
	sharp := buildFlight(t, cfg)
	zero := buildFlight(t, withNose(t, cfg, 0))
	if len(sharp.Solids()) != len(zero.Solids()) {
		t.Fatalf("solid count changed: %d vs %d", len(sharp.Solids()), len(zero.Solids()))
	}
	for i := range sharp.Solids() {
		v1, _ := kerngeo.Volume(sharp.Solids()[i])
		v2, _ := kerngeo.Volume(zero.Solids()[i])
		if math.Abs(v1-v2) > 1e-6 {
			t.Fatalf("solid %d volume changed with zero radius: %v vs %v", i, v1, v2)
		}
	}
}

// --- MillingFeatures: детали под фрезеровку --------------------------------

func TestMillingFeaturesCountTreadsFromModel(t *testing.T) {
	cfg := withNose(t, testConfig(t), 8)
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.MillingFeatures) != 1 {
		t.Fatalf("features = %d, want 1 (treads)", len(res.MillingFeatures))
	}
	f := res.MillingFeatures[0]
	if f.Role != "tread" {
		t.Fatalf("role = %q, want tread", f.Role)
	}
	// 15 ступеней из testConfig — по числу тел роли "tread" в модели, а не по
	// StepCount: для L/П-маршей это разные величины.
	if f.Quantity != 15 {
		t.Fatalf("quantity = %d, want 15", f.Quantity)
	}
	// Длина ребра — ширина марша (900 мм из testConfig).
	if math.Abs(f.EdgeLengthMM-900) > 1e-6 {
		t.Fatalf("edge length = %v, want flight width 900", f.EdgeLengthMM)
	}
	if math.Abs(f.RadiusMM-8) > 1e-6 {
		t.Fatalf("radius = %v, want 8", f.RadiusMM)
	}
}

func TestNoMillingFeaturesWithoutNoseRadius(t *testing.T) {
	// Металл: ни фаски, ни фрезеровки.
	res, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.MillingFeatures) != 0 {
		t.Fatalf("features = %v, want none without a nose radius", res.MillingFeatures)
	}
}

func TestMillingFeaturesCountLShapeBothSegments(t *testing.T) {
	// L-марш: ступени набираются из двух сегментов, и количество берётся из
	// модели. Проверяем, что оно равно числу тел "tread", а не StepCount.
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightLShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.LowerStepCount = 6
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.StepThickness = mustLength(t, 40)
	cfg.LandingWidth = mustLength(t, 900)
	cfg.Riser = true
	cfg.TreadNoseRadiusMM = engineering.Length(8)

	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	// L-марш: две детали под фрезеровку — ступени обеих сегментов и площадка.
	if len(res.MillingFeatures) != 2 {
		t.Fatalf("features = %d, want 2 (treads + landing)", len(res.MillingFeatures))
	}
	byRole := map[string]MillingFeature{}
	for _, f := range res.MillingFeatures {
		byRole[f.Role] = f
	}
	if _, ok := byRole["landing"]; !ok {
		t.Fatalf("L-shaped flight must mill its landing: %+v", res.MillingFeatures)
	}
	treads := 0
	for _, s := range res.Model.Solids() {
		if s.Role() == "tread" {
			treads++
		}
	}
	if got := byRole["tread"].Quantity; got != treads {
		t.Fatalf("quantity = %d, want %d (tread solids in the model)", got, treads)
	}
	if treads == 0 {
		t.Fatal("L-shaped flight must have treads")
	}
}

// --- Фаска на площадке --------------------------------------------------------
//
// Площадка — та же плита, что и проступь, и скругляется тем же радиусом из
// материала ступеней. Кромка, которой примыкает верхний марш, не трогается:
// она не видна и не фрезеруется.

func lShapeConfig(t *testing.T, left bool) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightLShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.LowerStepCount = 6
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.StepThickness = mustLength(t, 40)
	cfg.LandingWidth = mustLength(t, 900)
	cfg.Riser = true
	cfg.TreadNoseRadiusMM = engineering.Length(8)
	if left {
		cfg.Direction = engineering.TurnLeft
	}
	return cfg
}

func TestLandingIsChamferedOnFreeEdge(t *testing.T) {
	for _, left := range []bool{false, true} {
		name := "правый поворот"
		if left {
			name = "левый поворот"
		}
		cfg := lShapeConfig(t, left)
		res, err := Generate(context.Background(), cfg)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var landingTris int
		for _, pr := range res.Mesh.PartRanges {
			if pr.Role == "landing" {
				landingTris = pr.End - pr.Start
			}
		}
		// Прямоугольная площадка — 12 треугольников (4 точки сечения: 2 крышки
		// по 2 + 4 боковые грани по 2). Скругление добавляет 11 точек дуги.
		if landingTris <= 12 {
			t.Fatalf("%s: landing has %d triangles — the chamfer is missing", name, landingTris)
		}
		// Площадка с фаской должна быть корректным телом, иначе
		// manufacturing отвергает весь расчёт.
		for _, is := range res.Issues {
			if is.Severity == kerngeo.SeverityError {
				t.Fatalf("%s: %s: %s", name, is.Code, is.Message)
			}
		}
	}
}

func TestLandingChamferReducesVolumeNotBoundingBox(t *testing.T) {
	cfg := lShapeConfig(t, false)
	withChamfer, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	plain := *cfg
	plain.TreadNoseRadiusMM = 0
	without, err := Generate(context.Background(), &plain)
	if err != nil {
		t.Fatal(err)
	}
	if withChamfer.Measurement.Volume >= without.Measurement.Volume {
		t.Fatalf("chamfered volume %v must be less than %v",
			withChamfer.Measurement.Volume, without.Measurement.Volume)
	}
	b1 := withChamfer.Measurement.BoundingBox
	b2 := without.Measurement.BoundingBox
	if b1.Min.Sub(b2.Min).Norm() > 1e-6 || b1.Max.Sub(b2.Max).Norm() > 1e-6 {
		t.Fatal("chamfer must not change the bounding box")
	}
}

func TestSteelLandingHasNoChamfer(t *testing.T) {
	cfg := lShapeConfig(t, false)
	cfg.TreadNoseRadiusMM = 0
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, pr := range res.Mesh.PartRanges {
		if pr.Role == "landing" {
			if got := pr.End - pr.Start; got != 12 {
				t.Fatalf("square landing has %d triangles, want 12 (plain rectangle)", got)
			}
		}
	}
	if len(res.MillingFeatures) != 0 {
		t.Fatalf("no chamfer means no milling, got %+v", res.MillingFeatures)
	}
}

// --- Текстурные координаты ---------------------------------------------------
//
// Без UV все вершины получают uv=(0,0): текстура семплит один пиксель, и
// материал выглядит плоским цветом, сколько бы карт ни грузилось. Поэтому
// наличие и корректность UV — контракт, а не украшение.

func TestPreviewMeshHasUVForEveryVertex(t *testing.T) {
	res, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Mesh.UV) != len(res.Mesh.Vertices) {
		t.Fatalf("uv = %d, want one per vertex (%d)", len(res.Mesh.UV), len(res.Mesh.Vertices))
	}
	// Координаты в метрах, то есть для лестницы высотой 2700 мм они порядка
	// единиц-десятков, а не 0..1: иначе рисунок растягивался бы на деталь.
	maxAbs := 0.0
	for _, uv := range res.Mesh.UV {
		if math.Abs(uv.U) > maxAbs {
			maxAbs = math.Abs(uv.U)
		}
		if math.Abs(uv.V) > maxAbs {
			maxAbs = math.Abs(uv.V)
		}
	}
	if maxAbs < 1 {
		t.Fatalf("uv look normalized (max %v): they must be in metres", maxAbs)
	}
	if maxAbs > 20 {
		t.Fatalf("uv out of sane range for a 2700 mm flight: max %v", maxAbs)
	}
}

func TestUVNotIdenticalOnNeighbouringTreads(t *testing.T) {
	// Смещение UV внутри детали нужно, чтобы рисунок на соседних ступенях не
	// был пиксель в пиксель: без него ступени выглядели бы как копии.
	res, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	first := -1
	second := -1
	for i, pr := range res.Mesh.PartRanges {
		if pr.Role != "tread" {
			continue
		}
		if first < 0 {
			first = i
		} else {
			second = i
			break
		}
	}
	if first < 0 || second < 0 {
		t.Fatal("need at least two treads in the flight")
	}
	a := res.Mesh.PartRanges[first]
	b := res.Mesh.PartRanges[second]
	same := 0
	// Сравниваем по диапазону треугольников: вершины у соседних ступеней
	// разные, но их UV не должны совпадать целиком.
	for t1 := a.Start; t1 < a.End && t1 < len(res.Mesh.Triangles); t1++ {
		for t2 := b.Start; t2 < b.Start+1 && t2 < len(res.Mesh.Triangles); t2++ {
			tr1, tr2 := res.Mesh.Triangles[t1], res.Mesh.Triangles[t2]
			if res.Mesh.UV[tr1[0]] == res.Mesh.UV[tr2[0]] {
				same++
			}
		}
	}
	if same > 0 {
		t.Fatal("neighbouring treads must not share the same UV (pattern would tile identically)")
	}
}

// --- Габариты проступи после перехода на сечение -----------------------------
//
// Смена конструкции (экструзия сечения вдоль ширины вместо вертикальной
// экструзии прямоугольника) не должна была изменить ни одного габарита: это
// та же деталь, только с закруглённым носом. Проверяем числами, потому что на
// рендере при невыразительном свете отличить «съехавшую ступень» от ракурса
// невозможно.

func TestTreadKeepsItsFootprint(t *testing.T) {
	cfg := testConfig(t) // w=900, b=270, st=40, h=180, n=15
	withNose := withNose(t, cfg, 8)
	for _, c := range []struct {
		name string
		cfg  *engineering.StairConfiguration
	}{{"прямой нос", cfg}, {"скруглённый нос", withNose}} {
		model, err := BuildStraightFlight(c.cfg)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		solid := model.Solids()[firstTreadIndex()]
		bb := kerngeo.SolidBoundingBox(solid)
		w := 900.0
		b := 270.0
		st := 40.0
		if got := bb.Max.Y - bb.Min.Y; math.Abs(got-w) > 1e-6 {
			t.Fatalf("%s: ширина проступи %v, want %v", c.name, got, w)
		}
		// Глубина проступи = шаг + свес на толщину ступени (x0 = k·b − st).
		if got := bb.Max.X - bb.Min.X; math.Abs(got-(b+st)) > 1e-6 {
			t.Fatalf("%s: глубина проступи %v, want %v", c.name, got, b+st)
		}
		if got := bb.Max.Z - bb.Min.Z; math.Abs(got-st) > 1e-6 {
			t.Fatalf("%s: толщина проступи %v, want %v", c.name, got, st)
		}
		// Проступь должна начинаться от пола минус толщина: первая ступень
		// имеет свес st перед собой.
		if got := bb.Min.X; math.Abs(got+st) > 1e-6 {
			t.Fatalf("%s: первая ступень начинается с X=%v, want %v (свес)", c.name, got, -st)
		}
	}
}
