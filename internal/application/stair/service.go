// Package stair реализует application layer лестницы (BC-002): единый
// оркестратор сквозного расчёта проекта. Мета-уровень (прикладные
// сервисы) зависят от доменов и движков, но не от транспорта, HTTP,
// БД или UI (ADR-0006, DOM-0008). Все результаты детерминированы.
package stair

import (
	"context"
	"fmt"
	"time"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/geometry"
	engmfg "stairplatform/internal/engine/manufacturing"
	engprc "stairplatform/internal/engine/pricing"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
	kerngeo "stairplatform/internal/geometry"
)

// Config — пользовательские параметры лестницы (исходные, до Solver):
// высота подъёма, ширина, тип марша и целевые/производственные параметры.
type Config struct {
	Width             engineering.Length // мм — ширина марша
	Height            engineering.Length // мм — высота подъёма H
	Flight            engineering.FlightType
	StepHeight        engineering.Length // мм — целевая высота ступени h0
	StringerThickness engineering.Length // мм
	StepThickness     engineering.Length // мм
	Clearance         engineering.Length // мм
	RailingHeight     engineering.Length // мм
	// LandingWidth и LowerStepCount — специфичны для маршей с площадкой
	// (EDR-0005 L-образный, EDR-0006 П-образный).
	LandingWidth   engineering.Length // мм — ширина площадки Wp
	LowerStepCount int                // n1 — число ступеней нижнего марша
	// OuterRadius — специфичен для спиральной лестницы (EDR-0007):
	// наружный радиус марша R (радиус колонны r = R − W).
	OuterRadius engineering.Length
}

// Options — опциональные настройки расчёта; нулевое значение даёт дефолты.
type Options struct {
	// ComfortStep — шаг комфорта S (600–640); 0 → solver.DefaultComfortStep.
	ComfortStep float64
	// Rates — ставки цены; nil → engprc.DefaultRates().
	Rates *engprc.Rates
	// MachineRates — ставки машинных операций; nil → engmfg.DefaultMachineRates().
	MachineRates *engmfg.MachineRates
}

// Result — сквозной результат расчёта проекта (все этапы конвейера).
type Result struct {
	Validation     validation.Result
	Flight         solver.FlightResult  // прямой марш
	LShape         *solver.LShapeResult // L-образный марш (Flight == LShape)
	UShape         *solver.UShapeResult // П-образный марш (Flight == UShape)
	Spiral         *solver.SpiralResult // спиральный марш (Flight == Spiral)
	Measurement    geometry.Measurement
	GeometryIssues []kerngeo.ValidationIssue
	Mesh           *kerngeo.Mesh                // preview mesh для визуализации (ENG-GEO-0008)
	Package        *dommfg.ManufacturingPackage // полные Parts/BOM/CutList/Nesting
	Cost           *dommfg.ManufacturingCostDataset
	Price          *domprc.PriceBreakdown
}

// Service — прикладной сервис расчёта лестницы. Является единственной
// точкой входа сквозного конвейера для любых транспортов.
type Service struct {
	constraints *constraint.ConstraintSet
}

// NewService создаёт сервис со стандартным профилем правил (EDR-0002).
func NewService() *Service {
	return &Service{
		constraints: constraint.StandardProfile("STANDARD"),
	}
}

// Calculate выполняет полный конвейер: Solver → Validation → Geometry →
// Manufacturing → Cost → Price (BC-002, PRC-0002). При blocking-валидации
// возвращает Result с заполненным Validation (не ошибка); ошибка —
// только при невозможности выполнить расчёт (некорректный вход, сбой или
// отмена контекста). Контекст проверяется между стадиями (B2, EDR-0033).
func (s *Service) Calculate(ctx context.Context, cfg Config, opts Options) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	flight := flightLabel(cfg.Flight)
	start := time.Now()
	res, err := s.calculate(ctx, cfg, opts)
	valid := "true"
	if res == nil || res.Validation.Blocking {
		valid = "false"
	}
	if err != nil && res == nil {
		valid = "false"
	}
	calculateDuration.With(flight, valid).Observe(time.Since(start).Seconds())
	return res, err
}

