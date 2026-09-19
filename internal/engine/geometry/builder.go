// Package geometry implements the Geometry Engine (ENG-0002, ENG-GEO-0007):
// it builds parametric B-Rep models of stair flights from a StairConfiguration.
// The engine knows only the domain model and the domain-agnostic geometry
// kernel; parameters are the single source of truth of geometry (BC-002).
package geometry

import (
	"context"
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/scheduler"
	kerngeo "stairplatform/internal/geometry"
)

// flightSideInsetMM — величина, на которую каждый марш П-образной лестницы
// (платформенный режим) сужается от внешней кромки. Оба марша сужаются на
// одну и ту же величину, поэтому между ними образуется внутренний зазор
// 2·flightSideInsetMM = 100 мм. Площадка остаётся шириной 2·W — зазор
// получается внутренним за счёт сужения маршей, внешний габарит не меняется.
const flightSideInsetMM = 50.0

// StringerExtent возвращает вертикальный габарит профиля косоура
// прямого марша: от уровня пола (низ передней гранки) до верхнего
// седла пилы (H − StepThickness). Экспортируется, чтобы советник и
// раскрой использовали единый габарит косоура (BC-002).
func StringerExtent(heightMm, stepThMm float64) float64 {
	return heightMm - stepThMm
}

// BuildStraightFlight строит параметрическую B-Rep модель прямого марша
// (ENG-GEO-0007). Модель состоит из 2 косоуров и n проступей при
// StepThickness > 0 (EDR-0004); подступенки (3n тел) добавляются только при
// cfg.Riser — при false лестница имеет открытые ступени. Итого при
// StepThickness > 0 и Riser: 2+4n тела.
// Координаты: X — направление подъёма, Y — ширина, Z — высота (ADR-0008).
// Геометрия всегда вычисляется заново из параметров (BC-002).
func BuildStraightFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	n := cfg.StepCount
	t := cfg.StringerThickness.Millimeters()
	st := cfg.StepThickness.Millimeters()

	// Build-циклы параллелятся (B3, EDR-0034 §3.2): каждое тело строится
	// независимо (Extrude чистый), порядок в Compound фиксирован индексами
	// слотов (косоуры → проступи → подступенки) — детерминизм ADR-0003.
	builds := make([]func() (*kerngeo.Solid, error), 0, 4*n+2)

	// косоуры: два внутри ширины, симметрично относительно центра марша —
	// полосы [w/4−t/2, w/4+t/2] и [3w/4−t/2, 3w/4+t/2]. Ступени лежат на
	// сёдлах пилы сверху (седло = низ проступи).
	yA, yC := w/4-t/2, 3*w/4-t/2
	for i, y := range []float64{yA, yC} {
		i, y := i, y
		builds = append(builds, func() (*kerngeo.Solid, error) {
			profile := stringerProfile(n, b, h, st, y, t)
			solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 1, 0), t)
			if err != nil {
				return nil, fmt.Errorf("geometry: stringer %d: %w", i, err)
			}
			return solid.WithRole("stringer"), nil
		})
	}

	if st <= kerngeo.Precision {
		return buildCompound(builds)
	}

	// проступи: горизонтальные боксы во всю ширину [0,w], толщина по Z.
	// Верх проступи на уровне носика (k+1)·h, толщина st вниз (EDR-0004):
	// низ проступи ложится на седло пилы косоура. Проступь глубже шага на st
	// (охват [k·b−st, (k+1)·b]): её задняя кромка нависает над подступенком,
	// который выдвинут на столько же (см. ниже).
	for k := 0; k < n; k++ {
		k := k
		builds = append(builds, func() (*kerngeo.Solid, error) {
			x0, x1 := float64(k)*b-st, float64(k+1)*b
			z := float64(k+1)*h - st
			profile := []kerngeo.Point3{
				kerngeo.NewPoint3(x0, 0, z),
				kerngeo.NewPoint3(x1, 0, z),
				kerngeo.NewPoint3(x1, w, z),
				kerngeo.NewPoint3(x0, w, z),
			}
			solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 0, 1), st)
			if err != nil {
				return nil, fmt.Errorf("geometry: tread %d: %w", k, err)
			}
			return solid.WithRole("tread"), nil
		})
	}

	// подступенки (только при cfg.Riser, ENG-GEO-0007): вертикальное
	// полотно во всю ширину [0,w], толщиной по X. Классический подступенок:
	// верхний край упирается в нижнюю плоскость вышележащей проступи
	// ((k+1)·h − st), нижний — на нижележащую ступень (k·h), спина — в
	// вертикальный сброс косоура. Проступи глубже шага на st, поэтому
	// подступенок k выдвинут навстречу подъёму: охват [k·b−st, k·b] кладёт
	// своё дно целиком на проступь k−1 (она доходит до k·b), верх — под
	// проступь k, а спина (x=k·b) — на сброс гребёнки. Косоуры в этом
	// окне лежат ниже низа подступенка (седло на z=k·h−st), поэтому полотно
	// единое, без пазов: одно тело на ступень. Внизу марша (k=0) подступенок
	// стоит на полу перед передней гранью косоура — лицевая панель марша
	// [−st, 0] вместе со свесом первой проступи, спиной (x=0) вплотную к
	// передней грани. При выключенном флаге ступени открытые.
	if !cfg.Riser {
		return buildCompound(builds)
	}
	for k := 0; k < n; k++ {
		k := k
		x0 := float64(k)*b - st
		z0, z1 := float64(k)*h, float64(k+1)*h-st
		builds = append(builds, func() (*kerngeo.Solid, error) {
			profile := []kerngeo.Point3{
				kerngeo.NewPoint3(x0, 0, z0),
				kerngeo.NewPoint3(x0, 0, z1),
				kerngeo.NewPoint3(x0, w, z1),
				kerngeo.NewPoint3(x0, w, z0),
			}
			solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(1, 0, 0), st)
			if err != nil {
				return nil, fmt.Errorf("geometry: riser %d: %w", k, err)
			}
			return solid.WithRole("riser"), nil
		})
	}

	return buildCompound(builds)
}

