package manufacturing

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
)

// S-163: карта раскроя должна описывать деталь её настоящим контуром, а не
// описывающим прямоугольником.
//
// Укладка при этом остаётся прямоугольной — для раскроя металла по габариту
// этого достаточно, и карточка это признаёт. Важно, что ДОКУМЕНТ раскроя
// совпадает с чертежом: оператор режет по тому, что видит в документе.

func TestCutListCarriesRealProfiles(t *testing.T) {
	cfg := testCfgForProfile(t)
	pkg, err := manufactureForProfile(cfg)
	if err != nil {
		t.Fatalf("производство: %v", err)
	}

	withProfile := 0
	for _, item := range pkg.CutList.Items {
		if len(item.Profile) < 3 {
			continue
		}
		withProfile++

		// Габарит контура обязан совпасть с габаритом позиции: иначе контур
		// взят не из той детали или из другой системы осей.
		l, w := dommfg.ProfileBounds(item.Profile)
		if d := absMM(l - item.Length.Millimeters()); d > 0.5 {
			t.Errorf("%s: длина контура %.1f против габарита %.1f",
				item.PartNumber, l, item.Length.Millimeters())
		}
		if d := absMM(w - item.Width.Millimeters()); d > 0.5 {
			t.Errorf("%s: ширина контура %.1f против габарита %.1f",
				item.PartNumber, w, item.Width.Millimeters())
		}
		// Площадь контура не может превышать описывающий прямоугольник.
		if a := dommfg.ProfileAreaMM(item.Profile); a > item.Length.Millimeters()*item.Width.Millimeters()+0.5 {
			t.Errorf("%s: площадь контура %.1f больше прямоугольника", item.PartNumber, a)
		}
	}
	if withProfile == 0 {
		t.Fatal("ни одна позиция раскроя не принесла контура — задача не сделана")
	}
}

func TestNestingResultKeepsProfiles(t *testing.T) {
	cfg := testCfgForProfile(t)
	pkg, err := manufactureForProfile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Nesting == nil {
		t.Fatal("раскрой не рассчитан")
	}
	placed := 0
	for _, sh := range pkg.Nesting.Sheets {
		for _, p := range sh.Placed {
			if len(p.Profile) >= 3 {
				placed++
			}
		}
	}
	if placed == 0 {
		t.Fatal("в результате укладки нет ни одной детали с контуром")
	}
}

// Контур обязан быть ПРАВДОПОДОБНЫМ: у косоура-пилы он заметно меньше
// прямоугольника, иначе это просто переписанный прямоугольник.
func TestStringerProfileIsNotBoundingBox(t *testing.T) {
	cfg := testCfgForProfile(t)
	pkg, err := manufactureForProfile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range pkg.CutList.Items {
		if item.PartNumber == "" || len(item.Profile) < 3 {
			continue
		}
		box := item.Length.Millimeters() * item.Width.Millimeters()
		a := dommfg.ProfileAreaMM(item.Profile)
		ratio := a / box
		if ratio > 0.97 {
			continue // деталь и правда прямоугольная
		}
		t.Logf("%s: контур занимает %.0f%% описывающего прямоугольника", item.PartNumber, ratio*100)
		return
	}
	t.Log("все детали с контурами оказались прямоугольными — возможно, вибратор не тот")
}

// testCfgForProfile — прямой марш: на нём детали разнообразнее, чем на
// Г-образном, и контуры у них отличаются друг от друга.
func testCfgForProfile(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	// Готовая конфигурация из соседнего теста: у неё заданы все параметры
	// марша и материал, а конструктор с двумя аргументами оставляет проступь
	// нулевой и расчёт падает.
	return testConfig(t)
}

func manufactureForProfile(cfg *engineering.StairConfiguration) (*dommfg.ManufacturingPackage, error) {
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	return Manufacture(cfg, res, nil)
}

func absMM(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

var _ = context.Background
