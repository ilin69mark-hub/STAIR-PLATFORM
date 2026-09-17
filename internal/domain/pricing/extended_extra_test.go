package pricing

import (
	"strings"
	"testing"
)

func TestFinishingCostValidateNil(t *testing.T) {
	var fc *FinishingCost
	if err := fc.Validate(); err == nil {
		t.Fatal("nil finishing cost must error")
	}
}

func TestFinishingCostNegativeUnitPrice(t *testing.T) {
	fc := &FinishingCost{Type: FinishingPainting, Area: 1000, UnitPrice: NewMoney(-1), TotalPrice: NewMoney(-1000)}
	if err := fc.Validate(); err == nil {
		t.Fatal("negative unit price must error")
	}
}

func TestOperationCostValidateErrors(t *testing.T) {
	if err := (*OperationCost)(nil).Validate(); err == nil {
		t.Fatal("nil operation cost must error")
	}
	cases := []struct {
		name string
		oc   *OperationCost
	}{
		{"empty type", &OperationCost{OperationID: 1, EstimatedTime: 30, HourlyRate: NewMoney(5000), TotalCost: NewMoney(2500)}},
		{"empty machine", &OperationCost{OperationID: 1, OperationType: "cutting", EstimatedTime: 30, HourlyRate: NewMoney(5000), TotalCost: NewMoney(2500)}},
		{"negative time", &OperationCost{OperationID: 1, OperationType: "cutting", MachineType: "laser", EstimatedTime: -1, HourlyRate: NewMoney(5000), TotalCost: NewMoney(-1)}},
		{"negative rate", &OperationCost{OperationID: 1, OperationType: "cutting", MachineType: "laser", EstimatedTime: 30, HourlyRate: NewMoney(-1), TotalCost: NewMoney(-1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.oc.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestMaterialCostValidateErrors(t *testing.T) {
	if err := (*MaterialCost)(nil).Validate(); err == nil {
		t.Fatal("nil material cost must error")
	}
	cases := []struct {
		name string
		mc   *MaterialCost
	}{
		{"empty material", &MaterialCost{PartNumber: "P-1", Volume: 1000, Density: 7850, PricePerKg: NewMoney(1), TotalCost: NewMoney(1)}},
		{"zero volume", &MaterialCost{PartNumber: "P-1", Material: "steel", Volume: 0, Density: 7850, PricePerKg: NewMoney(1), TotalCost: NewMoney(0)}},
		{"zero density", &MaterialCost{PartNumber: "P-1", Material: "steel", Volume: 1000, Density: 0, PricePerKg: NewMoney(1), TotalCost: NewMoney(0)}},
		{"negative price", &MaterialCost{PartNumber: "P-1", Material: "steel", Volume: 1000, Density: 7850, PricePerKg: NewMoney(-1), TotalCost: NewMoney(-1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.mc.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestProjectPricingValidateErrors(t *testing.T) {
	if err := (*ProjectPricing)(nil).Validate(); err == nil {
		t.Fatal("nil project pricing must error")
	}
	if err := (&ProjectPricing{Currency: CurrencyRUB}).Validate(); err == nil {
		t.Fatal("empty project id must error")
	}
	if err := (&ProjectPricing{ProjectID: "p1", Currency: Currency{}}).Validate(); err == nil {
		t.Fatal("invalid currency must error")
	}
	if err := (&ProjectPricing{ProjectID: "p1", Currency: CurrencyRUB, Overhead: Rate(-1)}).Validate(); err == nil {
		t.Fatal("negative overhead must error")
	}
	if err := (&ProjectPricing{ProjectID: "p1", Currency: CurrencyRUB, Margin: Rate(-1)}).Validate(); err == nil {
		t.Fatal("negative margin must error")
	}
	if err := (&ProjectPricing{ProjectID: "p1", Currency: CurrencyRUB, Discount: Rate(-1)}).Validate(); err == nil {
		t.Fatal("negative discount must error")
	}
	if err := (&ProjectPricing{ProjectID: "p1", Currency: CurrencyRUB, Tax: Rate(-1)}).Validate(); err == nil {
		t.Fatal("negative tax must error")
	}
}

func TestProjectPricingValidateItemErrors(t *testing.T) {
	pp := &ProjectPricing{
		ProjectID: "p1",
		Currency:  CurrencyRUB,
		Materials: []MaterialCost{{PartNumber: "", Material: "steel", Volume: 1000, Density: 7850, PricePerKg: NewMoney(1), TotalCost: NewMoney(1)}},
	}
	if err := pp.Validate(); err == nil {
		t.Fatal("invalid material must error")
	}
	pp = &ProjectPricing{
		ProjectID:  "p1",
		Currency:   CurrencyRUB,
		Operations: []OperationCost{{OperationID: 0, OperationType: "cutting", MachineType: "laser"}},
	}
	if err := pp.Validate(); err == nil {
		t.Fatal("invalid operation must error")
	}
	pp = &ProjectPricing{
		ProjectID:  "p1",
		Currency:   CurrencyRUB,
		Finishings: []FinishingCost{{Type: FinishingType("bad"), Area: 1000, UnitPrice: NewMoney(1), TotalPrice: NewMoney(1000)}},
	}
	if err := pp.Validate(); err == nil {
		t.Fatal("invalid finishing must error")
	}
}

func TestCalculateBreakdownValidationError(t *testing.T) {
	pp := &ProjectPricing{ProjectID: "p1", Currency: CurrencyRUB}
	if _, err := pp.CalculateBreakdown(); err == nil {
		t.Fatal("breakdown of empty project must error")
	}
}

func TestCalculateBreakdownWithFinishing(t *testing.T) {
	pp := &ProjectPricing{
		ProjectID: "proj-x",
		Currency:  CurrencyRUB,
		Materials: []MaterialCost{
			{PartNumber: "P-1", Material: "steel", Volume: 1000000, Density: 7850, PricePerKg: NewMoney(100), TotalCost: NewMoney(785)},
		},
		Finishings: []FinishingCost{
			{Type: FinishingPainting, Area: 1000, UnitPrice: NewMoney(50), TotalPrice: NewMoney(50000)},
		},
		Overhead: MustRate(0),
		Margin:   MustRate(0),
		Discount: MustRate(0),
		Tax:      MustRate(0),
	}
	_, err := pp.CalculateBreakdown()
	if err == nil {
		t.Fatal("finishing in breakdown must fail labor-value consistency")
	}
	if got := err.Error(); !strings.Contains(got, "breakdown validation failed") {
		t.Fatalf("error = %q, want breakdown validation failure", got)
	}
}