// buildCompound выполняет все build-замыкания параллельно через Scheduler
// (B3, EDR-0034) и собирает Compound в порядке слотов. Ошибка — первый по
// индексу сбой либо отмена контекста (внутренний Background: отмена не
// требуется на стадии построения).
func buildCompound(builds []func() (*kerngeo.Solid, error)) (*kerngeo.Compound, error) {
	solids := make([]*kerngeo.Solid, len(builds))
	err := scheduler.New(0).Execute(context.Background(), len(builds), func(i int) error {
		s, e := builds[i]()
		if e != nil {
			return e
		}
		solids[i] = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return kerngeo.NewCompound(solids...), nil
}

// BuildLShapeFlight строит параметрическую B-Rep модель L-образной
// лестницы (EDR-0005, ENG-GEO-0007): нижний прямой марш (n1 ступеней),
// горизонтальная площадка на высоте H1 и верхний прямой марш (n2
// ступеней), повёрнутый на 90° по горизонтали. Координаты: X — направление
// подъёма нижнего марша, Y — его ширина, Z — высота (ADR-0008); верхний
// марш идёт вдоль +Y. Геометрия всегда вычисляется заново из параметров
// (BC-002). Роли тел проставляются для корректной декомпозиции.
func BuildLShapeFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	if cfg.Flight != engineering.FlightLShape {
		return nil, fmt.Errorf("geometry: configuration flight must be l_shape")
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n1 := cfg.LowerStepCount
	n2 := cfg.StepCount - n1
	wp := cfg.LandingWidth.Millimeters()
	// Глубина площадки (вдоль нижнего марша, X в плане). При 0 — квадратная
	// (равна ширине марша W).
	ld := cfg.LandingDepth.Millimeters()
	if ld <= 0 {
		ld = w
	}
	// EDR-0005 §4.5: H1 = n1·h — уровень площадки.
	h1 := float64(n1) * h

	// нижний марш в локальных координатах (без поворота).
	lowerModel, err := BuildStraightFlight(subFlight(cfg, h1, n1))
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}

	// верхний марш: строится как прямой в локальных координатах, затем
	// поворот на 90° вокруг Z (направление подъёма → вдоль +Y) и перенос
	// так, чтобы марш начинался с края площадки на высоте H1.
	upperModel, err := BuildStraightFlight(subFlight(cfg, float64(n2)*h, n2))
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}
	// Площадка и ориентация маршей зависят от направления поворота
	// (CONF-DIRECTION). Правосторонний поворот (по умолчанию): площадка в
	// плане [L1, L1+W]×[0, Wp], верхний марш — поворотом на +90° вокруг Z
	// (подъём → +Y), занимает [L1, L1+W]×[Wp, Wp+L2] на высоте H1.
	// Левосторонний (TurnLeft) — зеркальная в плане компоновка: площадка
	// [0, W]×[0, Wp] слева, нижний марш повёрнут на 180° и поднимается по
	// −X (его верх стыкуется с правой гранью площадки x = W), верхний марш
	// повёрнут на +90° и поднимается вдоль +Y от края площадки.
	l1 := float64(n1) * b
	left := cfg.Direction == engineering.TurnLeft
	landingX0 := l1
	lowerTransform := kerngeo.Identity()
	upperTransform := kerngeo.Translate(l1+w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))
	if left {
		landingX0 = w - ld
		lowerTransform = kerngeo.Translate(w+l1, w, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperTransform = kerngeo.Translate(w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))
	}

	// площадка: горизонтальная плита толщиной st на высоте H1, план
	// [landingX0, landingX0+Ld]×[0, Wp] (EDR-0005 §4.8), роль "landing".
	landing, err := buildLanding(ld, wp, landingX0, h1, st)
	if err != nil {
		return nil, err
	}

	// нижний марш: для правого поворота — без поворота; для левого —
	// повёрнут на 180° (левосторонняя компоновка). Порядок тел сохранён
	// (нижний марш → площадка → верхний марш) для детерминизма.
	solids := make([]*kerngeo.Solid, 0, len(lowerModel.Solids())+1+len(upperModel.Solids()))
	for _, s := range lowerModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, lowerTransform))
	}
	solids = append(solids, landing)
	for _, s := range upperModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, upperTransform))
	}
	return kerngeo.NewCompound(solids...), nil
}

