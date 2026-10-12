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
	RailingMesh *kerngeo.Mesh
	RoomMesh    *kerngeo.Mesh
	Issues      []kerngeo.ValidationIssue
	Measurement Measurement
	// MillingFeatures — детали, которым нужна фрезеровная обработка, с
	// длиной фрезеруемого ребра. Геометрия здесь единственный источник
	// истины: производство берёт длину отсюда, а не пересчитывает по
	// конфигурации — иначе расчёт обработки разъехался бы с моделью при
	// первом же изменении геометрии детали.
	//
	// Длина ребра — это и есть объём работы: скругление носка идёт по всей
	// ширине ступени, поэтому время фрезеровки пропорционально ширине марша,
	// и именно так оно и тарифицируется.
	MillingFeatures []MillingFeature
}

// MillingFeature — одна деталь, требующая фрезеровки.
type MillingFeature struct {
	// Role — роль тела в модели ("tread", "landing", …), по ней производство
	// сопоставляет деталь из раскроя.
	Role string
	// EdgeLengthMM — длина фрезеруемого ребра, мм.
	EdgeLengthMM float64
	// RadiusMM — радиус скругления, мм. В операцию попадает как описание
	// работы, на тариф не влияет.
	RadiusMM float64
	// Quantity — сколько таких деталей (ступени одинаковые, но считать их
	// по одной было бы неверно: 15 ступеней — это 15 носов по 900 мм).
	Quantity int
}

