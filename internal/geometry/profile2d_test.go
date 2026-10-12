package geometry

import (
	"math"
	"testing"
)

// S-163: точ��ый плоский контур детали вместо описывающего прямоугольника.
//
// Проверяется на двух формах, которые стоят в карте раскроя: прямоугольная
// пластина (контур совпадает с габаритом) и пилообразный косоур (контур
// заметно меньше прямоугольника — именно ради этого всё и затевалось).

func TestSolidProfile2DOnPlainPlate(t *testing.T) {
	p, ok := SolidProfile2D(plateSolid())
	if !ok {
		t.Fatal("контур прямоугольной плиты не найден")
	}
	if len(p) != 4 {
		t.Fatalf("вершин контура %d, для плиты ждали 4", len(p))
	}
	// Габарит обязан совпасть с SolidPlateBlank — это та же система осей.
	l, w := SolidPlateBlank(plateSolid())
	maxX, maxY := 0.0, 0.0
	for _, v := range p {
		maxX = math.Max(maxX, v.X)
		maxY = math.Max(maxY, v.Y)
	}
	if math.Abs(maxX-l) > 1e-6 || math.Abs(maxY-w) > 1e-6 {
		t.Fatalf("контур %.3f×%.3f разошёлся с габаритом %.3f×%.3f", maxX, maxY, l, w)
	}
}

func TestSolidProfile2DIsSmallerThanBoundingBox(t *testing.T) {
	// На пилообразном косоуре это главное: описывающий прямоугольник заметно
	// больше самой детали, и раскрой по нему тратит лишний материал.
	p, ok := SolidProfile2D(stringerLikeSolid())
	if !ok {
		t.Fatal("контур косоура не найден")
	}
	l, w := SolidPlateBlank(stringerLikeSolid())
	boxArea := l * w
	contourArea := ProfileAreaMM(p)
	if contourArea <= 0 {
		t.Fatal("площадь контура нулевая")
	}
	if contourArea >= boxArea {
		t.Fatalf("площадь контура %.0f не меньше прямоугольника %.0f — контур не извлечён",
			contourArea, boxArea)
	}
	// Для пилообразного профиля с зубьями контур должен быть заметно меньше:
	// зубья съедают примерно половину описывающего прямоугольника.
	if contourArea > boxArea*0.85 {
		t.Errorf("контур %.0f слишком близок к прямоугольнику %.0f", contourArea, boxArea)
	}
}

func TestProfileAreaOfRectangle(t *testing.T) {
	// Проверка самого метода площади: у прямоугольника 100×40 площадь 4000.
	if got := ProfileAreaMM([]Point2D{{0, 0}, {100, 0}, {100, 40}, {0, 40}}); math.Abs(got-4000) > 1e-6 {
		t.Errorf("площадь прямоугольника 100×40 = %.2f, ждали 4000", got)
	}
	// Вырожденные контуры дают ноль, а не мусор.
	if got := ProfileAreaMM(nil); got != 0 {
		t.Errorf("площадь nil-контура = %f", got)
	}
	if got := ProfileAreaMM([]Point2D{{0, 0}, {10, 0}}); got != 0 {
		t.Errorf("площадь двухточечного контура = %f", got)
	}
}

func TestSolidProfile2DCoordinatesAreNonNegative(t *testing.T) {
	// Начало координат в минимальной точке: контур готов к отрисовке «как
	// есть», без последующего сдвига. Отрицательная координата означала бы,
	// что оператор увидит деталь за пределами заготовки.
	for name, s := range map[string]*Solid{"плита": plateSolid(), "косоур": stringerLikeSolid()} {
		p, ok := SolidProfile2D(s)
		if !ok {
			t.Fatalf("%s: контур не найден", name)
		}
		for _, v := range p {
			if v.X < -1e-6 || v.Y < -1e-6 {
				t.Errorf("%s: отрицательная координата контура (%.3f, %.3f)", name, v.X, v.Y)
			}
		}
	}
}

func TestSolidProfile2DRejectsNilAndDegenerate(t *testing.T) {
	if _, ok := SolidProfile2D(nil); ok {
		t.Error("nil-тело вернуло контур")
	}
	// Плоское тело без толщины не вырезается из листа.
	if _, ok := SolidProfile2D(&Solid{}); ok {
		t.Error("вырожденное тело вернуло контур")
	}
}

// normalizeContour — приводит контур к каноническому виду. Без этого две
// ОДИНАКОВЫЕ детали давали разные последовательности точек (просто начало с
// разной вершины), и раскрой дробился на лишние позиции: проверено на
// L-образном марше, где карта раскроя выросла с 12 позиций до 17.
func TestNormalizeContourMakesIdenticalContoursEqual(t *testing.T) {
	// Один и тот же квадрат, но начало в разных вершинах.
	a := []Point2D{{0, 0}, {100, 0}, {100, 40}, {0, 40}}
	b := []Point2D{{100, 40}, {0, 40}, {0, 0}, {100, 0}} // начало в другой + обратный обход

	na := normalizeContour(a)
	nb := normalizeContour(b)

	if len(na) != len(nb) {
		t.Fatalf("длины контуров разошлись: %d и %d", len(na), len(nb))
	}
	for i := range na {
		if math.Abs(na[i].X-nb[i].X) > 1e-9 || math.Abs(na[i].Y-nb[i].Y) > 1e-9 {
			t.Fatalf("точка %d разошлась: %v и %v", i, na[i], nb[i])
		}
	}
	// Начало — минимальная вершина.
	minI := 0
	for i := 1; i < len(na); i++ {
		if na[i].X < na[minI].X || (na[i].X == na[minI].X && na[i].Y < na[minI].Y) {
			minI = i
		}
	}
	if minI != 0 {
		t.Errorf("начало контура не в минимальной вершине: индекс %d", minI)
	}
	// Площадь при нормализации не меняется.
	if math.Abs(ProfileAreaMM(na)-4000) > 1e-6 {
		t.Errorf("площадь после нормализации = %.2f, ждали 4000", ProfileAreaMM(na))
	}
}

func TestNormalizeContourKeepsAlreadyCanonical(t *testing.T) {
	// Уже канонический контур не должен переписываться: возврат того же среза.
	p := []Point2D{{0, 0}, {100, 0}, {100, 40}, {0, 40}}
	got := normalizeContour(p)
	if len(got) != 4 || got[0] != p[0] || got[3] != p[3] {
		t.Errorf("канонический контур изменён: %v", got)
	}
	// Вырожденные контуры проходят как есть.
	if len(normalizeContour(nil)) != 0 {
		t.Error("nil-контур изменился")
	}
	if len(normalizeContour([]Point2D{{0, 0}, {1, 1}})) != 2 {
		t.Error("двухточечный контур изменился")
	}
}
