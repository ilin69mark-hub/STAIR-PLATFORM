package geometry

import (
	"fmt"
	"math"
)

// Arc — дуга окружности в трёхмерном пространстве (ENG-GEO-0020).
// Дуга задана центром, радиусом, нормалью плоскости и углами.
// Все углы в радианах (ADR-0008).
type Arc struct {
	Center Point3
	Radius float64
	Normal Vector3
	Start  float64 // радианы
	End    float64 // радианы
}

// NewArc создаёт дугу окружности. Радиус должен быть положительным,
// нормаль — ненулевой.
func NewArc(center Point3, radius float64, normal Vector3, start, end float64) (*Arc, error) {
	if radius <= Precision {
		return nil, fmt.Errorf("geometry: arc radius must be positive, got %v", radius)
	}
	n, ok := normal.Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: arc normal must be non-zero")
	}
	if start == end {
		return nil, fmt.Errorf("geometry: arc start and end angles must differ")
	}
	return &Arc{
		Center: center,
		Radius: radius,
		Normal: n,
		Start:  start,
		End:    end,
	}, nil
}

// PointAt возвращает точку на дуге при параметре t ∈ [0, 1].
func (a *Arc) PointAt(t float64) Point3 {
	angle := a.Start + t*(a.End-a.Start)
	u, v := a.basisVectors()
	return Point3{
		X: a.Center.X + a.Radius*(u.X*math.Cos(angle)+v.X*math.Sin(angle)),
		Y: a.Center.Y + a.Radius*(u.Y*math.Cos(angle)+v.Y*math.Sin(angle)),
		Z: a.Center.Z + a.Radius*(u.Z*math.Cos(angle)+v.Z*math.Sin(angle)),
	}
}

// Length возвращает длину дуги (мм).
func (a *Arc) Length() float64 {
	angleSpan := math.Abs(a.End - a.Start)
	return a.Radius * angleSpan
}

// Tessellate аппроксимирует дугу полигоном из n сегментов (n ≥ 3).
func (a *Arc) Tessellate(n int) []Point3 {
	if n < 3 {
		n = 3
	}
	points := make([]Point3, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		points[i] = a.PointAt(t)
	}
	return points
}

// basisVectors вычисляет два ортонормированных вектора в плоскости дуги.
func (a *Arc) basisVectors() (Vector3, Vector3) {
	var candidate Vector3
	if math.Abs(a.Normal.X) < 0.9 {
		candidate = Vector3{X: 1, Y: 0, Z: 0}
	} else {
		candidate = Vector3{X: 0, Y: 1, Z: 0}
	}
	u, _ := a.Normal.Cross(candidate).Normalized()
	v, _ := a.Normal.Cross(u).Normalized()
	return u, v
}

// Bezier — кубическая кривая Безье (ENG-GEO-0021).
// Задана четырьмя контрольными точками: P0, P1, P2, P3.
type Bezier struct {
	P0, P1, P2, P3 Point3
}

// PointAt возвращает точку на кривой при параметре t ∈ [0, 1].
func (b *Bezier) PointAt(t float64) Point3 {
	u := 1 - t
	uu := u * u
	uuu := uu * u
	tt := t * t
	ttt := tt * t
	return Point3{
		X: uuu*b.P0.X + 3*uu*t*b.P1.X + 3*u*tt*b.P2.X + ttt*b.P3.X,
		Y: uuu*b.P0.Y + 3*uu*t*b.P1.Y + 3*u*tt*b.P2.Y + ttt*b.P3.Y,
		Z: uuu*b.P0.Z + 3*uu*t*b.P1.Z + 3*u*tt*b.P2.Z + ttt*b.P3.Z,
	}
}

// Tessellate аппроксимирует кривую Безье полигоном из n сегментов.
func (b *Bezier) Tessellate(n int) []Point3 {
	if n < 1 {
		n = 1
	}
	points := make([]Point3, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		points[i] = b.PointAt(t)
	}
	return points
}

// ApproximateLength приближённо вычисляет длину кривой (сумма длин сегментов).
func (b *Bezier) ApproximateLength(segments int) float64 {
	points := b.Tessellate(segments)
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += points[i-1].Distance(points[i])
	}
	return total
}

// Fillet — скругление ребра (ENG-GEO-0022).
type Fillet struct {
	Edge   [2]Point3
	Radius float64
	Normal Vector3
}

// NewFillet создаёт скругление. Радиус должен быть положительным.
func NewFillet(start, end Point3, radius float64, normal Vector3) (*Fillet, error) {
	if radius <= Precision {
		return nil, fmt.Errorf("geometry: fillet radius must be positive, got %v", radius)
	}
	n, ok := normal.Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: fillet normal must be non-zero")
	}
	return &Fillet{
		Edge:   [2]Point3{start, end},
		Radius: radius,
		Normal: n,
	}, nil
}

