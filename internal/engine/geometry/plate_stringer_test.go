package geometry

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// metalConfig — конфигурация металлокаркаса: стальной косоур 8 мм, ступень
// 40 мм, 16 ступеней. Именно её строит витрина.
func metalConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2800), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 16
	cfg.StepHeight = mustLength(t, 175)
	cfg.TreadDepth = mustLength(t, 280)
	cfg.StringerThickness = mustLength(t, 8)
	cfg.StepThickness = mustLength(t, 40)
	cfg.Material = "STEEL-S235"
	return cfg
}

// Косоур металлокаркаса — боковая пластина ПО КРАЯМ ширины. Прежние два
// косоура стояли внутри ширины (на w/4 и 3w/4), и в 3D это читалось как
// «каша из непонятных элементов»: ступень стояла на двух тонких пилах
// внутри габарита, как на рельсах.
func TestPlateStringerStandsAtSides(t *testing.T) {
	cfg := metalConfig(t)
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := cfg.Width.Millimeters()
	tn := cfg.StringerThickness.Millimeters()
	var mins, maxs []float64
	for _, solid := range model.Solids() {
		if solid.Role() != "stringer" {
			continue
		}
		lo, hi := solidYBounds(solid)
		mins = append(mins, lo)
		maxs = append(maxs, hi)
	}
	if len(mins) != 2 {
		t.Fatalf("строковых пластин = %d, ждём 2", len(mins))
	}
	// Пластины занимают края: [0,t] и [w−t,w].
	if !nearlyEqual(mins[0], 0) || !nearlyEqual(maxs[0], tn) {
		t.Fatalf("ближняя пластина y = [%v, %v], ждём [0, %v]", mins[0], maxs[0], tn)
	}
	if !nearlyEqual(mins[1], w-tn) || !nearlyEqual(maxs[1], w) {
		t.Fatalf("дальняя пластина y = [%v, %v], ждём [%v, %v]", mins[1], maxs[1], w-tn, w)
	}
}

// Ступени и подступенки идут ВО ВСЮ ШИРИНУ марша, включая плоскости
// косоуров: свеса за косоуры нет (решение владельца 2026-10-05). Прежний
// нахлёст 2 мм внутрь пластины убран — плоскости разводит фронтенд
// (ROLE_DEPTH_BIAS), а геометрия соответствует чертежу.
func TestPlateStringerTreadsSpanBetweenPlates(t *testing.T) {
	cfg := metalConfig(t)
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := cfg.Width.Millimeters()
	want0, want1 := 0.0, w
	var treads, risers int
	for _, solid := range model.Solids() {
		var lo, hi float64
		switch solid.Role() {
		case "tread":
			treads++
			lo, hi = solidYBounds(solid)
		case "riser":
			risers++
			lo, hi = solidYBounds(solid)
		default:
			continue
		}
		if !nearlyEqual(lo, want0) || !nearlyEqual(hi, want1) {
			t.Fatalf("%s y = [%v, %v], ждём [%v, %v] (во всю ширину марша)",
				solid.Role(), lo, hi, want0, want1)
		}
	}
	if treads != 16 || risers != 16 {
		t.Fatalf("проступей %d, подступенков %d, ждём 16 и 16", treads, risers)
	}
}

