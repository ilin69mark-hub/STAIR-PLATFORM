package manufacturing

import (
	"reflect"
	"testing"

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
	// + 32×3 finish.
	want := 2*(2+2*(4077.7350098112615+2660)/2000) + 15*3.21 + 15*3.04 + 32*3
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
