package stair

import (
	"context"
	"fmt"
	"math"
	"time"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/optimization"
)

// OptimizeTarget — целевая метрика оптимизации (меньше = лучше).
type OptimizeTarget string

const (
	// TargetPrice — итоговая цена (FinalPrice).
	TargetPrice OptimizeTarget = "price"
	// TargetCost — себестоимость (ProductionCost).
	TargetCost OptimizeTarget = "cost"
	// TargetMaterial — стоимость материала (Material).
	TargetMaterial OptimizeTarget = "material"
)

// Valid проверяет известность целевой метрики.
func (t OptimizeTarget) Valid() bool {
	switch t {
	case TargetPrice, TargetCost, TargetMaterial:
		return true
	}
	return false
}

// OptimizeRequest — параметры поиска оптимума (EDR-0032). Нулевые границы
// выводятся автоматически из высоты подъёма и нормативных ограничений.
type OptimizeRequest struct {
	Target          OptimizeTarget // по умолчанию TargetPrice
	Maximize        bool
	StepCountMin    int
	StepCountMax    int
	ComfortStepMin  float64
	ComfortStepMax  float64
	ComfortStepGrid float64
}

// OptimizeResult — итог поиска оптимума (EDR-0032 §3.4).
type OptimizeResult struct {
	Valid       bool
	Evaluated   int
	Target      OptimizeTarget
	Objective   float64 // значение цели лучшего кандидата (мажорные единицы валюты)
	BestConfig  Config
	BestResult  *Result // полный расчёт лучшей конфигурации
	ComfortStep float64 // шаг комфорта лучшего кандидата, мм
}

// Optimize выполняет детерминированный поиск оптимальной конфигурации марша
// (EDR-0032): перебор числа ступеней (и разбивки для L/U-маршей) по сетке
// шага комфорта; лучшим считается валидный кандидат с минимальным значением
// цели (цена/себестоимость/материал). Оценщик — существующий конвейер
// Calculate. Результат детерминирован (ADR-0003). Контекст отменяется
// между оценками (B2, EDR-0033 §3.1).
func (s *Service) Optimize(ctx context.Context, cfg Config, opts Options, req OptimizeRequest) (*OptimizeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	out, err := s.optimize(ctx, cfg, opts, req)
	optimizeDuration.With(flightLabel(cfg.Flight), string(outTarget(req.Target))).Observe(time.Since(start).Seconds())
	return out, err
}

