package geometry

import (
	"context"
	"fmt"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/scheduler"
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

// Generate строит параметрическую B-Rep модель марша (прямого,
// L-образного, П-образного или спирального согласно cfg.Flight),
// валидирует её (ENG-GEO-0018), измеряет (ENG-GEO-0013) и строит preview
// mesh (ENG-GEO-0008) — фасад Geometry Engine (ENG-0003). Предусловие: конфигурация с положительными решёнными
// параметрами (после Solver, ENG-0001); невыполнение возвращает ошибку.
// Валидация не блокирует результат: отчёт issues собирается в результат
// (валидная модель из корректных параметров не содержит ошибок уровня
// SeverityError). Результат детерминирован при детерминированной конфигурации.
// Контекст отмены (B2, EDR-0033 §3.1): errgroup.WithContext глобально
// отменяет обработку при отмене/ошибке любого воркера.
func Generate(ctx context.Context, cfg *engineering.StairConfiguration) (*GenerationResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("geometry: %w", ctx.Err())
	}
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
	case engineering.FlightSpiral:
		model, err = BuildSpiralFlight(cfg)
	default:
		model, err = BuildStraightFlight(cfg)
	}
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("geometry: %w", ctx.Err())
	}
	result := &GenerationResult{Model: model}

	// Каждая грань принадлежит ровно одному телу, поэтому кеш на тело даёт
	// тот же эффект дедупликации, что и общий кеш на весь вызов (EM-06,
	// B2.2), но позволяет обрабатывать тела параллельно без разделяемого
	// состояния. Тела неизменяемы — параллелизм внутри стадии безопасен.
	// Обработка выполняется Scheduler'ом (B3, EDR-0034 §3.4): результат
	// собирается по слотам индексов (детерминизм ADR-0003), ошибка —
	// первый по индексу сбой либо отмена контекста.
	solids := model.Solids()
	caches := make([]*kerngeo.TessellationCache, len(solids))
	for i := range caches {
		caches[i] = kerngeo.NewTessellationCache()
	}

	type solidOut struct {
		issues []kerngeo.ValidationIssue
		vol    float64
		area   float64
		verts  []kerngeo.Point3
		tris   [][3]int
		err    error
	}
	outs := make([]solidOut, len(solids))

	if err := scheduler.New(0).Execute(ctx, len(solids), func(i int) error {
		solid := solids[i]
		o := solidOut{}
		for _, issue := range kerngeo.ValidateCached(solid, caches[i]) {
			issue.Element = fmt.Sprintf("solid:%d/%s", i, issue.Element)
			o.issues = append(o.issues, issue)
		}
		o.vol, o.err = kerngeo.VolumeCached(solid, caches[i])
		if o.err == nil {
			o.area, o.err = kerngeo.SurfaceAreaCached(solid, caches[i])
		}
		if o.err == nil {
			o.verts, o.tris, o.err = meshSolid(solid, caches[i])
		}
		outs[i] = o
		return o.err
	}); err != nil {
		return nil, fmt.Errorf("geometry: solid processing: %w", err)
	}

	// сбор результатов в порядке индексов — детерминизм.
	for i := range outs {
		result.Issues = append(result.Issues, outs[i].issues...)
		result.Measurement.Volume += outs[i].vol
		result.Measurement.SurfaceArea += outs[i].area
	}
	result.Measurement.SolidCount = kerngeo.SolidCount(model)
	result.Measurement.BoundingBox = kerngeo.BoundingBox(model)

	// preview mesh — производная величина, собранная из слотов в порядке тел.
	result.Mesh = &kerngeo.Mesh{}
	base := 0
	for i := range outs {
		result.Mesh.Vertices = append(result.Mesh.Vertices, outs[i].verts...)
		for _, tr := range outs[i].tris {
			if err := result.Mesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
				return nil, err
			}
		}
		base += len(outs[i].verts)
	}
	return result, nil
}
