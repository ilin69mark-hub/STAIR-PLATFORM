package geometry

import (
	"fmt"
	"math"
)

// bboxAccum накапливает min/max без промежуточных срезов и без
// IsEmpty-проверки на каждой точке.
type bboxAccum struct {
	min, max Point3
	set      bool
}

func (a *bboxAccum) add(p Point3) {
	if !a.set {
		a.min, a.max, a.set = p, p, true
		return
	}
	if p.X < a.min.X {
		a.min.X = p.X
	}
	if p.Y < a.min.Y {
		a.min.Y = p.Y
	}
	if p.Z < a.min.Z {
		a.min.Z = p.Z
	}
	if p.X > a.max.X {
		a.max.X = p.X
	}
	if p.Y > a.max.Y {
		a.max.Y = p.Y
	}
	if p.Z > a.max.Z {
		a.max.Z = p.Z
	}
}

func (a *bboxAccum) bbox() BBox {
	if !a.set {
		return BBox{}
	}
	return BBox{Min: a.min, Max: a.max}
}

// accumulateSolidBounds добавляет в аккумулятор вершины всех граней тела.
// Обход идёт по рёбрам контуров, поэтому срез точек не выделяется: на
// металлокаркасе это сотни деталей, и лишние срезы на каждое тело
// доминировали в профиле.
func accumulateSolidBounds(a *bboxAccum, solid *Solid) {
	if solid == nil {
		return
	}
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			addWireBounds(a, face.Outer())
			for _, inner := range face.Inner() {
				addWireBounds(a, inner)
			}
		}
	}
}

// addWireBounds добавляет в аккумулятор точки контура провода. Замыкающая
// дублирующаяся точка не влияет на min/max, поэтому ребро целиком.
func addWireBounds(a *bboxAccum, w *Wire) {
	if w == nil {
		return
	}
	for _, e := range w.Edges() {
		first, second := e.Endpoints()
		a.add(first.Point())
		a.add(second.Point())
	}
}

// BoundingBox возвращает BBox всей модели — минимальный охват всех вершин
// всех тел (ENG-GEO-0013). Детерминированно: порядок тел Compound.
func BoundingBox(model *Compound) BBox {
	if model == nil {
		return BBox{}
	}
	var a bboxAccum
	for _, solid := range model.Solids() {
		accumulateSolidBounds(&a, solid)
	}
	return a.bbox()
}

// SolidBoundingBox возвращает BBox одного тела — минимальный охват всех его
// вершин (ENG-GEO-0013). Избегает создания промежуточного Compound.
func SolidBoundingBox(solid *Solid) BBox {
	var a bboxAccum
	accumulateSolidBounds(&a, solid)
	return a.bbox()
}

// SolidCount возвращает число твёрдых тел в модели.
func SolidCount(model *Compound) int {
	if model == nil {
		return 0
	}
	return len(model.Solids())
}

// solidPoints собирает все вершины всех граней тела (с дубликатами,
// в порядке обхода). Только для измерений.
func solidPoints(solid *Solid) []Point3 {
	var pts []Point3
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			wire, _ := contourPoints(face.Outer())
			pts = append(pts, wire...)
			for _, inner := range face.Inner() {
				if ip, ok := contourPoints(inner); ok {
					pts = append(pts, ip...)
				}
			}
		}
	}
	return pts
}

