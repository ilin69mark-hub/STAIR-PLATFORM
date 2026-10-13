package geometry

// railing_catalog_test.go — сверка констант движка ограждения с каталогом
// (S-209): ни одного числа ограждения в двух местах без сверки.
//
// Движок строит из своих констант, смета — из каталога engineering.
// Расхождение проявилось бы в готовом изделии, а не в тесте: этот класс
// уже кусался дважды (S-180 — диаметры, S-206 — длины труб).
//
// Что сверяется, а что нет:
//   - диаметры/стенки труб, габариты фитингов — здесь;
//   - высота стойки уже закрыта TestSteelPostMatchesCatalog — не дублируем;
//   - внутренности узлов (шпилька Ø12, шарнир, отверстия Ø9 и их шаг)
//     живут только в движке: в каталоге под них нет полей, а число
//     отверстий уже едино через RailFlangeHoles (смета читает его).
//     Выдумывать поля каталога ради сверки — хуже честного списка здесь.

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
	"stairplatform/models"
)

func mustTubeSpec(t *testing.T, id string) engineering.PartSpec {
	t.Helper()
	spec, ok := engineering.PartByID(id)
	if !ok {
		t.Fatalf("в каталоге нет %s", id)
	}
	return spec
}

// TestRailingTubeConstsMatchCatalog — диаметры и стенки труб движка равны
// каталожным (S-209). Движок рисует из констант, смета режет из каталога:
// разойдись они — труба в модели и труба в заказе разные.
func TestRailingTubeConstsMatchCatalog(t *testing.T) {
	for _, c := range []struct {
		id       string
		od, wall float64
	}{
		{"POST-001", balusterD, balusterWall},
		{"RIGL-001", rigelD, rigelWall},
	} {
		spec := mustTubeSpec(t, c.id)
		if spec.TubeOD != c.od || spec.TubeWall != c.wall {
			t.Errorf("%s: движок Ø%.1f×%.1f, каталог Ø%.1f×%.1f",
				c.id, c.od, c.wall, spec.TubeOD, spec.TubeWall)
		}
	}
}

// TestRailingFittingConstsMatchCatalog — габариты фитингов движка равны
// каталожным Dim (S-209). Сверяем собственные числа напрямую с каталогом
// (не bbox сборки: сборка наклонена под уклон марша, и её bbox зависит
// от уклона, а константы и Dim — нет). Расхождение в любую сторону валит
// тест: какая сторона права — сверять с эталоном FreeCAD.
func TestRailingFittingConstsMatchCatalog(t *testing.T) {
	for _, c := range []struct {
		id     string
		values [3]float64 // L, W, H движка против DimL, DimW, DimH
	}{
		// Наконечник: площадка 70×50,8 (длина овала, ширина; высоту
		// меряем сборкой в следующем тесте).
		{"TIP-001", [3]float64{holderPlateLen, holderPlateW, -1}},
		// Фланец: диск Ø86×8.
		{"FLNG-001", [3]float64{clampDiscD, clampDiscD, clampDiscT}},
		// Муфта: Ø43×34 (34 — S-209, а не 805,7: короткая муфта,
		// а не чехол на всю стойку).
		{"SLIP-001", [3]float64{clampMufD, clampMufD, clampMufH}},
		// Колпачок: Ø88×17.
		{"DECO-001", [3]float64{capOD, capOD, capH}},
		// Заглушка: трубка Ø39,2 в полость (крышка 42,4 — lidD =
		// balusterD, отдельного числа нет).
		{"ENDP-001", [3]float64{endCapD, endCapD, endCapD}},
	} {
		spec, ok := engineering.PartByID(c.id)
		if !ok {
			t.Fatalf("в каталоге нет %s", c.id)
		}
		want := [3]float64{spec.DimL, spec.DimW, spec.DimH}
		for i, v := range c.values {
			if v < 0 {
				continue
			}
			if math.Abs(v-want[i]) > 1e-9 {
				t.Errorf("%s: константа движка %.2f, каталог %.2f",
					c.id, v, want[i])
			}
		}
	}
}

// TestRailingHolderHeightStable — высота собранного наконечника не
// дрейфует: стек втулка/крышка/шпилька/шарнир/площадка меряем сборкой,
// потому что высотой нет отдельной константы.
//
// ВНИМАНИЕ: с TIP-001 DimH (95,4) высоту НЕ сверяем — намеренно.
// DimH снят с эталона, где наконечник регулируемый смоделирован
// в выдвинутом положении (7 тел), а движок строит номинал (5 тел):
// 92,8 против 95,4 — разные разрезы одного узла, а не расхождение
// (та же история, что в seed/README про TIP). Связь с каталогом —
// через footprint площадки 70×50,8 (гейт выше). Здесь фиксируем
// стабильность собственной сборки на двух уклонах.
func TestRailingHolderHeightStable(t *testing.T) {
	m, err := models.Load()
	if err != nil {
		t.Fatalf("манифест: %v", err)
	}
	ref, ok := m.ByID("RAIL-P")
	if !ok || ref.Tube == nil {
		t.Fatal("в манифесте нет RAIL-P с tube")
	}
	// Уклон эталона из bbox поручня: (h−D)/(l−D) вместо h/l, чтобы не
	// тащить диаметр трубы в числитель уклона (S-206: bbox — проекция
	// плюс диаметр, а не чистая геометрия).
	refSlope := (ref.BBox.H - ref.Tube.OD) / (ref.BBox.L - ref.Tube.OD)
	for _, c := range []struct {
		name  string
		slope float64
		want  float64
	}{
		{"уклон эталона", refSlope, 92.8},
		{"уклон сида", 180.0 / 270.0, 91.6},
	} {
		sols := railHolderAt(ProductionQuality, 0, 0, 0, c.slope)
		if len(sols) == 0 {
			t.Fatal("railHolderAt не построил тел")
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, s := range sols {
			bb := kerngeo.SolidBoundingBox(s)
			lo = math.Min(lo, bb.Min.Z)
			hi = math.Max(hi, bb.Max.Z)
		}
		if got := hi - lo; math.Abs(got-c.want) > 0.5 {
			t.Errorf("высота наконечника %s %.1f, ждали %.1f",
				c.name, got, c.want)
		}
	}
}
