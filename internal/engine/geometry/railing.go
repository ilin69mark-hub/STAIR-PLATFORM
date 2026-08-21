// Декоративные тела перил для 3D preview mesh (CONF-RAILING).
// Перила не входят в несущую модель и никогда не попадают в производственную
// декомпозицию (BOM), измерения и расчётное ядро: они строятся отдельно и
// вливаются только в mesh (производная величина, ENG-GEO-0008).
// Конвенция сторон совпадает с 2D-планом (StairPlan): для прямого марша
// 'left' — кромка y=0, 'right' — кромка y=Width при движении по +X.
// Для маршей с площадкой (L/U) та же конвенция применяется в локальных
// координатах каждого сегмента; левая/правая сторона отсчитывается по ходу
// подъёма (CONF-RAILING, EDR-0005 §4.10). Направление поворота (CONF-DIRECTION)
// учитывается поворотом сегмента целиком и перестановкой сторон.
package geometry

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// Роли декоративных тел перил (не участвуют в производственной декомпозиции).
const (
	roleRailing  = "railing"  // поручень
	roleBaluster = "baluster" // стойка
)

// Типовые габариты деталей перил (мм; визуализация, не производственный пак).
const (
	railWidth     = 50.0 // поручень: размер поперёк марша
	railThickness = 40.0 // поручень: размер поперёк наклона
	balusterSize  = 20.0 // стойка: сечение в плане
)

// railingSides возвращает координаты кромки марша для выбранной стороны:
// left → {0}, right → {W}, both → {0, W}, none → пусто.
func railingSides(side engineering.RailingSide, w float64) []float64 {
	switch side {
	case engineering.RailingLeft:
		return []float64{0}
	case engineering.RailingRight:
		return []float64{w}
	case engineering.RailingBoth:
		return []float64{0, w}
	default:
		return nil
	}
}

// railingEnabled — строим ли перила на сегменте вообще.
func railingEnabled(rh float64, side engineering.RailingSide) bool {
	return rh > 0 && side.Valid() && side != engineering.RailingNone
}

// flipSide меняет левую/правую сторону местами: применяется, когда сегмент
// повёрнут на нечётное число четверть-оборотов (90°/180°/270°) и локальная
// сторона, видимая пользователю по ходу подъёма, обратна кромке в локальных
// координатах (CONF-DIRECTION).
func flipSide(s engineering.RailingSide) engineering.RailingSide {
	switch s {
	case engineering.RailingLeft:
		return engineering.RailingRight
	case engineering.RailingRight:
		return engineering.RailingLeft
	default:
		return s
	}
}

// swapSidesForTurn возвращает сторону сегмента с учётом его поворота rotZ
// вокруг Z (конвенция: стороны отсчитываются по ходу подъёма в мире).
func swapSidesForTurn(side engineering.RailingSide, rotZ float64) engineering.RailingSide {
	// Поворот на θ в плане: локальная кромка «right» оказывается в мире на
	// стороне, противоположной правой руке поднимающегося, при любом нечётном
	// числе четверть-оборотов (θ ≠ 0 по модулю 2π). Тождественный поворот
	// (θ = 0) стороны не меняет.
	if cos := math.Cos(rotZ); cos >= 1-kerngeo.Precision {
		return side
	}
	return flipSide(side)
}

// railAlong строит прямолинейный брус поручня, занимающий ровно отрезок
// между a и b (высота поручня уже включена в координаты). Extrude
// выдавливает призму от плоскости профиля вдоль u на L, поэтому профиль
// строится вокруг середины отрезка и смещается к началу a; иначе призма
// заняла бы [середина, середина+L·u] и поручень «уезжал» на половину марша.
// Возвращает nil, если сегмент вырожден.
func railAlong(a, b kerngeo.Point3) *kerngeo.Solid {
	axis := b.Sub(a)
	L := axis.Norm()
	if L <= kerngeo.Precision {
		return nil
	}
	u, ok := axis.Normalized()
	if !ok {
		return nil
	}
	// направление ширины: перпендикуляр к оси в плане (горизонтально).
	wd := kerngeo.NewVector3(-u.Y, u.X, 0)
	if wdN, ok := wd.Normalized(); ok {
		wd = wdN
	} else {
		wd = kerngeo.NewVector3(1, 0, 0)
	}
	// направление толщины: перпендикуляр к оси и к ширине (в плоскости наклона).
	td, ok := u.Cross(wd).Normalized()
	if !ok {
		return nil
	}
	c := a.Add(axis.Scale(0.5))
	rw := railWidth / 2
	rt := railThickness / 2
	base := []kerngeo.Point3{
		c.Add(wd.Scale(rw)).Add(td.Scale(rt)),
		c.Add(wd.Scale(rw)).Add(td.Scale(-rt)),
		c.Add(wd.Scale(-rw)).Add(td.Scale(-rt)),
		c.Add(wd.Scale(-rw)).Add(td.Scale(rt)),
	}
	shift := u.Scale(-L / 2)
	profile := make([]kerngeo.Point3, len(base))
	for i := range base {
		profile[i] = base[i].Add(shift)
	}
	s, err := kerngeo.Extrude(profile, u, L)
	if err != nil {
		return nil
	}
	return s.WithRole(roleRailing)
}

