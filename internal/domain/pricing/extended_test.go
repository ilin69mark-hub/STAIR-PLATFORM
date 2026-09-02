package pricing

import (
	"testing"
)

func TestFinishingTypeIsValid(t *testing.T) {
	valid := []FinishingType{
		FinishingNone, FinishingPainting, FinishingPowderCoating,
		FinishingGalvanizing, FinishingHeatTreatment, FinishingAnodizing,
		FinishingBrushing, FinishingPolishing, FinishingSandblasting, FinishingCustom,
	}
	for _, ft := range valid {
		if !ft.IsValid() {
			t.Errorf("expected finishing type %q to be valid", ft)
		}
	}
	if FinishingType("invalid").IsValid() {
		t.Error("expected 'invalid' finishing type to be invalid")
	}
}

func TestFinishingCostValidation(t *testing.T) {
	fc := &FinishingCost{
		Type:       FinishingPainting,
		Area:       1000,
		UnitPrice:  NewMoney(50),
		TotalPrice: NewMoney(50000), // 1000 × 50
	}
	if err := fc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinishingCostInvalidType(t *testing.T) {
	fc := &FinishingCost{
		Type:       "invalid",
		Area:       1000,
		UnitPrice:  NewMoney(50),
		TotalPrice: NewMoney(50000),
	}
	if err := fc.Validate(); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestFinishingCostInvalidArea(t *testing.T) {
	fc := &FinishingCost{
		Type:       FinishingPainting,
		Area:       -1,
		UnitPrice:  NewMoney(50),
		TotalPrice: NewMoney(-50),
	}
	if err := fc.Validate(); err == nil {
		t.Fatal("expected error for negative area")
	}
}

func TestFinishingCostInvalidTotal(t *testing.T) {
	fc := &FinishingCost{
		Type:       FinishingPainting,
		Area:       1000,
		UnitPrice:  NewMoney(50),
		TotalPrice: NewMoney(999), // wrong
	}
	if err := fc.Validate(); err == nil {
		t.Fatal("expected error for wrong total")
	}
}

func TestOperationCostValidation(t *testing.T) {
	oc := &OperationCost{
		OperationID:   1,
		OperationType: "cutting",
		MachineType:   "laser-cutter",
		EstimatedTime: 30, // 30 минут
		HourlyRate:    NewMoney(5000),
		TotalCost:     NewMoney(2500), // 30/60 × 5000 = 2500
	}
	if err := oc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOperationCostInvalidID(t *testing.T) {
	oc := &OperationCost{
		OperationID:   0,
		OperationType: "cutting",
		MachineType:   "laser-cutter",
		EstimatedTime: 30,
		HourlyRate:    NewMoney(5000),
		TotalCost:     NewMoney(2500),
	}
	if err := oc.Validate(); err == nil {
		t.Fatal("expected error for invalid id")
	}
}

func TestOperationCostInvalidTotal(t *testing.T) {
	oc := &OperationCost{
		OperationID:   1,
		OperationType: "cutting",
		MachineType:   "laser-cutter",
		EstimatedTime: 30,
		HourlyRate:    NewMoney(5000),
		TotalCost:     NewMoney(999), // wrong
	}
	if err := oc.Validate(); err == nil {
		t.Fatal("expected error for wrong total")
	}
}

func TestMaterialCostValidation(t *testing.T) {
	mc := &MaterialCost{
		PartNumber: "P-001",
		Material:   "steel",
		Volume:     1000000, // 1000 см³
		Density:    7850,    // кг/м³
		PricePerKg: NewMoney(100),
		TotalCost:  NewMoney(785), // 1e6 × 7850 / 1e9 × 100 = 785
	}
	if err := mc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaterialCostInvalidPartNumber(t *testing.T) {
	mc := &MaterialCost{
		PartNumber: "",
		Material:   "steel",
		Volume:     1000000,
		Density:    7850,
		PricePerKg: NewMoney(100),
		TotalCost:  NewMoney(785),
	}
	if err := mc.Validate(); err == nil {
		t.Fatal("expected error for empty part number")
	}
}

func TestMaterialCostInvalidTotal(t *testing.T) {
	mc := &MaterialCost{
		PartNumber: "P-001",
		Material:   "steel",
		Volume:     1000000,
		Density:    7850,
		PricePerKg: NewMoney(100),
		TotalCost:  NewMoney(999), // wrong
	}
	if err := mc.Validate(); err == nil {
		t.Fatal("expected error for wrong total")
	}
}

func TestProjectPricingValidation(t *testing.T) {
	pp := &ProjectPricing{
		ProjectID: "proj-1",
		Currency:  CurrencyRUB,
		Materials: []MaterialCost{
			{
				PartNumber: "P-001",
				Material:   "steel",
				Volume:     1000000,
				Density:    7850,
				PricePerKg: NewMoney(100),
				TotalCost:  NewMoney(785),
			},
		},
		Operations: []OperationCost{
			{
				OperationID:   1,
				OperationType: "cutting",
				MachineType:   "laser-cutter",
				EstimatedTime: 30,
				HourlyRate:    NewMoney(5000),
				TotalCost:     NewMoney(2500),
			},
		},
		Overhead: MustRate(15),
		Margin:   MustRate(20),
		Discount: MustRate(5),
		Tax:      MustRate(20),
	}
	if err := pp.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectPricingEmptyItems(t *testing.T) {
	pp := &ProjectPricing{
		ProjectID: "proj-1",
		Currency:  CurrencyRUB,
		Overhead:  MustRate(15),
		Margin:    MustRate(20),
		Discount:  MustRate(5),
		Tax:       MustRate(20),
	}
	if err := pp.Validate(); err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestProjectPricingCalculateBreakdown(t *testing.T) {
	pp := &ProjectPricing{
		ProjectID: "proj-1",
		Currency:  CurrencyRUB,
		Materials: []MaterialCost{
			{
				PartNumber: "P-001",
				Material:   "steel",
				Volume:     1000000,
				Density:    7850,
				PricePerKg: NewMoney(100),
				TotalCost:  NewMoney(785),
			},
		},
		Operations: []OperationCost{
			{
				OperationID:   1,
				OperationType: "cutting",
				MachineType:   "laser-cutter",
				EstimatedTime: 30,
				HourlyRate:    NewMoney(5000),
				TotalCost:     NewMoney(2500),
			},
		},
		Overhead: MustRate(15),
		Margin:   MustRate(20),
		Discount: MustRate(5),
		Tax:      MustRate(20),
	}

	breakdown, err := pp.CalculateBreakdown()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем базовые суммы
	if breakdown.Material != NewMoney(785) {
		t.Fatalf("expected material 785, got %d", breakdown.Material)
	}
	if breakdown.Machine != NewMoney(2500) {
		t.Fatalf("expected machine 2500, got %d", breakdown.Machine)
	}

	// Проверяем цепочку
	base := NewMoney(785 + 2500) // 3285
	expectedOverhead := base.MulRate(MustRate(15))
	if breakdown.Overhead != expectedOverhead {
		t.Fatalf("expected overhead %d, got %d", expectedOverhead, breakdown.Overhead)
	}
}
