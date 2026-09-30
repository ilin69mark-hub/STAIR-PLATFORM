package manufacturing

import (
	"math"
	"reflect"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
)

func TestPlanOperationsRoutes(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Parts) != 32 {
		t.Fatalf("plan parts = %d, want 32", len(plan.Parts))
	}
	for _, part := range plan.Parts {
		if len(part.Operations) != 2 {
			t.Fatalf("part %q operations = %d, want 2", part.PartNumber, len(part.Operations))
		}
		cut, finish := part.Operations[0], part.Operations[1]
		if cut.Type != dommfg.OpCutting || cut.Sequence != 1 || cut.Machine != dommfg.MachineLaserCutter {
			t.Fatalf("part %q first operation = %+v, want cutting on laser", part.PartNumber, cut)
		}
		if finish.Type != dommfg.OpFinishing || finish.Sequence != 2 || finish.Machine != dommfg.MachineManualWorkstation {
			t.Fatalf("part %q second operation = %+v, want finishing on manual station", part.PartNumber, finish)
		}
		if !cut.OperatorRequired || !finish.OperatorRequired {
			t.Fatalf("part %q operations must require an operator", part.PartNumber)
		}
	}
}

func TestPlanOperationsCutTimes(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range plan.Parts {
		// Cutting: setup 2 + периметр/2000 мм/мин; Finishing: 3 мин.
		p := findPart(pkg, part.PartNumber)
		perimeter := 2 * (p.Length.Millimeters() + p.Width.Millimeters())
		wantCut := 2 + perimeter/2000
		if !nearlyEqual(part.Operations[0].EstimatedTime, wantCut) {
			t.Fatalf("part %q cut time = %v, want %v", part.PartNumber, part.Operations[0].EstimatedTime, wantCut)
		}
		if !nearlyEqual(part.Operations[1].EstimatedTime, 3) {
			t.Fatalf("part %q finish time = %v, want 3", part.PartNumber, part.Operations[1].EstimatedTime)
		}
	}
}

func TestPlanOperationsTotalTime(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	// stringers 2×(2+perim/2000) + treads 15×3.21 + risers 15×3.04
	// + 32×3 finish. Периметр заготовки косоура — полоса 4050×308, а не
	// блок 4050×2660: время резки считается по длине реза, и с блоком
	// косоур «пилил» бы в полтора раза больше, чем есть металла.
	want := 2*(2+2*(4050.0+308)/2000) + 15*(2+(800.0+310)/1000) + 15*(2+2*(800.0+140)/2000) + 32*3
	if !nearlyEqual(plan.TotalTime(), want) {
		t.Fatalf("total time = %v, want %v", plan.TotalTime(), want)
	}
}

func TestPlanOperationsDeterminism(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	a, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	b, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("operation planning must be deterministic")
	}
}

func TestPlanOperationsErrors(t *testing.T) {
	cfg := testConfig(t)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := PlanOperations(nil, DefaultMachineRates()); err == nil {
		t.Fatal("nil package must be rejected")
	}
	// материал деталей без скорости реза.
	rates := DefaultMachineRates()
	rates.CutSpeed = map[dommfg.MaterialCode]float64{}
	if _, err := PlanOperations(pkg, rates); err == nil {
		t.Fatal("missing cut speed must be rejected")
	}
	// отрицательная константа.
	bad := DefaultMachineRates()
	bad.SetupCutMin = -1
	if _, err := PlanOperations(pkg, bad); err == nil {
		t.Fatal("negative setup must be rejected")
	}
}

func TestMachineRatesValidate(t *testing.T) {
	if err := DefaultMachineRates().Validate(); err != nil {
		t.Fatalf("valid rates must pass: %v", err)
	}
	rates := DefaultMachineRates()
	rates.CutSpeed = nil
	if err := rates.Validate(); err == nil {
		t.Fatal("empty cut speeds must be rejected")
	}
	rates = DefaultMachineRates()
	rates.CutSpeed["STEEL-S235"] = 0
	if err := rates.Validate(); err == nil {
		t.Fatal("non-positive cut speed must be rejected")
	}
	rates = DefaultMachineRates()
	rates.FinishMin = -1
	if err := rates.Validate(); err == nil {
		t.Fatal("negative finish time must be rejected")
	}
}

// --- Фрезеровка кромки (скругление носа ступени) ---------------------------
//
// Фаска — это трудозатраты, поэтому обязана попадать в техмаршрут и в цену.
// Заготовка при этом остаётся прямоугольной: раскрой и закупка не меняются.

