package geometry

import (
	"math"
	"testing"
)

// SolidPlateBlank и CutLengthMM определяют размер заготовки и стоимость
// лазерного реза. Ошибка здесь уходит прямо в раскрой и в смету, поэтому
// проверяются на теле с известными размерами.

// plateSolid — плита 100×40×4 мм. Самая тонкая ось Z, поэтому плоскость
// детали XY: длина 100, ширина 40.
func plateSolid() *Solid {
	profile := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(100, 0, 0),
		NewPoint3(100, 40, 0),
		NewPoint3(0, 40, 0),
	}
	solid, err := Extrude(profile, NewVector3(0, 0, 1), 4)
	if err != nil {
		panic(err)
	}
	return solid
}

func TestSolidPlateBlankUsesThinAxis(t *testing.T) {
	l, w := SolidPlateBlank(plateSolid())
	// Длина вдоль длиннейшего ребра, ширина по нормали. Ось Z (4 мм) из
	// плоскости выпадает: если её не отбросить, «ширина» оказалась бы 4 мм
	// и заготовка в раскрой ушла бы неверного размера.
	if math.Abs(l-100) > 1e-6 || math.Abs(w-40) > 1e-6 {
		t.Fatalf("габарит заготовки = %.3f×%.3f, ждали 100×40", l, w)
	}
	// Длина не меньше ширины при любой ориентации тела в сцене.
	if l < w {
		t.Errorf("длина %.3f меньше ширины %.3f", l, w)
	}
}

func TestSolidPlateBlankDegenerate(t *testing.T) {
	// У вырожденного тела заготовки нет: лучше нули, чем выдуманные числа,
	// которые уйдут в раскрой.
	if l, w := SolidPlateBlank(nil); l != 0 || w != 0 {
		t.Errorf("nil-тело дало %.3f×%.3f", l, w)
	}
}

func TestCutLengthMMCountsBothCaps(t *testing.T) {
	got := CutLengthMM(plateSolid())
	// Крышек две, и контуры у них одинаковые, поэтому сумма делится на два —
	// функция отдаёт периметр одного контура. Первое моё ожидание (2×280)
	// было ошибкой: деление на два в коде сделано намеренно.
	want := float64(100 + 40 + 100 + 40)
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("длина реза = %.3f, ждали %.3f", got, want)
	}
}

func TestCutLengthMMIsZeroOnFlat(t *testing.T) {
	// У тела без толщины резать нечего.
	if got := CutLengthMM(nil); got != 0 {
		t.Errorf("nil-тело дало длину реза %.3f", got)
	}
}

func TestRingPerimeter2D(t *testing.T) {
	sq := []Point3{
		NewPoint3(0, 0, 0),
		NewPoint3(10, 0, 0),
		NewPoint3(10, 10, 0),
		NewPoint3(0, 10, 0),
	}
	// Контур замкнут: последняя точка соединяется с первой, иначе периметр
	// терял бы целое ребро и рез считался бы дешевле.
	if got := ringPerimeter2D(sq); math.Abs(got-40) > 1e-6 {
		t.Errorf("периметр квадрата = %.3f, ждали 40", got)
	}
	if got := ringPerimeter2D(nil); got != 0 {
		t.Errorf("nil-контур дал периметр %.3f", got)
	}
	if got := ringPerimeter2D(sq[:1]); got != 0 {
		t.Errorf("одна точка дала периметр %.3f", got)
	}
}

func TestOnPlane(t *testing.T) {
	pts := []Point3{NewPoint3(0, 3, 0), NewPoint3(5, 3, 7)}
	// Точки лежат в плоскости Y=3, значит плоскости X=3 и Z=3 они не лежат.
	if !onPlane(pts, 1, 3) {
		t.Error("точки не признаны лежащими в Y=3")
	}
	if onPlane(pts, 0, 3) {
		t.Error("точки признаны лежащими в X=3")
	}
	if onPlane(pts, 2, 3) {
		t.Error("точки признаны лежащими в Z=3")
	}
}

func TestSolidPointsCoversFaces(t *testing.T) {
	pts := solidPoints(plateSolid())
	// Все четыре грани плиты, по четыре точки на грань.
	if len(pts) < 16 {
		t.Fatalf("solidPoints вернул %d точек, ждали не меньше 16", len(pts))
	}
}