// SolidPlateBlank — габарит плоской детали в её СОБСТВЕННОЙ системе осей:
// длина вдоль длиннейшего ребра и ширина по нормали к нему. Это размеры
// заготовки для раскроя, а не габаритный блок по осям сцены.
//
// Нужно боковой пластине косоура: она стоит в наклонной плоскости, и её
// bbox по X/Z не является ни длиной, ни шириной листа. Ось вдоль
// длиннейшего ребра — это посадочная линия ступеней (верхнее ребро
// пластины), а ширина — расстояние от неё до нижнего ребра, то есть ровно
// та глубина пластины, что заявлена в чертеже.
func SolidPlateBlank(solid *Solid) (length, width float64) {
	// nil-тело — не размер «0×0», а отсутствие детали. Без проверки здесь
	// паника: solidPoints дёргает solid.Shells(). По коду это те же нули,
	// что и у вырожденного тела ниже, но без паники на пустом входе.
	if solid == nil {
		return 0, 0
	}
	pts := solidPoints(solid)
	if len(pts) < 3 {
		return 0, 0
	}
	best := 0.0
	// Тонкая ось — та, по которой тело тонкое: она выпадает из плоскости
	// детали. Остальные две задают её плоскость.
	bb := SolidBoundingBox(solid)
	ext := [3]float64{bb.Max.X - bb.Min.X, bb.Max.Y - bb.Min.Y, bb.Max.Z - bb.Min.Z}
	thin := 0
	for i := 1; i < 3; i++ {
		if ext[i] < ext[thin] {
			thin = i
		}
	}
	var axes [2]int
	k := 0
	for i := 0; i < 3; i++ {
		if i != thin {
			axes[k] = i
			k++
		}
	}
	// Длиннейшее РЕБРО контура — посадочная линия. Именно ребро, а не
	// самая длинная пара вершин: диагональ пятой вершины длиннее посадочной
	// линии, и по ней ширина пластины выходила бы 408 мм вместо 300.
	bx, by := 1.0, 0.0
	for _, ring := range solidRings(solid) {
		for i := range ring {
			a, c := ring[i], ring[(i+1)%len(ring)]
			dx := coord(c, axes[0]) - coord(a, axes[0])
			dy := coord(c, axes[1]) - coord(a, axes[1])
			if d := math.Hypot(dx, dy); d > best {
				best, bx, by = d, dx, dy
			}
		}
	}
	if best <= 0 {
		return 0, 0
	}
	uniq := dedupePoints(pts, 1e-6)
	ux, uy := bx/best, by/best
	// Проекции всех вершин: вдоль ребра и по нормали к нему.
	sMin, sMax := math.Inf(1), math.Inf(-1)
	pMin, pMax := math.Inf(1), math.Inf(-1)
	for _, p := range uniq {
		p0, p1 := coord(p, axes[0]), coord(p, axes[1])
		s := p0*ux + p1*uy
		q := -p0*uy + p1*ux
		sMin, sMax = math.Min(sMin, s), math.Max(sMax, s)
		pMin, pMax = math.Min(pMin, q), math.Max(pMax, q)
	}
	return sMax - sMin, pMax - pMin
}

// CutLengthMM — длина лазерного реза детали из листа (MMF-0006): контур
// детали ПЛЮС периметры всех отверстий и регулировочных овалов внутри.
//
// Лазер режет по КОНТУРУ, то есть по плоскости листа; рёбра вдоль толщины
// (боковые стенки) он не трогает — их лист уже имеет. Поэтому берутся только
// КРЫШКИ детали — грани, перпендикулярные её тонкой оси: их контуры и есть
// то, что вырезается. Крышек две, контуры у них одинаковые, поэтому сумма
// делится на два.
//
// Для детали без отверстий это периметр её сечения. Для плиты с отверстиями —
// периметр плюс отверстия, поэтому лазерный рез такой детали дороже.
func CutLengthMM(solid *Solid) float64 {
	if solid == nil {
		return 0
	}
	bb := SolidBoundingBox(solid)
	ext := [3]float64{bb.Max.X - bb.Min.X, bb.Max.Y - bb.Min.Y, bb.Max.Z - bb.Min.Z}
	thin := 0
	for i := 1; i < 3; i++ {
		if ext[i] < ext[thin] {
			thin = i
		}
	}
	lo, hi := [3]float64{bb.Min.X, bb.Min.Y, bb.Min.Z}[thin], [3]float64{bb.Max.X, bb.Max.Y, bb.Max.Z}[thin]
	if hi-lo <= Precision {
		return 0
	}
	var sum float64
	caps := 0
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			outer, ok := contourPoints(face.Outer())
			if !ok {
				continue
			}
			if !onPlane(outer, thin, lo) && !onPlane(outer, thin, hi) {
				continue
			}
			caps++
			sum += ringPerimeter2D(outer)
			for _, inner := range face.Inner() {
				if pts, ok := contourPoints(inner); ok {
					sum += ringPerimeter2D(pts)
				}
			}
		}
	}
	if caps == 0 {
		return 0
	}
	return sum / float64(caps) * float64(caps) / 2
}

// onPlane — все точки лежат на одной из двух плоскостей тонкой оси.
func onPlane(pts []Point3, axis int, val float64) bool {
	for _, p := range pts {
		v := p.X
		switch axis {
		case 1:
			v = p.Y
		case 2:
			v = p.Z
		}
		if math.Abs(v-val) > Precision {
			return false
		}
	}
	return true
}

