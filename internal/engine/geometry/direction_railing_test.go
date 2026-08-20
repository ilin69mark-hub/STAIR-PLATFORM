package geometry

import (
	"context"
	"math"
	"sort"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// --- CONF-DIRECTION: 3D-модель поворачивается как 2D-план ---

// TestBuildLShapeFlightLeftTurn проверяет левосторонний (зеркальный)
// поворот L-образной лестницы (CONF-DIRECTION=left): валидная модель,
// площадка [0,W]×[0,Wp], нижний марш поднимается по −X и стыкуется с
// правой гранью площадки, верхний марш поднимается по +Y от её верхнего
// края. Габарит: X∈[0, 2560], Y∈[0, 3457.7], Z∈[0, 2700].
func TestBuildLShapeFlightLeftTurn(t *testing.T) {
	cfg := lshapeConfig(t)
	cfg.Direction = engineering.TurnLeft
	model, err := BuildLShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Все тела валидны (без ошибок уровня SeverityError).
	for i, s := range model.Solids() {
		for _, issue := range kerngeo.Validate(s) {
			if issue.Severity == kerngeo.SeverityError {
				t.Fatalf("solid %d invalid: %v", i, issue)
			}
		}
	}
	if !pointExists(model, kerngeo.NewPoint3(0, 0, 1040)) {
		t.Fatal("landing bottom corner (0,0,1040) must exist (landing at left)")
	}
	if !pointExists(model, kerngeo.NewPoint3(0, 960, 1260)) {
		t.Fatal("upper first tread corner (0,960,1260) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(0, 3430, 2700)) {
		t.Fatal("upper last tread corner (0,3430,2700) must exist")
	}
	// нижний марш развёрнут: его передний подступенок теперь на x=2560.
	if !pointExists(model, kerngeo.NewPoint3(2560, 900, 0)) {
		t.Fatal("lower riser front corner (2560,900,0) must exist (flight flipped)")
	}
	bb := kerngeo.BoundingBox(model)
	if !nearlyEqual(bb.Min.X, 0) {
		t.Fatalf("bbox Min.X = %v, want 0", bb.Min.X)
	}
	if !nearlyEqual(bb.Max.X, 2560) {
		t.Fatalf("bbox Max.X = %v, want 2560", bb.Max.X)
	}
	if !nearlyEqual(bb.Max.Y, 3457.73501) {
		t.Fatalf("bbox Max.Y = %v, want 3457.7", bb.Max.Y)
	}
	if !nearlyEqual(bb.Max.Z, 2700) {
		t.Fatalf("bbox Max.Z = %v, want 2700", bb.Max.Z)
	}
}

// TestBuildLShapeFlightLeftTurnVolume проверяет положительность объёма
// (левосторонняя компоновка строится поворотами — ориентация граней
// сохранена, объём положителен).
func TestBuildLShapeFlightLeftTurnVolume(t *testing.T) {
	cfg := lshapeConfig(t)
	cfg.Direction = engineering.TurnLeft
	model, err := BuildLShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	right := lshapeConfig(t)
	rightModel, _ := BuildLShapeFlight(right)
	var vl, vr float64
	for _, s := range model.Solids() {
		v, e := kerngeo.Volume(s)
		if e != nil {
			t.Fatal(e)
		}
		vl += v
	}
	for _, s := range rightModel.Solids() {
		v, e := kerngeo.Volume(s)
		if e != nil {
			t.Fatal(e)
		}
		vr += v
	}
	if vl <= 0 {
		t.Fatalf("left-turn volume = %v, want positive", vl)
	}
	// объёмы совпадают у зеркальных компоновок (изометрия).
	if math.Abs(vl-vr) > 1e-3 {
		t.Fatalf("left vs right volume differ: %v vs %v", vl, vr)
	}
}

// TestBuildUShapeFlightLeftTurn проверяет левосторонний П-оборот: площадка
// [0,W]×[0,Wp], нижний марш поднимается по −X к правой грани площадки,
// верхний марш без поворота поднимается по +X от левого края площадки.
// Габарит: X∈[-40, 2560], Y∈[0, 1900], Z∈[0, 2700].
func TestBuildUShapeFlightLeftTurn(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.Direction = engineering.TurnLeft
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !pointExists(model, kerngeo.NewPoint3(0, 0, 1040)) {
		t.Fatal("landing bottom corner (0,0,1040) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(-40, 1000, 1260)) {
		t.Fatal("upper first tread corner (-40,1000,1260) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(2430, 1900, 2700)) {
		t.Fatal("upper last tread corner (2430,1900,2700) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(2560, 900, 0)) {
		t.Fatal("lower riser front corner (2560,900,0) must exist (flight flipped)")
	}
	bb := kerngeo.BoundingBox(model)
	if !nearlyEqual(bb.Min.X, -40) {
		t.Fatalf("bbox Min.X = %v, want -40 (upper stringer tail)", bb.Min.X)
	}
	if !nearlyEqual(bb.Max.X, 2560) {
		t.Fatalf("bbox Max.X = %v, want 2560", bb.Max.X)
	}
	if !nearlyEqual(bb.Max.Y, 1900) {
		t.Fatalf("bbox Max.Y = %v, want 1900", bb.Max.Y)
	}
	if !nearlyEqual(bb.Max.Z, 2700) {
		t.Fatalf("bbox Max.Z = %v, want 2700", bb.Max.Z)
	}
}

// TestBuildSpiralFlightDirection проверяет, что направление закрутки
// (CONF-SPIRAL-DIRECTION) зеркалит ступени: cw — угол растёт, ccw —
// угол убывает; обе модели имеют одинаковый габарит (симметрия).
func TestBuildSpiralFlightDirection(t *testing.T) {
	cw := spiralConfig(t)
	cw.SpiralDirection = engineering.SpiralCW
	cwModel, err := BuildSpiralFlight(cw)
	if err != nil {
		t.Fatal(err)
	}
	ccw := spiralConfig(t)
	ccw.SpiralDirection = engineering.SpiralCCW
	ccwModel, err := BuildSpiralFlight(ccw)
	if err != nil {
		t.Fatal(err)
	}
	// габариты совпадают — зеркальная симметрия плана.
	bw, bc := kerngeo.BoundingBox(cwModel), kerngeo.BoundingBox(ccwModel)
	for _, p := range []struct{ a, b float64 }{
		{bw.Min.X, bc.Min.X}, {bw.Max.X, bc.Max.X},
		{bw.Min.Y, bc.Min.Y}, {bw.Max.Y, bc.Max.Y},
		{bw.Min.Z, bc.Min.Z}, {bw.Max.Z, bc.Max.Z},
	} {
		if !nearlyEqual(p.a, p.b) {
			t.Fatalf("bbox mismatch cw vs ccw: %v vs %v", p.a, p.b)
		}
	}
	// первая ступень (k=0): cw — верхняя кромка на угле +δ, ccw — на −δ.
	delta := FullTurnSpiral / float64(cw.StepCount)
	cwPT := kerngeo.NewPoint3(800*math.Cos(delta), 800*math.Sin(delta), 180)
	if !pointExists(cwModel, cwPT) {
		t.Errorf("cw first step outer corner %v must exist", cwPT)
	}
	ccwPT := kerngeo.NewPoint3(800*math.Cos(-delta), 800*math.Sin(-delta), 180)
	if !pointExists(ccwModel, ccwPT) {
		t.Errorf("ccw first step outer corner %v must exist", ccwPT)
	}
}

// --- CONF-RAILING: декоративные перила только в preview mesh ---

// countDecomposedRoles возвращает количество тел по ролям в Compound.
func countDecomposedRoles(model *kerngeo.Compound) map[string]int {
	m := map[string]int{}
	for _, s := range model.Solids() {
		m[s.Role()]++
	}
	return m
}

// boundingBoxOf строит габарит набора тел (для декоративных перил).
func boundingBoxOf(solids []*kerngeo.Solid) kerngeo.BBox {
	var pts []kerngeo.Point3
	for _, s := range solids {
		for _, shell := range s.Shells() {
			for _, face := range shell.Faces() {
				for _, e := range face.Outer().Edges() {
					v1, v2 := e.Endpoints()
					pts = append(pts, v1.Point(), v2.Point())
				}
			}
		}
	}
	return kerngeo.NewBBox(pts...)
}

// TestGenerateRailingAffectsOnlyMesh проверяет, что перила добавляются
// только в mesh: измерения (SolidCount, объём) не меняются, mesh растёт.
func TestGenerateRailingAffectsOnlyMesh(t *testing.T) {
	base := testConfig(t)
	railed := testConfig(t)
	railed.Railing = engineering.RailingRight
	railed.RailingHeight = mustLength(t, 900)

	baseRes, err := Generate(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	railRes, err := Generate(context.Background(), railed)
	if err != nil {
		t.Fatal(err)
	}
	if railRes.Measurement.SolidCount != baseRes.Measurement.SolidCount {
		t.Fatalf("SolidCount changed by railing: %d -> %d",
			baseRes.Measurement.SolidCount, railRes.Measurement.SolidCount)
	}
	if math.Abs(railRes.Measurement.Volume-baseRes.Measurement.Volume) > 1e-6 {
		t.Fatalf("Volume changed by railing: %v -> %v",
			baseRes.Measurement.Volume, railRes.Measurement.Volume)
	}
	if len(railRes.Mesh.Vertices) <= len(baseRes.Mesh.Vertices) {
		t.Fatalf("railing mesh must have more vertices: %d <= %d",
			len(railRes.Mesh.Vertices), len(baseRes.Mesh.Vertices))
	}
	if len(railRes.Mesh.Triangles) <= len(baseRes.Mesh.Triangles) {
		t.Fatalf("railing mesh must have more triangles: %d <= %d",
			len(railRes.Mesh.Triangles), len(baseRes.Mesh.Triangles))
	}
}

// TestBuildRailingDecorStraight проверяет состав декоративных тел прямого
// марша: роли только railing/baluster, для 'right' — одна кромка (y=W).
func TestBuildRailingDecorStraight(t *testing.T) {
	cfg := testConfig(t)
	cfg.Railing = engineering.RailingRight
	cfg.RailingHeight = mustLength(t, 900)
	decor, err := BuildRailingDecor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(decor) == 0 {
		t.Fatal("railing decor must be non-empty")
	}
	roles := map[string]int{}
	for _, s := range decor {
		switch s.Role() {
		case roleRailing, roleBaluster:
			roles[s.Role()]++
		default:
			t.Fatalf("unexpected decor role %q", s.Role())
		}
	}
	if roles[roleRailing] != 1 {
		t.Fatalf("handrails = %d, want 1 (right side)", roles[roleRailing])
	}
	if roles[roleBaluster] != 15 {
		t.Fatalf("balusters = %d, want 15 (one per step)", roles[roleBaluster])
	}
	// правая сторона: кромка y=W присутствует в координатах перил
	// (центр габарита по Y — на кромке 900; поручень и стойки имеют
	// собственные габариты поверх/вокруг неё).
	bb := boundingBoxOf(decor)
	if !nearlyEqual((bb.Min.Y+bb.Max.Y)/2, 900) {
		t.Fatalf("railing bbox Y centre = %v, want 900 (right edge)",
			(bb.Min.Y+bb.Max.Y)/2)
	}
	// стойки на носиках проступей (как в 2D-плане): центр X — x=k·b.
	var balXs []float64
	for _, s := range decor {
		if s.Role() == roleBaluster {
			bs := boundingBoxOf([]*kerngeo.Solid{s})
			balXs = append(balXs, (bs.Min.X+bs.Max.X)/2)
		}
	}
	sort.Float64s(balXs)
	b := cfg.TreadDepth.Millimeters()
	for k, cx := range balXs {
		want := float64(k+1) * b
		if !nearlyEqual(cx, want) {
			t.Fatalf("baluster %d centre X = %v, want %v (nosing)", k+1, cx, want)
		}
	}
	// поручень занимает ровно отрезок [A,B] по линии носиков (регресс
	// railAlong: призма выдавливалась от середины, и поручень «уезжал»
	// на половину марша). Центр габарита поручня — (run/2, W, rh+H/2).
	var rails []kerngeo.BBox
	for _, s := range decor {
		if s.Role() == roleRailing {
			rails = append(rails, boundingBoxOf([]*kerngeo.Solid{s}))
		}
	}
	if len(rails) != 1 {
		t.Fatalf("handrail boxes = %d, want 1 (right side)", len(rails))
	}
	h := cfg.StepHeight.Millimeters()
	rh := cfg.RailingHeight.Millimeters()
	wantCX := float64(cfg.StepCount) * b / 2
	wantCZ := rh + float64(cfg.StepCount)*h/2
	if !nearlyEqual((rails[0].Min.X+rails[0].Max.X)/2, wantCX) {
		t.Fatalf("handrail centre X = %v, want %v (over nosings)",
			(rails[0].Min.X+rails[0].Max.X)/2, wantCX)
	}
	if !nearlyEqual((rails[0].Min.Y+rails[0].Max.Y)/2, 900) {
		t.Fatalf("handrail centre Y = %v, want 900 (right edge)",
			(rails[0].Min.Y+rails[0].Max.Y)/2)
	}
	if !nearlyEqual((rails[0].Min.Z+rails[0].Max.Z)/2, wantCZ) {
		t.Fatalf("handrail centre Z = %v, want %v (rh+H/2)",
			(rails[0].Min.Z+rails[0].Max.Z)/2, wantCZ)
	}
}

// TestBuildRailingDecorLShape проверяет перила L-образной лестницы по
// сегментам (CONF-RAILING): нижний марш, площадка, верхний марш.
func TestBuildRailingDecorLShape(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := lshapeConfig(t)
		cfg.Direction = dir
		cfg.RailingLower = engineering.RailingBoth
		cfg.RailingLanding = engineering.RailingRight
		cfg.RailingUpper = engineering.RailingLeft
		cfg.RailingHeight = mustLength(t, 900)
		decor, err := BuildRailingDecor(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(decor) == 0 {
			t.Fatalf("L-shape decor must be non-empty for direction %s", dir)
		}
		roles := map[string]int{}
		for _, s := range decor {
			roles[s.Role()]++
		}
		if roles[roleRailing] < 4 {
			t.Fatalf("direction %s: handrails = %d, want >= 4 (2 lower + 2 landing + 1 upper)",
				dir, roles[roleRailing])
		}
		if roles[roleBaluster] < 6 {
			t.Fatalf("direction %s: balusters = %d, want >= 6", dir, roles[roleBaluster])
		}
	}
}

// TestBuildRailingDecorSpiral проверяет сторону перил спирали по её
// направлению (CONF-SPIRAL-RAILING): перила на открытой кромке.
func TestBuildRailingDecorSpiral(t *testing.T) {
	mk := func(dir engineering.SpiralDirection, rail engineering.RailingSide) []*kerngeo.Solid {
		cfg := spiralConfig(t)
		cfg.SpiralDirection = dir
		cfg.Railing = rail
		cfg.RailingHeight = mustLength(t, 900)
		d, err := BuildRailingDecor(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if len(mk(engineering.SpiralCW, engineering.RailingRight)) == 0 {
		t.Fatal("cw + right must have railing")
	}
	if len(mk(engineering.SpiralCCW, engineering.RailingLeft)) == 0 {
		t.Fatal("ccw + left must have railing")
	}
	if len(mk(engineering.SpiralCW, engineering.RailingLeft)) != 0 {
		t.Fatal("cw + left is the wall side and must have no railing")
	}
}
