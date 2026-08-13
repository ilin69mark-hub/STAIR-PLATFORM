package pricing

import (
	"testing"
)

// validBreakdown строит согласованную структуру стоимости для тестов
// инвариантов. Суммы выверены: base=Material+Machine+Labor,
// ProductionCost=base+Overhead, Gross=Production+Margin,
// PreTax=Gross−Discount, Final=PreTax+Tax.
func validBreakdown() *PriceBreakdown {
	return &PriceBreakdown{
		Currency:       CurrencyRUB,
		Material:       NewMoney(200000),
		Machine:        NewMoney(30000),
		Labor:          NewMoney(50000),
		Overhead:       NewMoney(56000), // 20% от 280000
		ProductionCost: NewMoney(336000),
		Margin:         NewMoney(100800), // 30% от 336000
		Discount:       NewMoney(21840),  // 5% от 436800
		PreTax:         NewMoney(414960),
		Tax:            NewMoney(82992), // 20% от 414960
		FinalPrice:     NewMoney(497952),
		Lines: []CostComponent{
			{Name: "Материал", Category: CategoryMaterial, Amount: NewMoney(200000), Source: "material:STEEL-S235"},
			{Name: "Оборудование", Category: CategoryMachine, Amount: NewMoney(30000), Source: "machine:laser"},
			{Name: "Труд", Category: CategoryLabor, Amount: NewMoney(50000), Source: "labor:manual"},
			{Name: "Накладные", Category: CategoryOverhead, Amount: NewMoney(56000), Source: "overhead:rate"},
			{Name: "Наценка", Category: CategoryMargin, Amount: NewMoney(100800), Source: "margin:rate"},
			{Name: "Скидка", Category: CategoryDiscount, Amount: NewMoney(21840), Source: "discount:rate"},
			{Name: "Налог", Category: CategoryTax, Amount: NewMoney(82992), Source: "tax:rate"},
		},
	}
}

func TestPriceBreakdownValidate(t *testing.T) {
	b := validBreakdown()
	if err := b.Validate(); err != nil {
		t.Fatalf("valid breakdown must pass: %v", err)
	}
}

func TestPriceBreakdownValidateNil(t *testing.T) {
	var b *PriceBreakdown
	if err := b.Validate(); err == nil {
		t.Fatal("nil breakdown must be rejected")
	}
}

func TestPriceBreakdownValidateNegative(t *testing.T) {
	b := validBreakdown()
	b.FinalPrice = NewMoney(-1)
	if err := b.Validate(); err == nil {
		t.Fatal("negative final price must be rejected")
	}
}

func TestPriceBreakdownValidateChain(t *testing.T) {
	b := validBreakdown()
	b.ProductionCost = NewMoney(337000)
	if err := b.Validate(); err == nil {
		t.Fatal("inconsistent production cost must be rejected")
	}
	b = validBreakdown()
	b.PreTax = NewMoney(414000)
	if err := b.Validate(); err == nil {
		t.Fatal("inconsistent pre-tax must be rejected")
	}
}

func TestPriceBreakdownValidateLines(t *testing.T) {
	b := validBreakdown()
	b.Lines = nil
	if err := b.Validate(); err == nil {
		t.Fatal("empty lines must be rejected")
	}
	b = validBreakdown()
	b.Lines[0].Amount = NewMoney(200001)
	if err := b.Validate(); err == nil {
		t.Fatal("material line sum mismatch must be rejected")
	}
	b = validBreakdown()
	b.Lines[0].Name = ""
	if err := b.Validate(); err == nil {
		t.Fatal("line without name must be rejected")
	}
	b = validBreakdown()
	b.Lines[0].Category = CostCategory("bogus")
	if err := b.Validate(); err == nil {
		t.Fatal("invalid category must be rejected")
	}
	b = validBreakdown()
	b.Lines[0].Source = ""
	if err := b.Validate(); err == nil {
		t.Fatal("line without source must be rejected")
	}
}

func TestCostCategoryIsValid(t *testing.T) {
	for _, c := range []CostCategory{
		CategoryMaterial, CategoryMachine, CategoryLabor, CategoryOverhead,
		CategoryMargin, CategoryDiscount, CategoryTax,
	} {
		if !c.IsValid() {
			t.Fatalf("category %q must be valid", c)
		}
	}
	if CostCategory("bogus").IsValid() {
		t.Fatal("bogus category must be invalid")
	}
}
