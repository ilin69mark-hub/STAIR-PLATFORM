// Package stair реализует application layer лестницы (BC-002): единый
// оркестратор сквозного расчёта проекта. Мета-уровень (прикладные
// сервисы) зависят от доменов и движков, но не от транспорта, HTTP,
// БД или UI (ADR-0006, DOM-0008). Все результаты детерминированы.
package stair

import (
	"fmt"

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
	Flight         solver.FlightResult
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
// только при невозможности выполнить расчёт (некорректный вход, сбой).
func (s *Service) Calculate(cfg Config, opts Options) (*Result, error) {
	c, err := buildConfiguration(cfg)
	if err != nil {
		return nil, err
	}

	comfort := opts.ComfortStep
	if comfort == 0 {
		comfort = solver.DefaultComfortStep
	}
	flight, vr, err := solver.SolveChecked(c, s.constraints, comfort)
	if err != nil {
		return nil, err
	}
	if vr.Blocking {
		return &Result{Validation: vr}, nil
	}

	gen, err := geometry.Generate(c)
	if err != nil {
		return nil, fmt.Errorf("stair: geometry: %w", err)
	}

	pkg, err := engmfg.Manufacture(c, gen)
	if err != nil {
		return nil, fmt.Errorf("stair: manufacturing: %w", err)
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

	return &Result{
		Validation:     vr,
		Flight:         flight,
		Measurement:    gen.Measurement,
		GeometryIssues: gen.Issues,
		Mesh:           gen.Mesh,
		Package:        pkg,
		Cost:           ds,
		Price:          price,
	}, nil
}

// buildConfiguration собирает и валидирует параметрическую конфигурацию
// из исходных параметров пользователя.
func buildConfiguration(cfg Config) (*engineering.StairConfiguration, error) {
	c, err := engineering.NewStairConfiguration(cfg.Width, cfg.Height, cfg.Flight)
	if err != nil {
		return nil, fmt.Errorf("stair: %w", err)
	}
	c.StepHeight = cfg.StepHeight
	c.StringerThickness = cfg.StringerThickness
	c.StepThickness = cfg.StepThickness
	c.Clearance = cfg.Clearance
	c.RailingHeight = cfg.RailingHeight
	return c, nil
}
