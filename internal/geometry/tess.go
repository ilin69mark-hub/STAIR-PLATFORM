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

// TessellationCache кеширует триангуляцию граней по идентичности провода
// (*Wire). Используется для того, чтобы каждая грань модели триангулировалась
// ровно один раз за вызов Generate, а не 4 раза (валидация по объёму, Volume,
// SurfaceArea, preview mesh) — EM-06 Performance. Кеш пригоден только в
// пределах одного вызова, где грани неизменяемы и образуют стабильное
// множество идентичных объектов.
type TessellationCache struct {
	m map[*Wire]wireTess
}

// NewTessellationCache создаёт пустой кеш триангуляций.
func NewTessellationCache() *TessellationCache {
	return &TessellationCache{m: make(map[*Wire]wireTess)}
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
