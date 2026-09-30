package manufacturing

import (
	"reflect"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

func TestPrepareCostMetrics(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	ds, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}

	if ds.PartCount != 32 {
		t.Fatalf("part count = %d, want 32", ds.PartCount)
	}
	if ds.FastenerCount != 0 {
		t.Fatalf("fastener count = %d, want 0", ds.FastenerCount)
	}
	// операции: 32 детали × 2 (Cutting + Finishing).
	if ds.OperationCount != 64 {
		t.Fatalf("operation count = %d, want 64", ds.OperationCount)
	}
	if ds.OperationPlan == nil || len(ds.OperationPlan.Parts) != 32 {
		t.Fatalf("operation plan must contain 32 part routes, got %+v", ds.OperationPlan)
	}
	if len(ds.MaterialConsumption) != 1 || ds.MaterialConsumption[0].MaterialCode != "STEEL-S235" {
		t.Fatalf("consumption = %+v, want single STEEL-S235", ds.MaterialConsumption)
	}

	// Метрики производственного набора. Значения — после починки заготовки
	// деталей: заготовка это лист лазерного раскроя, а не габаритный блок.
	//
	// Масса проверяется не «точным числом», а правдоподобием: именно точное
	// число раньше держало баг. 16-ступенчатый стальной марш с проступями,
	// подступенками и косоуром весит сотни килограммов; тест закреплял
	// 10 364 кг, потому что косоур брался блоком 4050×2660×50 мм. По цене
	// 100 ₽/кг это 1,4 млн ₽ материала и 2,6 млн ₽ с наценкой — в девять с
	// половиной раз выше цельного дубового марша.
	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"part area", ds.PartArea, 7.8948e+06},
		{"sheet area", ds.SheetArea, 1.8e+07},
		{"waste area", ds.WasteArea, 1.01052e+07},
		{"volume", ds.Volume, 6.31584e+07},
		{"surface area", ds.SurfaceArea, 1.6421056e+07},
		{"waste percent", ds.WastePercent, 1.01052e+07 / 1.8e+07},
		{"utilization", ds.Utilization, 7.8948e+06 / 1.8e+07},
		{"machine time", ds.EstimatedMachineTime, 103.466},
		{"labor time", ds.EstimatedLaborTime, 96},
		{"production time", ds.EstimatedProductionTime, 199.466},
	}
	for _, c := range checks {
		if !nearlyEqual(c.got, c.want) {
			t.Fatalf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	// Правдоподобие массы: 16-ступенчатый стальной марш — сотни кг. Верхняя
	// граница 1200 кг оставляет запас на заготовку, но отсекает возврат
	// ошибки с габаритным блоком (там было бы тонны).
	if ds.Mass < 250 || ds.Mass > 1200 {
		t.Fatalf("mass = %v kg — неправдоподобно для 16 ступеней: проверь заготовку деталей (лист, а не блок)", ds.Mass)
	}
	if err := ds.Validate(); err != nil {
		t.Fatalf("dataset must validate: %v", err)
	}
}

func TestPrepareCostConsumption(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	ds, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	c := ds.MaterialConsumption[0]
	// масса потребления по материалу = суммарная масса изделия.
	if !nearlyEqual(c.Mass, ds.Mass) {
		t.Fatalf("consumption mass = %v, want %v", c.Mass, ds.Mass)
	}
	// расход листа = сумма площадей листов из раскроя.
	if !nearlyEqual(c.SheetArea, ds.SheetArea) {
		t.Fatalf("consumption sheet area = %v, want %v", c.SheetArea, ds.SheetArea)
	}
	// площадь деталей по каталогу = сумма площадей из CutList.
	var fromCutList float64
	for _, item := range pkg.CutList.Items {
		fromCutList += float64(item.Quantity) * item.Length.Millimeters() * item.Width.Millimeters()
	}
	if !nearlyEqual(c.PartArea, fromCutList) {
		t.Fatalf("consumption part area = %v, want %v", c.PartArea, fromCutList)
	}
}

func TestPrepareCostDeterminism(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	a, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	b, err := PrepareCost(pkg, mustMaterials(t), DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("cost preparation must be deterministic")
	}
}

func TestPrepareCostErrors(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := PrepareCost(nil, mustMaterials(t), DefaultMachineRates()); err == nil {
		t.Fatal("nil package must be rejected")
	}
	if _, err := PrepareCost(pkg, nil, DefaultMachineRates()); err == nil {
		t.Fatal("nil material registry must be rejected")
	}
	// материал деталей отсутствует в реестре.
	empty, err := dommfg.NewMaterialRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareCost(pkg, empty, DefaultMachineRates()); err == nil {
		t.Fatal("material missing from registry must be rejected")
	}
	// площадь по CutList превышает площадь листов раскроя.
	bad := &dommfg.ManufacturingPackage{}
	*bad = *pkg
	bad.CutList.Items[1].Quantity = 9999
	if _, err := PrepareCost(bad, mustMaterials(t), DefaultMachineRates()); err == nil {
		t.Fatal("cut list exceeding nesting sheets must be rejected")
	}
}
