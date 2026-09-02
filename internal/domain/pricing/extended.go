package pricing

import (
	"fmt"
)

// FinishingType — тип отделки (PRC-0015).
type FinishingType string

const (
	FinishingNone           FinishingType = "none"
	FinishingPainting       FinishingType = "painting"
	FinishingPowderCoating  FinishingType = "powder-coating"
	FinishingGalvanizing    FinishingType = "galvanizing"
	FinishingHeatTreatment  FinishingType = "heat-treatment"
	FinishingAnodizing      FinishingType = "anodizing"
	FinishingBrushing       FinishingType = "brushing"
	FinishingPolishing      FinishingType = "polishing"
	FinishingSandblasting   FinishingType = "sandblasting"
	FinishingCustom         FinishingType = "custom"
)

// IsValid проверяет корректность типа отделки.
func (t FinishingType) IsValid() bool {
	switch t {
	case FinishingNone, FinishingPainting, FinishingPowderCoating, FinishingGalvanizing,
		FinishingHeatTreatment, FinishingAnodizing, FinishingBrushing, FinishingPolishing,
		FinishingSandblasting, FinishingCustom:
		return true
	}
	return false
}

// FinishingCost — стоимость отделки одной операции (PRC-0015).
type FinishingCost struct {
	Type        FinishingType
	Area        float64 // мм² — площадь поверхности
	UnitPrice   Money   // цена за мм² в минорных единицах
	TotalPrice  Money   // итого = Area × UnitPrice
	Description string
}

// Validate проверяет инварианты стоимости отделки.
func (fc *FinishingCost) Validate() error {
	if fc == nil {
		return fmt.Errorf("pricing: finishing cost is required")
	}
	if !fc.Type.IsValid() {
		return fmt.Errorf("pricing: invalid finishing type %q", fc.Type)
	}
	if fc.Area <= 0 {
		return fmt.Errorf("pricing: finishing area must be positive, got %v", fc.Area)
	}
	if fc.UnitPrice < 0 {
		return fmt.Errorf("pricing: finishing unit price must not be negative")
	}
	// Проверяем согласованность TotalPrice
	expected := Money(int64(float64(fc.UnitPrice) * fc.Area))
	if fc.TotalPrice != expected {
		return fmt.Errorf("pricing: finishing total price %d does not match area × unit_price %d", fc.TotalPrice, expected)
	}
	return nil
}

// OperationCost — стоимость технологической операции (PRC-0008).
type OperationCost struct {
	OperationID   int
	OperationType string  // тип операции из manufacturing
	MachineType   string  // тип оборудования
	EstimatedTime float64 // минуты
	HourlyRate    Money   // ставка за час в минорных единицах
	TotalCost     Money   // итого = EstimatedTime / 60 × HourlyRate
}

// Validate проверяет инварианты стоимости операции.
func (oc *OperationCost) Validate() error {
	if oc == nil {
		return fmt.Errorf("pricing: operation cost is required")
	}
	if oc.OperationID <= 0 {
		return fmt.Errorf("pricing: operation id must be positive")
	}
	if oc.OperationType == "" {
		return fmt.Errorf("pricing: operation type is required")
	}
	if oc.MachineType == "" {
		return fmt.Errorf("pricing: machine type is required")
	}
	if oc.EstimatedTime < 0 {
		return fmt.Errorf("pricing: estimated time must not be negative")
	}
	if oc.HourlyRate < 0 {
		return fmt.Errorf("pricing: hourly rate must not be negative")
	}
	// Проверяем согласованность TotalCost
	expected := Money(int64(float64(oc.HourlyRate) * oc.EstimatedTime / 60))
	if oc.TotalCost != expected {
		return fmt.Errorf("pricing: operation total cost %d does not match time × rate %d", oc.TotalCost, expected)
	}
	return nil
}

// MaterialCost — стоимость материала для одной детали (PRC-0007).
type MaterialCost struct {
	PartNumber  string
	Material    string
	Volume      float64 // мм³
	Density     float64 // кг/м³
	PricePerKg  Money   // цена за кг в минорных единицах
	TotalCost   Money   // итого = Volume × Density / 1e9 × PricePerKg
}

// Validate проверяет инварианты стоимости материала.
func (mc *MaterialCost) Validate() error {
	if mc == nil {
		return fmt.Errorf("pricing: material cost is required")
	}
	if mc.PartNumber == "" {
		return fmt.Errorf("pricing: part number is required")
	}
	if mc.Material == "" {
		return fmt.Errorf("pricing: material is required")
	}
	if mc.Volume <= 0 {
		return fmt.Errorf("pricing: volume must be positive")
	}
	if mc.Density <= 0 {
		return fmt.Errorf("pricing: density must be positive")
	}
	if mc.PricePerKg < 0 {
		return fmt.Errorf("pricing: price per kg must not be negative")
	}
	// Проверяем согласованность TotalCost
	// Volume (мм³) × Density (кг/м³) / 1e9 = кг
	weightKg := mc.Volume * mc.Density / 1e9
	expected := Money(int64(float64(mc.PricePerKg) * weightKg))
	if mc.TotalCost != expected {
		return fmt.Errorf("pricing: material total cost %d does not match weight × price %d", mc.TotalCost, expected)
	}
	return nil
}

// ProjectPricing — полный расчёт стоимости проекта (PRC-0014).
type ProjectPricing struct {
	ProjectID   string
	Currency    Currency
	Materials   []MaterialCost
	Operations  []OperationCost
	Finishings  []FinishingCost
	Overhead    Rate // overhead rate (%)
	Margin      Rate // margin rate (%)
	Discount    Rate // discount rate (%)
	Tax         Rate // tax rate (%)
	Breakdown   *PriceBreakdown
}

