package manufacturing

import (
	"fmt"
	"math"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// MachineRates — детерминированные константы оценки времени операций
// (MFG-0009). Замещаются производственными справочниками (Machine Library)
// на следующих этапах; для MVP задают стандартный техмаршрут.
type MachineRates struct {
	CutSpeed    map[dommfg.MaterialCode]float64 // мм/мин — скорость реза лазером
	SetupCutMin float64                         // мин — наладка на деталь
	FinishMin   float64                         // мин — финишная операция на деталь
}

// DefaultMachineRates возвращает константы для материалов
// DefaultMaterialRegistry: лазерный рез листовой стали/алюминия/дерева.
func DefaultMachineRates() MachineRates {
	return MachineRates{
		// Скорость лазерного реза, мм/мин. Сталь/кортен — рядок по прокату,
		// древесина — по мягкости: орех и ясень режутся чуть медленнее дуба,
		// сосна — быстрее. Этап 1 добавил эти коды в каталог материалов.
		CutSpeed: map[dommfg.MaterialCode]float64{
			"STEEL-S235":   2000,
			"STEEL-CORTEN": 1800,
			"ALUM-5083":    4000,
			"WOOD-OAK":     10000,
			"WOOD-WALNUT":  8500,
			"WOOD-ASH":     9500,
			"WOOD-SOFT":    12000,
		},
		SetupCutMin: 2,
		FinishMin:   3,
	}
}

// Validate проверяет корректность констант.
func (r MachineRates) Validate() error {
	if len(r.CutSpeed) == 0 {
		return fmt.Errorf("manufacturing: machine rates have no cut speeds")
	}
	for code, speed := range r.CutSpeed {
		if math.IsNaN(speed) || math.IsInf(speed, 0) || speed <= 0 {
			return fmt.Errorf("manufacturing: machine rates have invalid cut speed for %q", code)
		}
	}
	for _, v := range []float64{r.SetupCutMin, r.FinishMin} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("manufacturing: machine rates have invalid time constants")
		}
	}
	return nil
}

// PlanOperations формирует технологический маршрут (MFG-0009) для каждой
// детали пакета. Маршрут детерминирован и определяется типом детали:
// Cutting (лазерный рез, seq 1) + Finishing/QC (ручная, seq 2). Время реза
// зависит от периметра детали и скорости материала; ID операций глобальны.
func PlanOperations(pkg *dommfg.ManufacturingPackage, rates MachineRates) (*dommfg.OperationPlan, error) {
	if pkg == nil {
		return nil, fmt.Errorf("manufacturing: package is required")
	}
	if err := rates.Validate(); err != nil {
		return nil, err
	}

	plan := &dommfg.OperationPlan{}
	opID := 0
	for _, part := range pkg.Parts {
		speed, ok := rates.CutSpeed[part.Material]
		if !ok {
			return nil, fmt.Errorf("manufacturing: no cut speed for material %q", part.Material)
		}
		perimeter := 2 * (part.Length.Millimeters() + part.Width.Millimeters())
		cutTime := rates.SetupCutMin + perimeter/speed

		opID++
		cutting := dommfg.Operation{
			ID: opID, PartNumber: part.Number,
			Type: dommfg.OpCutting, Sequence: 1, Machine: dommfg.MachineLaserCutter,
			EstimatedTime: cutTime, OperatorRequired: true,
		}
		opID++
		finishing := dommfg.Operation{
			ID: opID, PartNumber: part.Number,
			Type: dommfg.OpFinishing, Sequence: 2, Machine: dommfg.MachineManualWorkstation,
			EstimatedTime: rates.FinishMin, OperatorRequired: true,
		}

		plan.Parts = append(plan.Parts, dommfg.PartOperationPlan{
			PartNumber: part.Number,
			Operations: []dommfg.Operation{cutting, finishing},
		})
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return plan, nil
}