// ringPerimeter2D — длина замкнутого кольца.
func ringPerimeter2D(pts []Point3) float64 {
	if len(pts) < 2 {
		return 0
	}
	var p float64
	for i := range pts {
		j := (i + 1) % len(pts)
		p += pts[i].Distance(pts[j])
	}
	return p
}

// solidRings — контуры всех граней тела: по ним ищутся РЁБРА детали.
func solidRings(solid *Solid) [][]Point3 {
	var rings [][]Point3
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			if pts, ok := contourPoints(face.Outer()); ok && len(pts) > 1 {
				rings = append(rings, pts)
			}
		}
	}
	return rings
}

func coord(p Point3, axis int) float64 {
	switch axis {
	case 0:
		return p.X
	case 1:
		return p.Y
	default:
		return p.Z
	}
}

func dedupePoints(pts []Point3, tol float64) []Point3 {
	out := make([]Point3, 0, len(pts))
	for _, p := range pts {
		dup := false
		for _, q := range out {
			if math.Abs(p.X-q.X) <= tol && math.Abs(p.Y-q.Y) <= tol && math.Abs(p.Z-q.Z) <= tol {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, p)
		}
	}
	return out
}

// Volume возвращает объём замкнутого твёрдого тела (мм³) по формуле Гаусса:
// знаковая сумма тетраэдров от начала координат на каждой триангулированной
// грани оболочки (ENG-GEO-0013). Требуется согласованная наружная ориентация
// граней; результат положителен для корректного тела.
func Volume(solid *Solid) (float64, error) {
	return NewTessellationCache().volume(solid)
}

// VolumeCached — вариант Volume, использующий разделяемый кеш триангуляций,
// чтобы каждая грань триангулировалась один раз за вызов конвейера (EM-06).
func VolumeCached(solid *Solid, cache *TessellationCache) (float64, error) {
	return cache.volume(solid)
}

func (c *TessellationCache) volume(solid *Solid) (float64, error) {
	shells := solid.Shells()
	if len(shells) == 0 {
		return 0, fmt.Errorf("geometry: solid has no shells")
	}
	var vol float64
	for _, shell := range shells {
		for _, face := range shell.Faces() {
			t := c.FaceOf(face)
			if !t.Closed {
				return 0, fmt.Errorf("geometry: open face contour")
			}
			if t.WireErr != nil {
				return 0, t.WireErr
			}
			if t.TrisErr != nil {
				return 0, t.TrisErr
			}
			for _, tr := range t.Tris {
				vol += tetraVolume(t.Points[tr[0]], t.Points[tr[1]], t.Points[tr[2]])
			}
		}
	}
	return vol, nil
}

// SurfaceArea возвращает площадь поверхности тела (мм²) как сумму площадей
// всех триангулированных граней оболочки (ENG-GEO-0013).
func SurfaceArea(solid *Solid) (float64, error) {
	return NewTessellationCache().surfaceArea(solid)
}

// SurfaceAreaCached — вариант SurfaceArea, использующий разделяемый кеш
// триангуляций (EM-06).
func SurfaceAreaCached(solid *Solid, cache *TessellationCache) (float64, error) {
	return cache.surfaceArea(solid)
}

func (c *TessellationCache) surfaceArea(solid *Solid) (float64, error) {
	shells := solid.Shells()
	if len(shells) == 0 {
		return 0, fmt.Errorf("geometry: solid has no shells")
	}
	var area float64
	for _, shell := range shells {
		for _, face := range shell.Faces() {
			t := c.FaceOf(face)
			if !t.Closed {
				return 0, fmt.Errorf("geometry: open face contour")
			}
			if t.WireErr != nil {
				return 0, t.WireErr
			}
			if t.TrisErr != nil {
				return 0, t.TrisErr
			}
			for _, tr := range t.Tris {
				area += triangleArea(t.Points[tr[0]], t.Points[tr[1]], t.Points[tr[2]])
			}
		}
	}
	return area, nil
}

// triangleArea возвращает площадь треугольника по длине векторного
// произведения двух его сторон.
func triangleArea(a, b, c Point3) float64 {
	return 0.5 * b.Sub(a).Cross(c.Sub(a)).Norm()
}

