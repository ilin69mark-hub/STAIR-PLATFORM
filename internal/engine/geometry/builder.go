// Package geometry implements the Geometry Engine (ENG-0002, ENG-GEO-0007):
// it builds parametric B-Rep models of stair flights from a StairConfiguration.
// The engine knows only the domain model and the domain-agnostic geometry
// kernel; parameters are the single source of truth of geometry (BC-002).
package geometry

import (
	"fmt"

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
		solids = append(solids, solid)
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
		solids = append(solids, solid)
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
		solids = append(solids, solid)
	}

	return kerngeo.NewCompound(solids...), nil
}

// validateFlight проверяет входные параметры перед построением модели.
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
