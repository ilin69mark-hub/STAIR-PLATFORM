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
		landingX0 = 0
		lowerTransform = kerngeo.Translate(w+l1, w, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperTransform = kerngeo.Translate(w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))
	}

	// площадка: горизонтальная плита толщиной st на высоте H1, план
	// [landingX0, landingX0+W]×[0, Wp] (EDR-0005 §4.8), роль "landing".
	landing, err := buildLanding(w, wp, landingX0, h1, st)
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

// BuildUShapeFlight строит параметрическую B-Rep модель П-образной
// лестницы (EDR-0006, ENG-GEO-0007): нижний прямой марш (n1 ступеней),
// горизонтальная площадка на высоте H1 и верхний прямой марш (n2
// ступеней), параллельный нижнему и развёрнутый на 180° по горизонтали.
// Координаты: X — направление подъёма нижнего марша, Y — его ширина,
// Z — высота (ADR-0008); верхний марш возвращается вдоль −X. Геометрия
// всегда вычисляется заново из параметров (BC-002). Роли тел
// проставляются для корректной декомпозиции.
func BuildUShapeFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	if cfg.Flight != engineering.FlightUShape {
		return nil, fmt.Errorf("geometry: configuration flight must be u_shape")
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n1 := cfg.LowerStepCount
	n2 := cfg.StepCount - n1
	wp := cfg.LandingWidth.Millimeters()
	// EDR-0006 §4.5: H1 = n1·h — уровень площадки.
	h1 := float64(n1) * h

	// нижний марш в локальных координатах (без поворота).
	lowerModel, err := BuildStraightFlight(subFlight(cfg, h1, n1))
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}

	// верхний марш: строится как прямой в локальных координатах, затем
	// поворот на 180° вокруг Z (направление подъёма → вдоль −X) и перенос
	// так, чтобы марш начинался с края площадки на высоте H1 и шёл вдоль
	// площадки параллельно нижнему маршу.
	upperModel, err := BuildStraightFlight(subFlight(cfg, float64(n2)*h, n2))
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}
	// Площадка и ориентация маршей зависят от направления П-оборота
	// (CONF-DIRECTION). Правосторонний (по умолчанию): площадка
	// [L1, L1+W]×[0, Wp], верхний марш — поворотом на 180° вокруг Z
	// (RotateZ(π), подъём → −X), занимает [L1, L1+W]×[Wp, Wp+W] на высоте
	// H1 и возвращается параллельно нижнему маршу (EDR-0006 §4.8).
	// Левосторонний (TurnLeft) — зеркальная в плане компоновка: площадка
	// [0, W]×[0, Wp] слева, нижний марш повёрнут на 180° и поднимается по
	// −X (верх на правой грани x = W), верхний марш без поворота
	// поднимается вдоль +X от левого края площадки.
	l1 := float64(n1) * b
	left := cfg.Direction == engineering.TurnLeft
	landingX0 := l1
	lowerTransform := kerngeo.Identity()
	upperTransform := kerngeo.Translate(l1+w, wp+w, h1).Mul(kerngeo.RotateZ(math.Pi))
	if left {
		landingX0 = 0
		lowerTransform = kerngeo.Translate(w+l1, w, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperTransform = kerngeo.Translate(0, wp, h1)
	}

	// площадка: горизонтальная плита толщиной st на высоте H1, план
	// [landingX0, landingX0+W]×[0, Wp] (EDR-0006 §4.8), роль "landing".
	landing, err := buildLanding(w, wp, landingX0, h1, st)
	if err != nil {
		return nil, err
	}

	// нижний марш: для правого поворота — без поворота; для левого —
	// повёрнут на 180° (левосторонняя компоновка). Порядок тел сохранён.
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
	// вниз (поперёк марша в плоскости XZ). Сзади срез перпендикулярен
	// маршу из головы пилы, спереди спинка упирается в пол (z = 0).
	L := math.Hypot(b, h)
	pTop := kerngeo.NewPoint3(
		float64(n)*b+t*h/L, // (n·b, n·h−st) + t·(h, −b)/L
		yOff,
		float64(n)*h-st-t*b/L,
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