// tetraVolume возвращает знаковый объём тетраэдра (0, a, b, c):
// a·(b×c)/6. Для наружно ориентированной грани вклад положителен.
func tetraVolume(a, b, c Point3) float64 {
	va := Vector3(a)
	vb := Vector3(b)
	vc := Vector3(c)
	return va.Dot(vb.Cross(vc)) / 6
}

// Point2D — точка плоского контура детали в её СОБСТВЕННОЙ системе осей:
// вдоль X — длина, вдоль Y — ширина. Это те же оси, что и у SolidPlateBlank,
// поэтому длина контура[0].X совпадает с её возвратом.
type Point2D struct {
	X float64
	Y float64
}

// SolidProfile2D — ТОЧНЫЙ плоский контур детали, то есть многоугольник,
// который вырезается из листа (S-163).
//
// SolidPlateBlank отдаёт описывающий ПРЯМОУГОЛЬНИК: у боковой пластины
// косоура это 5024×300, хотя по чертежу деталь пилообразная и её описывающий
// прямоугольник заметно больше самой детали. Для раскроя металла по габариту
// этого достаточно (так и сделано), но документ раскроя должен совпадать с
// чертежом — иначе оператор режет не по тому документу, по которому считалось.
//
// Возвращается наружный контур КРЫШКИ (грани, перпендикулярной тонкой оси),
// без отверстий: отверстия — это дополнительные внутренние контуры, и для
// описания вырезаемого пятна нужен именно внешний. Начало координат — в
// минимальной точке контура, поэтому координаты неотрицательны и контур
// готов к отрисовке как есть.
//
// Второе значение — «контур найден». Для тел, у которых крышки не
// распознались (вырожденные, составные), возвращается false, и вызывающий
// обязан остаться на прямоугольнике: лучше описывающий прямоугольник, чем
// выдуманный многоугольник.
func SolidProfile2D(solid *Solid) ([]Point2D, bool) {
	outer, _, ok := SolidProfileWithHoles2D(solid)
	return outer, ok
}

// SolidProfileWithHoles2D — точный плоский контур детали и её ОТВЕРСТИЯ в
// одной и той же системе координат.
//
// Зачем отверстия рядом с контуром, а не отдельно. Чтобы вырезать деталь,
// нужны оба: лазер идёт по внешнему контуру и по каждому отверстию внутри.
// Раньше наружу отдавался только внешний контур, и отверстия в документе
// реза не появлялись вовсе — число их знала только счётка в смете
// (Part.HoleCount), но оператор на станке их не видел.
//
// Оба набора приводятся ОДНИМ поворотом и одним сдвигом, что и внешний
// контур: отверстие, вытащенное в другую систему, оказалось бы по другому
// краю детали, и это выглядело бы как ошибка раскроя.
func SolidProfileWithHoles2D(solid *Solid) ([]Point2D, [][]Point2D, bool) {
	f, ok := NewProfileFrame(solid)
	if !ok {
		return nil, nil, false
	}
	if len(f.outer) < 3 {
		return nil, nil, false
	}

	// Проецирование точки контура в оси профиля. Один и тот же сдвиг для
	// внешнего контура и для отверстий — иначе они разъедутся.
	project := func(pts []Point3) []Point2D {
		uniq := dedupePoints(pts, 1e-6)
		out := make([]Point2D, 0, len(uniq))
		for _, p := range uniq {
			p0, p1 := coord(p, f.axes[0]), coord(p, f.axes[1])
			out = append(out, Point2D{X: p0*f.ux + p1*f.uy, Y: -p0*f.uy + p1*f.ux})
		}
		return out
	}

	rawOuter := project(f.outer)
	minX, minY := math.Inf(1), math.Inf(1)
	for _, p := range rawOuter {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
	}
	shift := func(pts []Point2D) []Point2D {
		for i := range pts {
			pts[i].X -= minX
			pts[i].Y -= minY
		}
		return pts
	}
	outer := normalizeContour(shift(rawOuter))
	if len(outer) < 3 {
		return nil, nil, false
	}

	holes := make([][]Point2D, 0, 4)
	for _, w := range f.inner {
		pts, ok := contourPoints(w)
		if !ok || len(pts) < 3 {
			continue
		}
		h := shift(project(pts))
		if len(h) >= 3 {
			holes = append(holes, h)
		}
	}
	return outer, holes, true
}

