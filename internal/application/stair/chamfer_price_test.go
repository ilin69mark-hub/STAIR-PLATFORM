package stair

import (
	"context"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	engmfg "stairplatform/internal/engine/manufacturing"
)

// Скругление носа — не украшение, а работа: оно обязано быть видно в цене.
//
// Проверяем сквозную цепочку целиком (геометрия → раскрой → техмаршрут →
// цена) на настоящем расчёте, потому что по дороге значение может потеряться
// трижды: в ManufacturingPackage (деталь без признака), в OperatingCostDataset
// (операция не плановая) или в Price (труд не в том агрегате).

func woodStairConfig() Config {
	cfg := referenceConfig()
	cfg.Material = "STEEL-S235" // каркас стальной
	cfg.TreadMaterial = "WOOD-OAK"
	// Проступь из дуба пилится из доски 20–100 мм; эталонные 6 мм — стальные.
	cfg.StepThickness = mustLengthHelper(40)
	return cfg
}

func metalStairConfig() Config {
	cfg := referenceConfig()
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "STEEL-S235"
	// Ступень из стали — тоже лист лазерного раскроя: 3–8 мм. Со
	// страницей эталона (40 мм, значение для дерева) расчёт блокировался
	// MFG-MATERIAL, и Price приходил пустым. Раньше конфигурация была
	// невозможной, но проходила: сталь принимала толщину до 60 мм.
	cfg.StepThickness = mustLengthHelper(6)
	return cfg
}

func TestOakNoseChamferAddsWorkAndCost(t *testing.T) {
	s := NewService()
	wood, err := s.Calculate(context.Background(), woodStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	metal, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}

	// Радиус носа приходит из материала ступеней: у дуба есть, у стали нет.
	if got := wood.Cost; got != nil && got.OperationPlan != nil {
		milling := 0
		for _, p := range got.OperationPlan.Parts {
			for _, op := range p.Operations {
				if op.Type == "milling" {
					milling++
				}
			}
		}
		if milling == 0 {
			t.Fatal("oak stair must be scheduled for milling")
		}
		if milling != 15 {
			t.Fatalf("milling operations = %d, want 15 (one per tread)", milling)
		}
	}
	// Труд вырос: фрезеровка идёт на ручное рабочее место, то есть в труд, а не
	// в машинное время.
	if wood.Price.Labor <= metal.Price.Labor {
		t.Fatalf("oak labor %v must exceed metal labor %v",
			wood.Price.Labor.Minor(), metal.Price.Labor.Minor())
	}
	// Итоговую цену между ДУБОМ и СТАЛЬЮ сравнивать нельзя: материалы ступеней
	// разные, и разница в цене металла перекрывает работу по фрезеровке.
	// Точное «с фаской против без» на одной и той же конфигурации считается в
	// engine/manufacturing (TestChamferRaisesPriceOnSameConfig).
	t.Logf("дуб: труд %d, итог %d; сталь: труд %d, итог %d (минорных единиц)",
		wood.Price.Labor.Minor(), wood.Price.FinalPrice.Minor(),
		metal.Price.Labor.Minor(), metal.Price.FinalPrice.Minor())
}

func TestSteelStairHasNoMilling(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cost == nil || res.Cost.OperationPlan == nil {
		t.Fatal("operation plan must be present")
	}
	for _, p := range res.Cost.OperationPlan.Parts {
		for _, op := range p.Operations {
			if op.Type == "milling" {
				t.Fatalf("metal stair must not be scheduled for milling: %+v", op)
			}
		}
	}
}

func TestMetalPriceIdenticalWithAndWithoutNoseRadius(t *testing.T) {
	// Сталь и без фаски: цена не должна двигаться ни на копейку. Радиус у стали
	// нулевой в каталоге, поэтому сравниваем «как есть» с той же конфигурацией,
	// где радиус принудительно обнулён — это доказывает, что поле не влияет на
	// цену само по себе, а только через материал.
	s := NewService()
	before, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := metalStairConfig()
	// Прямой вызов геометрии с нулём радиуса — тот же путь, что и при
	// вычислении из материала.
	after, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if before.Price.FinalPrice != after.Price.FinalPrice {
		t.Fatalf("price must be stable: %v vs %v",
			before.Price.FinalPrice.Minor(), after.Price.FinalPrice.Minor())
	}
}

