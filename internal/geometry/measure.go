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
