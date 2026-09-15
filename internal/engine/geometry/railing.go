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
	case engineering.RailingNone:
		return nil
	default:
		return nil
	}
}

// railingEnabled — строим ли перила на сегменте вообще.
func railingEnabled(rh float64, side engineering.RailingSide) bool {
	return rh > 0 && side.Valid() && side != engineering.RailingNone
}

// edgeInset сдвигает координату кромки внутрь ступени на halfSize, чтобы
// наружная грань детали (поручня или стойки) совпала с кромкой ступени.
// edge=0 (лево) → halfSize; edge=W (право) → W−halfSize.
func edgeInset(edge, w, halfSize float64) float64 {
	if edge < w/2 {
		return halfSize
	}
	return w - halfSize
}

// landingInset сдвигает точку периметра площадки внутрь на halfSize
// perpendicular кромке, на которой находится точка.
func landingInset(p kerngeo.Point3, leftEdgeX, rightEdgeX, wp, halfSize float64) kerngeo.Point3 {
	switch {
	case math.Abs(p.Y) < kerngeo.Precision:
		return kerngeo.NewPoint3(p.X, halfSize, p.Z)
	case math.Abs(p.Y-wp) < kerngeo.Precision:
		return kerngeo.NewPoint3(p.X, wp-halfSize, p.Z)
	case math.Abs(p.X-leftEdgeX) < kerngeo.Precision:
		return kerngeo.NewPoint3(leftEdgeX+halfSize, p.Y, p.Z)
	case math.Abs(p.X-rightEdgeX) < kerngeo.Precision:
		return kerngeo.NewPoint3(rightEdgeX-halfSize, p.Y, p.Z)
	}
	return p
}

