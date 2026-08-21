package geometry

import (
	"context"
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// ushapeConfig возвращает валидную конфигурацию П-образной лестницы
// (H=2700, n=15, h=180, b=270, W=900, n1=6, Wp=1000, T=50, st=40).
func ushapeConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightUShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.StepThickness = mustLength(t, 40)
	cfg.LowerStepCount = 6
	cfg.LandingWidth = mustLength(t, 1000)
	return cfg
}

// TestBuildUShapeFlightCount проверяет состав модели: нижний марш
// (2+2n1), площадка (1) и верхний марш (2+2n2) твёрдых тел.
func TestBuildUShapeFlightCount(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// n1=6: 14 тел (2 косоура + 6 проступей + 6 подступенков); площадка: 1;
	// n2=9: 20 тел. Итого 35.
	if len(model.Solids()) != 35 {
		t.Fatalf("solids = %d, want 35", len(model.Solids()))
	}
}

// TestBuildUShapeFlightLandingPosition проверяет положение площадки:
// верх на уровне H1 = n1·h = 1080, план [L1, L1+W]×[0,Wp] = [1620,2520]×[0,1000].
func TestBuildUShapeFlightLandingPosition(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !pointExists(model, kerngeo.NewPoint3(1620, 0, 1040)) {
		t.Fatal("landing bottom corner (1620,0,1040) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(2520, 1800, 1080)) {
		t.Fatal("landing top corner (2520,1800,1080) must exist")
	}
}

// TestBuildUShapeFlightLowerFlightPositions проверяет позиции нижнего
// марша (совпадают с прямым маршем): последняя проступь на z∈[1040,1080],
// первый подступенок — лицевая панель на x∈[−40,0].
func TestBuildUShapeFlightLowerFlightPositions(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !pointExists(model, kerngeo.NewPoint3(1620, 850, 1080)) {
		t.Fatal("lower last tread top corner (1620,850,1080) must exist (inset)")
	}
	if !pointExists(model, kerngeo.NewPoint3(-40, 850, 0)) {
		t.Fatal("lower first riser front corner (−40,850,0) must exist (inset)")
	}
}

// TestBuildUShapeFlightUpperFlightPositions проверяет, что верхний марш
// развёрнут на 180°: возвращается вдоль −X параллельно нижнему маршу и
// примыкает к площадке той же кромкой (X=L1), что и нижний марш. Поворот на
// 180° вокруг Z (EDR-0006 §4.8): локальный (x,y,z) → (1620−x, 1800−y, 1080+z).
func TestBuildUShapeFlightUpperFlightPositions(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// первая проступь верхнего марша: локальные x∈[0,270], y∈[0,900],
	// z∈[140,180] → глобальные x∈[1350,1620], y∈[900,1800], z∈[1080,1260].
	if !pointExists(model, kerngeo.NewPoint3(1620, 950, 1080)) {
		t.Fatal("upper first tread corner (1620,950,1080) must exist (inset)")
	}
	// последняя проступь: локальные x∈[2160,2430], y∈[0,900], z∈[1580,1620]
	// → глобальные x∈[−810,−540], y∈[900,1800], z∈[2660,2700].
	if !pointExists(model, kerngeo.NewPoint3(-810, 1800, 2700)) {
		t.Fatal("upper last tread corner (-810,1800,2700) must exist")
	}
	bb := kerngeo.BoundingBox(model)
	if !nearlyEqual(bb.Max.X, 2520) {
		t.Fatalf("model bbox max X = %v, want 2520 (landing edge)", bb.Max.X)
	}
	if !nearlyEqual(bb.Max.Y, 1800) {
		t.Fatalf("model bbox max Y = %v, want 1800 (2W)", bb.Max.Y)
	}
	if !nearlyEqual(bb.Max.Z, 2700) {
		t.Fatalf("model bbox max Z = %v, want 2700 (H)", bb.Max.Z)
	}
}

// TestBuildUShapeFlightRoles проверяет семантические роли тел: для
// повёрнутого верхнего марша роли обязательны (без них decompose
// классифицировал бы stringer как riser).
func TestBuildUShapeFlightRoles(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, s := range model.Solids() {
		counts[s.Role()]++
	}
	if counts["stringer"] != 4 {
		t.Fatalf("stringer count = %d, want 4 (2 per flight)", counts["stringer"])
	}
	if counts["tread"] != 15 {
		t.Fatalf("tread count = %d, want 15", counts["tread"])
	}
	if counts["riser"] != 15 {
		t.Fatalf("riser count = %d, want 15 (single panel per step, 6+9)", counts["riser"])
	}
	if counts["landing"] != 1 {
		t.Fatalf("landing count = %d, want 1", counts["landing"])
	}
}

func TestBuildUShapeFlightErrors(t *testing.T) {
	// не-U конфигурация отклоняется.
	if _, err := BuildUShapeFlight(testConfig(t)); err == nil {
		t.Fatal("straight config must be rejected")
	}
	// Wp < W отклоняется доменной валидацией (StepCount > 1).
	cfg := ushapeConfig(t)
	cfg.LandingWidth = mustLength(t, 500)
	if _, err := BuildUShapeFlight(cfg); err == nil {
		t.Fatal("landing width below stair width must be rejected")
	}
	// нулевой нижний марш.
	cfg = ushapeConfig(t)
	cfg.LowerStepCount = 0
	if _, err := BuildUShapeFlight(cfg); err == nil {
		t.Fatal("zero lower step count must be rejected")
	}
	if _, err := BuildUShapeFlight(nil); err == nil {
		t.Fatal("nil config must be rejected")
	}
}

func TestGenerateUShape(t *testing.T) {
	cfg := ushapeConfig(t)
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Measurement.SolidCount != 35 {
		t.Fatalf("solid count = %d, want 35", res.Measurement.SolidCount)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("expected no validation issues, got %+v", res.Issues)
	}
	if res.Mesh == nil || len(res.Mesh.Triangles) == 0 {
		t.Fatal("preview mesh must be present")
	}
	// объём идентичен L-образному (вращение не меняет объём): нижний и
	// верхний марши + площадка. Площадка имеет ширину 2W: 2W·W·st = 1800·900·40.
	// Профиль косоура (шнуровка): n1=6 → 236469.33, n2=9 → 358044.28, толщина 50.
	// Марши сужены на flightSideInsetMM=50 (wEff=850): проступи/подступенки
	// — полотна 850 шириной; площадка — по-прежнему 2W=1800 на 900.
	want := 2.0*(236469.33*50) + 6.0*(310*850*40) + 6.0*(850*140*40) +
		2.0*(358044.28*50) + 9.0*(310*850*40) + 9.0*(850*140*40) +
		1800*900*40
	if !nearlyEq(res.Measurement.Volume, want) {
		t.Fatalf("volume = %v, want %v", res.Measurement.Volume, want)
	}
}

func TestGenerateUShapeDeterminism(t *testing.T) {
	a, err := Generate(context.Background(), ushapeConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(context.Background(), ushapeConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if a.Measurement != b.Measurement {
		t.Fatal("u-shape generation must be deterministic")
	}
	if len(a.Mesh.Vertices) != len(b.Mesh.Vertices) || len(a.Mesh.Triangles) != len(b.Mesh.Triangles) {
		t.Fatal("u-shape mesh must be deterministic")
	}
}

// TestUShapeBBoxMath — независимая проверка геометрии: полный bbox должен
// покрывать оба марша и площадку (низ косоура на полу z=0, верх — H−st;
// верхний марш примыкает к площадке той же кромкой, что нижний, и выдвинут
// на st за кромку площадки в −X).
func TestUShapeBBoxMath(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bb := kerngeo.BoundingBox(model)
	if math.Abs(bb.Min.X+837.73500981) > 1e-6 || bb.Min.Y < -1e-6 || bb.Min.Z > 1e-6 {
		t.Fatalf("bbox min = %+v, want near (−837.73500981,0,0)", bb.Min)
	}
	if math.Abs(bb.Max.X-2520) > 1e-6 || math.Abs(bb.Max.Y-1800) > 1e-6 || math.Abs(bb.Max.Z-2700) > 1e-6 {
		t.Fatalf("bbox max = %+v, want (2520,1800,2700)", bb.Max)
	}
}

// TestBuildUShapeWinderFlight проверяет геометрию П-образной лестницы с
// поворотными ступенями (CONF-TURN-KIND=winder): вместо площадки строится
// веер из nw поворотных ступеней (роль "winder"); площадки нет. Поворотные
// ступени наследуют h/b прямых маршей и поднимают на общий H = n·h.
func TestBuildUShapeWinderFlight(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.TurnKind = engineering.TurnWinder
	cfg.WinderCount = 3
	model, err := BuildUShapeWinderFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// 2 косоура × 2 марша (4) + 12 проступей (6+6) + 12 подступенков (6+6)
	// + 3 поворотные ступени = 31 тело. Поворотные ступени имеют роль
	// "winder", отличную от "tread".
	if len(model.Solids()) != 31 {
		t.Fatalf("solids = %d, want 31", len(model.Solids()))
	}
	counts := map[string]int{}
	for _, s := range model.Solids() {
		counts[s.Role()]++
	}
	if counts["stringer"] != 4 {
		t.Fatalf("stringer = %d, want 4", counts["stringer"])
	}
	if counts["tread"] != 12 {
		t.Fatalf("tread = %d, want 12 (6+6; поворотные — отдельно)", counts["tread"])
	}
	if counts["riser"] != 12 {
		t.Fatalf("riser = %d, want 12 (6+6)", counts["riser"])
	}
	if counts["winder"] != 3 {
		t.Fatalf("winder = %d, want 3", counts["winder"])
	}
	if counts["landing"] != 0 {
		t.Fatalf("landing = %d, want 0 (winder mode has no landing)", counts["landing"])
	}
	// Стык нижнего марша и веера: (l1, w, h1) = (1620, 900, 1080).
	if !pointExists(model, kerngeo.NewPoint3(1620, 900, 1080)) {
		t.Fatal("lower/winder junction (1620,900,1080) must exist")
	}
	// Детерминизм сборки.
	model2, err := BuildUShapeWinderFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Solids()) != len(model2.Solids()) {
		t.Fatal("winder build must be deterministic")
	}
}

// TestBuildUShapeWinderFlightErrors — поворотные ступени: nw ≥ 3 и верхний
// марш должен оставаться (n2 = n − n1 − nw ≥ 1).
func TestBuildUShapeWinderFlightErrors(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.TurnKind = engineering.TurnWinder
	cfg.WinderCount = 2
	if _, err := BuildUShapeWinderFlight(cfg); err == nil {
		t.Fatal("winder count < 3 must be rejected")
	}
	cfg.WinderCount = 12 // n2 = 15 − 6 − 12 = −3 < 1
	if _, err := BuildUShapeWinderFlight(cfg); err == nil {
		t.Fatal("winder count leaving no upper flight must be rejected")
	}
}

// TestBuildUShapeRailingAlignsWithFlight проверяет, что декоративные перила
// П-образного марша (платформенный режим) пространственно совпадают с моделью
// ступеней: их охват по X/Y равен охвату модели. Регресс бага, когда перила
// верхнего марша строились по старой ширине wp вместо 2·W и без разворота на
// 180°, из-за чего они «уезжали» по Y относительно ступеней.
// TestBuildUShapeRailingAlignsWithFlight проверяет, что декоративные перила
// П-образного марша (платформенный режим) пространственно совпадают с моделью
// ступеней для ОБОИХ направлений поворота: их охват по X/Y — подмножество
// охвата модели. Регресс бага, когда перила верхнего марша использовали
// старую ширину wp и смещались по Y относительно ступеней.
func TestBuildUShapeRailingAlignsWithFlight(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := ushapeConfig(t)
		cfg.Direction = dir
		cfg.Railing = engineering.RailingRight
		cfg.RailingLower = engineering.RailingRight
		cfg.RailingLanding = engineering.RailingRight
		cfg.RailingUpper = engineering.RailingRight
		cfg.RailingHeight = mustLength(t, 900)

		flight, err := BuildUShapeFlight(cfg)
		if err != nil {
			t.Fatal(err)
		}
		railings, err := BuildRailingDecor(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(railings) == 0 {
			t.Fatal("railings must be built when railing side is set")
		}

		fb := kerngeo.BoundingBox(flight)
		rb := kerngeo.BoundingBox(kerngeo.NewCompound(railings...))

		// Перила сидят на ступенях внутри линии движения, поэтому их охват по
		// X/Y — подмножество охвата модели. Допуск 30 мм перекрывает половину
		// толщины перил (T/2 = 25 мм): перила центрированы по кромке проступи и
		// выступают на 25 мм за габарит ступеней.
		const tol = 30.0
		if rb.Min.X < fb.Min.X-tol || rb.Max.X > fb.Max.X+tol {
			t.Fatalf("%s: railing X bbox [%v,%v] must lie within flight X bbox [%v,%v]±%v",
				dir, rb.Min.X, rb.Max.X, fb.Min.X, fb.Max.X, tol)
		}
		// Перила верхнего марша занимают у ступеней полосу Y∈[W,2W]. До бага
		// они строились по старой ширине wp и смещались по Y на ~100 мм
		// (wp−W), выходя за габарит модели. Допуск 30 мм перекрывает половину
		// толщины перил (T/2 = 25 мм), но ловит баговый сдвиг.
		if rb.Min.Y < fb.Min.Y-tol || rb.Max.Y > fb.Max.Y+tol {
			t.Fatalf("%s: railing Y bbox [%v,%v] must lie within flight Y bbox [%v,%v]±%v (alignment bug)",
				dir, rb.Min.Y, rb.Max.Y, fb.Min.Y, fb.Max.Y, tol)
		}
	}
}

// TestBuildUShapeFlightIsUTurn проверяет, что П-образный марш — настоящий
// U-поворот на 180°: нижний и верхний марши поднимаются в противоположных
// направлениях по X (знаки подъёма противоположны). Регресс бага, когда левый
// вариант делал оба марша параллельными (один знак подъёма — не разворот).
func TestBuildUShapeFlightIsUTurn(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := ushapeConfig(t)
		cfg.Direction = dir
		model, err := BuildUShapeFlight(cfg)
		if err != nil {
			t.Fatal(err)
		}
		lower, upper := splitFlightsByZ(model, cfg)
		lowerAsc := ascentX(lower)
		upperAsc := ascentX(upper)
		if lowerAsc == 0 || upperAsc == 0 {
			t.Fatalf("%s: flight ascent along X must be non-zero (lower=%v upper=%v)", dir, lowerAsc, upperAsc)
		}
		if lowerAsc*upperAsc > 0 {
			t.Fatalf("%s: flights must ascend in opposite X directions (U-turn), got lower=%v upper=%v", dir, lowerAsc, upperAsc)
		}
	}
}

// TestBuildUShapeFlightGap проверяет внутренний зазор 100 мм между маршами
// П-образной лестницы (платформенный режим): каждый марш сужен на
// flightSideInsetMM=50, поэтому зазор = 2·flightSideInsetMM = 100. Проверяем
// для обоих направлений поворота.
func TestBuildUShapeFlightGap(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := ushapeConfig(t)
		cfg.Direction = dir
		model, err := BuildUShapeFlight(cfg)
		if err != nil {
			t.Fatal(err)
		}
		lower, upper := splitFlightsByZ(model, cfg)
		if len(lower) == 0 || len(upper) == 0 {
			t.Fatalf("%s: both flights must be present", dir)
		}
		lb := kerngeo.BoundingBox(kerngeo.NewCompound(lower...))
		ub := kerngeo.BoundingBox(kerngeo.NewCompound(upper...))
		gap := ub.Min.Y - lb.Max.Y
		if math.Abs(gap-100) > 1e-6 {
			t.Fatalf("%s: gap between flights = %v, want 100", dir, gap)
		}
	}
}