// Tessellate аппроксимирует скругление дугой из n сегментов.
func (f *Fillet) Tessellate(n int) []Point3 {
	if n < 3 {
		n = 3
	}
	start := f.Edge[0]
	end := f.Edge[1]

	mid := Point3{
		X: (start.X + end.X) / 2,
		Y: (start.Y + end.Y) / 2,
		Z: (start.Z + end.Z) / 2,
	}

	edgeRadius := mid.Distance(start)

	center := Point3{
		X: mid.X + f.Normal.X*edgeRadius,
		Y: mid.Y + f.Normal.Y*edgeRadius,
		Z: mid.Z + f.Normal.Z*edgeRadius,
	}

	arc := &Arc{
		Center: center,
		Radius: edgeRadius,
		Normal: f.Normal,
		Start:  0,
		End:    math.Pi,
	}
	return arc.Tessellate(n)
}

// Chamfer — фаска на ребре (ENG-GEO-0023).
type Chamfer struct {
	Edge   [2]Point3
	Width  float64
	Normal Vector3
}

// NewChamfer создаёт фаску. Ширина должна быть положительной.
func NewChamfer(start, end Point3, width float64, normal Vector3) (*Chamfer, error) {
	if width <= Precision {
		return nil, fmt.Errorf("geometry: chamfer width must be positive, got %v", width)
	}
	n, ok := normal.Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: chamfer normal must be non-zero")
	}
	return &Chamfer{
		Edge:   [2]Point3{start, end},
		Width:  width,
		Normal: n,
	}, nil
}

// Tessellate аппроксимирует фаску четырьмя точками.
func (c *Chamfer) Tessellate() []Point3 {
	start := c.Edge[0]
	end := c.Edge[1]

	offset := c.Normal.Scale(c.Width)

	return []Point3{
		start,
		end,
		end.Add(offset),
		start.Add(offset),
	}
}

// NGon — n-угольник в плоскости (ENG-GEO-0024).
type NGon struct {
	Vertices []Point3
	Normal   Vector3
}

// NewNGon создаёт n-угольник по вершинам. Вершин должно быть ≥ 3.
func NewNGon(vertices []Point3, normal Vector3) (*NGon, error) {
	if len(vertices) < 3 {
		return nil, fmt.Errorf("geometry: ngon must have at least 3 vertices, got %d", len(vertices))
	}
	n, ok := normal.Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: ngon normal must be non-zero")
	}
	return &NGon{
		Vertices: vertices,
		Normal:   n,
	}, nil
}

// RegularNGon создаёт правильный n-угольник заданного радиуса.
func RegularNGon(n int, center Point3, radius float64, normal Vector3) (*NGon, error) {
	if n < 3 {
		return nil, fmt.Errorf("geometry: regular ngon must have at least 3 sides, got %d", n)
	}
	if radius <= Precision {
		return nil, fmt.Errorf("geometry: regular ngon radius must be positive")
	}
	norm, ok := normal.Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: regular ngon normal must be non-zero")
	}

	u, v := basisVectors(norm)
	vertices := make([]Point3, n)
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		vertices[i] = Point3{
			X: center.X + radius*(u.X*math.Cos(angle)+v.X*math.Sin(angle)),
			Y: center.Y + radius*(u.Y*math.Cos(angle)+v.Y*math.Sin(angle)),
			Z: center.Z + radius*(u.Z*math.Cos(angle)+v.Z*math.Sin(angle)),
		}
	}
	return &NGon{Vertices: vertices, Normal: norm}, nil
}

// Area возвращает площадь n-угольника (формула Шнайрера).
func (n *NGon) Area() float64 {
	if len(n.Vertices) < 3 {
		return 0
	}
	area := 0.0
	for i := 0; i < len(n.Vertices); i++ {
		j := (i + 1) % len(n.Vertices)
		area += n.Vertices[i].X*n.Vertices[j].Y - n.Vertices[j].X*n.Vertices[i].Y
	}
	return math.Abs(area) / 2
}

// ToSolid превращает n-угольник в тонкое твёрдое тело.
func (n *NGon) ToSolid(thickness float64) (*Solid, error) {
	if thickness <= Precision {
		return nil, fmt.Errorf("geometry: ngon thickness must be positive")
	}
	return Extrude(n.Vertices, n.Normal, thickness)
}

// basisVectors вычисляет два ортонормированных вектора в плоскости.
func basisVectors(normal Vector3) (Vector3, Vector3) {
	var candidate Vector3
	if math.Abs(normal.X) < 0.9 {
		candidate = Vector3{X: 1, Y: 0, Z: 0}
	} else {
		candidate = Vector3{X: 0, Y: 1, Z: 0}
	}
	u, _ := normal.Cross(candidate).Normalized()
	v, _ := normal.Cross(u).Normalized()
	return u, v
}