// subFlight создаёт конфигурацию прямого марша-секции (нижний/верхний
// марш многомаршевой лестницы): высота и число ступеней переопределяются,
// остальные параметры наследуются от родительской конфигурации.
func subFlight(cfg *engineering.StairConfiguration, height float64, steps int) *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             cfg.Width,
		Height:            engineering.Length(height),
		Length:            cfg.Length,
		Angle:             cfg.Angle,
		Flight:            engineering.FlightStraight,
		StepCount:         steps,
		StepHeight:        cfg.StepHeight,
		StepWidth:         cfg.StepWidth,
		TreadDepth:        cfg.TreadDepth,
		Clearance:         cfg.Clearance,
		RailingHeight:     cfg.RailingHeight,
		StringerLength:    engineering.Length(float64(steps) * cfg.TreadDepth.Millimeters()),
		StringerThickness: cfg.StringerThickness,
		StepThickness:     cfg.StepThickness,
		Riser:             cfg.Riser,
	}
}

// subFlightWidth — то же, что subFlight, но с явно заданной шириной марша
// (переопределяет cfg.Width). Используется П-образной лестницей в
// платформенном режиме, чтобы сузить марши на flightSideInsetMM и получить
// внутренний зазор, не затрагивая веерный режим (subFlight).
func subFlightWidth(cfg *engineering.StairConfiguration, height float64, steps int, width engineering.Length) *engineering.StairConfiguration {
	c := subFlight(cfg, height, steps)
	c.Width = width
	return c
}

// BuildUShapeFlight строит параметрическую B-Rep модель П-образной
// лестницы (EDR-0006, ENG-GEO-0007). Поворот между маршами выбирается
// полем cfg.TurnKind: площадка (TurnPlatform, по умолчанию) либо поворотные
// ступени (TurnWinder). Диспетчеризация по TurnKind сохраняет обратную
// совместимость: пустой/неизвестный TurnKind трактуется как площадка.
func BuildUShapeFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	if cfg.Flight != engineering.FlightUShape {
		return nil, fmt.Errorf("geometry: configuration flight must be u_shape")
	}
	if cfg.TurnKind == engineering.TurnWinder {
		return BuildUShapeWinderFlight(cfg)
	}
	return buildUShapePlatform(cfg)
}