func TestPlanOperationsAddsMillingForChamferedPart(t *testing.T) {
	part := dommfg.Part{
		Number: "P1", Kind: dommfg.PartTread, Material: "WOOD-OAK",
		Thickness: engineering.Length(40), Length: engineering.Length(310),
		Width: engineering.Length(900),
		// Нос длиной 900 мм (вся ширина марша), радиус 8 мм.
		MillEdgeLengthMM: 900, MillRadiusMM: 8,
	}
	pkg := &dommfg.ManufacturingPackage{Parts: []dommfg.Part{part}}
	plan, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	ops := plan.Parts[0].Operations
	if len(ops) != 3 {
		t.Fatalf("operations = %d, want 3 (cut, mill, finish): %+v", len(ops), ops)
	}
	if ops[1].Type != dommfg.OpMilling {
		t.Fatalf("second operation = %q, want %q", ops[1].Type, dommfg.OpMilling)
	}
	// Фрезеровка — работа мастера: идёт на ручное рабочее место, иначе цена
	// посчитала бы её по машинному тарифу.
	if ops[1].Machine != dommfg.MachineManualWorkstation {
		t.Fatalf("milling machine = %q, want %q (ручная операция)", ops[1].Machine, dommfg.MachineManualWorkstation)
	}
	// Последовательности строго возрастают: рез → фрезеровка → финиш.
	if ops[0].Sequence != 1 || ops[1].Sequence != 2 || ops[2].Sequence != 3 {
		t.Fatalf("sequences = %d,%d,%d — want 1,2,3", ops[0].Sequence, ops[1].Sequence, ops[2].Sequence)
	}
	// Время = наладка + длина/подача = 1 + 900/350.
	rates := DefaultMachineRates()
	want := rates.MillSetupMin + 900/rates.MillFeed["WOOD-OAK"]
	if math.Abs(ops[1].EstimatedTime-want) > 1e-9 {
		t.Fatalf("milling time = %v, want %v", ops[1].EstimatedTime, want)
	}
}

func TestPlanOperationsSkipsMillingWithoutChamfer(t *testing.T) {
	// Металлическая ступень: фасок нет, маршрут прежний — две операции.
	part := dommfg.Part{
		Number: "P1", Kind: dommfg.PartTread, Material: "STEEL-S235",
		Thickness: engineering.Length(6), Length: engineering.Length(310),
		Width: engineering.Length(900),
	}
	pkg := &dommfg.ManufacturingPackage{Parts: []dommfg.Part{part}}
	plan, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatal(err)
	}
	ops := plan.Parts[0].Operations
	if len(ops) != 2 {
		t.Fatalf("operations = %d, want 2 (cut, finish)", len(ops))
	}
	for _, op := range ops {
		if op.Type == dommfg.OpMilling {
			t.Fatal("metal stair must not be scheduled for milling")
		}
	}
}

func TestPlanOperationsRejectsMillingWithoutFeed(t *testing.T) {
	// Фрезеровка нужна, а подачи по материалу нет — это ошибка ставок, а не
	// повод молча пропустить операцию и занизить цену.
	part := dommfg.Part{
		Number: "P1", Kind: dommfg.PartTread, Material: "SOME-UNKNOWN",
		Thickness: engineering.Length(40), Length: engineering.Length(310),
		Width: engineering.Length(900), MillEdgeLengthMM: 900,
	}
	rates := DefaultMachineRates()
	rates.CutSpeed["SOME-UNKNOWN"] = 5000
	pkg := &dommfg.ManufacturingPackage{Parts: []dommfg.Part{part}}
	if _, err := PlanOperations(pkg, rates); err == nil {
		t.Fatal("expected an error: milling without a feed rate must not be silently skipped")
	}
}

func TestMillFeedZeroOnlyForSteel(t *testing.T) {
	rates := DefaultMachineRates()
	if rates.MillFeed["STEEL-S235"] != 0 {
		t.Fatal("steel must have no milling feed: no chamfers on metal")
	}
	for _, code := range []dommfg.MaterialCode{"WOOD-OAK", "WOOD-WALNUT", "WOOD-ASH", "WOOD-SOFT"} {
		if rates.MillFeed[code] <= 0 {
			t.Fatalf("wood %q must have a positive mill feed", code)
		}
	}
}
