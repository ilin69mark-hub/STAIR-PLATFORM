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
// Role — опциональная семантическая метка тела (например, роль детали),
// не является частью топологии и не влияет на геометрию.
// holes — число отверстий, просверленных в теле: тоже не топология, а
// монтажная характеристика (MNT-0001, крепёж). Ноль — отверстий нет.
type Solid struct {
	shells []*Shell
	role   string
	holes  int
}

// NewSolid создаёт твёрдое тело из оболочек.
func NewSolid(shells ...*Shell) *Solid { return &Solid{shells: shells} }

// NewSolidRole создаёт твёрдое тело из оболочек с семантической меткой.
func NewSolidRole(role string, shells ...*Shell) *Solid {
	return &Solid{shells: shells, role: role}
}

// WithRole возвращает копию тела с заданной меткой (тело неизменяемо).
func (s *Solid) WithRole(role string) *Solid {
	return &Solid{shells: s.shells, role: role, holes: s.holes}
}

// Role возвращает семантическую метку тела (пустая строка — не задана).
func (s *Solid) Role() string { return s.role }

// WithHoles возвращает копию тела с заданным числом отверстий (тело
// неизменяемо).
//
// Зачем. Отверстия уже есть в геометрии пяток обоймы выреза — их режет
// лазер вместе с контуром. Но производство до сих пор не знало их числа,
// поэтому крепёж в смету не попадал: пришлось бы считать отверстия заново
// и рисковать разойтись с моделью. Теперь число отверстий едет вместе с
// телом, и смета получает ровно столько крепежа, сколько просверлено.
func (s *Solid) WithHoles(n int) *Solid {
	if n < 0 {
		n = 0
	}
	return &Solid{shells: s.shells, role: s.role, holes: n}
}

// Holes возвращает число отверстий в теле (0 — отверстий нет).
func (s *Solid) Holes() int { return s.holes }

// Shells возвращает оболочки твёрдого тела.
func (s *Solid) Shells() []*Shell { return s.shells }

// Compound — композит нескольких твёрдых тел.
type Compound struct {
	solids []*Solid
}

// NewCompound создаёт композит из твёрдых тел.
func NewCompound(solids ...*Solid) *Compound { return &Compound{solids: solids} }

// TransformSolid возвращает новое твёрдое тело, полученное применением
// преобразования t ко всем вершинам тела (ENG-GEO-0101). Топология и
// семантическая метка роли сохраняются. Результат детерминирован.
func TransformSolid(s *Solid, t Transform) *Solid {
	if s == nil {
		return nil
	}
	// Тождественное преобразование: тело возвращается как есть. Тела
	// неизменяемы (см. Topology), поэтому копия не нужна. Правосторонний
	// L-марш строит нижний марш без поворота, и раньше он глубоко копировался
	// ради ничего — на 96 тел металлокаркаса это были миллионы выделений.
	if t.IsIdentity() {
		return s
	}
	shells := make([]*Shell, len(s.shells))
	for i, sh := range s.shells {
		faces := make([]*Face, len(sh.faces))
		for j, f := range sh.faces {
			outer := transformWire(f.outer, t)
			inner := make([]*Wire, len(f.inner))
			for k, w := range f.inner {
				inner[k] = transformWire(w, t)
			}
			faces[j] = NewFace(outer, inner...)
		}
		shells[i] = NewShell(faces...)
	}
	return &Solid{shells: shells, role: s.role, holes: s.holes}
}

// transformWire возвращает провод с преобразованными вершинами.
//
// Вершины и рёбра выделяются СЛАЙБАМИ: на каждое ребро раньше приходилось
// три отдельных new (две вершины и ребро), и на металлокаркасе с сотнями деталей
// это было вторым по величине узким местом конвейера.
func transformWire(w *Wire, t Transform) *Wire {
	n := len(w.edges)
	if n == 0 {
		return NewWire()
	}
	verts := make([]Vertex, 2*n)
	edgeSlab := make([]Edge, n)
	edges := make([]*Edge, n)
	for i, e := range w.edges {
		v1, v2 := e.Endpoints()
		verts[2*i] = Vertex{point: t.Apply(v1.Point())}
		verts[2*i+1] = Vertex{point: t.Apply(v2.Point())}
		edgeSlab[i] = Edge{v1: &verts[2*i], v2: &verts[2*i+1]}
		edges[i] = &edgeSlab[i]
	}
	return &Wire{edges: edges}
}

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