// uShapePlatformTransforms возвращает трансформы нижнего и верхнего маршей и
// габариты площадки П-образной лестницы с площадкой (платформенный режим).
// Верхний марш развёрнут на 180° относительно нижнего (подъём в
// противоположную сторону); в левом варианте (TurnLeft) раскладка зеркальна
// в плане. Те же самые трансформы используют перила (buildLURNailing), поэтому
// рассинхрон между ступенями и перилами исключён по построению.
func uShapePlatformTransforms(cfg *engineering.StairConfiguration) (lowerT, upperT kerngeo.Transform, landingX0, landingY float64) {
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	n1 := cfg.LowerStepCount
	h1 := float64(n1) * h
	l1 := float64(n1) * b
	landingY = 2 * w
	// Зазор 100 мм между маршами: каждый марш сужается на flightSideInsetMM
	// от внешней кромки (wEff = W − flightSideInsetMM); площадка остаётся
	// 2·W, зазор получается внутренним.
	wEff := w - flightSideInsetMM
	left := cfg.Direction == engineering.TurnLeft
	landingX0 = l1
	lowerT = kerngeo.Identity()
	// Верхний марш — подъём в противоположную сторону (180°): правый вариант
	// развёрнут на RotateZ(π) и само вращение сужает марш до [W+50, 2W];
	// левый — зеркален и поднимается по +X из левого края площадки, занимая
	// Y∈[W+50, 2W]. Нижний (левый) — Y∈[0, wEff]. Оба варианта примыкают к
	// площадке той же кромкой, что и раньше (узел поворота — в одном углу
	// площадки, а не классический switchback через всю площадку).
	upperT = kerngeo.Translate(l1, 2*w, h1).Mul(kerngeo.RotateZ(math.Pi))
	if left {
		landingX0 = 0
		lowerT = kerngeo.Translate(w+l1, wEff, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperT = kerngeo.Translate(w, w+flightSideInsetMM, h1)
	}
	return
}

// buildUShapePlatform строит П-образную лестницу с площадкой (EDR-0006 §4):
// нижний прямой марш (n1), горизонтальная площадка на высоте H1 и верхний
// прямой марш (n2), параллельный нижнему и развёрнутый на 180°. Площадка
// соединяет оба марша на одном Z-уровне H1 и охватывает полную ширину
// 2·W (внешние кромки обоих маршей), будучи по ширине равной двум пролётам.
func buildUShapePlatform(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	w := cfg.Width.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n1 := cfg.LowerStepCount
	n2 := cfg.StepCount - n1
	// Трансформы маршей и габариты площадки вычисляются одной функцией и
	// переиспользуются перилами (uShapePlatformTransforms), чтобы перила
	// гарантированно совпадали со ступенями (EDR-0006 §4.8).
	lowerTransform, upperTransform, landingX0, landingY := uShapePlatformTransforms(cfg)
	// EDR-0006 §4.5: H1 = n1·h — уровень площадки.
	h1 := float64(n1) * h
	// Марши сужаются на flightSideInsetMM (внутренний зазор 100 мм), площадка
	// остаётся 2·W (см. uShapePlatformTransforms).
	wEff := w - flightSideInsetMM

	lowerModel, err := BuildStraightFlight(subFlightWidth(cfg, h1, n1, engineering.Length(wEff)))
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}
	upperModel, err := BuildStraightFlight(subFlightWidth(cfg, float64(n2)*h, n2, engineering.Length(wEff)))
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}

	// площадка: плита толщиной st на высоте H1, план
	// [landingX0, landingX0+W]×[0, 2W], роль "landing".
	landing, err := buildLanding(w, landingY, landingX0, h1, st)
	if err != nil {
		return nil, err
	}

	solids := make([]*kerngeo.Solid, 0, len(lowerModel.Solids())+1+len(upperModel.Solids()))
	for _, s := range lowerModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, lowerTransform))
	}
	solids = append(solids, landing)
	for _, s := range upperModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, upperTransform))
	}
	return kerngeo.NewCompound(solids...), nil
}