// Профиль пластины: пять вершин, верхнее ребро — посадочная линия ступеней
// (прямая, без зубьев), нижнее — та же линия, сдвинутая на глубину пластины.
func TestPlateStringerProfileIsStraightPlate(t *testing.T) {
	n, b, h, st, depth := 16, 280.0, 175.0, 40.0, StringerPlateDepthMM
	pts := plateStringerProfile(n, b, h, st, depth)
	// 5 вершин: пол, передняя грань, верхнее ребро, задний срез, пятка.
	if len(pts) != 5 {
		t.Fatalf("вершин профиля = %d, ждём 5 (прямая пластина без зубьев)", len(pts))
	}
	// Передняя грань вертикальна: от пола до первой посадки.
	if !nearlyEqual(pts[0].X, 0) || !nearlyEqual(pts[0].Z, 0) {
		t.Fatalf("передний нижний угол = %v, ждём (0,0)", pts[0])
	}
	if !nearlyEqual(pts[1].X, 0) || !nearlyEqual(pts[1].Z, h-st) {
		t.Fatalf("передний верхний угол = %v, ждём (0, %v)", pts[1], h-st)
	}
	// Верхнее ребро заканчивается на ПОСЛЕДНЕЙ посадке (n−1)·b, и срез
	// вертикален. Полки сверху нет: с ней верх был тоньше марша, пластина
	// «заужалась» к верху.
	lastSeatX := float64(n-1) * b
	topZ := float64(n)*h - st
	if !nearlyEqual(pts[2].X, lastSeatX) || !nearlyEqual(pts[2].Z, topZ) {
		t.Fatalf("последняя посадка = %v, ждём (%v, %v)", pts[2], lastSeatX, topZ)
	}
	if !nearlyEqual(pts[3].X, lastSeatX) {
		t.Fatalf("задний срез X = %v, ждём %v (вертикальный)", pts[3].X, lastSeatX)
	}
	// Пятка стоит на полу.
	if !nearlyEqual(pts[4].Z, 0) {
		t.Fatalf("пятка Z = %v, ждём 0 (пластина стоит на полу)", pts[4].Z)
	}
	// Пластина — полоса постоянной глубины ПО НОРМАЛИ к линии подъёма.
	// Задний срез вертикальный, поэтому вертикальный зазор между рёбрами
	// равен depth/cos(угла) = depth·L/b, а ПЕРПЕНДИКУЛЯРНЫЙ — ровно depth.
	// Проверяем именно перпендикулярное расстояние: именно оно заявлено в
	// чертеже как «пластина 300 мм».
	L := math.Hypot(b, h)
	vertGap := pts[2].Z - pts[3].Z
	perp := vertGap * b / L
	if math.Abs(perp-depth) > 1e-6 {
		t.Fatalf("перпендикулярная глубина пластины = %v, ждём %v (вертикальный зазор %v)",
			perp, depth, vertGap)
	}
	// Верхнее ребро (посадки) и нижнее — обе прямые с уклоном подъёма h/b.
	tdx, tdz := pts[2].X-pts[1].X, pts[2].Z-pts[1].Z
	if math.Abs(tdx*h-tdz*b) > 1e-6 {
		t.Fatalf("верхнее ребро (%v, %v) не параллельно подъёму (%v, %v)", tdx, tdz, b, h)
	}
	// Нижнее ребро — продолжение той же прямой (вниз до пола).
	bdx, bdz := pts[4].X-pts[3].X, pts[4].Z-pts[3].Z
	if math.Abs(bdx*h-bdz*b) > 1e-6 {
		t.Fatalf("нижнее ребро (%v, %v) не параллельно подъёму (%v, %v)", bdx, bdz, b, h)
	}
}

// Глубина пластины 300 мм — это расстояние ПО НОРМАЛИ, и оно не должно
// зависеть от угла марша. Регрессия: сдвиг низа был задан вертикальным
// отрезком depth·b/L, и перпендикулярная глубина выходила depth·(b/L)² —
// 233 мм при подъёме 150, 216 при 175, 199 при 200, 167 при 250.
func TestPlateDepthIsConstantAcrossFlightAngles(t *testing.T) {
	for _, h := range []float64{150, 175, 200, 250} {
		b, st, n := 280.0, 40.0, 16
		pts := plateStringerProfile(n, b, h, st, StringerPlateDepthMM)
		perp := (pts[2].Z - pts[3].Z) * b / math.Hypot(b, h)
		if math.Abs(perp-StringerPlateDepthMM) > 1e-6 {
			t.Errorf("подъём %v: перпендикулярная глубина = %v, ждём %v",
				h, perp, StringerPlateDepthMM)
		}
		// Пятка не уезжает за задний срез: иначе контур самопересекается.
		if pts[4].X > pts[3].X {
			t.Errorf("подъём %v: пятка X = %v уехала за задний срез %v", h, pts[4].X, pts[3].X)
		}
	}
}

