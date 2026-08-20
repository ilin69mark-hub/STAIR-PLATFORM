package geometry

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// FullTurnSpiral — полный угол поворота спиральной лестницы (EDR-0007
// §4.3): 360° = 2π радиан.
const FullTurnSpiral = 2 * math.Pi

// arcSegments — число сегментов полигональной аппроксимации дуги каждой
// ступени и колонны (EDR-0007 §5 "Precision"): 8 сегментов на сектор.
const arcSegments = 8

// BuildSpiralFlight строит параметрическую B-Rep модель спиральной
// лестницы с центральной колонной (EDR-0007, ENG-GEO-0007). Модель
// состоит из центральной колонны (1 цилиндр) и n веерных ступеней.
// Координаты: центр колонны в начале координат, объём занимает
// |x|,|y| ∈ [0, R], z ∈ [0, H] (ADR-0008). Центральная колонна —
// цилиндр радиуса r = R − W от пола до высоты H, роль "column";
// ступень k — сектор кольца [r, R] на углу k·δ, верх на высоте
// (k+1)·h, толщина st вниз (EDR-0004), роль "tread".
// Геометрия всегда вычисляется заново из параметров (BC-002).
func BuildSpiralFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	if err := validateSpiral(cfg); err != nil {
		return nil, err
	}
	r := cfg.OuterRadius.Millimeters() - cfg.Width.Millimeters() // радиус колонны
	R := cfg.OuterRadius.Millimeters()                           // наружный радиус марша
	h := cfg.StepHeight.Millimeters()
	n := cfg.StepCount
	st := cfg.StepThickness.Millimeters()
	delta := FullTurnSpiral / float64(n)

	solids := make([]*kerngeo.Solid, 0, n+1)

	// центральная колонна: цилиндр радиуса r, от пола до высоты H.
	col, err := buildColumn(r, cfg.Height.Millimeters())
	if err != nil {
		return nil, err
	}
	solids = append(solids, col)

	// веерные ступени: сектор кольца [r, R] на углу k·δ, верх на (k+1)·h.
	// Направление закрутки (CONF-SPIRAL-DIRECTION): cw (по умолчанию) —
	// угол растёт (против часовой при взгляде сверху в проекции на XY без
	// отражения), ccw — угол убывает: план зеркалится по горизонтали
	// (как в 2D-схеме StaPlan при смене направления).
	sign := 1.0
	if cfg.SpiralDirection == engineering.SpiralCCW {
		sign = -1
	}
	for k := 0; k < n; k++ {
		a0 := sign * float64(k) * delta
		a1 := a0 + sign*delta
		// верх проступи на высоте (k+1)·h; толщина st вниз (z0 = (k+1)h − st).
		zTop := float64(k+1) * h
		profile := sectorRing(r, R, a0, a1, zTop-st)
		solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 0, 1), st)
		if err != nil {
			return nil, fmt.Errorf("geometry: spiral tread %d: %w", k, err)
		}
		solids = append(solids, solid.WithRole("tread"))
	}

	return kerngeo.NewCompound(solids...), nil
}

// buildColumn строит цилиндр радиуса r высотой height вдоль оси Z,
// аппроксимируя окружность полигоном arcSegments (EDR-0007 §5).
func buildColumn(r, height float64) (*kerngeo.Solid, error) {
	profile := circlePolygon(r)
	solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 0, 1), height)
	if err != nil {
		return nil, fmt.Errorf("geometry: spiral column: %w", err)
	}
	return solid.WithRole("column"), nil
}

// circlePolygon аппроксимирует окружность радиуса r в плоскости z=0
// регулярным полигоном из arcSegments вершин (EDR-0007 §5).
func circlePolygon(r float64) []kerngeo.Point3 {
	pts := make([]kerngeo.Point3, 0, arcSegments)
	for i := 0; i < arcSegments; i++ {
		a := 2 * math.Pi * float64(i) / float64(arcSegments)
		pts = append(pts, kerngeo.NewPoint3(r*math.Cos(a), r*math.Sin(a), 0))
	}
	return pts
}

// sectorRing строит профиль клина (сектора кольца) в плоскости z=z0:
// дуга снаружи радиуса R от a0 до a1 (CCW), затем радиальный срез внутрь,
// дуга внутри радиуса r обратно от a1 до a0 (EDR-0007 §4.8). Каждая дуга
// аппроксимируется arcSegments сегментами (EDR-0007 §5). Наружная дуга
// идёт первой, чтобы ориентирующая нормаль полигона совпадала со всей
// областью (корректная экструзия и объём).
func sectorRing(r, R, a0, a1, z0 float64) []kerngeo.Point3 {
	pts := make([]kerngeo.Point3, 0, 2*arcSegments+2)
	for i := 0; i <= arcSegments; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(arcSegments)
		pts = append(pts, kerngeo.NewPoint3(R*math.Cos(a), R*math.Sin(a), z0))
	}
	for i := 0; i <= arcSegments; i++ {
		a := a1 - (a1-a0)*float64(i)/float64(arcSegments)
		pts = append(pts, kerngeo.NewPoint3(r*math.Cos(a), r*math.Sin(a), z0))
	}
	return pts
}

// validateSpiral проверяет конфигурацию спиральной лестницы (EDR-0007).
func validateSpiral(cfg *engineering.StairConfiguration) error {
	if err := validateFlight(cfg); err != nil {
		return err
	}
	if cfg.Flight != engineering.FlightSpiral {
		return fmt.Errorf("geometry: configuration flight must be spiral")
	}
	if cfg.OuterRadius.Millimeters() <= cfg.Width.Millimeters() {
		return fmt.Errorf("geometry: outer radius must exceed stair width for spiral")
	}
	return nil
}