// balusterAt строит вертикальную стойку в точке (x, y) от высоты z0 на
// height вверх. Возвращает nil при вырожденной высоте или ошибке экструзии.
func balusterAt(x, y, z0, height float64) *kerngeo.Solid {
	if height <= kerngeo.Precision {
		return nil
	}
	hw := balusterSize / 2
	profile := []kerngeo.Point3{
		kerngeo.NewPoint3(x-hw, y-hw, z0),
		kerngeo.NewPoint3(x+hw, y-hw, z0),
		kerngeo.NewPoint3(x+hw, y+hw, z0),
		kerngeo.NewPoint3(x-hw, y+hw, z0),
	}
	s, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 0, 1), height)
	if err != nil {
		return nil
	}
	return s.WithRole(roleBaluster)
}

// straightRailingSolids строит декоративные перила прямого сегмента в
// локальных координатах (x — подъём от 0 до n·b, y — ширина [0, w], z —
// высота до n·h). Поручень — наклонный брус по линии носиков: от
// (0, edge, rh) до (n·b, edge, n·h+rh); стойки — вертикальные бруски в
// каждом носике (x = k·b, z ∈ [k·h, k·h+rh]).
func straightRailingSolids(n int, b, h, rh, w float64, side engineering.RailingSide) []*kerngeo.Solid {
	sides := railingSides(side, w)
	if len(sides) == 0 || rh <= 0 || n <= 0 {
		return nil
	}
	run := float64(n) * b
	H := float64(n) * h
	var sols []*kerngeo.Solid
	for _, edge := range sides {
		A := kerngeo.NewPoint3(0, edge, rh)
		B := kerngeo.NewPoint3(run, edge, H+rh)
		if s := railAlong(A, B); s != nil {
			sols = append(sols, s)
		}
		for k := 1; k <= n; k++ {
			x := float64(k) * b
			if s := balusterAt(x, edge, float64(k)*h, rh); s != nil {
				sols = append(sols, s)
			}
		}
	}
	return sols
}

// landingRailingSolids строит горизонтальный поручень-«П» вдоль открытых
// кромок площадки (CONF-RAILING сегмент «площадка»). Открытая сторона — та,
// где марши примыкают к площадке (X=x0 для правого поворота, X=w для
// левого). Правосторонний поворот: от (x0,0) по нижней кромке к (x0+w,0), по
// правой кромке к (x0+w,wp) и по дальней короткой кромке к (x0,wp); левый —
// зеркально. Поручень на высоте h1+rh. Стойки на площадке не строим
// (декоративный упрощённый контур; уточнить при визуальной проверке).
func landingRailingSolids(w, wp, rh, h1 float64, x0 float64, left bool) []*kerngeo.Solid {
	if rh <= 0 {
		return nil
	}
	var corners []kerngeo.Point3
	if left {
		corners = []kerngeo.Point3{
			kerngeo.NewPoint3(w, 0, h1+rh),
			kerngeo.NewPoint3(0, 0, h1+rh),
			kerngeo.NewPoint3(0, wp, h1+rh),
			kerngeo.NewPoint3(w, wp, h1+rh),
		}
	} else {
		corners = []kerngeo.Point3{
			kerngeo.NewPoint3(x0, 0, h1+rh),
			kerngeo.NewPoint3(x0+w, 0, h1+rh),
			kerngeo.NewPoint3(x0+w, wp, h1+rh),
			kerngeo.NewPoint3(x0, wp, h1+rh),
		}
	}
	var sols []*kerngeo.Solid
	for i := 0; i+1 < len(corners); i++ {
		if s := railAlong(corners[i], corners[i+1]); s != nil {
			sols = append(sols, s)
		}
	}
	return sols
}

