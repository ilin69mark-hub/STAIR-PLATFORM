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
	Model        *kerngeo.Compound
	Mesh         *kerngeo.Mesh
	RailingMesh  *kerngeo.Mesh
	RoomMesh     *kerngeo.Mesh
	Issues       []kerngeo.ValidationIssue
	Measurement  Measurement
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
	// Свободное пространство перед первой ступенью (EDR-0023) — для ВСЕХ
	// типов марша (прямой, L, П, спираль). Сдвигаем модель от стены на
	// ApproachSpace по оси X, оставляя перед входом (первой ступенью) свободную
	// зону; требуемая ширина помещения автоматически учитывается в fit-check
	// (ниже), так как зона входит в габаритный бокс модели.
	approach := cfg.ApproachSpace.Millimeters()
	if approach == 0 {
		approach = 1000
	}
	if approach > 0 {
		t := kerngeo.Translate(approach, 0, 0)
		solids := model.Solids()
		shifted := make([]*kerngeo.Solid, len(solids))
		for i, s := range solids {
			shifted[i] = kerngeo.TransformSolid(s, t)
		}
		model = kerngeo.NewCompound(shifted...)
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

	// Декоративные перила (CONF-RAILING): добавляются только в preview mesh.
	// Они не являются частью несущей модели и не участвуют в измерениях
	// (SolidCount, объём, площадь, габарит) и в производственной декомпозиции.
	if err := appendRailingMesh(result, cfg); err != nil {
		return nil, err
	}

	// Декоративный «пол комнаты» и проверка вписываемости лестницы в заданный
	// периметр (fit-check). Пол не входит в несущую модель и не влияет на
	// измерения; передаётся отдельным RoomMesh, чтобы 3D-вьювер мог
	// отрисовать периметр помещения выделенным цветом. Если габариты лестницы
	// превышают заданные размеры помещения — неблокирующее предупреждение.
	if cfg.RoomWidth.Millimeters() > 0 && cfg.RoomLength.Millimeters() > 0 {
		rw := cfg.RoomWidth.Millimeters()
		rl := cfg.RoomLength.Millimeters()
		if room := buildRoomSolid(rw, rl); room != nil {
			if verts, tris, rerr := meshSolid(room, kerngeo.NewTessellationCache()); rerr == nil {
				rb := 0
				result.RoomMesh = &kerngeo.Mesh{}
				result.RoomMesh.Vertices = append(result.RoomMesh.Vertices, verts...)
				for _, tr := range tris {
					if aerr := result.RoomMesh.AddTriangle(rb+tr[0], rb+tr[1], rb+tr[2]); aerr != nil {
						return nil, aerr
					}
				}
			}
		}
		bb := result.Measurement.BoundingBox
		if bb.Max.X > rw+kerngeo.Precision || bb.Max.Y > rl+kerngeo.Precision {
			result.Issues = append(result.Issues, kerngeo.ValidationIssue{
				Code:     "room_fit",
				Severity: kerngeo.SeverityWarning,
				Element:  "room",
				Message: fmt.Sprintf(
					"Лестница не помещается в заданный периметр помещения: нужно помещение не менее %.0f×%.0f мм (X — направление марша, Y — ширина марша), сейчас %d×%d мм",
					bb.Max.X, bb.Max.Y, int(rw), int(rl),
				),
			})
		}
	}

	return result, nil
}

// buildRoomSolid строит тонкую декоративную плиту «пола комнаты» размером
// rw×rl (мм) в плоскости XY на уровне z≈0, роль "room". Используется только
// для визуализации периметра помещения в 3D-вьювере.
func buildRoomSolid(rw, rl float64) *kerngeo.Solid {
	p := []kerngeo.Point3{
		kerngeo.NewPoint3(0, 0, -20),
		kerngeo.NewPoint3(rw, 0, -20),
		kerngeo.NewPoint3(rw, rl, -20),
		kerngeo.NewPoint3(0, rl, -20),
	}
	s, err := kerngeo.Extrude(p, kerngeo.NewVector3(0, 0, 1), 20)
	if err != nil {
		return nil
	}
	return s.WithRole("room")
}

// appendRailingMesh строит декоративные тела перил (BuildRailingDecor) и
// добавляет их в preview mesh. Валидация перил выполняется, но их тела не
// влияют на Measurement и Model (ENG-GEO-0008: mesh — производная величина).
func appendRailingMesh(result *GenerationResult, cfg *engineering.StairConfiguration) error {
	decor, err := BuildRailingDecor(cfg)
	if err != nil {
		return fmt.Errorf("geometry: railing decor: %w", err)
	}
	// Марш сдвинут от стены на ApproachSpace (EDR-0023) для ВСЕХ типов: перила
	// строятся в исходных координатах, поэтому сдвигаем декор синхронно с маршем,
	// иначе перила визуально «отрываются» от марша на 3D-отрисовке.
	approach := cfg.ApproachSpace.Millimeters()
	if approach == 0 {
		approach = 1000
	}
	if approach != 0 {
		t := kerngeo.Translate(approach, 0, 0)
		for i, s := range decor {
			decor[i] = kerngeo.TransformSolid(s, t)
		}
	}
	// Перила вынесены в отдельный RailingMesh (как RoomMesh): 3D-вьювер
	// рисует их сплошным материалом БЕЗ каркаса (EdgesGeometry), чтобы между
	// балясинами и поручнями не появлялись лишние линии (см. GeometryViewer).
	if result.RailingMesh == nil {
		result.RailingMesh = &kerngeo.Mesh{}
	}
	base := len(result.RailingMesh.Vertices)
	for _, solid := range decor {
		cache := kerngeo.NewTessellationCache()
		for _, issue := range kerngeo.ValidateCached(solid, cache) {
			issue.Element = fmt.Sprintf("decor:%s/%s", solid.Role(), issue.Element)
			result.Issues = append(result.Issues, issue)
		}
		verts, tris, err := meshSolid(solid, cache)
		if err != nil {
			return fmt.Errorf("geometry: railing mesh: %w", err)
		}
		result.RailingMesh.Vertices = append(result.RailingMesh.Vertices, verts...)
		for _, tr := range tris {
			if err := result.RailingMesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
				return fmt.Errorf("geometry: railing mesh triangle: %w", err)
			}
		}
		base += len(verts)
	}
	return nil
}
