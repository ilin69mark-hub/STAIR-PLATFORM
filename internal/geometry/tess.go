package geometry

// FaceTess — результат триангуляции и контура одной грани.
type FaceTess struct {
	Points  []Point3 // упорядоченные точки контура (без замыкающего дубликата)
	Closed  bool     // контур замкнут
	Tris    [][3]int // индексы треугольников в Points
	WireErr error    // ошибка получения контура
	TrisErr error    // ошибка триангуляции
}

// wireTess — мемоизированная триангуляция одного провода (грани).
type wireTess struct {
	tess FaceTess
	done bool
}

// faceTess — мемоизированная триангуляция грани вместе с отверстиями.
type faceTess struct {
	tess FaceTess
	done bool
}

// TessellationCache кеширует триангуляцию граней по идентичности провода
// (*Wire). Используется для того, чтобы каждая грань модели триангулировалась
// ровно один раз за вызов Generate, а не 4 раза (валидация по объёму, Volume,
// SurfaceArea, preview mesh) — EM-06 Performance. Кеш пригоден только в
// пределах одного вызова, где грани неизменяемы и образуют стабильное
// множество идентичных объектов.
type TessellationCache struct {
	m  map[*Wire]wireTess
	mf map[*Face]faceTess
}

// NewTessellationCache создаёт пустой кеш триангуляций.
func NewTessellationCache() *TessellationCache {
	return &TessellationCache{
		m:  make(map[*Wire]wireTess),
		mf: make(map[*Face]faceTess),
	}
}

// Face возвращает контур и триангуляцию грани, вычисляя их один раз.
// Повторные обращения к тому же проводу возвращают кешированный результат.
func (c *TessellationCache) Face(w *Wire) FaceTess {
	if c == nil {
		return tessFaceNoCache(w)
	}
	if e, ok := c.m[w]; ok {
		return e.tess
	}
	t := tessFaceNoCache(w)
	c.m[w] = wireTess{tess: t, done: true}
	return t
}

// FaceOf возвращает контур и триангуляцию грани ЦЕЛИКОМ — вместе с
// отверстиями. Грань без отверстий даёт тот же результат, что Face, поэтому
// вызывающий код (объём, площадь, меш) должен использовать именно её.
func (c *TessellationCache) FaceOf(f *Face) FaceTess {
	if f == nil {
		return FaceTess{WireErr: errFaceFewEdges}
	}
	if len(f.Inner()) == 0 {
		return c.Face(f.Outer())
	}
	if c == nil {
		return tessFaceWithHoles(f)
	}
	if e, ok := c.mf[f]; ok {
		return e.tess
	}
	t := tessFaceWithHoles(f)
	c.mf[f] = faceTess{tess: t, done: true}
	return t
}

// tessFaceWithHoles триангулирует грань с отверстиями: контур — внешний,
// точки и треугольники — результат TriangulateWithHoles (он возвращает свои
// точки: углы трапеций лежат на рёбрах, а не во входных вершинах).
func tessFaceWithHoles(f *Face) FaceTess {
	t := FaceTess{}
	outer, closed := contourPoints(f.Outer())
	t.Points = outer
	t.Closed = closed
	if len(outer) < 3 {
		t.WireErr = errFaceFewEdges
		return t
	}
	inners := make([][]Point3, 0, len(f.Inner()))
	for i, w := range f.Inner() {
		pts, ok := contourPoints(w)
		if !ok || len(pts) < 3 {
			t.WireErr = errFaceFewEdges
			_ = i
			return t
		}
		inners = append(inners, pts)
	}
	pts, tris, err := TriangulateWithHoles(outer, inners)
	t.Points = pts
	t.Tris = tris
	t.TrisErr = err
	return t
}

// tessFaceNoCache вычисляет контур и триангуляцию провода без кеширования.
func tessFaceNoCache(w *Wire) FaceTess {
	t := FaceTess{}
	if w == nil || len(w.Edges()) < 3 {
		t.WireErr = errFaceFewEdges
		return t
	}
	pts, closed := contourPoints(w)
	t.Points = pts
	t.Closed = closed
	tris, err := Triangulate(pts)
	t.Tris = tris
	t.TrisErr = err
	return t
}

var errFaceFewEdges = &wireErr{}

type wireErr struct{}

func (e *wireErr) Error() string { return "geometry: face has fewer than 3 edges" }