// winderRailingSolids строит декоративный поручень поворотных ступеней
// П-образной лестницы (CONF-RAILING сегмент «поворот»): изогнутый поручень
// вдоль внешней дуги веера поворотных ступеней (радиус ro) от уровня H1 до
// верха поворота (H1 + nw·h) на высоте rh. Поручень — ломаная из прямых
// сегментов по дуге; стойки не строим (декоративное упрощение, как для
// площадки). Геометрия веера совпадает с buildWinders.
func winderRailingSolids(w, h, rh float64, n1, nw int, l1, wp float64, left bool) []*kerngeo.Solid {
	if rh <= 0 || nw < 3 {
		return nil
	}
	h1 := float64(n1) * h
	// Внутренние ребра стыка маршей (как в BuildUShapeWinderFlight).
	var pLower, pUpper kerngeo.Point3
	if left {
		pLower = kerngeo.NewPoint3(w, w, h1)
		pUpper = kerngeo.NewPoint3(0, wp, h1+float64(nw)*h)
	} else {
		pLower = kerngeo.NewPoint3(l1, w, h1)
		pUpper = kerngeo.NewPoint3(l1, w+wp, h1+float64(nw)*h)
	}
	// Корректная геометрия веера: центр O — середина pLower–pUpper.
	ox, oy := (pLower.X+pUpper.X)/2, (pLower.Y+pUpper.Y)/2
	ux, uy := pLower.X-ox, pLower.Y-oy
	ul := math.Hypot(ux, uy)
	if ul < kerngeo.Precision {
		return nil
	}
	ux, uy = ux/ul, uy/ul
	ri := ul
	ro := ri + w
	a0 := math.Atan2(uy, ux)

	zRail := h1 + float64(nw)*h + rh
	segs := nw + 1
	var pts []kerngeo.Point3
	for i := 0; i <= segs; i++ {
		a := a0 + math.Pi*float64(i)/float64(segs)
		pts = append(pts, kerngeo.NewPoint3(ox+ro*math.Cos(a), oy+ro*math.Sin(a), zRail))
	}
	var sols []*kerngeo.Solid
	for i := 0; i+1 < len(pts); i++ {
		if s := railAlong(pts[i], pts[i+1]); s != nil {
			sols = append(sols, s)
		}
	}
	return sols
}

// buildSpiralRailing строит перила спирали по наружной кромке марша (радиус
// R): поручень — ломаная из прямых сегментов над носиками, стойки — в
// середине каждой ступени. Сторона соответствует открытой кромке
// (CONF-SPIRAL-RAILING): для cw — right, для ccw — left; раньше невыполнения
// условия перила не строятся (стена).
func buildSpiralRailing(cfg *engineering.StairConfiguration, rh float64) []*kerngeo.Solid {
	if cfg.Railing != cfg.SpiralDirection.DefaultRailing() {
		return nil
	}
	R := cfg.OuterRadius.Millimeters()
	h := cfg.StepHeight.Millimeters()
	n := cfg.StepCount
	delta := FullTurnSpiral / float64(n)
	sign := 1.0
	if cfg.SpiralDirection == engineering.SpiralCCW {
		sign = -1
	}
	pts := make([]kerngeo.Point3, 0, n+1)
	for k := 0; k <= n; k++ {
		a := sign * float64(k) * delta
		pts = append(pts, kerngeo.NewPoint3(R*math.Cos(a), R*math.Sin(a), float64(k)*h+rh))
	}
	var sols []*kerngeo.Solid
	for i := 0; i < n; i++ {
		if s := railAlong(pts[i], pts[i+1]); s != nil {
			sols = append(sols, s)
		}
		a := sign * (float64(i) + 0.5) * delta
		if s := balusterAt(R*math.Cos(a), R*math.Sin(a), float64(i+1)*h, rh); s != nil {
			sols = append(sols, s)
		}
	}
	return sols
}

