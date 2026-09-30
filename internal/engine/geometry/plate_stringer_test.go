package geometry

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
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

// Ступени и подступенки идут МЕЖДУ пластинами: торец проступи упирается в
// боковую грань косоура (как на Ниоре), а не перекрывает косоур поверх.
func TestPlateStringerTreadsSpanBetweenPlates(t *testing.T) {
	cfg := metalConfig(t)
	model, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	tn := cfg.StringerThickness.Millimeters()
	w := cfg.Width.Millimeters()
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
		if !nearlyEqual(lo, tn) || !nearlyEqual(hi, w-tn) {
			t.Fatalf("%s y = [%v, %v], ждём [%v, %v] (между пластинами)",
				solid.Role(), lo, hi, tn, w-tn)
		}
	}
	if treads != 16 || risers != 16 {
		t.Fatalf("проступей %d, подступенков %d, ждём 16 и 16", treads, risers)
	}
}

// Профиль пластины: пять вершин, верхнее ребро — посадочная линия ступеней
// (прямая, без зубьев), нижнее — та же линия, сдвинутая на глубину пластины.
func TestPlateStringerProfileIsStraightPlate(t *testing.T) {
	n, b, h, st, y, tn, depth := 16, 280.0, 175.0, 40.0, 0.0, 8.0, StringerPlateDepthMM
	pts := plateStringerProfile(n, b, h, st, y, tn, depth)
	// 7 вершин: пол, передняя грань, посадки, полка, низ полки, пятка.
	if len(pts) != 7 {
		t.Fatalf("вершин профиля = %d, ждём 7 (прямая пластина без зубьев)", len(pts))
	}
	// Передняя грань вертикальна: от пола до первой посадки.
	if !nearlyEqual(pts[0].X, 0) || !nearlyEqual(pts[0].Z, 0) {
		t.Fatalf("передний нижний угол = %v, ждём (0,0)", pts[0])
	}
	if !nearlyEqual(pts[1].X, 0) || !nearlyEqual(pts[1].Z, h-st) {
		t.Fatalf("передний верхний угол = %v, ждём (0, %v)", pts[1], h-st)
	}
	// Верхнее ребро: последняя посадка, затем горизонтальная полка до
	// конца марша.
	if !nearlyEqual(pts[2].X, float64(n-1)*b) || !nearlyEqual(pts[2].Z, float64(n)*h-st) {
		t.Fatalf("последняя посадка = %v, ждём (%v, %v)", pts[2], float64(n-1)*b, float64(n)*h-st)
	}
	if !nearlyEqual(pts[3].X, float64(n)*b) || !nearlyEqual(pts[3].Z, float64(n)*h-st) {
		t.Fatalf("конец полки = %v, ждём (%v, %v)", pts[3], float64(n)*b, float64(n)*h-st)
	}
	// Низ полки — на глубину пластины ниже её верха.
	if !nearlyEqual(pts[4].Z, float64(n)*h-st-depth) {
		t.Fatalf("низ полки Z = %v, ждём %v", pts[4].Z, float64(n)*h-st-depth)
	}
	// Пятка стоит на полу.
	last := pts[len(pts)-1]
	if !nearlyEqual(last.Z, 0) {
		t.Fatalf("пятка Z = %v, ждём 0 (пластина стоит на полу)", last.Z)
	}
	// Глубина пластины по нормали к посадочной линии: нижнее ребро — это
	// посадочная линия, сдвинутая на depth·(h, −b)/L.
	L := math.Hypot(b, h)
	offsetZ := firstZOffset(h, st, b, L, depth)
	// z нижнего ребра в x = 0 должно быть firstZ − depth·b/L.
	lowAt0 := h - st - depth*b/L
	if math.Abs((offsetZ) - lowAt0) > 1e-9 {
		t.Fatalf("z нижнего ребра при x=0 = %v, ждём %v", offsetZ, lowAt0)
	}
}

// firstZOffset — вспомогательное проверочное значение: z нижнего ребра
// пластины в точке x = 0.
func firstZOffset(h, st, b, L, depth float64) float64 {
	return h - st - depth*b/L
}

// Посадка ступеней на ребро: низ проступи каждой ступени лежит на верхнем
// ребре пластины. Это то, что держит ступень — главное отличие от гребёнки.
func TestPlateStringerSeatLineCarriesTreads(t *testing.T) {
	cfg := metalConfig(t)
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n := cfg.StepCount
	pts := plateStringerProfile(n, b, h, st, 0, cfg.StringerThickness.Millimeters(), StringerPlateDepthMM)
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