func (s *Service) calculate(ctx context.Context, cfg Config, opts Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	c, err := buildConfiguration(cfg)
	if err != nil {
		return nil, err
	}

	comfort := opts.ComfortStep
	if comfort == 0 {
		comfort = solver.DefaultComfortStep
	}

	res := &Result{}
	switch cfg.Flight {
	case engineering.FlightLShape:
		lres, vr, err := solver.SolveCheckedLShape(c, s.constraints, comfort)
		if err != nil {
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: vr}, nil
		}
		res.Validation = vr
		res.LShape = &lres
	case engineering.FlightUShape:
		ures, vr, err := solver.SolveCheckedUShape(c, s.constraints, comfort)
		if err != nil {
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: vr}, nil
		}
		res.Validation = vr
		res.UShape = &ures
	case engineering.FlightSpiral:
		sres, vr, err := solver.SolveCheckedSpiral(c, s.constraints)
		if err != nil {
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: vr}, nil
		}
		res.Validation = vr
		res.Spiral = &sres
	default:
		flight, vr, err := solver.SolveChecked(c, s.constraints, comfort)
		if err != nil {
			return nil, err
		}
		if vr.Blocking {
			return &Result{Validation: vr}, nil
		}
		res.Validation = vr
		res.Flight = flight
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	gen, err := geometry.Generate(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("stair: geometry: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	pkg, err := engmfg.Manufacture(c, gen)
	if err != nil {
		return nil, fmt.Errorf("stair: manufacturing: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	rates := opts.MachineRates
	if rates == nil {
		m := engmfg.DefaultMachineRates()
		rates = &m
	}
	ds, err := engmfg.PrepareCost(pkg, engmfg.DefaultMaterialRegistry(), *rates)
	if err != nil {
		return nil, fmt.Errorf("stair: cost: %w", err)
	}

	priceRates := engprc.DefaultRates()
	if opts.Rates != nil {
		priceRates = *opts.Rates
	}
	price, err := engprc.Price(ds, priceRates)
	if err != nil {
		return nil, fmt.Errorf("stair: pricing: %w", err)
	}

	res.Measurement = gen.Measurement
	res.GeometryIssues = gen.Issues
	res.Mesh = gen.Mesh
	res.Package = pkg
	res.Cost = ds
	res.Price = price
	return res, nil
}

// flightLabel нормализует тип марша в label метрики (пустое значение —
// прямой марш).
func flightLabel(f engineering.FlightType) string {
	if f == "" {
		return string(engineering.FlightStraight)
	}
	return string(f)
}

// buildConfiguration собирает и валидирует параметрическую конфигурацию
// из исходных параметров пользователя.
// ValidateConfig — дешёвая валидация конфигурации до постановки в очередь
// (EDR-0035 §3.4): тот же проверочный шаг, что buildConfiguration в начале
// Calculate, но без конвейера. Используется async-эндпоинтом, чтобы
// заведомо невалидный вход отбраковать сразу (422), а не гнать в воркер.
func ValidateConfig(cfg Config) error {
	if _, err := buildConfiguration(cfg); err != nil {
		return fmt.Errorf("stair: %w", err)
	}
	return nil
}

// ValidateConfig — метод сервиса (тот же шаг, что и функция ValidateConfig);
// нужен транспортному/ассистентному слою через интерфейс (DOM-0008).
func (s *Service) ValidateConfig(cfg Config) error {
	return ValidateConfig(cfg)
}

func buildConfiguration(cfg Config) (*engineering.StairConfiguration, error) {
	c := &engineering.StairConfiguration{
		Width:      cfg.Width,
		Height:     cfg.Height,
		Flight:     cfg.Flight,
		StepCount:  1,
		StepHeight: cfg.Height,
	}
	c.StepHeight = cfg.StepHeight
	c.StringerThickness = cfg.StringerThickness
	c.StepThickness = cfg.StepThickness
	c.Clearance = cfg.Clearance
	c.RailingHeight = cfg.RailingHeight
	c.LandingWidth = cfg.LandingWidth
	c.LowerStepCount = cfg.LowerStepCount
	c.OuterRadius = cfg.OuterRadius
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	return c, nil
}
