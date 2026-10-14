package geometry

import (
	"math"
	"testing"
)

// ПОДМЕНА МНОГОУГОЛЬНИКА ОКРУЖНОСТЬЮ (Фаза 0.2).
//
// Владелец потребовал «честную дугу» в документе реза: отверстие должно
// попадать в DXF как CIRCLE, а не как многоугольник.
//
// Подмена возможна только если многоугольник — верное приближение
// окружности. Иначе документ станет точнее на бумаге и грубее на станке:
// оператор вырежет отверстие по окружности другого размера, чем ту, на
// которой считали диаметр детали.
//
// Здесь проверяется именно эта граница.

// TestCircleFromPointsAcceptsFaithfulPolygon — многоугольник, построенный по
// допуску, распознаётся как окружность.
//
// В таблице ровно те отверстия, которые ПРИХОДЯТСЯ В ДОКУМЕНТ РЕЗА, то есть
// режутся лазером у нас. Это отверстия обоймы выреза (CombFrame.HoleSegments
// = 24). У них прогиб 0,0428 и 0,0513 мм — оба в пределах CircleFitToleranceMM.
func TestCircleFromPointsAcceptsFaithfulPolygon(t *testing.T) {
	for _, c := range []struct {
		name   string
		radius float64
		segs   int
	}{
		{"отверстие пятки Ø10", 5, 24},
		{"отверстие анкера Ø12", 6, 24},
	} {
		pts := circlePoints(c.radius, c.segs, 30, 20)
		got, ok := CircleFromPoints(pts)
		if !ok {
			t.Errorf("%s: окружность не распознана (сегментов %d)", c.name, c.segs)
			continue
		}
		if math.Abs(got.Radius-c.radius) > circleFitPrecisionMM {
			t.Errorf("%s: радиус %.12f вместо %.12f", c.name, got.Radius, c.radius)
		}
		if math.Hypot(got.Center.X-30, got.Center.Y-20) > circleFitPrecisionMM {
			t.Errorf("%s: центр (%.12f, %.12f) вместо (30, 20)", c.name, got.Center.X, got.Center.Y)
		}
	}
}

// TestCircleFromPointsRejectsPurchasedPartHoles — отверстия ПОКУПНЫХ деталей
// окружностью не признаются, и это правильно.
//
// Анкерное отверстие цанги Ø9 строится по допуску railTubeToleranceMM = 0,19 мм,
// то есть 11 сегментов и прогиб 0,1823 мм — втрое больше CircleFitToleranceMM.
// Подменять его на CIRCIE означало бы сдвинуть границу отверстия на 0,18 мм,
// и оператор вырезал бы не то отверстие, на котором считали диаметр.
//
// Цанга — ПОКУПНАЯ деталь, у нас она не режется и в документ реза не попадает,
// так что на деле подмены и не требуется. Тест фиксирует именно эту границу:
// распознавание включается там, где подмена безопасна, и выключено там, где
// она сдвинула бы деталь.
func TestCircleFromPointsRejectsPurchasedPartHoles(t *testing.T) {
	const radius, tol = 4.5, 0.19
	segs := kerngeoSegments(radius, tol)
	pts := circlePoints(radius, segs, 0, 0)
	got, ok := CircleFromPoints(pts)
	if ok {
		t.Errorf("отверстие Ø9 покупной детали признано окружностью: "+
			"прогиб %.4f мм при пороге %.2f", got.chordDeviationMM(pts), CircleFitToleranceMM)
	}
	if segs != 11 {
		t.Errorf("сегментов по допуску 0,19 мм на Ø9: %d, ожидалось 11 — "+
			"расчёт в допуске разошёлся", segs)
	}
	t.Logf("отверстие Ø9 цанги: %d сегментов, прогиб %.4f мм — подмена не выполняется",
		segs, Circle2D{Center: Point2D{}, Radius: radius}.chordDeviationMM(pts))
}

// TestCircleFromPointsRejectsCoarsePolygon — грубый многоугольник окружностью
// НЕ признаётся.
//
// Это главный гейт подмены. Если многоугольник из 8 точек на Ø9 (прогиб
// больше 0,8 мм) проходил бы как окружность, документ реза стал бы описывать
// отверстие на 0,8 мм больше настоящего — и это выглядело бы как ошибка
// оператора, а не как ошибка документа.
func TestCircleFromPointsRejectsCoarsePolygon(t *testing.T) {
	for _, c := range []struct {
		name   string
		radius float64
		segs   int
	}{
		{"грубый Ø9, 6 сегментов", 4.5, 6},
		{"грубый Ø9, 8 сегментов", 4.5, 8},
		{"грубой нос R10, 6 сегментов", 10, 6},
	} {
		pts := circlePoints(c.radius, c.segs, 0, 0)
		if got, ok := CircleFromPoints(pts); ok {
			t.Errorf("%s: многоугольник признан окружностью R=%.4f, "+
				"прогиб был %.4f мм", c.name, got.Radius, got.chordDeviationMM(pts))
		}
	}
}

