package geometry

import (
	"fmt"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// Measurement — результаты измерений модели (ENG-GEO-0013).
type Measurement struct {
	SolidCount  int
	Volume      float64
	SurfaceArea float64
	BoundingBox kerngeo.BBox
}

// GenerationResult — результат работы Geometry Engine (ENG-0002): модель,
// preview mesh, отчёт валидации и измерения. Mesh и измерения являются
// производными величинами и никогда не являются источником истины.
type GenerationResult struct {
	Model       *kerngeo.Compound
	Mesh        *kerngeo.Mesh
	Issues      []kerngeo.ValidationIssue
	Measurement Measurement
}

// Generate строит параметрическую B-Rep модель марша (прямого или
// L-образного согласно cfg.Flight), валидирует её (ENG-GEO-0018), измеряет
// (ENG-GEO-0013) и строит preview mesh (ENG-GEO-0008) — фасад Geometry
// Engine (ENG-0003). Предусловие: конфигурация с положительными решёнными
// параметрами (после Solver, ENG-0001); невыполнение возвращает ошибку.
// Валидация не блокирует результат: отчёт issues собирается в результат
// (валидная модель из корректных параметров не содержит ошибок уровня
// SeverityError). Результат детерминирован при детерминированной конфигурации.
func Generate(cfg *engineering.StairConfiguration) (*GenerationResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("geometry: configuration is required")
	}
	var model *kerngeo.Compound
	var err error
	switch cfg.Flight {
	case engineering.FlightLShape:
		model, err = BuildLShapeFlight(cfg)
	case engineering.FlightUShape:
		model, err = BuildUShapeFlight(cfg)
	default:
		model, err = BuildStraightFlight(cfg)
	}
	if err != nil {
		return nil, err
	}
	result := &GenerationResult{Model: model}

	// валидация всех тел модели.
	for i, solid := range model.Solids() {
		for _, issue := range kerngeo.Validate(solid) {
			issue.Element = fmt.Sprintf("solid:%d/%s", i, issue.Element)
			result.Issues = append(result.Issues, issue)
		}
	}

	// измерения модели.
	result.Measurement.SolidCount = kerngeo.SolidCount(model)
	result.Measurement.BoundingBox = kerngeo.BoundingBox(model)
	for _, solid := range model.Solids() {
		vol, err := kerngeo.Volume(solid)
		if err != nil {
			return nil, fmt.Errorf("geometry: volume: %w", err)
		}
		area, err := kerngeo.SurfaceArea(solid)
		if err != nil {
			return nil, fmt.Errorf("geometry: surface area: %w", err)
		}
		result.Measurement.Volume += vol
		result.Measurement.SurfaceArea += area
	}

	// preview mesh — производная величина.
	result.Mesh, err = ToPreviewMesh(model)
	if err != nil {
		return nil, err
	}
	return result, nil
}