// buildModel строит B-Rep модель по типу марша и сдвигает её от стены на
// ApproachSpace. Общая часть для Generate и ModelBoundingBox: обе точки входа
// обязаны давать ОДИН И ТОТ ЖЕ габарит, иначе проверка вписываемости в
// вариациях считала бы не то.
func buildModel(ctx context.Context, cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	model, err := buildGeometry(ctx, cfg)
	if err != nil {
		return nil, err
	}
	shift := approachShift(cfg)
	if shift > 0 {
		t := kerngeo.Translate(shift, 0, 0)
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
	return model, nil
}

// approachShift — сдвиг модели от стены на ApproachSpace (норма 1000–1200 мм).
// Применяется ко ВСЕМ типам марша (прямой, L, П), оставляя перед
// входом (первой ступенью) свободную зону; зона входит в габаритный бокс
// модели, поэтому требуемая ширина помещения учитывается в fit-check.
// Решение владельца 2026-10-12 (S-167).
func approachShift(cfg *engineering.StairConfiguration) float64 {
	approach := cfg.ApproachSpace.Millimeters()
	if approach == 0 {
		approach = 1000
	}
	return approach
}

// buildGeometry строит B-Rep модель марша БЕЗ сдвига от стены.
func buildGeometry(ctx context.Context, cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	if dbgHook != nil {
		dbgHook()
	}
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
	case engineering.FlightStraight:
		model, err = BuildStraightFlight(cfg)
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
	return model, nil
}

// ModelBoundingBox возвращает ТОЛЬКО габарит модели марша — без валидации,
// триангуляции и preview mesh.
//
// Зачем: подбор вариантов (variation.ForRoomFit) перебирает около тысячи
// конфигураций на кейс и из результата Geometry Engine берёт РОВНО габарит
// (X, Y). Полный Generate на металлокаркасе — это 96 тел, триангуляция всех
// граней с отверстиями и сборка меша, то есть ~97% лишней работы на каждый
// кандидат.
//
// Габарит совпадает с Measurement.BoundingBox из Generate: обе точки входа
// берут одну и ту же модель из buildGeometry, а сдвиг от стены прибавляется
// к X габарита — трансляция не меняет размеры бокса, поэтому копировать
// вершины ради сдвига не нужно.
func ModelBoundingBox(ctx context.Context, cfg *engineering.StairConfiguration) (kerngeo.BBox, error) {
	model, err := buildGeometry(ctx, cfg)
	if err != nil {
		return kerngeo.BBox{}, err
	}
	bb := kerngeo.BoundingBox(model)
	if shift := approachShift(cfg); shift > 0 {
		bb.Min.X += shift
		bb.Max.X += shift
	}
	return bb, nil
}

// Generate строит параметрическую B-Rep модель марша (прямого,
// L-образного или П-образного согласно cfg.Flight),
// валидирует её (ENG-GEO-0018), измеряет (ENG-GEO-0013) и строит preview
// mesh (ENG-GEO-0008) — фасад Geometry Engine (ENG-0003). Предусловие: конфигурация с положительными решёнными
// параметрами (после Solver, ENG-0001); невыполнение возвращает ошибку.
//
// Валидация не блокирует результат: отчёт issues собирается в результат
// (валидная модель из корректных параметров не содержит ошибок уровня
// SeverityError). Результат детерминирован при детерминированной конфигурации.
// Контекст отмены (B2, EDR-0033 §3.1): errgroup.WithContext глобально
// отменяет обработку при отмене/ошибке любого воркера.
func Generate(ctx context.Context, cfg *engineering.StairConfiguration) (*GenerationResult, error) {
	model, err := buildModel(ctx, cfg)
	if err != nil {
		return nil, err
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

	// Общая точка отсчёта UV для всех деталей: минимальный угол габарита
	// модели. Считается до параллельной стадии, чтобы все тела мешали
	// одинаково (см. meshSolid).
	uvOrigin := kerngeo.BoundingBox(model).Min

	type solidOut struct {
		issues []kerngeo.ValidationIssue
		vol    float64
		area   float64
		verts  []kerngeo.Point3
		uvs    []kerngeo.Point2
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
			o.verts, o.uvs, o.tris, o.err = meshSolid(solid, caches[i], uvOrigin)
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
	// Детали под фрезеровку: длина ребра берётся из габарита построенной
	// модели, поэтому совпадает с тем, что действительно изготовят.
	result.MillingFeatures = millingFeaturesOf(cfg, model)

	// preview mesh — производная величина, собранная из слотов в порядке тел.
	// Ёмкости считаются заранее: при append без резерва срез вершин меша
	// перевыделялся логарифмически, и на металлокаркасе это были гигабайты
	// копирования за один вызов Generate.
	nv, nuv, ntr := 0, 0, 0
	for i := range outs {
		nv += len(outs[i].verts)
		nuv += len(outs[i].uvs)
		ntr += len(outs[i].tris)
	}
	result.Mesh = kerngeo.NewMesh(nv, nuv, ntr)
	base := 0
	for i := range outs {
		result.Mesh.Vertices = append(result.Mesh.Vertices, outs[i].verts...)
		result.Mesh.UV = append(result.Mesh.UV, outs[i].uvs...)
		start := len(result.Mesh.Triangles)
		for _, tr := range outs[i].tris {
			if err := result.Mesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
				return nil, err
			}
		}
		// Роль тела (этап 1): 3D-вьювер красит ступени/косоуры/площадку
		// разными материалами по диапазону треугольников.
		result.Mesh.PartRanges = append(result.Mesh.PartRanges, kerngeo.PartRange{
			Solid: i,
			Role:  solids[i].Role(),
			Start: start,
			End:   len(result.Mesh.Triangles),
		})
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
			if verts, uvs, tris, rerr := meshSolid(room, kerngeo.NewTessellationCache(), kerngeo.Point3{}); rerr == nil {
				rb := 0
				result.RoomMesh = &kerngeo.Mesh{}
				result.RoomMesh.Vertices = append(result.RoomMesh.Vertices, verts...)
				result.RoomMesh.UV = append(result.RoomMesh.UV, uvs...)
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

	// Просвет под перекрытием (S-168). Считается ПО ГОТОВОЙ МОДЕЛИ, а не по
	// числу, введённому пользователем: для каждой проступи берётся вертикаль
	// от её верха до низа перекрытия, и проверяется худшая ступень — самая
	// высокая из тех, что ещё находятся ПОД перекрытием. Чем выше ступень,
	// тем меньше просвет, поэтому худшая — последняя под перекрытием, а не
	// первая. Без CeilingHeight (0) проверка не выполняется.
	if cfg.CeilingHeight.Millimeters() > 0 {
		if clear := minTreadClearance(model, cfg.CeilingHeight.Millimeters()); clear > 0 && clear < MinHeadroomMM {
			result.Issues = append(result.Issues, kerngeo.ValidationIssue{
				Code:     "clearance_fit",
				Severity: kerngeo.SeverityWarning,
				Element:  "ceiling",
				Message: fmt.Sprintf(
					"Просвет под перекрытием %.0f мм — меньше нормы %.0f мм. Поднимите перекрытие или уменьшите высоту этажа",
					clear, MinHeadroomMM,
				),
			})
		}
	}

	return result, nil
}

// MinHeadroomMM — минимальный проход под перекрытием над маршем (EDR-0002,
// GEO-CLEARANCE). Используется геометрической проверкой просвета.
const MinHeadroomMM = 2000.0

// minTreadClearance возвращает худший просвет между верхом проступи и низом
// перекрытия, мм. Рассматриваются только ступени, расположенные ПОД
// перекрытием: ступень на уровне самого перекрытия — это точка выхода на
// этаж, проходить под ней не нужно, и её просвет всегда был бы нулём.
// Возвращает 0, если под перекрытием нет ни одной ступени.
func minTreadClearance(model *kerngeo.Compound, ceiling float64) float64 {
	worst := 0.0
	for _, s := range model.Solids() {
		if s.Role() != "tread" {
			continue
		}
		bb := kerngeo.SolidBoundingBox(s)
		top := bb.Max.Z
		clear := ceiling - top
		if clear <= kerngeo.Precision {
			continue // ступень на уровне перекрытия или выше — не проходим
		}
		if worst == 0 || clear < worst {
			worst = clear
		}
	}
	return worst
}

// millingFeaturesOf собирает список деталей под фрезеровку по ГОТОВОЙ модели.
//
// Количество ступеней берётся подсчётом тел с ролью "tread", а не из
// конфигурации: у L/П-маршей ступени набираются из двух сегментов, у П-лестницы
// на поворотных ступенях — из вееров, и любой расчёт «n1 + n2» в конфигурации
// рано или поздно разошёлся бы с тем, что реально построено. Единственный
// источник истины — сама модель.
//
// Длина ребра носа равна ширине марша: нос идёт по всей ширине проступи, и
// именно эта длина тарифицируется как работа.
func millingFeaturesOf(cfg *engineering.StairConfiguration, model *kerngeo.Compound) []MillingFeature {
	radius := cfg.TreadNoseRadiusMM.Millimeters()
	if radius <= kerngeo.Precision || model == nil {
		// Металл (или явный ноль): фасок нет, фрезеровка не нужна.
		return nil
	}
	treads, landings := 0, 0
	for _, solid := range model.Solids() {
		switch solid.Role() {
		case "tread":
			treads++
		case "landing":
			landings++
		}
	}
	if treads == 0 && landings == 0 {
		return nil
	}
	bb := kerngeo.BoundingBox(model)
	width := bb.Max.Y - bb.Min.Y
	if width <= kerngeo.Precision {
		return nil
	}
	features := make([]MillingFeature, 0, 2)
	if treads > 0 {
		features = append(features, MillingFeature{
			Role: "tread", EdgeLengthMM: width, RadiusMM: radius, Quantity: treads,
		})
	}
	// Площадка: фрезеруется её ВНЕШНЯЯ кромка вдоль Y, то есть на всю ширину
	// марша — столько же, сколько нос ступени. Кромка, которой примыкает
	// верхний марш, не фрезеруется (см. landingRoundSide).
	if landings > 0 {
		features = append(features, MillingFeature{
			Role: "landing", EdgeLengthMM: width, RadiusMM: radius, Quantity: landings,
		})
	}
	return features
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
	// Для railing=none BuildRailingDecor возвращает nil — создаём пустой меш
	// с пустыми слайсами (JSON [] а не null) чтобы фронт не падал на null.length.
	if len(decor) == 0 {
		result.RailingMesh = &kerngeo.Mesh{Vertices: []kerngeo.Point3{}, Triangles: [][3]int{}}
		return nil
	}
	if result.RailingMesh == nil {
		result.RailingMesh = &kerngeo.Mesh{}
	}
	base := len(result.RailingMesh.Vertices)
	// Точка отсчёта UV одна на всю модель: считаем её ОДИН раз до цикла, иначе
	// габарит пересчитывался заново на каждом теле декора.
	uvOrigin := kerngeo.BoundingBox(result.Model).Min
	for _, solid := range decor {
		cache := kerngeo.NewTessellationCache()
		for _, issue := range kerngeo.ValidateCached(solid, cache) {
			issue.Element = fmt.Sprintf("decor:%s/%s", solid.Role(), issue.Element)
			result.Issues = append(result.Issues, issue)
		}
		verts, uvs, tris, err := meshSolid(solid, cache, uvOrigin)
		if err != nil {
			return fmt.Errorf("geometry: railing mesh: %w", err)
		}
		result.RailingMesh.Vertices = append(result.RailingMesh.Vertices, verts...)
		result.RailingMesh.UV = append(result.RailingMesh.UV, uvs...)
		start := len(result.RailingMesh.Triangles)
		for _, tr := range tris {
			if err := result.RailingMesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
				return fmt.Errorf("geometry: railing mesh triangle: %w", err)
			}
		}
		// Роль декора (поручень/балясина) — для материала ограждения (этап 1).
		result.RailingMesh.PartRanges = append(result.RailingMesh.PartRanges, kerngeo.PartRange{
			Role:  solid.Role(),
			Start: start,
			End:   len(result.RailingMesh.Triangles),
		})
		base += len(verts)
	}
	return nil
}

// SetDbgCounter — счётчик вызовов buildModel для диагностики.
func SetDbgCounter(f func()) { dbgHook = f }

var dbgHook func()
