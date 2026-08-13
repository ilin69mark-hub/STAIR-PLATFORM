package pricing

import (
	"fmt"
)

// CostCategory — категория элемента стоимости (PRC-0004 Cost Categories).
type CostCategory string

const (
	CategoryMaterial CostCategory = "material"
	CategoryMachine  CostCategory = "machine"
	CategoryLabor    CostCategory = "labor"
	CategoryOverhead CostCategory = "overhead"
	CategoryMargin   CostCategory = "margin"
	CategoryDiscount CostCategory = "discount"
	CategoryTax      CostCategory = "tax"
)

// IsValid проверяет корректность категории.
func (c CostCategory) IsValid() bool {
	switch c {
	case CategoryMaterial, CategoryMachine, CategoryLabor, CategoryOverhead,
		CategoryMargin, CategoryDiscount, CategoryTax:
		return true
	}
	return false
}

// CostComponent — элемент структуры себестоимости (PRC-0004 Cost Object):
// сумма с категорией и источником для полной трассируемости расчёта.
type CostComponent struct {
	Name     string
	Category CostCategory
	Amount   Money
	Source   string // трассировка: материал/ставка/группа операций
}

// PriceBreakdown — структурированный результат расчёта стоимости
// (PRC-0004 Output, Production Cost → Final Price). Все суммы в одной
// базовой валюте. Цепочка: base = Material + Machine + Labor;
// ProductionCost = base + Overhead; Gross = ProductionCost + Margin;
// PreTax = Gross − Discount; FinalPrice = PreTax + Tax.
type PriceBreakdown struct {
	Currency Currency

	Material       Money
	Machine        Money
	Labor          Money
	Overhead       Money
	ProductionCost Money
	Margin         Money
	Discount       Money
	PreTax         Money
	Tax            Money
	FinalPrice     Money

	// Lines — детализация элементов (трассируемость до источников).
	Lines []CostComponent
}

// Validate проверяет инварианты структуры стоимости: валидная валюта,
// неотрицательные суммы, согласованность цепочки расчёта и соответствие
// итогов суммы детализирующих позиций по каждой категории.
func (b *PriceBreakdown) Validate() error {
	if b == nil {
		return fmt.Errorf("pricing: price breakdown is required")
	}
	if err := b.Currency.Validate(); err != nil {
		return err
	}
	for _, v := range []Money{
		b.Material, b.Machine, b.Labor, b.Overhead, b.ProductionCost,
		b.Margin, b.Discount, b.PreTax, b.Tax, b.FinalPrice,
	} {
		if v < 0 {
			return fmt.Errorf("pricing: price breakdown contains a negative amount")
		}
	}

	base := b.Material.Add(b.Machine).Add(b.Labor)
	if b.ProductionCost != base.Add(b.Overhead) {
		return fmt.Errorf("pricing: production cost must equal base plus overhead")
	}
	gross := b.ProductionCost.Add(b.Margin)
	if b.PreTax != gross.Sub(b.Discount) {
		return fmt.Errorf("pricing: pre-tax price must equal gross minus discount")
	}
	if b.FinalPrice != b.PreTax.Add(b.Tax) {
		return fmt.Errorf("pricing: final price must equal pre-tax plus tax")
	}

	if len(b.Lines) == 0 {
		return fmt.Errorf("pricing: price breakdown has no cost lines")
	}
	// Суммы позиций по категориям сходятся к итогам.
	sums := map[CostCategory]Money{}
	for i, line := range b.Lines {
		if line.Name == "" {
			return fmt.Errorf("pricing: line %d has no name", i)
		}
		if !line.Category.IsValid() {
			return fmt.Errorf("pricing: line %d has invalid category %q", i, line.Category)
		}
		if line.Source == "" {
			return fmt.Errorf("pricing: line %d has no source", i)
		}
		if line.Amount < 0 {
			return fmt.Errorf("pricing: line %d has negative amount", i)
		}
		sums[line.Category] = sums[line.Category].Add(line.Amount)
	}
	want := map[CostCategory]Money{
		CategoryMaterial: b.Material,
		CategoryMachine:  b.Machine,
		CategoryLabor:    b.Labor,
		CategoryOverhead: b.Overhead,
		CategoryMargin:   b.Margin,
		CategoryDiscount: b.Discount,
		CategoryTax:      b.Tax,
	}
	for cat, sum := range sums {
		if want[cat] != sum {
			return fmt.Errorf("pricing: %s lines total %d, want %d", cat, sum, want[cat])
		}
	}
	return nil
}