// Посадка ступеней на ребро: низ проступи каждой ступени лежит на верхнем
// ребре пластины. Это то, что держит ступень — главное отличие от гребёнки.
func TestPlateStringerSeatLineCarriesTreads(t *testing.T) {
	cfg := metalConfig(t)
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n := cfg.StepCount
	pts := plateStringerProfile(n, b, h, st, StringerPlateDepthMM)
	// Верхнее ребро: от первой посадки (0, h−st) до последней
	// ((n−1)·b, n·h−st).
	x0, z0 := pts[1].X, pts[1].Z
	x1, z1 := pts[2].X, pts[2].Z
	for k := 0; k < n; k++ {
		xk, zk := float64(k)*b, float64(k+1)*h-st
		frac := (xk - x0) / (x1 - x0)
		want := z0 + frac*(z1-z0)
		if math.Abs(zk-want) > 1e-6 {
			t.Fatalf("посадка %d: Z = %v, на ребре %v", k, zk, want)
		}
	}
}

// Деревянный каркас остаётся гребёнкой: толстый косоур без кода материала.
func TestWoodFrameKeepsComb(t *testing.T) {
	cfg := testConfig(t) // StringerThickness = 50 мм, материал не задан
	if PlateStringerFrame(cfg) {
		t.Fatal("толстый косоур без кода материала не должен считаться металлом")
	}
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := cfg.Width.Millimeters()
	tn := cfg.StringerThickness.Millimeters()
	var mins []float64
	for _, solid := range model.Solids() {
		if solid.Role() == "stringer" {
			lo, _ := solidYBounds(solid)
			mins = append(mins, lo)
		}
	}
	// Прежнее поведение: косоуры внутри ширины, на w/4 и 3w/4.
	if !nearlyEqual(mins[0], w/4-tn/2) || !nearlyEqual(mins[1], 3*w/4-tn/2) {
		t.Fatalf("косоуры дерева y = %v, ждём [%v, %v]", mins, w/4-tn/2, 3*w/4-tn/2)
	}
}

// Металл без кода материала опознаётся по толщине (автоназначение): 8 мм —
// лист, 50 мм — пиломатериал.
func TestPlateFrameMaterialAutoByThickness(t *testing.T) {
	metal := metalConfig(t)
	metal.Material = ""
	if !PlateStringerFrame(metal) {
		t.Fatal("косоур 8 мм без кода материала должен считаться листовым")
	}
	wood := testConfig(t)
	if PlateStringerFrame(wood) {
		t.Fatal("косоур 50 мм не должен считаться листовым")
	}
}

// Заготовка лазерного раскроя = длина пластины по склону × глубина по
// нормали. Считается из тела, поэтому одинаково работает для прямого
// марша и для сегментов Г- и П-образных лестниц.
func TestPlateBlankMatchesPartGeometry(t *testing.T) {
	pts := plateStringerProfile(16, 280, 175, 40, StringerPlateDepthMM)
	solid, err := kerngeo.Extrude(pts, kerngeo.NewVector3(0, 1, 0), 8)
	if err != nil {
		t.Fatal(err)
	}
	length, width := kerngeo.SolidPlateBlank(solid)
	if math.Abs(width-StringerPlateDepthMM) > 1 {
		t.Fatalf("ширина заготовки = %v, ждём %v (глубина пластины)", width, StringerPlateDepthMM)
	}
	// Длина больше горизонтального габарита марша: пластина идёт по склону.
	if length <= (16-1)*280 {
		t.Fatalf("длина заготовки = %v, должна быть больше горизонтального габарита %v", length, 15*280)
	}
	want, _ := PlateBlankMM(16, 280, 175, 40, StringerPlateDepthMM)
	if math.Abs(length-want) > 1 {
		t.Fatalf("длина по телу = %v, расчёт по профилю = %v — расходятся", length, want)
	}
}