// Профильная рамка тела (ProfileFrame) — тонкая ось, две оси в плоскости и
// поворот базиса вдоль самого длинного ребра контура.
//
// Зачем она отдельная. Профиль детали (SolidProfile2D) и её положение в
// сборке (SolidPlacement) обязаны быть в ОДНОЙ системе координат: если у них
// разные тонкая ось или разный поворот, чертёж детали напечатается в одном
// месте, а на раскрое и на сборке она встанет в другом — и это будет
// выглядеть как ошибка раскроя, а не как ошибка печати чертежа. Рамка одна,
// вычисляется один раз, обе функции её читают.
//
// Тело должно быть ПЛОСКИМ (призматичным): у него есть тонкая ось, вдоль
// неё — крышки, и контур крышки описывает вырезаемое пятно. У составных тел
// тонкой оси нет, и рамка не строится.
type ProfileFrame struct {
	// thin — индекс тонкой оси (0=X, 1=Y, 2=Z): вдоль неё лежит толщина.
	thin int
	// axes — индексы двух оставшихся осей, в порядке возрастания.
	axes [2]int
	// ux, uy — компоненты поворота базиса в плоскости (axes[0], axes[1]).
	ux, uy float64
	// outer — контур первой найденной крышки в мировых координатах.
	outer []Point3
	// inner — контуры отверстий на той же крышке, что и outer.
	//
	// Берутся ИМЕННО с этой крышки: у другой крышки контуры лежат в
	// параллельной плоскости на расстоянии толщины, и в двухмерный профиль
	// детали они попали бы по краю в зеркальном положении.
	inner []*Wire
	// Origin — мировая точка, которой соответствует начало профиля (0, 0).
	Origin Point3
	// U, V — единичные мировые векторы вдоль осей профиля.
	U, V Vector3
	// N — единичная мировая нормаль к плоскости детали (тонкая ось).
	N Vector3
}

// NewProfileFrame строит профильную рамку тела. Второе значение — «рамка
// построена».
func NewProfileFrame(solid *Solid) (ProfileFrame, bool) {
	if solid == nil {
		return ProfileFrame{}, false
	}
	bb := SolidBoundingBox(solid)
	ext := [3]float64{bb.Max.X - bb.Min.X, bb.Max.Y - bb.Min.Y, bb.Max.Z - bb.Min.Z}
	thin := 0
	for i := 1; i < 3; i++ {
		if ext[i] < ext[thin] {
			thin = i
		}
	}
	lo, hi := [3]float64{bb.Min.X, bb.Min.Y, bb.Min.Z}[thin],
		[3]float64{bb.Max.X, bb.Max.Y, bb.Max.Z}[thin]
	if hi-lo <= Precision {
		return ProfileFrame{}, false
	}

	// Наружный контур первой найденной крышки: крышек две, и контуры у них
	// одинаковые, поэтому достаточно одной.
	//
	// Запоминаем, с какой именно крышки взят контур (capAt): начало профиля
	// обязано лежать на ТОЙ ЖЕ плоскости. Иначе профиль с нижней крышки
	// отобразился бы в мир на толщину детали выше, чем он на самом деле, —
	// сдвиг 4 мм у пятки и 8 мм у косоура. Для чертежа это мелочь, для
	// монтажа — ошибка на миллиметр, которая не имеет права быть молчаливой.
	var outer []Point3
	var inner []*Wire
	capAt := lo
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			pts, ok := contourPoints(face.Outer())
			if !ok || len(pts) < 3 {
				continue
			}
			switch {
			case onPlane(pts, thin, lo):
				outer, capAt = pts, lo
				inner = face.Inner()
			case onPlane(pts, thin, hi):
				outer, capAt = pts, hi
				inner = face.Inner()
			default:
				continue
			}
			break
		}
		if len(outer) > 0 {
			break
		}
	}
	if len(outer) < 3 {
		return ProfileFrame{}, false
	}

	// Оси плоскости детали — две оставшиеся, и базис вдоль самого длинного
	// ребра контура: так длина совпадает с SolidPlateBlank.
	var axes [2]int
	k := 0
	for i := 0; i < 3; i++ {
		if i != thin {
			axes[k] = i
			k++
		}
	}
	bx, by, best := 1.0, 0.0, 0.0
	for i := range outer {
		a, c := outer[i], outer[(i+1)%len(outer)]
		dx := coord(c, axes[0]) - coord(a, axes[0])
		dy := coord(c, axes[1]) - coord(a, axes[1])
		if d := math.Hypot(dx, dy); d > best {
			best, bx, by = d, dx, dy
		}
	}
	if best <= 0 {
		return ProfileFrame{}, false
	}
	ux, uy := bx/best, by/best

	// Мировые единичные векторы осей профиля. U смотрит вдоль самого
	// длинного ребра контура, V поперёк, N — вдоль тонкой оси.
	e0, e1, e2 := axisVec(axes[0]), axisVec(axes[1]), axisVec(thin)
	f := ProfileFrame{
		thin:  thin,
		axes:  axes,
		ux:    ux,
		uy:    uy,
		outer: outer,
		inner: inner,
		U: Vector3{
			X: ux*e0.X + uy*e1.X,
			Y: ux*e0.Y + uy*e1.Y,
			Z: ux*e0.Z + uy*e1.Z,
		},
		V: Vector3{
			X: -uy*e0.X + ux*e1.X,
			Y: -uy*e0.Y + ux*e1.Y,
			Z: -uy*e0.Z + ux*e1.Z,
		},
		N: e2,
	}
	// Начало профиля (0, 0) — мировая точка с минимальными координатами по U
	// и V, лежащая в плоскости ТОЙ ЖЕ крышки, с которой взят контур (capAt).
	minU, minV := math.Inf(1), math.Inf(1)
	for _, p := range outer {
		minU = math.Min(minU, p.X*f.U.X+p.Y*f.U.Y+p.Z*f.U.Z)
		minV = math.Min(minV, p.X*f.V.X+p.Y*f.V.Y+p.Z*f.V.Z)
	}
	f.Origin = NewPoint3(
		minU*f.U.X+minV*f.V.X+capAt*e2.X,
		minU*f.U.Y+minV*f.V.Y+capAt*e2.Y,
		minU*f.U.Z+minV*f.V.Z+capAt*e2.Z,
	)
	return f, true
}

