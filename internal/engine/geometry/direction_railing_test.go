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
	if !nearlyEqual(bb.Max.Y, 3430) {
		t.Fatalf("bbox Max.Y = %v, want 3430", bb.Max.Y)
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

// TestBuildUShapeFlightLeftTurn проверяет левосторонний П-оборот: нижний
// марш вдоль +X, площадка шириной 2W на стыке, верхний марш развёрнут на
// 180° в полосе Y∈[W,2W]. Габарит: X∈[-810, 2560], Y∈[0, 1800], Z∈[0, 2700].
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
	if !pointExists(model, kerngeo.NewPoint3(900, 950, 1080)) {
		t.Fatal("upper first tread corner (900,950,1080) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(3330, 1800, 2700)) {
		t.Fatal("upper last tread corner (3330,1800,2700) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(2560, 850, 0)) {
		t.Fatal("lower riser front corner (2560,850,0) must exist (flight flipped, inset)")
	}
	bb := kerngeo.BoundingBox(model)
	if !nearlyEqual(bb.Min.X, 0) {
		t.Fatalf("bbox Min.X = %v, want 0 (landing left edge)", bb.Min.X)
	}
	if !nearlyEqual(bb.Max.X, 3330) {
		t.Fatalf("bbox Max.X = %v, want 3330 (upper stringer tail, vertical cut)", bb.Max.X)
	}
	if !nearlyEqual(bb.Max.Y, 1800) {
		t.Fatalf("bbox Max.Y = %v, want 1800", bb.Max.Y)
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
	if len(railRes.RailingMesh.Vertices) <= len(baseRes.RailingMesh.Vertices) {
		t.Fatalf("railing mesh must have more vertices: %d <= %d",
			len(railRes.RailingMesh.Vertices), len(baseRes.RailingMesh.Vertices))
	}
	if len(railRes.RailingMesh.Triangles) <= len(baseRes.RailingMesh.Triangles) {
		t.Fatalf("railing mesh must have more triangles: %d <= %d",
			len(railRes.RailingMesh.Triangles), len(baseRes.RailingMesh.Triangles))
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
	// правая сторона: поручень и стойки сдвинуты внутрь ступени (edgeInset),
	// наружная грань совпадает с кромкой y=W=900. Центр габарита по Y:
	// поручень: 900−railWidth/2=875; стойка: 900−balusterSize/2=890;
	// combined bbox Y: [850, 900], center = 875.
	bb := boundingBoxOf(decor)
	if !nearlyEqual((bb.Min.Y+bb.Max.Y)/2, 875) {
		t.Fatalf("railing bbox Y centre = %v, want 875 (right edge inset)",
			(bb.Min.Y+bb.Max.Y)/2)
	}
	// стойки на центрах ступеней: центр X — x=(k-0.5)·b.
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
		want := (float64(k+1) - 0.5) * b
		if !nearlyEqual(cx, want) {
			t.Fatalf("baluster %d centre X = %v, want %v (step centre)", k+1, cx, want)
		}
	}
	// поручень идёт по центрам ступеней от (b/2, edge, h+rh) до
	// ((n-0.5)·b, edge, H+rh-h/2).
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
	wantCZ := rh + (float64(cfg.StepCount)+1)*h/2
	if !nearlyEqual((rails[0].Min.X+rails[0].Max.X)/2, wantCX) {
		t.Fatalf("handrail centre X = %v, want %v (over nosings)",
			(rails[0].Min.X+rails[0].Max.X)/2, wantCX)
	}
	if !nearlyEqual((rails[0].Min.Y+rails[0].Max.Y)/2, 900-railWidth/2) {
		t.Fatalf("handrail centre Y = %v, want %v (right edge inset)",
			(rails[0].Min.Y+rails[0].Max.Y)/2, 900-railWidth/2)
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

// TestBuildLShapeLandingRailingOpensPassage проверяет, что перила площадки
// L-образного марша (платформенный режим) НЕ перекрывают проходы: дальняя
// кромка площадки (Y=Wp, откуда верхний марш уходит вдоль +Y) остаётся
// открытой, и участок боковой кромки марша Y∈[0,W] (проход к нижнему маршу)
// тоже открыт. Число сегментов поручня: «Г» (2) при Wp==W и «[» (3) при
// Wp>W (тогда огораживается и верхний участок боковой кромки Y∈[W,Wp]).
// Проверяем оба направления поворота (CONF-DIRECTION): правый и зеркальный
// левый.
func TestBuildLShapeLandingRailingOpensPassage(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := lshapeConfig(t)
		cfg.Direction = dir
		w := cfg.Width.Millimeters()
		wp := cfg.LandingWidth.Millimeters()
		rh := 900.0
		h1 := float64(cfg.LowerStepCount) * cfg.StepHeight.Millimeters()
		b := cfg.TreadDepth.Millimeters()
		l1 := float64(cfg.LowerStepCount) * b
		left := dir == engineering.TurnLeft
		x0 := l1
		if left {
			x0 = 0
		}
		// closeFar=false — L-образный марш: дальняя кромка Y=wp открыта.
		sols := landingRailingSolids(w, wp, rh, h1, b, x0, left, false, engineering.RailingBoth)

		rails := 0
		for _, s := range sols {
			if s.Role() == roleRailing {
				rails++
			}
		}
		wantRails := 2
		if wp > w {
			wantRails = 3
		}
		if rails != wantRails {
			t.Fatalf("%s: landing handrail segments = %d, want %d (Wp=%v W=%v)",
				dir, rails, wantRails, wp, w)
		}
		// Ни один поручень не должен быть центрирован на дальней кромке
		// Y=wp (проход к верхнему маршу). Допуск — половина толщины
		// поручня.
		const tol = railThickness / 2
		for _, s := range sols {
			if s.Role() != roleRailing {
				continue
			}
			bb := boundingBoxOf([]*kerngeo.Solid{s})
			cy := (bb.Min.Y + bb.Max.Y) / 2
			if math.Abs(cy-wp) < tol {
				t.Fatalf("%s: landing handrail centered at Y=%v (~Wp=%v), blocks passage to upper flight",
					dir, cy, wp)
			}
		}
		// Проход к нижнему маршу: на боковой кромке марша (X=x0 / X=w)
		// поручень не должен лежать в полосе Y∈[0,W] (центр Y<=W по
		// допуску). Верхний участок Y∈[W,Wp] при Wp>W — огорожен и тут
		// проверяется отдельным тестом.
		flightX := x0
		if left {
			flightX = w
		}
		for _, s := range sols {
			if s.Role() != roleRailing {
				continue
			}
			bb := boundingBoxOf([]*kerngeo.Solid{s})
			cx := (bb.Min.X + bb.Max.X) / 2
			if math.Abs(cx-flightX) < tol {
				// поручень на боковой кромке марша: он должен быть
				// смещён к верхнему участку (центр Y>=W), иначе
				// перекрывает проход к нижнему маршу.
				cy := (bb.Min.Y + bb.Max.Y) / 2
				if cy < w-tol {
					t.Fatalf("%s: landing handrail on flight-side edge at Y=%v (<W=%v) blocks lower-flight passage",
						dir, cy, w)
				}
			}
		}
	}
}

// TestBuildLShapeLandingRailingFullPerimeter проверяет, что при LandingWidth>W
// («площадка больше марша») площадка L-образного марша огораживается по всему
// открытому периметру без пропущенных кусков: на боковой кромке марша
// (X=x0 для правого / X=w для левого поворота) есть поручень, охватывающий
// весь внешний участок Y∈[W,Wp]; проходы к маршам (Y=wp и Y∈[0,W] на
// боковой кромке) при этом остаются открытыми.
func TestBuildLShapeLandingRailingFullPerimeter(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := lshapeConfig(t)
		cfg.Direction = dir
		w := cfg.Width.Millimeters()
		wp := cfg.LandingWidth.Millimeters()
		if wp <= w {
			t.Fatalf("test needs Wp>W (got Wp=%v W=%v)", wp, w)
		}
		rh := 900.0
		h1 := float64(cfg.LowerStepCount) * cfg.StepHeight.Millimeters()
		b := cfg.TreadDepth.Millimeters()
		l1 := float64(cfg.LowerStepCount) * b
		left := dir == engineering.TurnLeft
		x0 := l1
		if left {
			x0 = 0
		}
		sols := landingRailingSolids(w, wp, rh, h1, b, x0, left, false, engineering.RailingBoth)

		flightX := x0
		if left {
			flightX = w
		}
		// ищем поручень, центрированный на боковой кромке марша (X≈flightX)
		// и охватывающий верхний участок Y∈[w,wp] (его bbox достигает
		// Y, близкого к wp).
		const tol = railThickness / 2
		found := false
		for _, s := range sols {
			if s.Role() != roleRailing {
				continue
			}
			bb := boundingBoxOf([]*kerngeo.Solid{s})
			cx := (bb.Min.X + bb.Max.X) / 2
			if math.Abs(cx-flightX) >= tol {
				continue
			}
			// поручень на боковой кромке: должен охватывать Y∈[w,wp]
			if bb.Min.Y <= w+tol && bb.Max.Y >= wp-tol {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: no landing handrail encloses flight-side edge Y∈[W,Wp] (Wp=%v W=%v) — gap remains",
				dir, wp, w)
		}
	}
}

// TestBuildLShapeLandingRailingSides проверяет, что выбор стороны перил
// площадки L-образного марша работает: RailingRight — только правая вертикаль
// (большая X), RailingLeft — только левая (меньшая X), RailingBoth — весь
// открытый периметр. Выбор по плану, зеркально для левого поворота. Проходы
// (Y=wp к верхнему маршу и Y∈[0,W] к нижнему на пристеночной вертикали)
// остаются открытыми.
func TestBuildLShapeLandingRailingSides(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := lshapeConfig(t)
		cfg.Direction = dir
		w := cfg.Width.Millimeters()
		wp := cfg.LandingWidth.Millimeters()
		rh := 900.0
		h1 := float64(cfg.LowerStepCount) * cfg.StepHeight.Millimeters()
		b := cfg.TreadDepth.Millimeters()
		l1 := float64(cfg.LowerStepCount) * b
		left := dir == engineering.TurnLeft
		// Контракт landingRailingSolids: левый/правый края площадки всегда
		// задаются переданным x0 (левый = x0, правый = x0+w) независимо от
		// флага left. Здесь x0=l1 для обоих направлений (синтетический
		// параметр: проверяем только логику выбора стороны).
		leftEdgeX, rightEdgeX := l1, l1+w
		const tol = railThickness / 2

		checkOneSide := func(side engineering.RailingSide, expectX float64) {
			sols := landingRailingSolids(w, wp, rh, h1, b, l1, left, false, side)
			rails := 0
			for _, s := range sols {
				if s.Role() == roleRailing {
					rails++
				}
			}
			if rails != 1 {
				t.Fatalf("%s/%s: landing handrail segments = %d, want 1 (single side)", dir, side, rails)
			}
			// единственный поручень центрирован на нужной вертикали
			for _, s := range sols {
				if s.Role() != roleRailing {
					continue
				}
				bb := boundingBoxOf([]*kerngeo.Solid{s})
				cx := (bb.Min.X + bb.Max.X) / 2
				if math.Abs(cx-expectX) >= tol {
					t.Fatalf("%s/%s: handrail centered at X=%v, want ~%v", dir, side, cx, expectX)
				}
				// не перекрывает проход к верхнему маршу (Y=wp)
				cy := (bb.Min.Y + bb.Max.Y) / 2
				if math.Abs(cy-wp) < tol {
					t.Fatalf("%s/%s: handrail centered at Y=%v (~Wp), blocks upper-flight passage", dir, side, cy)
				}
			}
		}
		checkOneSide(engineering.RailingRight, rightEdgeX)
		checkOneSide(engineering.RailingLeft, leftEdgeX)

		// RailingBoth — полный периметр (низ + 2 вертикали; для Wp>W
		// левая/правая вертикали могут быть короче из-за проходов).
		both := landingRailingSolids(w, wp, rh, h1, b, l1, left, false, engineering.RailingBoth)
		bothRails := 0
		for _, s := range both {
			if s.Role() == roleRailing {
				bothRails++
			}
		}
		wantBoth := 2
		if wp > w {
			wantBoth = 3
		}
		if bothRails != wantBoth {
			t.Fatalf("%s: RailingBoth landing handrail segments = %d, want %d", dir, bothRails, wantBoth)
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
