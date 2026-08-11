package geometry

// Topology описывает структуру геометрической модели без учёта размеров
// (ENG-GEO-0004). Иерархия: Vertex → Edge → Wire → Face → Shell → Solid → Compound.
// Топология неизменяема в рамках одной ревизии: типы не имеют методов-мутаторов;
// геометрические размеры не являются частью топологии.

// Vertex — вершина (позиция в пространстве).
type Vertex struct {
	point Point3
}

// NewVertex создаёт вершину.
func NewVertex(p Point3) *Vertex { return &Vertex{point: p} }

// Point возвращает позицию вершины.
func (v *Vertex) Point() Point3 { return v.point }

// Edge — ребро между двумя вершинами.
type Edge struct {
	v1, v2 *Vertex
}

// NewEdge создаёт ребро между вершинами.
func NewEdge(v1, v2 *Vertex) *Edge { return &Edge{v1: v1, v2: v2} }

// Endpoints возвращает вершины ребра.
func (e *Edge) Endpoints() (*Vertex, *Vertex) { return e.v1, e.v2 }

// IsDegenerate возвращает true, если обе вершины совпадают.
func (e *Edge) IsDegenerate() bool { return e.v1 == e.v2 }

// Wire — последовательность соединённых рёбер (замкнутая или открытая).
type Wire struct {
	edges []*Edge
}

// NewWire создаёт провод из рёбер.
func NewWire(edges ...*Edge) *Wire { return &Wire{edges: edges} }

// Edges возвращает рёбра провода.
func (w *Wire) Edges() []*Edge { return w.edges }

// IsClosed возвращает true, если провод замкнут (конец последнего ребра
// совпадает с началом первого).
func (w *Wire) IsClosed() bool {
	if len(w.edges) < 3 {
		return false
	}
	first, _ := w.edges[0].Endpoints()
	_, last := w.edges[len(w.edges)-1].Endpoints()
	return first == last
}

// Face — грань, ограниченная внешним контуром и опциональными отверстиями.
type Face struct {
	outer *Wire
	inner []*Wire
}

// NewFace создаёт грань по внешнему контуру и отверстиям.
func NewFace(outer *Wire, inner ...*Wire) *Face {
	return &Face{outer: outer, inner: inner}
}

// Outer возвращает внешний контур грани.
func (f *Face) Outer() *Wire { return f.outer }

// Inner возвращает отверстия грани.
func (f *Face) Inner() []*Wire { return f.inner }

// Shell — набор граней (замкнутая оболочка твёрдого тела).
type Shell struct {
	faces []*Face
}

// NewShell создаёт оболочку из граней.
func NewShell(faces ...*Face) *Shell { return &Shell{faces: faces} }

// Faces возвращает грани оболочки.
func (s *Shell) Faces() []*Face { return s.faces }

// Solid — замкнутое твёрдое тело, состоящее из оболочек.
type Solid struct {
	shells []*Shell
}

// NewSolid создаёт твёрдое тело из оболочек.
func NewSolid(shells ...*Shell) *Solid { return &Solid{shells: shells} }

// Shells возвращает оболочки твёрдого тела.
func (s *Solid) Shells() []*Shell { return s.shells }

// Compound — композит нескольких твёрдых тел.
type Compound struct {
	solids []*Solid
}

// NewCompound создаёт композит из твёрдых тел.
func NewCompound(solids ...*Solid) *Compound { return &Compound{solids: solids} }

// Solids возвращает твёрдые тела композита.
func (c *Compound) Solids() []*Solid { return c.solids }

// TopoKind — классификация топологического объекта (для метаданных ревизии).
type TopoKind string

const (
	TopoVertex   TopoKind = "vertex"
	TopoEdge     TopoKind = "edge"
	TopoWire     TopoKind = "wire"
	TopoFace     TopoKind = "face"
	TopoShell    TopoKind = "shell"
	TopoSolid    TopoKind = "solid"
	TopoCompound TopoKind = "compound"
)

// Kind возвращает классификацию топологического типа.
func Kind(any any) TopoKind {
	switch any.(type) {
	case *Vertex:
		return TopoVertex
	case *Edge:
		return TopoEdge
	case *Wire:
		return TopoWire
	case *Face:
		return TopoFace
	case *Shell:
		return TopoShell
	case *Solid:
		return TopoSolid
	case *Compound:
		return TopoCompound
	default:
		return ""
	}
}