// TestBuildUShapeLowerRailingMirrorsTurn проверяет, что перила нижнего марша
// П-образной лестницы зеркальны по направлению поворота: при RailingLower=
// Right правый поворот даёт перила у внутренней кромки (Y≈wEff), левый — у
// внешней (Y≈0). Регресс бага, когда нижние перила не зеркалились.
func TestBuildUShapeLowerRailingMirrorsTurn(t *testing.T) {
	lowerRailingCenterY := func(dir engineering.TurnDirection) float64 {
		cfg := ushapeConfig(t)
		cfg.Direction = dir
		cfg.RailingLower = engineering.RailingRight
		cfg.RailingLanding = engineering.RailingNone
		cfg.RailingUpper = engineering.RailingNone
		cfg.RailingHeight = mustLength(t, 900)
		decor, err := BuildRailingDecor(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(decor) == 0 {
			t.Fatalf("%s: lower railing must be built", dir)
		}
		bb := kerngeo.BoundingBox(kerngeo.NewCompound(decor...))
		return (bb.Min.Y + bb.Max.Y) / 2
	}
	rightY := lowerRailingCenterY(engineering.TurnRight)
	leftY := lowerRailingCenterY(engineering.TurnLeft)
	const wEff = 900 - 50
	if math.Abs(rightY-wEff) > 30 {
		t.Fatalf("right lower railing center Y = %v, want ~%v (internal edge)", rightY, wEff)
	}
	if math.Abs(leftY-0) > 30 {
		t.Fatalf("left lower railing center Y = %v, want ~0 (external edge)", leftY)
	}
	if rightY <= leftY {
		t.Fatalf("lower railing must mirror by turn: right=%v should exceed left=%v", rightY, leftY)
	}
}

// TestBuildUShapeLandingRailingIsU проверяет, что перила площадки П-образной
// лестницы образуют контур «П» (3 сегмента поручня) и охватывают всю ширину
// площадки по Y ∈ [0, 2W] при обоих направлениях поворота.
func TestBuildUShapeLandingRailingIsU(t *testing.T) {
	for _, dir := range []engineering.TurnDirection{engineering.TurnRight, engineering.TurnLeft} {
		cfg := ushapeConfig(t)
		cfg.Direction = dir
		cfg.RailingLanding = engineering.RailingRight
		cfg.RailingLower = engineering.RailingNone
		cfg.RailingUpper = engineering.RailingNone
		cfg.RailingHeight = mustLength(t, 900)
		decor, err := BuildRailingDecor(cfg)
		if err != nil {
			t.Fatal(err)
		}
		rails := 0
		for _, s := range decor {
			if s.Role() == roleRailing {
				rails++
			}
		}
		if rails != 3 {
			t.Fatalf("%s: landing railing segments = %d, want 3 (П contour)", dir, rails)
		}
		bb := kerngeo.BoundingBox(kerngeo.NewCompound(decor...))
		// центральная линия поручня проходит по Y=0 и Y=2W=1800; тело
		// поручня (railWidth=50) выступает на ±25 за габарит.
		if math.Abs(bb.Min.Y) > 30 || math.Abs(bb.Max.Y-1800) > 30 {
			t.Fatalf("%s: landing railing Y coverage = [%v,%v], want ~[0,1800]", dir, bb.Min.Y, bb.Max.Y)
		}
	}
}

// splitFlightsByZ разделяет проступи (роль "tread") на нижний и верхний марш
// по уровню площадки H1 = n1·h.
func splitFlightsByZ(model *kerngeo.Compound, cfg *engineering.StairConfiguration) (lower, upper []*kerngeo.Solid) {
	h1 := float64(cfg.LowerStepCount) * cfg.StepHeight.Millimeters()
	for _, s := range model.Solids() {
		if s.Role() != "tread" {
			continue
		}
		bb := kerngeo.SolidBoundingBox(s)
		zc := (bb.Min.Z + bb.Max.Z) / 2
		if zc <= h1 {
			lower = append(lower, s)
		} else {
			upper = append(upper, s)
		}
	}
	return
}

// ascentX возвращает смещение центра проступи по X между самой нижней и
// самой верхней ступенью марша (положительное — подъём вдоль +X).
func ascentX(solids []*kerngeo.Solid) float64 {
	minZ, maxZ := math.Inf(1), math.Inf(-1)
	var cxMin, cxMax float64
	for _, s := range solids {
		bb := kerngeo.SolidBoundingBox(s)
		zc := (bb.Min.Z + bb.Max.Z) / 2
		cx := (bb.Min.X + bb.Max.X) / 2
		if zc < minZ {
			minZ, cxMin = zc, cx
		}
		if zc > maxZ {
			maxZ, cxMax = zc, cx
		}
	}
	return cxMax - cxMin
}
