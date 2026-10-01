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
	// roleRailingGlass — стеклянное заполнение ограждения. Отдельная роль, а
	// не «ещё один baluster», потому что материал принципиально другой:
	// стекло пропускает свет (transmission), металл и дерево — нет. Роль
	// приходит в mesh как PartRange, и вьювер вешает на этот диапазон
	// физический материал с прозрачностью.
	roleRailingGlass = "railing_glass"
)

// Типовые габариты деталей перил (мм; визуализация, не производственный пак).
const (
	railWidth     = 50.0 // поручень: размер поперёк марша
	railThickness = 40.0 // поручень: размер поперёк наклона
	balusterSize  = 20.0 // стойка: сечение в плане

	// railProfileRadius — скругление углов профиля поручня. Прямоугольный
	// брус 50×40 с острыми рёбрами выглядит в рендере как металлическая
	// рейка: на нём ломается блик, и ограждение читается как «чертёж», а не
	// как изделие. Скруглённый прямоугольник — тот же габарит, но с
	// непрерывным бликом по всей длине, как у настоящего поручня.
	// 14 мм < min(50, 40)/2 — дуги соседних углов не пересекаются.
	railProfileRadius = 14.0
	// railProfileSegments — фасетов на углу профиля. 4 на 90° даёт шаг
	// ~11°: силуэт читается гладким, а меш остаётся дешёвым (декор, не BOM).
	railProfileSegments = 4

	// glassThickness — толщина стеклянного заполнения (10 мм — нормативное
	// закалённое стекло для ограждений высотой до 1,2 м).
	glassThickness = 10.0
	// glassRailGap — зазор между верхом стекла и низом поручня. Стекло
	// вставляется в профиль-прижим, а не упирается в поручень: без зазора
	// панель «склеивается» с брусом на рендере и стеклянный контур
	// перестаёт читаться.
	glassRailGap = 6.0
	// glassBaseGap — подъём низа панели над ступенью: стекло стоит в башмаке,
	// а его нижняя грань не должна лежать в одной плоскости с верхней гранью
	// ступени (иначе z-fighting по всей длине контакта).
	glassBaseGap = 2.0
	// glassNoseClear — отступ вертикальной кромки панели от линии носка внутрь
	// марша: кромка не должна совпадать с плоскостью носка (z-fighting).
	glassNoseClear = 1.0
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

// dedupeProfile убирает из замкнутого профиля точки, слитые с соседями ближе
// tol, и снимает совпадение первой и последней точки (контур замыкает сам
// Extrude). tol сравнения — существующий, но недостаточный для этого
// pushDistinct: он ловит только точное совпадение с ПОСЛЕДНЕЙ точкой и не
// смотрит на замыкание контура.
func dedupeProfile(pts []kerngeo.Point3, tol float64) []kerngeo.Point3 {
	out := make([]kerngeo.Point3, 0, len(pts))
	for _, p := range pts {
		if n := len(out); n > 0 && out[n-1].Distance(p) <= tol {
			continue
		}
		out = append(out, p)
	}
	for len(out) > 1 && out[0].Distance(out[len(out)-1]) <= tol {
		out = out[:len(out)-1]
	}
	return out
}

// roundProfile скругляет все углы замкнутого профиля, заданного точками в
// порядке обхода. Использует kerngeo.RoundCorner — тот же примитив, что и
// фаска на носке проступи (ENG-GEO-0023), поэтому скругление поручня и скругление
// ступени считаются одним кодом и ведут себя одинаково.
//
// RoundCorner возвращает только ВНУТРЕННИЕ точки дуги (касательные не входят),
// причём дуга идёт от касательной на ребре к ПРЕДЫДУЩЕМУ углу к касательной на
// ребре к СЛЕДУЮЩЕМУ. Поэтому для каждого угла порядок вывода такой: [внутренние
// точки дуги, касательная к следующему углу] — последняя точка предыдущего угла
// и есть первая точка дуги текущего, обход получается непрерывным, а профиль
// замкнутым.
//
// Порядок «сначала касательная, потом дуга» здесь ломает обход: дуга начинается
// с ДРУГОГО конца ребра, контур самопересекается на каждом углу, и тело
// получается вывернутым (объём отрицательный) —Extrude такого тела отдаёт
// «polygon is not simple» уже на триангуляции.
//
// Вырожденный угол (прямой или развёрнутый) оставляется как есть: скруглять
// нечего, и ошибка не должна ронять всё ограждение.
func roundProfile(pts []kerngeo.Point3, radius float64, segments int) []kerngeo.Point3 {
	n := len(pts)
	if n < 3 || radius <= kerngeo.Precision {
		return pts
	}
	out := make([]kerngeo.Point3, 0, n*segments)
	for i := 0; i < n; i++ {
		prev, corner, next := pts[(i-1+n)%n], pts[i], pts[(i+1)%n]
		arc, err := kerngeo.RoundCorner(prev, corner, next, radius, segments)
		if err != nil || len(arc) == 0 {
			out = append(out, corner)
			continue
		}
		out = append(out, arc...)
		// Касательная на ребре corner→next: длина от угла равна
		// radius/tan(половина угла), как в RoundCorner. Она же завершает
		// дугу и открывает следующую.
		toNext := next.Sub(corner)
		l := toNext.Norm()
		if l <= kerngeo.Precision {
			out = append(out, corner)
			continue
		}
		tangentLen := math.Min(radius, l/2)
		out = append(out, corner.Add(toNext.Scale(tangentLen/l)))
	}
	return out
}

// railAlong строит прямолинейный брус поручня со скруглённым профилем,
// занимающий ровно отрезок между a и b (высота поручня уже включена в
// координаты). Extrude выдавливает призму от плоскости профиля вдоль u на L,
// поэтому профиль строится вокруг середины отрезка и смещается к началу a;
// иначе призма заняла бы [середина, середина+L·u] и поручень «уезжал» на
// половину марша. Возвращает nil, если сегмент вырожден.
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
	// Габарит 50×40 — по рёбрам, скругление срезает углы ВНУТРЬ профиля и
	// внешние размеры не меняет: поручень встаёт в те же габариты, что и
	// прямоугольный, иначе он вышел бы за кромку ступени.
	radius := math.Min(railProfileRadius, math.Min(rw, rt))
	base := roundProfile([]kerngeo.Point3{
		c.Add(wd.Scale(rw)).Add(td.Scale(rt)),
		c.Add(wd.Scale(rw)).Add(td.Scale(-rt)),
		c.Add(wd.Scale(-rw)).Add(td.Scale(-rt)),
		c.Add(wd.Scale(-rw)).Add(td.Scale(rt)),
	}, radius, railProfileSegments)
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

// prismAtSloped строит вертикальную призму с прямоугольным сечением в плане
// (halfX по X, halfY по Y) и НАКЛОННОЙ верхней гранью: z = zBase + height +
// slope·(x − xc). Нижняя грань горизонтальна. Это общая форма и для стойки
// (сечение квадратное, halfX = halfY), и для стеклянной панели ограждения
// (сечение вытянутое вдоль марша, halfY — половина толщины стекла).
// slope = h/b — тангенс угла поручня, поэтому верхняя грань панели идёт
// параллельно поручню, а не «ступенкой».
func prismAtSloped(xc, y, halfX, halfY, zBase, height, slope float64, role string) *kerngeo.Solid {
	if height <= kerngeo.Precision {
		return nil
	}
	zTopLow := zBase + height + slope*(-halfX)
	zTopHigh := zBase + height + slope*(+halfX)
	b := [4]kerngeo.Point3{
		kerngeo.NewPoint3(xc-halfX, y-halfY, zBase),
		kerngeo.NewPoint3(xc+halfX, y-halfY, zBase),
		kerngeo.NewPoint3(xc+halfX, y+halfY, zBase),
		kerngeo.NewPoint3(xc-halfX, y+halfY, zBase),
	}
	t := [4]kerngeo.Point3{
		kerngeo.NewPoint3(xc-halfX, y-halfY, zTopLow),
		kerngeo.NewPoint3(xc+halfX, y-halfY, zTopHigh),
		kerngeo.NewPoint3(xc+halfX, y+halfY, zTopHigh),
		kerngeo.NewPoint3(xc-halfX, y+halfY, zTopLow),
	}
	faces := []*kerngeo.Face{
		hexFace(b[0], b[3], b[2], b[1]),
		hexFace(t[0], t[1], t[2], t[3]),
		hexFace(b[0], b[1], t[1], t[0]),
		hexFace(b[1], b[2], t[2], t[1]),
		hexFace(b[2], b[3], t[3], t[2]),
		hexFace(b[3], b[0], t[0], t[3]),
	}
	return kerngeo.NewSolidRole(role, kerngeo.NewShell(faces...))
}

// balusterAtSloped строит стойку с наклонной верхней гранью под углом
// slope = rise/run (тангенс угла поручня). Нижняя грань горизонтальна
// на z0; верхняя наклонена: z = z0 + height + slope*(x_local − x).
// Используется для прямых маршей, где поручень идёт под углом h/b.
func balusterAtSloped(x, y, z0, height, slope float64) *kerngeo.Solid {
	hw := balusterSize / 2
	return prismAtSloped(x, y, hw, hw, z0, height, slope, roleBaluster)
}

// glassPanelStepped строит стеклянную панель ограждения между двумя
// соседними стойками прямого марша.
//
// НИЖНЯЯ кромка повторяет ступени. Панель наклонена вместе с маршем, и если
// опустить её на одну высоту, она на половине длины уйдёт в грунт (в ступень
// высотой h) или, наоборот, повиснет над ступенью с зазором. Поэтому профиль
// строится в плоскости XZ лесенкой: xStep — линия носка ступени, и панель
// лежит на нижней ступени до носка и на верхней после него.
//
// ВЕРХНЯЯ кромка параллельна поручню: та же формула, что у оси поручня
// (z = h/2 + rh + slope·x), поэтому стекло уходит ровно в зазор glassRailGap
// под брусом и нигде не пересекает его.
//
// Смещение панели внутрь марша (xStep−glassNoseClear) и вверх от ступени
// (glassBaseGap) — не украшение: кромки панели иначе лежат в одной плоскости
// с носком и с верхней гранью ступени, а это z-fighting на самом видном
// месте конструкции.
func glassPanelStepped(x0, x1, xStep, y, zLow, zHigh, slope, rh float64) *kerngeo.Solid {
	// Верхняя кромка стекла — параллельно оси поручня и ниже НИЗА бруса.
	//
	// Опорная точка наклона — ЦЕНТР панели xc, а не линия носка xStep. Ось
	// поручня проходит через верх стойки: над центром стойки она ровно на rh
	// над ступенью, то есть axis(x) = zLow + rh + slope·(x − xc). Если
	// отсчитать наклон от носка, вся панель поднимется на slope·st (для
	// 180/270 и st=40 это 27 мм) и стекло уйдёт В поручень.
	//
	// Сдвиг вниз считается по ПЕРПЕНДИКУЛЯРУ к оси, а не по вертикали: низ
	// бруса — это точка оси, сдвинутая на railThickness/2 перпендикулярно, и
	// для наклонного поручня это выше, чем вертикальный сдвиг на ту же
	// величину. Вертикальная аппроксимация здесь дала бы зазор в 3 мм
	// вместо 9 и стекло снова срослось бы с брусом.
	cosTheta := 1 / math.Sqrt(1+slope*slope)
	xc := (x0 + x1) / 2
	top := func(x float64) float64 {
		axis := zLow + rh + slope*(x-xc)
		return axis - railThickness/2*cosTheta - glassRailGap
	}
	// Вырожденные конфигурации, где профиль не лечь: поручень вплотную к
	// ступени (панели без высоты) либо толщина проступи больше выноса, и
	// линия носка уходит за левый край панели. Без этих проверок точки
	// профиля идут назад по X, контур самопересекается — и падает триангуляция
	// ВСЕЙ сцены на «polygon is not simple», включая несвязанные марши.
	if top(x0) <= zHigh+glassBaseGap || top(x1) <= zHigh+glassBaseGap {
		return nil
	}
	eps := math.Min(glassThickness, (x1-x0)/4)
	if xStep <= x0+eps+glassNoseClear {
		xStep = x0 + eps + glassNoseClear
	}
	if xStep >= x1-eps {
		return nil
	}
	yNear := y - glassThickness/2
	pt := func(x, z float64) kerngeo.Point3 {
		return kerngeo.NewPoint3(x, yNear, z)
	}
	profile := dedupeProfile([]kerngeo.Point3{
		pt(x0, zLow+glassBaseGap),
		pt(xStep-glassNoseClear, zLow+glassBaseGap),
		pt(xStep-glassNoseClear, zHigh+glassBaseGap),
		pt(x1, zHigh+glassBaseGap),
		pt(x1, top(x1)),
		pt(x0, top(x0)),
	}, glassThickness)
	s, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 1, 0), glassThickness)
	if err != nil {
		return nil
	}
	return s.WithRole(roleRailingGlass)
}

