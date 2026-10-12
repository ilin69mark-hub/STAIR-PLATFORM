package pricing

import (
	"context"
	"reflect"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	enggeo "stairplatform/internal/engine/geometry"
	engmfg "stairplatform/internal/engine/manufacturing"
)

// testDataset собирает полный конвейер геометрия→производство для теста цены.
func testDataset(t *testing.T) *dommfg.ManufacturingCostDataset {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	// Материал задаём ЯВНО. Раньше он выводился из толщины: 50 мм подходили
	// стали (диапазон 2–60), и тесты молча проверяли стальной марш на
	// конфигурации без материала. С выходом стали на выпуск 3–8 мм такой
	// вывод стал давать дуб, и тесты проверяли бы не то.
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "STEEL-S235"
	cfg.StepThickness = mustLength(t, 40)

	gen, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := engmfg.Manufacture(cfg, gen)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ds, err := engmfg.PrepareCost(pkg, reg, engmfg.DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

func mustLength(t *testing.T, mm float64) engineering.Length {
	t.Helper()
	l, err := engineering.NewLength(mm)
	if err != nil {
		t.Fatalf("NewLength(%v): %v", mm, err)
	}
	return l
}

func TestDefaultRates(t *testing.T) {
	r := DefaultRates()
	if err := r.Validate(); err != nil {
		t.Fatalf("default rates must validate: %v", err)
	}
	if r.Currency != domprc.CurrencyRUB {
		t.Fatalf("default currency = %+v, want RUB", r.Currency)
	}
}

// TestEveryCatalogMaterialHasPrice (этап 1, вариант Б) — инвариант:
// каждый материал встроенного каталога обязан иметь цену в DefaultRates.
// Без этого новый материал проходит каталог, а расчёт падает в рантайме
// (nil/0 ₽ за кг) — цена становится фиктивной.
func TestEveryCatalogMaterialHasPrice(t *testing.T) {
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		t.Fatalf("material registry: %v", err)
	}
	rates := DefaultRates()
	for _, m := range reg.Materials() {
		price, ok := rates.Material[m.Code]
		if !ok {
			t.Errorf("material %q has no price in DefaultRates", m.Code)
			continue
		}
		if price.Minor() <= 0 {
			t.Errorf("material %q has non-positive price %v", m.Code, price)
		}
	}
	// Все цены строго положительны и в разумном диапазоне (₽/кг).
	for code, price := range rates.Material {
		if perKg := price.Minor(); perKg < 1000 || perKg > 200000 {
			t.Errorf("material %q price %d minor units looks implausible", code, perKg)
		}
	}
}

func TestPriceChainConsistency(t *testing.T) {
	ds := testDataset(t)
	b, err := Price(ds, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	// Крепёж и герметик входят в базу накладных наравне с материалами и
	// работой: дюбель стоит 15 ₽, картридж герметика 500 ₽, но наценку и
	// налог на них тоже платят.
	if b.ProductionCost.Minor() != b.Material.Minor()+b.Machine.Minor()+b.Labor.Minor()+b.Fastener.Minor()+b.Sealant.Minor()+b.Overhead.Minor() {
		t.Fatal("production cost chain inconsistent")
	}
	if b.PreTax.Minor() != b.ProductionCost.Minor()+b.Margin.Minor()-b.Discount.Minor() {
		t.Fatal("pre-tax chain inconsistent")
	}
	if b.FinalPrice.Minor() != b.PreTax.Minor()+b.Tax.Minor() {
		t.Fatal("final price chain inconsistent")
	}
	// 5 строк крепежа (глухарь и дюбель по полю, по перекрытию, плюс обойма
	// выреза) + 1 строка герметика + 1 материал + станок + труд + накладные
	// + маржа + скидка + НДС + по одной строке на покупную позицию (S-162).
	purchasedLines := 0
	for _, l := range b.Lines {
		if l.Source != "" && len(l.Source) >= 9 && l.Source[:9] == "purchased" {
			purchasedLines++
		}
	}
	want := 13 + purchasedLines
	if len(b.Lines) != want {
		t.Fatalf("lines = %d, want %d (13 базовых + %d покупных)", len(b.Lines), want, purchasedLines)
	}
	if purchasedLines == 0 {
		t.Fatal("покупные позиции не попали в смету ни одной строкой — разрыв вернулся")
	}
}

func TestPriceDeterminism(t *testing.T) {
	ds := testDataset(t)
	a, err := Price(ds, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Price(ds, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("pricing must be deterministic")
	}
}

func TestPriceErrors(t *testing.T) {
	ds := testDataset(t)

	if _, err := Price(nil, DefaultRates()); err == nil {
		t.Fatal("nil dataset must be rejected")
	}

	// некорректный dataset (нарушена целостность сумм).
	bad := testDataset(t)
	bad.Mass = 1
	if _, err := Price(bad, DefaultRates()); err == nil {
		t.Fatal("invalid dataset must be rejected")
	}

	// материал без цены в ставках.
	rates := DefaultRates()
	delete(rates.Material, "STEEL-S235")
	if _, err := Price(ds, rates); err == nil {
		t.Fatal("material without price must be rejected")
	}

	// материал с нулевой площадью деталей.
	badDS := testDataset(t)
	badDS.MaterialConsumption[0].PartArea = 0
	if _, err := Price(badDS, DefaultRates()); err == nil {
		t.Fatal("zero part area must be rejected")
	}
}

// TestPriceExactValues фиксирует детерминированные значения для эталонной
// конфигурации (n=15, стандартные ставки) как инвариант расчёта.
func TestPriceExactValues(t *testing.T) {
	ds := testDataset(t)
	b, err := Price(ds, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		got  domprc.Money
		want int64
	}{
		// Эталон — стальной марш: 15 ступеней, косоур-гребёнка и проступи
		// листом 12 мм, 98 деталей (пятки выреза включены).
		// Материал 242 807,01 ₽, финал 523 509,14 ₽.
		//
		// Значения пересчитаны под раскрой по полю лазерного стола (шпатка
		// технолога 2026-10-05): косоур длиной 4845 мм длиннее поля 3000 мм и
		// режется на 2 куска со сварным стыком «шип-паз», отсюда 98 деталей
		// вместо 96, +2 операции и контур стыка в машинном времени. Лист стали
		// теперь один типоразмер 6000×1500, и трёх листов хватает на 8/5/4 мм
		// — площадь закупки выросла с 21,1 до 27,0 млн мм², отсюда и подорожание.
		//
		// Материал вырос с 128 150 ₽ (лист 8 мм) и с 163 792 ₽ (раскрой по полю
		// стола, но всё ещё 8 мм): заготовка стала 12 мм — верх выпуска листа,
		// выросшего вместе с нормативом на листовой косоур (строки 8 / 10 / 12
		// мм). Масса эталона 893 кг.
		//
		// До починки заготовки деталей те же конфигурации давали 10 364 кг и
		// 3 271 385 ₽: косоур брался габаритным блоком 4050×2660×50 мм, то
		// есть 1,08 м³ стали на один косоур. Правдоподобие массы проверяется
		// в engine/manufacturing (TestPrepareCostMetrics).
		// Крепёж добавлен 2026-10-08: 238 штук по 15 ₽ = 3 570 ₽. Это немного,
		// но попадает в базу накладных и маржи, поэтому итог растёт сильнее
		// самой суммы: overhead +71 400, margin +128 520.
		{"material", b.Material, 24280701},
		{"machine", b.Machine, 3686386},
		{"labor", b.Labor, 1470000},
		{"fastener", b.Fastener, 357000},
		// Герметик: площадь швов 0,6922 м² × 1 мм = 0,6922 л, делим на
		// 0,31 л картриджа и округляем ВВЕРХ → 3 картриджа × 500 ₽.
		{"sealant", b.Sealant, 150000},
		{"overhead", b.Overhead, 5988817},
		{"production", b.ProductionCost, 35932904},
		{"margin", b.Margin, 10779871},
		{"discount", b.Discount, 2335639},
		{"pre-tax", b.PreTax, 44377136},
		{"tax", b.Tax, 8875427},
		{"final", b.FinalPrice, 53252563},
	}
	for _, c := range checks {
		if c.got.Minor() != c.want {
			t.Fatalf("%s = %d, want %d", c.name, c.got.Minor(), c.want)
		}
	}
}