// flipSide меняет левую/правую сторону местами: применяется, когда сегмент
// повёрнут на нечётное число четверть-оборотов (90°/180°/270°) и локальная
// сторона, видимая пользователем по ходу подъёма, обратна кромке в локальных
// координатах (CONF-DIRECTION).
func flipSide(s engineering.RailingSide) engineering.RailingSide {
	switch s {
	case engineering.RailingLeft:
		return engineering.RailingRight
	case engineering.RailingRight:
		return engineering.RailingLeft
	case engineering.RailingNone, engineering.RailingBoth:
		return s
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
// height вверх. Верхняя грань горизонтальна (для площадок и спиралей).
// Возвращает nil при вырожденной высоте или ошибке экструзии.
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

// hexFace создаёт грань-четырёхугольник из 4 точек.
func hexFace(a, b, c, d kerngeo.Point3) *kerngeo.Face {
	v0 := kerngeo.NewVertex(a)
	v1 := kerngeo.NewVertex(b)
	v2 := kerngeo.NewVertex(c)
	v3 := kerngeo.NewVertex(d)
	e0 := kerngeo.NewEdge(v0, v1)
	e1 := kerngeo.NewEdge(v1, v2)
	e2 := kerngeo.NewEdge(v2, v3)
	e3 := kerngeo.NewEdge(v3, v0)
	return kerngeo.NewFace(kerngeo.NewWire(e0, e1, e2, e3))
}

// balusterAtSloped строит стойку с наклонной верхней гранью под углом
// slope = rise/run (тангенс угла поручня). Нижняя грань горизонтальна
// на z0; верхняя наклонена: z = z0 + height + slope*(x_local − x).
// Используется для прямых маршей, где поручень идёт под углом h/b.
func balusterAtSloped(x, y, z0, height, slope float64) *kerngeo.Solid {
	if height <= kerngeo.Precision {
		return nil
	}
	hw := balusterSize / 2
	zBase := z0
	zTopLow := z0 + height + slope*(-hw)
	zTopHigh := z0 + height + slope*(+hw)
	b := [4]kerngeo.Point3{
		kerngeo.NewPoint3(x-hw, y-hw, zBase),
		kerngeo.NewPoint3(x+hw, y-hw, zBase),
		kerngeo.NewPoint3(x+hw, y+hw, zBase),
		kerngeo.NewPoint3(x-hw, y+hw, zBase),
	}
	t := [4]kerngeo.Point3{
		kerngeo.NewPoint3(x-hw, y-hw, zTopLow),
		kerngeo.NewPoint3(x+hw, y-hw, zTopHigh),
		kerngeo.NewPoint3(x+hw, y+hw, zTopHigh),
		kerngeo.NewPoint3(x-hw, y+hw, zTopLow),
	}
	faces := []*kerngeo.Face{
		hexFace(b[0], b[3], b[2], b[1]),
		hexFace(t[0], t[1], t[2], t[3]),
		hexFace(b[0], b[1], t[1], t[0]),
		hexFace(b[1], b[2], t[2], t[1]),
		hexFace(b[2], b[3], t[3], t[2]),
		hexFace(b[3], b[0], t[0], t[3]),
	}
	return kerngeo.NewSolidRole(roleBaluster, kerngeo.NewShell(faces...))
}

// straightRailingSolids строит декоративные перила прямого сегмента в
// локальных координатах (x — подъём от 0 до n·b, y — ширина [0, w], z —
// высота до n·h). Поручень и стойки сдвинуты на центры ступеней ((k−0.5)·b)
// по X и внутрь по Y (edgeInset), чтобы балясины не выступали за кромки и
// не утопали в следующую ступень.
func straightRailingSolids(n int, b, h, rh, w float64, side engineering.RailingSide) []*kerngeo.Solid {
	sides := railingSides(side, w)
	if len(sides) == 0 || rh <= 0 || n <= 0 {
		return nil
	}
	H := float64(n) * h
	var sols []*kerngeo.Solid
	slope := h / b
		railOverhang := b / 2
	for _, edge := range sides {
		railY := edgeInset(edge, w, railWidth/2)
		balY := edgeInset(edge, w, balusterSize/2)
		A := kerngeo.NewPoint3(b/2-railOverhang, railY, h+rh-slope*railOverhang)
		B := kerngeo.NewPoint3((float64(n)-0.5)*b+railOverhang, railY, H+rh+slope*railOverhang)
		if s := railAlong(A, B); s != nil {
			sols = append(sols, s)
		}
		for k := 1; k <= n; k++ {
			x := (float64(k) - 0.5) * b
			if s := balusterAtSloped(x, balY, float64(k)*h, rh, slope); s != nil {
				sols = append(sols, s)
			}
		}
	}
	return sols
}

// landingRailingSolids строит горизонтальный поручень вдоль открытых
// кромок площадки (CONF-RAILING сегмент «площадка») и опорные стойки под ним.
// Поручень на высоте h1+rh; стойки (балясины) — вертикальные бруски с тем же
// шагом b, что на маршах, от поверхности площадки (z=h1) до поручня (h1+rh).
//
// Края площадки в плане: leftEdgeX (меньшая X) и rightEdgeX (большая X);
// rightEdgeX-leftEdgeX = w (ширина марша). «Лево»/«право» перил площадки
// выбираются ПО ПЛАНУ: RailingLeft — левая вертикаль (leftEdgeX), RailingRight
// — правая (rightEdgeX), RailingBoth — обе вертикали + низ (весь открытый
// периметр). Это согласуется с «лево/право» при движении по площадке к
// верхнему маршу и зеркально для левого поворота.
//
// closeFar управляет дальней кромкой (Y=wp), откуда верхний марш уходит
// вдоль +Y. Для П-образного марша (closeFar=true) площадка охватывает 2·W и
// оба марша примыкают к одной боковой кромке, поэтому строим «П» (низ +
// внешний бок + дальняя кромка), выбор стороны (side) не учитывается.
// Для L-образного (closeFar=false) верхний марш отходит от дальней кромки,
// поэтому её оставляем открытой. Нижний марш примыкает к площадке по одной
// из вертикалей ровно на ширину w, так что на этой кромке Y∈[0,w] — проход,
// а участок Y∈[w,wp] (если wp>w) — внешний и тоже огораживается. Проход к
// нижнему маршу (Y∈[0,w] на пристеночной вертикали) и к верхнему (Y=wp)
// остаются открытыми при любом выборе стороны.
func landingRailingSolids(w, wp, rh, h1, b float64, x0 float64, left, closeFar bool, side engineering.RailingSide) []*kerngeo.Solid {
	if rh <= 0 {
		return nil
	}
	zRail := h1 + rh
	zBase := h1
	// Контур охватывает ВСЕ внешние кромки площадки, кроме кромок-проходов
	// (где примыкают марши). Это гарантирует, что при LandingWidth > Width
	// («площадка больше марша») на площадке не остаётся неогороженных
	// кусков. Каждый сегмент — пара точек поручня (z=zRail) и базы стойки
	// (z=zBase) с теми же (x,y).
	//
	// Нижний марш примыкает к площадке по боковой кромке (X=x0 для правого
	// поворота, X=w для левого) ровно на ширину марша w, поэтому на этой
	// кромке Y∈[0,w] — проход, а участок Y∈[w,wp] (если wp>w) — внешний и
	// тоже огораживается. Дальняя кромка Y=wp — проход к верхнему маршу
	// (L-марш, closeFar=false) и остаётся открытой; для П-марша (closeFar)
	// она, наоборот, входит в контур «П».
	type seg [2]kerngeo.Point3
	var top, base []seg
	add := func(ax, ay, bx, by float64) {
		top = append(top, seg{
			kerngeo.NewPoint3(ax, ay, zRail),
			kerngeo.NewPoint3(bx, by, zRail),
		})
		base = append(base, seg{
			kerngeo.NewPoint3(ax, ay, zBase),
			kerngeo.NewPoint3(bx, by, zBase),
		})
	}
	// Края площадки в плане (rightEdgeX-leftEdgeX = w — X-пролёт площадки).
	// Левый/правый края определяются переданным x0 (для левого поворота L-марша
	// это w−ld, чтобы площадка совпадала с геометрией buildLanding).
	leftEdgeX, rightEdgeX := x0, x0+w
	// flightSideX — вертикаль, к которой примыкает нижний марш; на ней
	// Y∈[0,w] — проход, остальное (Y∈[w,wp]) при wp>w — внешний участок,
	// который при выборе этой стороны тоже огораживается.
	flightSideX := leftEdgeX
	if left {
		flightSideX = rightEdgeX
	}
	railEdge := func(cx float64) {
		y0 := 0.0
		if cx == flightSideX {
			y0 = w // пропускаем проход к нижнему маршу
		}
		if wp > y0 {
			add(cx, wp, cx, y0)
		}
	}
	switch {
	case closeFar:
		// П-образный марш: «П» (низ + внешний бок + дальняя кромка).
		// Выбор стороны не учитывается (по плану — только L-марш).
		outerX := rightEdgeX
		if left {
			outerX = leftEdgeX
		}
		otherX := leftEdgeX
		if outerX == leftEdgeX {
			otherX = rightEdgeX
		}
		add(leftEdgeX, 0, rightEdgeX, 0) // низ
		add(outerX, 0, outerX, wp)       // внешний бок
		add(outerX, wp, otherX, wp)      // дальняя кромка (проход к верхнему маршу)
	case side == engineering.RailingRight:
		railEdge(rightEdgeX)
	case side == engineering.RailingLeft:
		railEdge(leftEdgeX)
	default: // RailingBoth (и RailingNone не должен приходить — вызов загорожен)
		add(leftEdgeX, 0, rightEdgeX, 0) // низ
		railEdge(rightEdgeX)
		railEdge(leftEdgeX)
	}

	var sols []*kerngeo.Solid
	for i := range top {
		p0 := landingInset(top[i][0], leftEdgeX, rightEdgeX, wp, railWidth/2)
		p1 := landingInset(top[i][1], leftEdgeX, rightEdgeX, wp, railWidth/2)
		if s := railAlong(p0, p1); s != nil {
			sols = append(sols, s)
		}
	}
	// Балясины вдоль контура с тем же линейным шагом b, что на маршах.
	if b > kerngeo.Precision {
		for i := range base {
			p0, p1 := base[i][0], base[i][1]
			segVec := p1.Sub(p0)
			L := segVec.Norm()
			if L <= kerngeo.Precision {
				continue
			}
			dir, _ := segVec.Normalized()
			// Первый сегмент начинаем с угла (d=0), остальные — с шага b,
			// чтобы не дублировать стойку в общем углу со смежным сегментом.
			start := 0.0
			if i > 0 {
				start = b
			}
			for d := start; d <= L+kerngeo.Precision; d += b {
				dd := d
				if dd > L {
					dd = L
				}
				p := p0.Add(dir.Scale(dd))
				p = landingInset(p, leftEdgeX, rightEdgeX, wp, balusterSize/2)
				if s := balusterAt(p.X, p.Y, h1, rh); s != nil {
					sols = append(sols, s)
				}
			}
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
	rRail := ro - railWidth/2
	var pts []kerngeo.Point3
	for i := 0; i <= segs; i++ {
		a := a0 + math.Pi*float64(i)/float64(segs)
		pts = append(pts, kerngeo.NewPoint3(ox+rRail*math.Cos(a), oy+rRail*math.Sin(a), zRail))
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
	rRail := R - railWidth/2
	rBal := R - balusterSize/2
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
		pts = append(pts, kerngeo.NewPoint3(rRail*math.Cos(a), rRail*math.Sin(a), float64(k)*h+rh))
	}
	var sols []*kerngeo.Solid
	for i := 0; i < n; i++ {
		if s := railAlong(pts[i], pts[i+1]); s != nil {
			sols = append(sols, s)
		}
		a := sign * (float64(i) + 0.5) * delta
		if s := balusterAt(rBal*math.Cos(a), rBal*math.Sin(a), float64(i+1)*h, rh); s != nil {
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
	ld := cfg.LandingDepth.Millimeters()
	if ld <= 0 {
		ld = w
	}
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
		landingXExt := ld
		if left {
			if cfg.Flight == engineering.FlightLShape {
				x0 = w - ld
			}
			// для U-образного x0 остаётся 0 (площадка [0, W]×[0, 2W])
		}
		// Платформенная площадка L-марша: дальняя кромка Y=wp открыта
		// (верхний марш отходит от неё), поэтому контур «Г», а не «П»,
		// чтобы перила не перекрывали проход ко второму маршу. X-пролёт
		// площадки = ld (глубина), Y-размер = landingY (ширина Wp / 2W).
		closeFar := cfg.Flight == engineering.FlightUShape
		sols = append(sols, landingRailingSolids(landingXExt, landingY, rh, h1, b, x0, left, closeFar, cfg.RailingLanding)...)
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