// glassPanelFlat строит вертикальную панель ограждения вдоль отрезка p0→p1 на
// ПЛОСКОЙ поверхности (площадка): нижняя кромка горизонтальна. Профиль строится
// в вертикальной плоскости отрезка и выдавливается поперёк на толщину стекла.
func glassPanelFlat(p0, p1 kerngeo.Point3, zBase, height, thickness float64) *kerngeo.Solid {
	if height <= kerngeo.Precision {
		return nil
	}
	dir := p1.Sub(p0)
	if dir.Norm() <= kerngeo.Precision {
		return nil
	}
	u, ok := dir.Normalized()
	if !ok {
		return nil
	}
	// Поперечное направление: горизонтальный перпендикуляр к отрезку.
	wd := kerngeo.NewVector3(-u.Y, u.X, 0)
	if wdN, ok := wd.Normalized(); ok {
		wd = wdN
	} else {
		return nil
	}
	z0 := zBase + glassBaseGap
	z1 := z0 + height
	// Профиль лежит в плоскости, сдвинутой на половину толщины «назад», и
	// выдавливается к панели: так стекло центрируется на линии перил.
	back := wd.Scale(-thickness / 2)
	at := func(p kerngeo.Point3, z float64) kerngeo.Point3 {
		return kerngeo.NewPoint3(p.Add(back).X, p.Add(back).Y, z)
	}
	profile := []kerngeo.Point3{
		at(p0, z0),
		at(p1, z0),
		at(p1, z1),
		at(p0, z1),
	}
	s, err := kerngeo.Extrude(dedupeProfile(profile, thickness), wd, thickness)
	if err != nil {
		return nil
	}
	return s.WithRole(roleRailingGlass)
}