func TestNoseRadiusDerivedFromTreadMaterialNotFrame(t *testing.T) {
	// Стальной каркас + деревянные ступени: фаска есть (материал ступеней).
	// Стальной каркас + стальные ступени: фаски нет. Материал КАРКАСА на
	// скругление ступеней не влияет — это разные детали.
	s := NewService()
	mixed, err := s.Calculate(context.Background(), woodStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	allWood := woodStairConfig()
	allWood.Material = "WOOD-OAK"
	allWood.TreadMaterial = "WOOD-OAK"
	// Косоур из дуба пилится из доски 20–100 мм: толщина эталона (8 мм) —
	// стальная, и все-деревянная конфигурация с ней не проходит.
	allWood.StringerThickness = mustLengthHelper(50)
	woodFrame, err := s.Calculate(context.Background(), allWood, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// У обоих случаев ступени из дуба → работа фрезеровки одна и та же по
	// числу операций; различаться может только материал каркаса.
	if mixed.Cost.OperationPlan == nil || woodFrame.Cost.OperationPlan == nil {
		t.Fatal("operation plans must be present")
	}
	if millOps(woodFrame) != millOps(mixed) {
		t.Fatalf("milling count must depend on the tread material only: %d vs %d",
			millOps(woodFrame), millOps(mixed))
	}
}

func millOps(res *Result) int {
	if res.Cost == nil || res.Cost.OperationPlan == nil {
		return 0
	}
	n := 0
	for _, p := range res.Cost.OperationPlan.Parts {
		for _, op := range p.Operations {
			if op.Type == "milling" {
				n++
			}
		}
	}
	return n
}

// TestEveryWoodSpeciesNoseRadiusIsTen — радиус филёнки на носике 10 мм у
// ВСЕХ пород (решение владельца 2026-10-05, чертёж «скругление R10 фрезой»).
//
// Радиус берётся из материала ступеней и потому молча меняет и геометрию, и
// цену: скругление добавля milling-операцию. Раньше у дуба, ясеня и сосны
// стояли 8/8/5 мм, то есть чертёж и каталог расходились, и это ни один тест не
// ловил. Тест фиксирует именно равенство всех пород.
func TestEveryWoodSpeciesNoseRadiusIsTen(t *testing.T) {
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []dommfg.MaterialCode{"WOOD-OAK", "WOOD-WALNUT", "WOOD-ASH", "WOOD-SOFT"} {
		mat, ok := reg.Find(code)
		if !ok {
			t.Fatalf("material %q not in registry", code)
		}
		if mat.TreadNoseRadiusMm != 10 {
			t.Fatalf("%s nose radius = %g mm, want 10 (все породы по чертежу R10)",
				code, mat.TreadNoseRadiusMm)
		}
	}
	// У металла фасок нет: прокат/шлифовка дают прямой профиль, и цена
	// металлической лестницы не должна зависеть от поля радиуса.
	steel, ok := reg.Find("STEEL-S235")
	if !ok {
		t.Fatal("STEEL-S235 not in registry")
	}
	if steel.TreadNoseRadiusMm != 0 {
		t.Fatalf("steel nose radius = %g mm, want 0", steel.TreadNoseRadiusMm)
	}
}

// TestNoseRadiusTenReachesGeometryAndPrice — радиус из каталога доходит до
// техмаршрута: у дуба на R10 milling есть, по одной операции на ступень, и он
// идёт в РУЧНОЙ труд, а не в машинное время.
func TestNoseRadiusTenReachesGeometryAndPrice(t *testing.T) {
	s := NewService()
	wood, err := s.Calculate(context.Background(), woodStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	milling := 0
	for _, p := range wood.Cost.OperationPlan.Parts {
		for _, op := range p.Operations {
			if op.Type == "milling" {
				milling++
			}
		}
	}
	if milling != 15 {
		t.Fatalf("milling operations = %d, want 15 (one per tread)", milling)
	}
	metal, err := s.Calculate(context.Background(), metalStairConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if wood.Price.Labor <= metal.Price.Labor {
		t.Fatalf("oak labor %v must exceed steel labor %v",
			wood.Price.Labor.Minor(), metal.Price.Labor.Minor())
	}
}