// Validate проверяет инварианты расчёта проекта.
func (pp *ProjectPricing) Validate() error {
	if pp == nil {
		return fmt.Errorf("pricing: project pricing is required")
	}
	if pp.ProjectID == "" {
		return fmt.Errorf("pricing: project id is required")
	}
	if err := pp.Currency.Validate(); err != nil {
		return err
	}
	if err := pp.Overhead.Validate(); err != nil {
		return fmt.Errorf("pricing: overhead: %v", err)
	}
	if err := pp.Margin.Validate(); err != nil {
		return fmt.Errorf("pricing: margin: %v", err)
	}
	if err := pp.Discount.Validate(); err != nil {
		return fmt.Errorf("pricing: discount: %v", err)
	}
	if err := pp.Tax.Validate(); err != nil {
		return fmt.Errorf("pricing: tax: %v", err)
	}

	// Проверяем детализацию
	if len(pp.Materials) == 0 && len(pp.Operations) == 0 && len(pp.Finishings) == 0 {
		return fmt.Errorf("pricing: project has no cost items")
	}

	for i, m := range pp.Materials {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("pricing: material %d: %v", i, err)
		}
	}
	for i, o := range pp.Operations {
		if err := o.Validate(); err != nil {
			return fmt.Errorf("pricing: operation %d: %v", i, err)
		}
	}
	for i, f := range pp.Finishings {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("pricing: finishing %d: %v", i, err)
		}
	}

	return nil
}

// CalculateBreakdown рассчитывает PriceBreakdown по компонентам.
func (pp *ProjectPricing) CalculateBreakdown() (*PriceBreakdown, error) {
	if err := pp.Validate(); err != nil {
		return nil, err
	}

	var materialTotal, machineTotal, laborTotal Money
	for _, m := range pp.Materials {
		materialTotal = materialTotal.Add(m.TotalCost)
	}
	for _, o := range pp.Operations {
		machineTotal = machineTotal.Add(o.TotalCost)
	}
	// Labor = 0 для MVP (будет добавлен позже)

	// Overhead = (Material + Machine + Labor) × OverheadRate
	base := materialTotal.Add(machineTotal).Add(laborTotal)
	overhead := base.MulRate(pp.Overhead)

	// ProductionCost = base + Overhead
	productionCost := base.Add(overhead)

	// Margin = ProductionCost × MarginRate
	margin := productionCost.MulRate(pp.Margin)

	// Gross = ProductionCost + Margin
	gross := productionCost.Add(margin)

	// Discount = Gross × DiscountRate
	discount := gross.MulRate(pp.Discount)

	// PreTax = Gross - Discount
	preTax := gross.Sub(discount)

	// Tax = PreTax × TaxRate
	tax := preTax.MulRate(pp.Tax)

	// FinalPrice = PreTax + Tax
	finalPrice := preTax.Add(tax)

	// Формируем детализацию
	lines := make([]CostComponent, 0)

	for _, m := range pp.Materials {
		lines = append(lines, CostComponent{
			Name:     fmt.Sprintf("Material: %s (%s)", m.PartNumber, m.Material),
			Category: CategoryMaterial,
			Amount:   m.TotalCost,
			Source:   fmt.Sprintf("part:%s,material:%s", m.PartNumber, m.Material),
		})
	}
	for _, o := range pp.Operations {
		lines = append(lines, CostComponent{
			Name:     fmt.Sprintf("Operation: %s (%s)", o.OperationType, o.MachineType),
			Category: CategoryMachine,
			Amount:   o.TotalCost,
			Source:   fmt.Sprintf("op:%d,type:%s,machine:%s", o.OperationID, o.OperationType, o.MachineType),
		})
	}
	for _, f := range pp.Finishings {
		lines = append(lines, CostComponent{
			Name:     fmt.Sprintf("Finishing: %s", f.Type),
			Category: CategoryLabor,
			Amount:   f.TotalPrice,
			Source:   fmt.Sprintf("finishing:%s,area:%.0f", f.Type, f.Area),
		})
	}

	// Overhead
	lines = append(lines, CostComponent{
		Name:     "Overhead",
		Category: CategoryOverhead,
		Amount:   overhead,
		Source:   fmt.Sprintf("rate:%.2f%%", pp.Overhead.Percent()),
	})

	// Margin
	lines = append(lines, CostComponent{
		Name:     "Margin",
		Category: CategoryMargin,
		Amount:   margin,
		Source:   fmt.Sprintf("rate:%.2f%%", pp.Margin.Percent()),
	})

	// Discount (если есть)
	if discount > 0 {
		lines = append(lines, CostComponent{
			Name:     "Discount",
			Category: CategoryDiscount,
			Amount:   discount,
			Source:   fmt.Sprintf("rate:%.2f%%", pp.Discount.Percent()),
		})
	}

	// Tax (если есть)
	if tax > 0 {
		lines = append(lines, CostComponent{
			Name:     "Tax",
			Category: CategoryTax,
			Amount:   tax,
			Source:   fmt.Sprintf("rate:%.2f%%", pp.Tax.Percent()),
		})
	}

	breakdown := &PriceBreakdown{
		Currency:       pp.Currency,
		Material:       materialTotal,
		Machine:        machineTotal,
		Labor:          laborTotal,
		Overhead:       overhead,
		ProductionCost: productionCost,
		Margin:         margin,
		Discount:       discount,
		PreTax:         preTax,
		Tax:            tax,
		FinalPrice:     finalPrice,
		Lines:          lines,
	}

	if err := breakdown.Validate(); err != nil {
		return nil, fmt.Errorf("pricing: breakdown validation failed: %v", err)
	}

	return breakdown, nil
}
