package manufacturing

import (
	"fmt"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// plateSurfaceArea — площадь поверхности прямоугольной пластины
// (l×w×t, мм): две плоскости плюс кромки.
func plateSurfaceArea(l, w, t float64) float64 {
	return 2 * (l*w + l*t + w*t)
}

// PrepareCost формирует производственный набор данных (MFG-0015) из
// комплекта ManufacturingPackage. Выполняет только физические расчёты:
// площади (детали и листы, отходы), объём, масса по плотности материала,
// поверхность, счётчики. Временные оценки и техмаршрут поставляются
// Machine Operations (MFG-0009). Финансовые значения (цены, тарифы,
// прибыль, налоги) набор не содержит (правила MFG-0015). Результат
// детерминирован.
func PrepareCost(pkg *dommfg.ManufacturingPackage, materials *dommfg.MaterialRegistry, rates MachineRates) (*dommfg.ManufacturingCostDataset, error) {
	if pkg == nil {
		return nil, fmt.Errorf("manufacturing: package is required")
	}
	if err := pkg.Validate(); err != nil {
		return nil, fmt.Errorf("manufacturing: %w", err)
	}
	if materials == nil {
		return nil, fmt.Errorf("manufacturing: material registry is required")
	}

	// Расход по CutList: площади деталей, объём, масса, поверхность.
	type matAgg struct {
		partArea    float64
		volume      float64
		surfaceArea float64
	}
	agg := make(map[dommfg.MaterialCode]*matAgg)
	var order []dommfg.MaterialCode
	for _, item := range pkg.CutList.Items {
		if _, ok := materials.Find(item.MaterialCode); !ok {
			return nil, fmt.Errorf("manufacturing: material %q is not in registry", item.MaterialCode)
		}
		if _, ok := agg[item.MaterialCode]; !ok {
			order = append(order, item.MaterialCode)
			agg[item.MaterialCode] = &matAgg{}
		}
		a := agg[item.MaterialCode]
		q := float64(item.Quantity)
		l, w, t := item.Length.Millimeters(), item.Width.Millimeters(), item.Thickness.Millimeters()
		vol := q * t * l * w
		a.partArea += q * l * w
		a.volume += vol
		a.surfaceArea += q * plateSurfaceArea(l, w, t)
	}

	// Расход листов по Nesting: площади листов и отходы по материалам.
	sheetArea := make(map[dommfg.MaterialCode]float64)
	for _, s := range pkg.Nesting.Sheets {
		sheetArea[s.MaterialCode] += s.Length.Millimeters() * s.Width.Millimeters()
	}

	dataset := &dommfg.ManufacturingCostDataset{}
	for _, code := range order {
		a := agg[code]
		m, _ := materials.Find(code)
		waste := sheetArea[code] - a.partArea
		if waste < 0 {
			return nil, fmt.Errorf("manufacturing: nesting placed area exceeds cut list area for %q", code)
		}
		dataset.MaterialConsumption = append(dataset.MaterialConsumption, dommfg.MaterialConsumption{
			MaterialCode: code,
			PartArea:     a.partArea,
			WasteArea:    waste,
			SheetArea:    sheetArea[code],
			Volume:       a.volume,
			Mass:         a.volume * 1e-9 * m.Density,
		})
		dataset.PartArea += a.partArea
		dataset.WasteArea += waste
		dataset.SheetArea += sheetArea[code]
		dataset.Volume += a.volume
		dataset.SurfaceArea += a.surfaceArea
		dataset.Mass += a.volume * 1e-9 * m.Density
	}
	dataset.PartCount = dommfg.Quantity(len(pkg.Parts))
	dataset.FastenerCount = 0 // в текущей модели крепёж отсутствует; счётчик подготовлен

	plan, err := PlanOperations(pkg, rates)
	if err != nil {
		return nil, err
	}
	dataset.OperationPlan = plan
	for _, part := range plan.Parts {
		for _, op := range part.Operations {
			dataset.OperationCount++
			if op.Machine == dommfg.MachineManualWorkstation {
				dataset.EstimatedLaborTime += op.EstimatedTime
			} else {
				dataset.EstimatedMachineTime += op.EstimatedTime
			}
		}
	}
	dataset.EstimatedProductionTime = dataset.EstimatedMachineTime + dataset.EstimatedLaborTime

	if dataset.SheetArea > 0 {
		dataset.WastePercent = dataset.WasteArea / dataset.SheetArea
		dataset.Utilization = dataset.PartArea / dataset.SheetArea
	}

	if err := dataset.Validate(); err != nil {
		return nil, err
	}
	return dataset, nil
}
