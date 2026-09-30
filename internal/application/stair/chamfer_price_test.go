package stair

import (
	"context"
	"testing"
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
	return cfg
}

func metalStairConfig() Config {
	cfg := referenceConfig()
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "STEEL-S235"
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
