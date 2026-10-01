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
	// MillFeed — скорость фрезеровки кромки, мм/мин, по материалу. Скругление
	// носа ступени (филёнка) — ручная операция на кромкообрубочном станке или
	// фрезером с ручным прижимом, поэтому она медленнее лазерного реза на
	// порядок и тарифицируется как ТРУД, а не машинное время: станок у неё
	// один, а работа — мастера. Подачи ниже — заглушки до появления машинного
	// справочника (MFG-0009); твёрдые породы идут медленнее мягких.
	MillFeed map[dommfg.MaterialCode]float64
	// MillSetupMin — наладка на деталь перед фрезеровкой: упор уголка,
	// глубина, переустановка заготовки. Как и SetupCutMin, считается на
	// КАЖДУЮ деталь, поэтому партия из 15 ступеней платит наладку 15 раз.
	MillSetupMin float64
}

// DefaultMachineRates возвращает константы для материалов
// DefaultMaterialRegistry: лазерный рез листовой стали/алюминия/дерева.
func DefaultMachineRates() MachineRates {
	return MachineRates{
		// Скорость лазерного реза, мм/мин. Сталь/кортен — рядок по прокату,
		// древесина — по мягкости: орех и ясень режутся чуть медленнее дуба,
		// сосна — быстрее. Этап 1 добавил эти коды в каталог материалов.
		CutSpeed: map[dommfg.MaterialCode]float64{
			"STEEL-S235":  2000,
			"WOOD-OAK":    10000,
			"WOOD-WALNUT": 8500,
			"WOOD-ASH":    9500,
			"WOOD-SOFT":   12000,
		},
		// Скорость фрезеровки кромки, мм/мин. Ручной фрезер по дубу держит
		// примерно 350 мм/мин, по сосне — вдвое быстрее (мягче режет, меньше
		// нагрузка на кромку).
		MillFeed: map[dommfg.MaterialCode]float64{
			"STEEL-S235":  0, // фасок по металлу нет: операция не планируется
			"WOOD-OAK":    350,
			"WOOD-WALNUT": 300,
			"WOOD-ASH":    400,
			"WOOD-SOFT":   600,
		},
		SetupCutMin:  2,
		FinishMin:    3,
		MillSetupMin: 1,
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
	if len(r.MillFeed) == 0 {
		return fmt.Errorf("manufacturing: machine rates have no mill feeds")
	}
	for code, feed := range r.MillFeed {
		// Ноль допустим только для металла: там фасок нет, и подача не
		// понадобится. Для древесины нулевая подача — ошибка ставок.
		if math.IsNaN(feed) || math.IsInf(feed, 0) || feed < 0 {
			return fmt.Errorf("manufacturing: machine rates have invalid mill feed for %q", code)
		}
		if feed == 0 && r.CutSpeed[code] > 0 && code != "STEEL-S235" {
			return fmt.Errorf("manufacturing: mill feed for %q must be positive", code)
		}
	}
	for _, v := range []float64{r.SetupCutMin, r.FinishMin, r.MillSetupMin} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("manufacturing: machine rates have invalid time constants")
		}
	}
	return nil
}

// PlanOperations формирует технологический маршрут (MFG-0009) для каждой
// детали пакета. Маршрут детерминирован и определяется типом детали:
// Cutting (лазерный рез, seq 1) → Milling (фрезеровка кромки, seq 2, только
// для деталей с MillEdgeLengthMM > 0) → Finishing/QC (ручная, последняя).
// Время реза зависит от периметра детали и скорости материала, время
// фрезеровки — от длины кромки и подачи; ID операций глобальны.
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
		ops := []dommfg.Operation{cutting}
		seq := 2

		// Скругление носа (филёнка) — отдельная операция между резом и
		// финишем: заготовка режется прямоугольной, кромку обрубают после.
		// Время = наладка + длина кромки / подача по материалу. Операция идёт
		// на ручном рабочем месте, поэтому PrepareCost зачтёт её как ТРУД, а не
		// как машинное время: у мастера это основная работа, а у станка — нет.
		if part.NeedsMilling() {
			feed, ok := rates.MillFeed[part.Material]
			if !ok {
				return nil, fmt.Errorf("manufacturing: no mill feed for material %q", part.Material)
			}
			if feed <= 0 {
				return nil, fmt.Errorf("manufacturing: part %q needs milling but mill feed for %q is zero",
					part.Number, part.Material)
			}
			millTime := rates.MillSetupMin + part.MillEdgeLengthMM/feed
			opID++
			ops = append(ops, dommfg.Operation{
				ID: opID, PartNumber: part.Number,
				Type: dommfg.OpMilling, Sequence: seq, Machine: dommfg.MachineManualWorkstation,
				EstimatedTime: millTime, OperatorRequired: true,
			})
			seq++
		}

		opID++
		ops = append(ops, dommfg.Operation{
			ID: opID, PartNumber: part.Number,
			Type: dommfg.OpFinishing, Sequence: seq, Machine: dommfg.MachineManualWorkstation,
			EstimatedTime: rates.FinishMin, OperatorRequired: true,
		})

		plan.Parts = append(plan.Parts, dommfg.PartOperationPlan{
			PartNumber: part.Number,
			Operations: ops,
		})
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return plan, nil
}