// BuildUShapeWinderFlight строит П-образную лестницу с поворотными
// ступенями (EDR-0006 §4, поворот на 180°): нижний прямой марш (n1),
// набор из nw поворотных ступеней (веер на 180° в просвете шириной Wp
// между маршами) и верхний прямой марш (n2 = n − n1 − nw). Поворотные
// ступени наследуют высоту h и проступь b прямых маршей; каждая поднимает
// на h, общий подъём марша H = n·h. Геометрия веера опирается на тот же
// примитив sectorRing, что и спиральная лестница (спираль — веер на 360°,
// поворот — на 180°). Роль тел: "winder" для поворотных ступеней.
func BuildUShapeWinderFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	if cfg.Flight != engineering.FlightUShape {
		return nil, fmt.Errorf("geometry: configuration flight must be u_shape")
	}
	if cfg.TurnKind != engineering.TurnWinder {
		return nil, fmt.Errorf("geometry: configuration turn kind must be winder")
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n1 := cfg.LowerStepCount
	nw := cfg.WinderCount
	n2 := cfg.StepCount - n1 - nw
	if n2 < 1 {
		return nil, fmt.Errorf("geometry: upper flight must have at least 1 step (n − n1 − nw = %d)", n2)
	}
	wp := cfg.LandingWidth.Millimeters() // ширина просвета между маршами
	h1 := float64(n1) * h
	l1 := float64(n1) * b
	left := cfg.Direction == engineering.TurnLeft

	lowerModel, err := BuildStraightFlight(subFlight(cfg, h1, n1))
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}
	upperModel, err := BuildStraightFlight(subFlight(cfg, float64(n2)*h, n2))
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}

	// Внутренние ребра стыка маршей с поворотом (нижний верх /
	// верхний низ), через которые поворотные ступени соединяют марши.
	var pLower, pUpper kerngeo.Point3
	lowerTransform := kerngeo.Identity()
	upperTransform := kerngeo.Translate(l1, w+wp+w, h1+float64(nw)*h).Mul(kerngeo.RotateZ(math.Pi))
	if left {
		pLower = kerngeo.NewPoint3(w, w, h1)
		pUpper = kerngeo.NewPoint3(0, wp, h1+float64(nw)*h)
		lowerTransform = kerngeo.Translate(w+l1, w, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperTransform = kerngeo.Translate(0, w+wp, h1+float64(nw)*h).Mul(kerngeo.RotateZ(math.Pi))
	} else {
		pLower = kerngeo.NewPoint3(l1, w, h1)
		pUpper = kerngeo.NewPoint3(l1, w+wp, h1+float64(nw)*h)
	}

	winders, err := buildWinders(w, h, st, n1, nw, pLower, pUpper)
	if err != nil {
		return nil, err
	}

	solids := make([]*kerngeo.Solid, 0, len(lowerModel.Solids())+len(winders)+len(upperModel.Solids()))
	for _, s := range lowerModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, lowerTransform))
	}
	solids = append(solids, winders...)
	for _, s := range upperModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, upperTransform))
	}
	return kerngeo.NewCompound(solids...), nil
}

// buildWinders строит nw поворотных ступеней — веер на 180° между
// внутренними ребрами стыка pLower (нижний марш) и pUpper (верхний марш).
// Веер центрируется в середине отрезка pLower–pUpper; внутренний радиус ri
// достигает внутренних рёбер обоих маршей, внешний ro = ri + W — их внешних
// кромок (EDR-0006 §4.8). Каждая ступень — сектор кольца, выдавленный по Z
// на толщину st, верх на высоте H1 + (k+1)·h (общий подъём h). Роль "winder".
func buildWinders(w, h, st float64, n1, nw int, pLower, pUpper kerngeo.Point3) ([]*kerngeo.Solid, error) {
	if nw < 3 {
		return nil, fmt.Errorf("geometry: winder count must be at least 3")
	}
	h1 := float64(n1) * h
	ox, oy := (pLower.X+pUpper.X)/2, (pLower.Y+pUpper.Y)/2
	ux, uy := pLower.X-ox, pLower.Y-oy
	ul := math.Hypot(ux, uy)
	if ul < kerngeo.Precision {
		return nil, fmt.Errorf("geometry: degenerate winder pivot")
	}
	ux, uy = ux/ul, uy/ul
	ri := ul     // расстояние от центра веера до внутренних рёбер (половина просвета)
	ro := ri + w // до внешних кромок маршей
	a0 := math.Atan2(uy, ux)
	dth := math.Pi / float64(nw)

	sols := make([]*kerngeo.Solid, 0, nw)
	for k := 0; k < nw; k++ {
		aa := a0 + float64(k)*dth
		ab := aa + dth
		zTop := h1 + float64(k+1)*h
		profile := sectorRing(ri, ro, aa, ab, zTop-st)
		solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 0, 1), st)
		if err != nil {
			return nil, fmt.Errorf("geometry: winder %d: %w", k, err)
		}
		solid = kerngeo.TransformSolid(solid, kerngeo.Translate(ox, oy, 0))
		sols = append(sols, solid.WithRole("winder"))
	}
	return sols, nil
}

