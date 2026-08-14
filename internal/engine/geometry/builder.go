// Package geometry implements the Geometry Engine (ENG-0002, ENG-GEO-0007):
// it builds parametric B-Rep models of stair flights from a StairConfiguration.
// The engine knows only the domain model and the domain-agnostic geometry
// kernel; parameters are the single source of truth of geometry (BC-002).
package geometry

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// stringerHeel — материал под нижней гранью выреза косоура (мм).
// STAIR-DOC не задаёт значение; heel делает профиль косоура строго
// простым полигоном (нижняя кромка не проходит через впадины пилы),
// что гарантирует корректную триангуляцию.
const stringerHeel = 50.0

// BuildStraightFlight строит параметрическую B-Rep модель прямого марша
// (ENG-GEO-0007). Модель состоит из 2 косоуров и n проступей + n
// подступенков (всего 2n+2 твёрдых тел) при StepThickness > 0; при
// StepThickness == 0 ступени не строятся (EDR-0004).
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

	solids := make([]*kerngeo.Solid, 0, 2*n+2)

	// косоуры: левый на y∈[0,t], правый на y∈[w-t,w].
	for _, y := range []float64{0, w - t} {
		profile := stringerProfile(n, b, h, y)
		solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 1, 0), t)
		if err != nil {
			return nil, fmt.Errorf("geometry: stringer: %w", err)
		}
		solids = append(solids, solid.WithRole("stringer"))
	}

	if st <= kerngeo.Precision {
		return kerngeo.NewCompound(solids...), nil
	}

	// проступи: горизонтальные боксы между косоурами, толщина по Z.
	for k := 0; k < n; k++ {
		x0, x1 := float64(k)*b, float64(k+1)*b
		z := float64(k+1) * h
		profile := []kerngeo.Point3{
			kerngeo.NewPoint3(x0, t, z),
			kerngeo.NewPoint3(x1, t, z),
			kerngeo.NewPoint3(x1, w-t, z),
			kerngeo.NewPoint3(x0, w-t, z),
		}
		solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 0, 1), st)
		if err != nil {
			return nil, fmt.Errorf("geometry: tread %d: %w", k, err)
		}
		solids = append(solids, solid.WithRole("tread"))
	}

	// подступенки: вертикальные боксы между косоурами, толщина по X.
	for k := 0; k < n; k++ {
		x := float64(k) * b
		z0, z1 := float64(k)*h, float64(k+1)*h
		profile := []kerngeo.Point3{
			kerngeo.NewPoint3(x, t, z0),
			kerngeo.NewPoint3(x, t, z1),
			kerngeo.NewPoint3(x, w-t, z1),
			kerngeo.NewPoint3(x, w-t, z0),
		}
		solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(1, 0, 0), st)
		if err != nil {
			return nil, fmt.Errorf("geometry: riser %d: %w", k, err)
		}
		solids = append(solids, solid.WithRole("riser"))
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
	lower := &engineering.StairConfiguration{
		Width:             cfg.Width,
		Height:            engineering.Length(h1),
		Length:            cfg.Length,
		Angle:             cfg.Angle,
		Flight:            engineering.FlightStraight,
		StepCount:         n1,
		StepHeight:        cfg.StepHeight,
		StepWidth:         cfg.StepWidth,
		TreadDepth:        cfg.TreadDepth,
		Clearance:         cfg.Clearance,
		RailingHeight:     cfg.RailingHeight,
		StringerLength:    engineering.Length(float64(n1) * cfg.TreadDepth.Millimeters()),
		StringerThickness: cfg.StringerThickness,
		StepThickness:     cfg.StepThickness,
	}
	lowerModel, err := BuildStraightFlight(lower)
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}

	// верхний марш: строится как прямой в локальных координатах, затем
	// поворот на 90° вокруг Z (направление подъёма → вдоль +Y) и перенос
	// так, чтобы марш начинался с края площадки на высоте H1.
	upper := &engineering.StairConfiguration{
		Width:             cfg.Width,
		Height:            engineering.Length(float64(n2) * h),
		Length:            cfg.Length,
		Angle:             cfg.Angle,
		Flight:            engineering.FlightStraight,
		StepCount:         n2,
		StepHeight:        cfg.StepHeight,
		StepWidth:         cfg.StepWidth,
		TreadDepth:        cfg.TreadDepth,
		Clearance:         cfg.Clearance,
		RailingHeight:     cfg.RailingHeight,
		StringerLength:    engineering.Length(float64(n2) * cfg.TreadDepth.Millimeters()),
		StringerThickness: cfg.StringerThickness,
		StepThickness:     cfg.StepThickness,
	}
	upperModel, err := BuildStraightFlight(upper)
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}
	// Площадка: план [L1, L1+W]×[0, Wp], верх на уровне H1 (EDR-0005 §4.8).
	// Верхний марш (в локальных координатах: подъём вдоль +X, ширина вдоль
	// +Y) поворачивается на +90° вокруг Z: подъём → вдоль +Y, ширина →
	// вдоль -X, затем переносится так, чтобы марш занимал
	// [L1, L1+W]×[Wp, Wp+L2] на высоте H1.
	l1 := float64(n1) * b
	upperTransform := kerngeo.Translate(l1+w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))

	// площадка: горизонтальная плита толщиной st на высоте H1, план
	// [L1, L1+W]×[0, Wp] (EDR-0005 §4.8), роль "landing".
	landing, err := buildLanding(w, wp, l1, h1, st)
	if err != nil {
		return nil, err
	}

	solids := append([]*kerngeo.Solid{}, lowerModel.Solids()...)
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

// stringerProfile строит строго простой профиль косоура в плоскости XZ
// при y = yOff: пилообразный верх (впадины на уровне (k·b, k·h), выступы
// на уровне (k·b, (k+1)·h)) и прямая нижняя кромка, отстоящая от впадин
// на stringerHeel. Вершины: (0,0), выступ, впадина, ..., (n·b, n·h),
// (n·b, n·h−heel), (0, −heel).
func stringerProfile(n int, b, h, yOff float64) []kerngeo.Point3 {
	pts := make([]kerngeo.Point3, 0, 2*n+3)
	pts = append(pts, kerngeo.NewPoint3(0, yOff, 0))
	for k := 0; k < n; k++ {
		pts = append(pts,
			kerngeo.NewPoint3(float64(k)*b, yOff, float64(k+1)*h),
			kerngeo.NewPoint3(float64(k+1)*b, yOff, float64(k+1)*h),
		)
	}
	L := float64(n) * b
	pts = append(pts,
		kerngeo.NewPoint3(L, yOff, float64(n)*h-stringerHeel),
		kerngeo.NewPoint3(0, yOff, -stringerHeel),
	)
	return pts
}