// straightRailingSolids строит декоративные перила прямого сегмента в
// локальных координатах (x — подъём от 0 до n·b, y — ширина [0, w], z —
// высота до n·h). Поручень и стойки сдвинуты на центры ступеней ((k−0.5)·b)
// по X и внутрь по Y (edgeInset), чтобы балясины не выступали за кромки и
// не утопали в следующую ступень.
//
// Между стойками k и k+1 ставится стеклянная панель. Верхний и нижний пролёты
// (по b/2 за подъёмом и спуском) стеклом не закрываются: снизу марш упирается
// в пол, сверху открывается на площадку/междуэтажное перекрытие — там панели
// некуда опираться.
//
// st — толщина проступи: ею задаётся линия носка, на которой нижняя кромка
// панели переходит с нижней ступени на верхнюю.
func straightRailingSolids(n int, b, h, rh, w, st float64, side engineering.RailingSide) []*kerngeo.Solid {
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
		// Стекло между стойками: панель k закрывает ступень k, поэтому её
		// верхняя ступень — k+1, а линия носка — k·b−st (передняя грань
		// проступи этой ступени).
		for k := 1; k < n; k++ {
			x0 := (float64(k) - 0.5) * b
			x1 := (float64(k) + 0.5) * b
			zLow := float64(k) * h
			zHigh := float64(k+1) * h
			xStep := float64(k)*b - st
			if s := glassPanelStepped(x0, x1, xStep, balY, zLow, zHigh, slope, rh); s != nil {
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
// landingRailingSolids строит перила площадки.
//
// ПАРАМЕТРЫ (GEOM-02, forensic 2026-09-27). Здесь принципиально разведены
// ТРИ разные величины, которые раньше смешивались:
//
//	w       — X-пролёт площадки (её глубина вдоль нижнего марша);
//	wp      — Y-размер площадки (Wp для L-марша, 2W для П-марша);
//	flightW — ширина ПРОХОДА, то есть ширина нижнего марша (Width).
//
// Раньше третьего не было вовсе, и проход вычислялся как `y0 = w`, то есть
// по глубине площадки. Пока LandingDepth всегда обнулялся решателем
// (SOLVER-03) и равен��я Width, подмена была незаметна. С починкой
// SOLVER-03 она стала бы живой: при ld > Width ограждение исчезало бы
// полностью, а при Width < ld < Wp оставалась бы неогороженная полоса.
func landingRailingSolids(w, wp, flightW, rh, h1, b float64, x0 float64, left, closeFar bool, side engineering.RailingSide) []*kerngeo.Solid {
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
			// Проход к нижнему маршу — по ШИРИНЕ марша (flightW), а не по
			// глубине площадки. flightW = 0 допускается (значение не задано):
			// тогда кромка огораживается целиком, что безопаснее, чем
			// неоговороженный участок.
			y0 = flightW
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
	// Позиции стоек запоминаются: между соседними ставится стеклянная панель
	// (высота — от площадки до низа поручня минус зазоры башмака и прижима).
	if b > kerngeo.Precision {
		panelH := rh - railThickness/2 - glassRailGap - glassBaseGap
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
			var posts []kerngeo.Point3
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
				posts = append(posts, p)
			}
			// Стекло между стойками. Отступ на половину сечения стойки с
			// каждого конца, чтобы панель зажималась между ними, а не
			// проходила сквозь.
			clear := balusterSize / 2
			for j := 0; j+1 < len(posts); j++ {
				a := posts[j].Add(dir.Scale(clear))
				c := posts[j+1].Add(dir.Scale(-clear))
				if c.Sub(a).Norm() <= kerngeo.Precision {
					continue
				}
				if s := glassPanelFlat(a, c, h1, panelH, glassThickness); s != nil {
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
		return straightRailingSolids(n, b, h, rh, w, cfg.StepThickness.Millimeters(), cfg.Railing), nil
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
		for _, s := range straightRailingSolids(n1, b, h, rh, flightW, cfg.StepThickness.Millimeters(), side) {
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
		if cfg.Flight == engineering.FlightUShape {
			// GEOM-07: площадка П-марша строится с X-пролётом = ширина
			// марша (builder.go:359 — buildLanding(w, landingY, …)), а не
			// ld. Раньше перила получали ld и при ld ≠ W контур перил не
			// совпадал с контуром площадки.
			landingXExt = w
		}
		if left {
			if cfg.Flight == engineering.FlightLShape {
				x0 = w - ld
			} else {
				// GEOM-03 (forensic 2026-09-27): П-образный марш при левом
				// повороте строит площадку с landingX0 = 0 (см. builder.go
				// buildUShapePlatform: `if left { landingX0 = 0 }`), то есть
				// площадка лежит [0, W]×[0, 2W]. Код здесь оставлял x0 = l1
				// (= n1·b, до 1890 мм), из-за чего перила площадки уезжали на
				// l1 в сторону от самой площадки: площадка X[0..900], перила
				// X[1865..2790] — смещение 1865 мм при марше 900 мм.
				//
				// Комментарий в коде утверждал именно это («x0 остаётся 0»), но
				// условие проверяло только L-марш. Комментарий описывал
				// намерение, а не поведение — сильный признак регрессии.
				x0 = 0
			}
		}
		// Платформенная площадка L-марша: дальняя кромка Y=wp открыта
		// (верхний марш отходит от неё), поэтому контур «Г», а не «П»,
		// чтобы перила не перекрывали проход ко второму маршу. X-пролёт
		// площадки = ld (глубина), Y-размер = landingY (ширина Wp / 2W).
		closeFar := cfg.Flight == engineering.FlightUShape
		// GEOM-02: проход по ширине марша, а не по глубине площадки.
		sols = append(sols, landingRailingSolids(landingXExt, landingY, w, rh, h1, b, x0, left, closeFar, cfg.RailingLanding)...)
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
		for _, s := range straightRailingSolids(n2, b, h, rh, flightW, cfg.StepThickness.Millimeters(), swapSidesForTurn(side, rot)) {
			sols = append(sols, kerngeo.TransformSolid(s, upperT))
		}
	}

	return sols, nil
}
