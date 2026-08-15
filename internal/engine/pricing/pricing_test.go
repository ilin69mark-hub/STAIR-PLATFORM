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
	cfg.StepThickness = mustLength(t, 40)

	gen, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := engmfg.Manufacture(cfg, gen)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := engmfg.PrepareCost(pkg, engmfg.DefaultMaterialRegistry(), engmfg.DefaultMachineRates())
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

func TestPriceChainConsistency(t *testing.T) {
	ds := testDataset(t)
	b, err := Price(ds, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	if b.ProductionCost.Minor() != b.Material.Minor()+b.Machine.Minor()+b.Labor.Minor()+b.Overhead.Minor() {
		t.Fatal("production cost chain inconsistent")
	}
	if b.PreTax.Minor() != b.ProductionCost.Minor()+b.Margin.Minor()-b.Discount.Minor() {
		t.Fatal("pre-tax chain inconsistent")
	}
	if b.FinalPrice.Minor() != b.PreTax.Minor()+b.Tax.Minor() {
		t.Fatal("final price chain inconsistent")
	}
	if len(b.Lines) != 7 {
		t.Fatalf("lines = %d, want 7 (1 material + machine + labor + overhead + margin + discount + tax)", len(b.Lines))
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
		{"material", b.Material, 159359787},
		{"machine", b.Machine, 1444667},
		{"labor", b.Labor, 480000},
		{"overhead", b.Overhead, 32256891},
		{"production", b.ProductionCost, 193541345},
		{"margin", b.Margin, 58062404},
		{"discount", b.Discount, 12580187},
		{"pre-tax", b.PreTax, 239023562},
		{"tax", b.Tax, 47804712},
		{"final", b.FinalPrice, 286828274},
	}
	for _, c := range checks {
		if c.got.Minor() != c.want {
			t.Fatalf("%s = %d, want %d", c.name, c.got.Minor(), c.want)
		}
	}
}