func (s *Service) optimize(ctx context.Context, cfg Config, opts Options, req OptimizeRequest) (*OptimizeResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: optimize: %w", err)
	}
	if req.Target == "" {
		req.Target = TargetPrice
	}
	if !req.Target.Valid() {
		return nil, fmt.Errorf("stair: unknown optimization target %q", req.Target)
	}
	if _, err := buildConfiguration(cfg); err != nil {
		return nil, err
	}

	hm := cfg.Height.Millimeters()
	// Нормативный диапазон высоты ступени 150–200 мм (EDR-0002).
	nMin := int(math.Ceil(hm / 200.0))
	nMax := int(math.Floor(hm / 150.0))
	if nMin < 1 {
		nMin = 1
	}
	if req.StepCountMin > 0 {
		nMin = req.StepCountMin
	}
	if req.StepCountMax > 0 {
		nMax = req.StepCountMax
	}
	// Спиральный марш: шаг проступи определяется радиусом, а не шагом
	// комфорта (EDR-0007 §4). Расширяем диапазон радиусным ограничением
	// bWalk ∈ [260, 320] (2π·(R−W/3)/n), чтобы не пропустить допустимые n.
	if cfg.Flight == engineering.FlightSpiral {
		rw := cfg.OuterRadius.Millimeters() - cfg.Width.Millimeters()/3.0
		if rw > 0 {
			nLo := int(math.Floor(2 * math.Pi * rw / 320.0))
			nHi := int(math.Ceil(2 * math.Pi * rw / 260.0))
			if nLo > 0 && nLo < nMin {
				nMin = nLo
			}
			if nHi > nMax {
				nMax = nHi
			}
		}
	}
	if nMin > nMax {
		return &OptimizeResult{Valid: false, Target: req.Target}, nil
	}

	// Шаг комфорта [600, 640] (EDR-0001); спиральный марш не имеет его как
	// свободного параметра — диапазон вырождается в одну точку.
	sMin, sMax, sGrid := 600.0, 640.0, 2.0
	if cfg.Flight == engineering.FlightSpiral {
		sMin, sMax = 0, 0
	}
	if req.ComfortStepMin > 0 {
		sMin = req.ComfortStepMin
	}
	if req.ComfortStepMax > 0 {
		sMax = req.ComfortStepMax
	}
	if req.ComfortStepGrid > 0 {
		sGrid = req.ComfortStepGrid
	}

	// Диапазон ступеней нижнего марша: [1, n-1] для L/U; вырожден иначе.
	n1Min, n1Max := 1, 1
	if cfg.Flight == engineering.FlightLShape || cfg.Flight == engineering.FlightUShape {
		n1Max = nMax - 1
		if n1Max < 1 {
			n1Max = 1
		}
	}

	// Оценщик: кандидат → конфигурация → конвейер → цель.
	eval := func(k optimization.Candidate) (bool, optimization.Objective) {
		cand := cfg
		cand.StepHeight = engineering.Length(hm / float64(k.StepCount))
		cand.LowerStepCount = k.LowerStepCount
		o := opts
		if k.ComfortStep > 0 {
			o.ComfortStep = k.ComfortStep
		}
		res, err := s.Calculate(ctx, cand, o)
		if err != nil || res.Validation.Blocking || res.Price == nil {
			return false, 0
		}
		return true, optimization.Objective(objectiveValue(res, req.Target))
	}

	r := optimization.Search(ctx, eval, optimization.Options{
		StepCountMin: nMin,
		StepCountMax: nMax,
		LowerStepMin: n1Min,
		LowerStepMax: n1Max,
		ComfortMin:   sMin,
		ComfortMax:   sMax,
		ComfortStep:  sGrid,
		Maximize:     req.Maximize,
	})
	if r.Cancelled {
		return nil, fmt.Errorf("stair: optimize: %w", ctx.Err())
	}

	out := &OptimizeResult{Valid: r.Valid, Evaluated: r.Evaluated, Target: req.Target}
	if !r.Valid {
		return out, nil
	}

	// Полный расчёт лучшего кандидата (повторный детерминированный прогон).
	best := cfg
	best.StepHeight = engineering.Length(hm / float64(r.Best.StepCount))
	best.LowerStepCount = r.Best.LowerStepCount
	bo := opts
	if r.Best.ComfortStep > 0 {
		bo.ComfortStep = r.Best.ComfortStep
	}
	br, err := s.Calculate(ctx, best, bo)
	if err != nil {
		return nil, fmt.Errorf("stair: optimize best: %w", err)
	}
	out.BestConfig = best
	out.BestResult = br
	out.Objective = objectiveValue(br, req.Target)
	out.ComfortStep = r.Best.ComfortStep
	return out, nil
}

// objectiveValue возвращает целевую метрику результата в мажорных единицах
// валюты (для ранжирования и вывода).
func objectiveValue(res *Result, t OptimizeTarget) float64 {
	if res.Price == nil {
		return 0
	}
	cur := res.Price.Currency
	switch t {
	case TargetCost:
		return res.Price.ProductionCost.Major(cur)
	case TargetMaterial:
		return res.Price.Material.Major(cur)
	default:
		return res.Price.FinalPrice.Major(cur)
	}
}

// outTarget нормализует целевую метрику для label метрики (пустое значение
// — цена по умолчанию).
func outTarget(t OptimizeTarget) OptimizeTarget {
	if t == "" {
		return TargetPrice
	}
	return t
}
