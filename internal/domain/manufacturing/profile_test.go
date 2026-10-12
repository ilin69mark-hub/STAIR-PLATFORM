package manufacturing

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// Функции контура живут в домене, а не в ядре: это данные документа раскроя,
// и доменный слой не зависит от internal/geometry (ADR-0006). Поэтому и
// тесты здесь — покрытие пакета считает только собственные тесты.

func TestProfileAreaMM(t *testing.T) {
	// Прямоугольник 100×40.
	rect := []Point2D{{0, 0}, {100, 0}, {100, 40}, {0, 40}}
	if got := ProfileAreaMM(rect); math.Abs(got-4000) > 1e-6 {
		t.Errorf("площадь прямоугольника = %.2f, ждали 4000", got)
	}
	// Обход в обратную сторону даёт ту же площадь: знак не должен влиять.
	rev := []Point2D{{0, 40}, {100, 40}, {100, 0}, {0, 0}}
	if got := ProfileAreaMM(rev); math.Abs(got-4000) > 1e-6 {
		t.Errorf("площадь при обратном обходе = %.2f, ждали 4000", got)
	}
	// Вырожденные контуры дают ноль, а не мусор.
	if got := ProfileAreaMM(nil); got != 0 {
		t.Errorf("площадь nil-контура = %f", got)
	}
	if got := ProfileAreaMM([]Point2D{{0, 0}, {10, 0}}); got != 0 {
		t.Errorf("площадь двухточечного контура = %f", got)
	}
}

func TestProfileBounds(t *testing.T) {
	l, w := ProfileBounds([]Point2D{{10, 20}, {110, 20}, {110, 60}, {10, 60}})
	if math.Abs(l-100) > 1e-6 || math.Abs(w-40) > 1e-6 {
		t.Errorf("габарит контура = %.2f×%.2f, ждали 100×40", l, w)
	}
	// Контур может быть сдвинут — габарит от этого не меняется.
	l2, w2 := ProfileBounds([]Point2D{{-50, -10}, {50, -10}, {50, 30}})
	if math.Abs(l2-100) > 1e-6 || math.Abs(w2-40) > 1e-6 {
		t.Errorf("габарит сдвинутого контура = %.2f×%.2f, ждали 100×40", l2, w2)
	}
	if l, w := ProfileBounds(nil); l != 0 || w != 0 {
		t.Errorf("габарит nil-контура = %.2f×%.2f", l, w)
	}
	// Одна точка — вырожденный контур, но габарит вычислим.
	l1, w1 := ProfileBounds([]Point2D{{5, 7}})
	if l1 != 0 || w1 != 0 {
		t.Errorf("габарит одной точки = %.2f×%.2f, ждали 0×0", l1, w1)
	}
}

func TestProfilesMatchBounds(t *testing.T) {
	// Совпадение в пределах допуска.
	p := []Point2D{{0, 0}, {2428.7, 0}, {2428.7, 300}, {0, 300}}
	if !ProfilesMatchBounds(p, 2428.7, 300, 0.5) {
		t.Error("точный контур не признан совпадающим с габаритом")
	}
	if !ProfilesMatchBounds(p, 2428.9, 300.4, 0.5) {
		t.Error("контур в пределах допуска не признан совпадающим")
	}
	// Именно тот случай с косоуром: контур тела вдвое длиннее заготовки.
	if ProfilesMatchBounds(p, 4845.3, 300, 0.5) {
		t.Error("контур, взятый не из той заготовки, признан совпадающим")
	}
	// Без контура и с вырожденным — никогда.
	if ProfilesMatchBounds(nil, 100, 40, 0.5) {
		t.Error("nil-контур признан совпадающим")
	}
	if ProfilesMatchBounds([]Point2D{{0, 0}, {10, 0}}, 10, 0, 0.5) {
		t.Error("двухточечный контур признан совпадающим")
	}
}

// PurchasedParts собирается из каталога engineering.PurchasedParts(). Сверка
// обязательна: пока она не было, каталог деталей жил без единого
// вызывающего (S-180), и новая покупная позиция появилась бы в каталоге
// молча — без вида детали и без цены.
func TestPurchasedPartsCoverCatalog(t *testing.T) {
	specs := engineering.PurchasedParts()
	got := PurchasedParts()

	if len(got) != len(specs) {
		t.Fatalf("позиций каталога %d, а в реестре %d — часть выпала без вида детали",
			len(specs), len(got))
	}
	for _, s := range specs {
		if _, ok := purchasedKindByCatalogID[s.ID]; !ok {
			t.Errorf("позиция каталога %q (%s) не сопоставлена с видом детали", s.ID, s.Name)
		}
	}
	// Порядок стабильный: иначе строки сметы прыгали бы между расчётами.
	for i := 1; i < len(got); i++ {
		if got[i-1].CatalogID > got[i].CatalogID {
			t.Fatalf("порядок нарушен: %q перед %q", got[i-1].CatalogID, got[i].CatalogID)
		}
	}
}

func TestPurchasedPartCatalogID(t *testing.T) {
	// Вид детали, которого в каталоге нет, даёт пустую строку, а не ошибку:
	// отсутствие позиции — это отсутствие цены, а не падение расчёта.
	if id := PurchasedPartCatalogID(PartRailTop); id != "RAIL-P" {
		t.Errorf("PartRailTop -> %q, ждали RAIL-P", id)
	}
	if id := PurchasedPartCatalogID(PartTread); id != "" {
		t.Errorf("PartTread -> %q, хотя его в каталоге покупных нет", id)
	}
}
