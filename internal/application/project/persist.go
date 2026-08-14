package project

import (
	"stairplatform/internal/application/stair"
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
		ClearanceMM:         cfg.Clearance.Millimeters(),
		RailingHeightMM:     cfg.RailingHeight.Millimeters(),
		ComfortStepMM:       opts.ComfortStep,
		LandingWidthMM:      cfg.LandingWidth.Millimeters(),
		LowerStepCount:      cfg.LowerStepCount,
	}
}