// TestCircleFromPointsRejectsNonCircle — контур, который не круглый, не
// проходит даже при идеальной подгонке точек.
func TestCircleFromPointsRejectsNonCircle(t *testing.T) {
	rect := []Point2D{{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 20}, {X: 0, Y: 20}}
	if _, ok := CircleFromPoints(rect); ok {
		t.Error("прямоугольник признан окружностью")
	}
	// Стадион-овал: две прямые и две дуги. Точки лежат на двух разных
	// окружностях, поэтому круглым быть не может.
	oval := stadiumPoints(70, 50.8)
	if _, ok := CircleFromPoints(oval); ok {
		t.Error("овал признан окружностью")
	}
	// Треугольник с тремя точками проходит через circleThrough, но прогиб
	// рёбер равен половине радиуса — и это ловится.
	tri := []Point2D{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}}
	if _, ok := CircleFromPoints(tri); ok {
		t.Error("треугольник из трёх точек признан окружностью — проверка прогиба не работает")
	}
}

// TestCircleFromPointsRejectsDegenerate — вырожденные случаи не проходят.
func TestCircleFromPointsRejectsDegenerate(t *testing.T) {
	cases := []struct {
		name string
		pts  []Point2D
	}{
		{"две точки", []Point2D{{X: 0, Y: 0}, {X: 10, Y: 0}}},
		{"пустой контур", nil},
		{"совпадающие точки", []Point2D{{X: 1, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 1}}},
		{"коллинеарные", []Point2D{{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 10, Y: 0}}},
	}
	for _, c := range cases {
		if _, ok := CircleFromPoints(c.pts); ok {
			t.Errorf("%s: распознано как окружность", c.name)
		}
	}
}

// TestCircleSubstitutionDoesNotChangeThePart — ключевое свойство подмены.
//
// Замена многоугольника окружностью не должна двигать границу отверстия
// дальше, чем на погрешность САМОГО построения. Иначе документ реза
// описывал бы деталь, которой нет.
func TestCircleSubstitutionDoesNotChangeThePart(t *testing.T) {
	const radius = 5.0
	segs := kerngeoSegments(radius, 0.055)
	pts := circlePoints(radius, segs, 12, 7)
	got, ok := CircleFromPoints(pts)
	if !ok {
		t.Fatalf("окружность не распознана: %d сегментов", segs)
	}
	// Прогиб многоугольника внутрь окружности.
	dev := got.chordDeviationMM(pts)
	if dev > 0.055 {
		t.Errorf("прогиб %.6f мм больше допуска построения 0,055", dev)
	}
	// Контрольная величина: прогиб должен совпадать с формулой R(1−cos(π/N)).
	want := radius * (1 - math.Cos(math.Pi/float64(segs)))
	if math.Abs(dev-want) > 1e-6 {
		t.Errorf("прогиб %.9f не совпадает с формулой %.9f", dev, want)
	}
	t.Logf("Ø%g: %d сегментов, прогиб %.6f мм, порог %.2f — подмена допустима",
		radius*2, segs, dev, CircleFitToleranceMM)
}

// circlePoints — окружность заданным числом точек, тем же способом, что и
// circleRing в движке геометрии.
func circlePoints(radius float64, segs int, cx, cy float64) []Point2D {
	out := make([]Point2D, segs)
	for i := range segs {
		a := 2 * math.Pi * float64(i) / float64(segs)
		out[i] = Point2D{X: cx + radius*math.Cos(a), Y: cy + radius*math.Sin(a)}
	}
	return out
}

// stadiumPoints — овал длиной length и шириной width: две прямые и две дуги.
func stadiumPoints(length, width float64) []Point2D {
	r := width / 2
	half := length/2 - r
	var out []Point2D
	arc := func(cx float64, from, to float64) {
		const n = 16
		for i := 0; i <= n; i++ {
			a := from + (to-from)*float64(i)/float64(n)
			out = append(out, Point2D{X: cx + r*math.Cos(a), Y: r * math.Sin(a)})
		}
	}
	arc(half, -math.Pi/2, math.Pi/2)
	arc(-half, math.Pi/2, 3*math.Pi/2)
	return out
}

// kerngeoSegments — число сегментов по допуску, как считает движок.
func kerngeoSegments(radius, tol float64) int {
	denom := 2 * math.Acos(1-tol/radius)
	if denom <= 0 {
		return 4
	}
	return int(math.Ceil(2 * math.Pi / denom))
}
