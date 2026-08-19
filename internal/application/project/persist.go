package project

import (
	"fmt"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/geometry"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
	kerngeo "stairplatform/internal/geometry"
)

// Snapshot — канонический экспортный документ расчёта (ввод в /export);
// содержит инварианты и полный результат конвейера. Сериализуется
// инфраструктурой (encoding/json разрешён только вне application,
// ADR-0006). Формат стабилен для версии API v1 (API-0015 Versioning).
type Snapshot struct {
	ProjectID string `json:"project_id"`

	Validation    validation.Result                       `json:"validation"`
	Flight        solver.FlightResult                     `json:"flight"`
	LShape        *solver.LShapeResult                    `json:"lshape,omitempty"`
	UShape        *solver.UShapeResult                    `json:"ushape,omitempty"`
	Spiral        *solver.SpiralResult                    `json:"spiral,omitempty"`
	// Производственные параметры конфигурации (эхо, BC-002): толщина
	// проступи, высота перил, наличие подступенков — для 2D-рендера.
	StepThickness float64                                 `json:"step_thickness"`
	RailingHeight float64                                 `json:"railing_height"`
	Riser         bool                                    `json:"riser"`
	StringerThickness float64                             `json:"stringer_thickness"`
	Measurement   geometry.Measurement                    `json:"measurement"`
	Mesh          *kerngeo.Mesh                           `json:"mesh,omitempty"`
	IssueCount    int                                     `json:"issue_count"`
	Manufacturing *manufacturing.ManufacturingPackage     `json:"manufacturing,omitempty"`
	Pricing       *pricing.PriceBreakdown                 `json:"pricing,omitempty"`
	Cost          *manufacturing.ManufacturingCostDataset `json:"cost,omitempty"`
}

// NewSnapshot строит экспортный документ из результата конвейера.
func NewSnapshot(projectID string, res *stair.Result) Snapshot {
	return Snapshot{
		ProjectID:     projectID,
		Validation:    res.Validation,
		Flight:        res.Flight,
		LShape:        res.LShape,
		UShape:        res.UShape,
		Spiral:        res.Spiral,
		StepThickness: res.StepThickness.Millimeters(),
		RailingHeight: res.RailingHeight.Millimeters(),
		Riser:         res.Riser,
		StringerThickness: res.StringerThickness.Millimeters(),
		Measurement:   res.Measurement,
		Mesh:          res.Mesh,
		IssueCount:    len(res.GeometryIssues),
		Manufacturing: res.Package,
		Pricing:       res.Price,
		Cost:          res.Cost,
	}
}

// toConfigEntity преобразует входную конфигурацию в сущность для БД.
func toConfigEntity(projectID string, cfg stair.Config, opts stair.Options) *StairConfiguration {
	return &StairConfiguration{
		ProjectID:           projectID,
		WidthMM:             cfg.Width.Millimeters(),
		HeightMM:            cfg.Height.Millimeters(),
		Flight:              string(cfg.Flight),
		StepHeightMM:        cfg.StepHeight.Millimeters(),
		StringerThicknessMM: cfg.StringerThickness.Millimeters(),
		StepThicknessMM:     cfg.StepThickness.Millimeters(),
		Riser:               cfg.Riser,
		ClearanceMM:         cfg.Clearance.Millimeters(),
		RailingHeightMM:     cfg.RailingHeight.Millimeters(),
		ComfortStepMM:       opts.ComfortStep,
		LandingWidthMM:      cfg.LandingWidth.Millimeters(),
		LowerStepCount:      cfg.LowerStepCount,
		OuterRadiusMM:       cfg.OuterRadius.Millimeters(),
	}
}

// fromConfigEntity восстанавливает stair.Config из сохранённой ревизии
// (обратное к toConfigEntity, EDR-0022 §3.4). Используется CAD-экспортом.
// ComfortStep отсутствует в stair.Config — он возвращается в Options.
func fromConfigEntity(e *StairConfiguration) (stair.Config, stair.Options, error) {
	mk := func(v float64) (engineering.Length, error) {
		l, err := engineering.NewLength(v)
		if err != nil {
			return 0, fmt.Errorf("project: invalid stored %v mm: %w", v, err)
		}
		return l, nil
	}
	width, err := mk(e.WidthMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	height, err := mk(e.HeightMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	step, err := mk(e.StepHeightMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	stringer, err := mk(e.StringerThicknessMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	thick, err := mk(e.StepThicknessMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	clearance, err := mk(e.ClearanceMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	railing, err := mk(e.RailingHeightMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	landing, err := mk(e.LandingWidthMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	outer, err := mk(e.OuterRadiusMM)
	if err != nil {
		return stair.Config{}, stair.Options{}, err
	}
	cfg := stair.Config{
		Width:             width,
		Height:            height,
		Flight:            engineering.FlightType(e.Flight),
		StepHeight:        step,
		StringerThickness: stringer,
		StepThickness:     thick,
		Riser:             e.Riser,
		Clearance:         clearance,
		RailingHeight:     railing,
		LandingWidth:      landing,
		LowerStepCount:    e.LowerStepCount,
		OuterRadius:       outer,
	}
	return cfg, stair.Options{ComfortStep: e.ComfortStepMM}, nil
}