// BuildRailingDecor строит декоративные тела перил в координатах готовой
// модели. Возвращает nil, если перила не заданы либо высота не положительна.
// Используется только визуализацией (preview mesh).
func BuildRailingDecor(cfg *engineering.StairConfiguration) ([]*kerngeo.Solid, error) {
	rh := cfg.RailingHeight.Millimeters()
	if rh <= 0 {
		return nil, nil
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	n := cfg.StepCount
	switch cfg.Flight {
	case engineering.FlightStraight:
		return straightRailingSolids(n, b, h, rh, w, cfg.Railing), nil
	case engineering.FlightLShape, engineering.FlightUShape:
		return buildLURNailing(cfg, rh)
	case engineering.FlightSpiral:
		return buildSpiralRailing(cfg, rh), nil
	default:
		return nil, fmt.Errorf("geometry: unknown flight kind %q", cfg.Flight)
	}
}

// buildLURNailing — перила L/U по сегментам: нижний марш, площадка, верхний
// марш. Сегменты поворачиваются так же, как в BuildLShapeFlight и
// BuildUShapeFlight (CONF-DIRECTION), а стороны корректируются поворотом.
func buildLURNailing(cfg *engineering.StairConfiguration, rh float64) ([]*kerngeo.Solid, error) {
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	wp := cfg.LandingWidth.Millimeters()
	n1 := cfg.LowerStepCount
	n2 := cfg.StepCount - n1
	h1 := float64(n1) * h
	l1 := float64(n1) * b
	left := cfg.Direction == engineering.TurnLeft
	// Эффективная ширина марша: П-образный (платформенный) сужается на
	// flightSideInsetMM для внутреннего зазора 100 мм; L-образный и веерный
	// остаются на полной ширине W.
	flightW := w
	if cfg.Flight == engineering.FlightUShape {
		flightW = w - flightSideInsetMM
	}

	var sols []*kerngeo.Solid

	// Нижний марш. При левом повороте нижний марш зеркалится на RotateZ(π)
	// (см. BuildLShapeFlight / uShapePlatformTransforms), поэтому сторона
	// перил передаётся напрямую (как для правого поворота): после поворота
	// сегмента целиком локальная кромка RailingRight оказывается на
	// внешней/зеркальной стороне. Узкая ширина flightW учитывается и в
	// профиле перил, и в трансформе (для left — Translate(w+l1, flightW, 0)).
	if railingEnabled(rh, cfg.RailingLower) {
		side := cfg.RailingLower
		lowerT := kerngeo.Identity()
		if left {
			lowerT = kerngeo.Translate(w+l1, flightW, 0).Mul(kerngeo.RotateZ(math.Pi))
		}
		for _, s := range straightRailingSolids(n1, b, h, rh, flightW, side) {
			sols = append(sols, kerngeo.TransformSolid(s, lowerT))
		}
	}

	// Площадка (режим площадки) либо поворотные ступени (режим поворота).
	// Платформенная площадка П-образного марша охватывает 2·W (совпадает с
	// buildUShapePlatform); у L-образного — ширина Wp.
	landingY := wp
	if cfg.Flight == engineering.FlightUShape {
		landingY = 2 * w
	}
	if cfg.TurnKind == engineering.TurnWinder {
		if railingEnabled(rh, cfg.RailingLanding) {
			sols = append(sols, winderRailingSolids(w, h, rh, n1, cfg.WinderCount, l1, wp, left)...)
		}
	} else if railingEnabled(rh, cfg.RailingLanding) {
		x0 := l1
		if left {
			x0 = 0
		}
		sols = append(sols, landingRailingSolids(w, landingY, rh, h1, x0, left)...)
	}

	// Верхний марш.
	if railingEnabled(rh, cfg.RailingUpper) {
		side := cfg.RailingUpper
		var upperT kerngeo.Transform
		rot := 0.0
		if cfg.Flight == engineering.FlightLShape {
			if left {
				upperT = kerngeo.Translate(w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))
				rot = math.Pi / 2
			} else {
				upperT = kerngeo.Translate(l1+w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))
				rot = math.Pi / 2
			}
		} else {
			// П-образный: верхний марш переиспользует трансформ из
			// uShapePlatformTransforms, совпадающий со ступенями. Левый
			// вариант — Translate(0, w, h1) (подъём +X, на 180° к нижнему
			// маршу); правый — Translate(l1+w, 2*w, h1)·RotateZ(π) (подъём −X).
			_, upperT, _, _ = uShapePlatformTransforms(cfg)
			if left {
				rot = 0.0
			} else {
				rot = math.Pi
			}
		}
		for _, s := range straightRailingSolids(n2, b, h, rh, flightW, swapSidesForTurn(side, rot)) {
			sols = append(sols, kerngeo.TransformSolid(s, upperT))
		}
	}

	return sols, nil
}