// buildLanding строит твёрдое тело горизонтальной прямоугольной плиты
// площадки толщиной st с верхней гранью на уровне h1 (EDR-0005 §4.8):
// план [x0, x0+w] × [0, Wp] — ширина марша W вдоль +X, ширина площадки
// Wp вдоль +Y.
func buildLanding(w, wp, x0, h1, st float64) (*kerngeo.Solid, error) {
	p := []kerngeo.Point3{
		kerngeo.NewPoint3(x0, 0, h1-st),
		kerngeo.NewPoint3(x0+w, 0, h1-st),
		kerngeo.NewPoint3(x0+w, wp, h1-st),
		kerngeo.NewPoint3(x0, wp, h1-st),
	}
	solid, err := kerngeo.Extrude(p, kerngeo.NewVector3(0, 0, 1), st)
	if err != nil {
		return nil, fmt.Errorf("geometry: landing: %w", err)
	}
	return solid.WithRole("landing"), nil
}

func validateFlight(cfg *engineering.StairConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("geometry: configuration is required")
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("geometry: %w", err)
	}
	if cfg.StepHeight.Millimeters() <= 0 {
		return fmt.Errorf("geometry: step height must be positive")
	}
	if cfg.TreadDepth.Millimeters() <= 0 {
		return fmt.Errorf("geometry: tread depth must be positive")
	}
	if cfg.Width.Millimeters() <= 2*cfg.StringerThickness.Millimeters() {
		return fmt.Errorf("geometry: width must exceed two stringer thicknesses")
	}
	return nil
}

// stringerProfile строит строго простой профиль косоура-гребёнки в
// плоскости XZ при y = yOff: пила с посадочными местами на уровне низа
// проступи ((k+1)·h − st) и вертикальными сбросами h на границах ступеней
// (седло = низ проступи, ENG-GEO-0007). Низ доски — спинка, параллельная
// линии посадочных мест на расстоянии t (StringerThickness) по нормали;
// сзади (у верхней ступени) — перпендикулярный срез из головы пилы,
// спереди — упор в пол и вертикальная передняя грань (контур начинается
// с передней грани). Вершины: (0, 0), (0, h−st), седло/сброс, …,
// (n·b, n·h−st), P_top, F.
func stringerProfile(n int, b, h, st, yOff, t float64) []kerngeo.Point3 {
	pts := make([]kerngeo.Point3, 0, 2*n+3)
	pts = append(pts,
		kerngeo.NewPoint3(0, yOff, 0),
		kerngeo.NewPoint3(0, yOff, h-st),
	)
	for k := 0; k < n-1; k++ {
		pts = append(pts,
			kerngeo.NewPoint3(float64(k+1)*b, yOff, float64(k+1)*h-st),
			kerngeo.NewPoint3(float64(k+1)*b, yOff, float64(k+2)*h-st),
		)
	}
	pts = append(pts, kerngeo.NewPoint3(float64(n)*b, yOff, float64(n)*h-st))

	// Спинка: линия посадочных мест, сдвинутая на толщину t по нормали
	// вниз (поперёк марша в плоскости XZ). Верхний срез вертикален
	// (перпендикулярен полу, как нижний), спинка упирается в пол (z = 0).
	L := math.Hypot(b, h)
	pTop := kerngeo.NewPoint3(
		float64(n)*b, // верхний срез вертикален (x = n·b)
		yOff,
		float64(n)*h-st-t*b/L, // z = n·h − st − t·b/L
	)
	fx := pTop.X - pTop.Z/h*b
	if fx < 0 {
		fx = 0
	}
	if fx > float64(n)*b {
		fx = float64(n) * b
	}
	pts = append(pts,
		pTop,
		kerngeo.NewPoint3(fx, yOff, 0),
	)
	return pts
}