// axisVec — единичный мировой вектор мировой оси с индексом i.
func axisVec(i int) Vector3 {
	switch i {
	case 0:
		return Vector3{X: 1}
	case 1:
		return Vector3{Y: 1}
	default:
		return Vector3{Z: 1}
	}
}

// normalizeContour приводит контур к каноническому виду: начало в лексикографически
// минимальной вершине, обход против часовой стрелки.
//
// Без этого две ОДИНАКОВЫЕ детали дают разные последовательности точек —
// просто потому, что контур начинается с разной вершины. При сравнении контуров
// (например, при группировке позиций раскроя) это выглядит как разные детали,
// и раскрой дробится на лишние позиции. Площадь и габарит от поворота не
// меняются — меняется только запись, то есть результат становится
// воспроизводимым.
func normalizeContour(p []Point2D) []Point2D {
	if len(p) < 3 {
		return p
	}
	// Ориентация: делаем обход против часовой стрелки (положительная площадь).
	var area float64
	for i := range p {
		j := (i + 1) % len(p)
		area += p[i].X*p[j].Y - p[j].X*p[i].Y
	}
	if area < 0 {
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
		}
	}
	// Начало — минимальная вершина.
	start := 0
	for i := 1; i < len(p); i++ {
		if p[i].X < p[start].X || (p[i].X == p[start].X && p[i].Y < p[start].Y) {
			start = i
		}
	}
	if start == 0 {
		return p
	}
	rot := make([]Point2D, 0, len(p))
	rot = append(rot, p[start:]...)
	rot = append(rot, p[:start]...)
	return rot
}

// ProfileAreaMM — площадь многоугольника по правилу шнуровки. Нужна, чтобы
// отличать настоящий контур от описывающего прямоугольника: площадь контура
// заведомо меньше площади прямоугольника на габарит, и это лучший признак
// того, что контур извлечён, а не подставлен.
func ProfileAreaMM(p []Point2D) float64 {
	if len(p) < 3 {
		return 0
	}
	var a float64
	for i := range p {
		j := (i + 1) % len(p)
		a += p[i].X*p[j].Y - p[j].X*p[i].Y
	}
	return math.Abs(a) / 2
}
