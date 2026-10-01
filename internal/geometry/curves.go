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
//
// Было: дуга строилась в плоскости с нормалью f.Normal, то есть ПЕРПЕНДИКУЛЯРНО
// ребру, и радиусом бралась половина длины ребра — заданный f.Radius
// игнорировался. Для ребра (0,0,0)→(100,0,0) с нормалью (0,0,1) результатом
// была полуокружность радиусом 50 мм в плоскости x=50, не касающаяся ребра
// концами. Теперь дуга лежит в плоскости, содержащей И ребро, И нормаль, и
// имеет заданный радиус: центр — середина ребра, смещённая на радиус по
// нормали, дуга идёт от середины ребра до её отражения за ребром.
func (f *Fillet) Tessellate(n int) []Point3 {
	if n < 3 {
		n = 3
	}
	start := f.Edge[0]
	end := f.Edge[1]

	edgeLen := start.Distance(end)
	if edgeLen <= Precision {
		// Вырожденное ребро: отдаём сам ребро, скруглять нечего.
		return []Point3{start, end}
	}

	// Плоскость дуги содержит ребро и нормаль. Её базис: −Normal (от центра к
	// ребру) и направление самого ребра. При параллельных векторах (нормаль
	// вдоль ребра) скруглять нечего — отдаём ребро.
	dir := Vector3{X: end.X - start.X, Y: end.Y - start.Y, Z: end.Z - start.Z}
	edgeDir, ok := dir.Normalized()
	if !ok || edgeDir.Cross(f.Normal).Norm() <= Precision {
		return []Point3{start, end}
	}

	// Полуокружность радиусом R не помещается на ребро короче 2R: дуга ушла бы
	// за его концы. Отдаём ребро — вызывающий код должен задать меньший радиус.
	if f.Radius > edgeLen/2+Precision {
		return []Point3{start, end}
	}

	mid := Point3{
		X: (start.X + end.X) / 2,
		Y: (start.Y + end.Y) / 2,
		Z: (start.Z + end.Z) / 2,
	}
	center := mid.Add(f.Normal.Scale(f.Radius))

	// Дуга считается напрямую, а не через Arc: у Arc свой выбор базиса внутри
	// плоскости, поэтому порядок точек зависел бы от того, какая ось попалась
	// «кандидатом». Порядок точек в профиле определяет ориентацию граней тела,
	// то есть детерминизм сетки (ADR-0003). Здесь базис задан явно:
	// e1 = −Normal (от центра к ребру), e2 = направление ребра.
	e1 := f.Normal.Scale(-1)
	points := make([]Point3, n+1)
	for i := 0; i <= n; i++ {
		theta := math.Pi * float64(i) / float64(n)
		points[i] = center.
			Add(e1.Scale(f.Radius * math.Cos(theta))).
			Add(edgeDir.Scale(f.Radius * math.Sin(theta)))
	}
	return points
}

// RoundCorner скругляет УГОЛ плоского профиля дугой заданного радиуса и
// возвращает точки дуги БЕЗ концов (концы — точки касания — остаются на
// месте в профиле).
//
// Это то, что нужно для скругления носка проступи: угол между верхней
// гранью и передней гранью заменяется дугой, и проступь после экструзии
// получает круглый нос. Существующий Fillet для этого не годится — он
// скругляет РЕБРО (полуокружность поперёк кромки), а не угол профиля.
//
// Геометрия: u и v — единичные лучи из угла к соседним вершинам, α — угол
// между ними, b — их биссектриса. Центр дуги лежит на биссектрисе на
// расстоянии r/sin(α/2) от угла, точки касания — на расстоянии
// r/tan(α/2) от угла вдоль каждого луча.
//
// Радиус ограничен тем, что точка касания должна уместиться в оба ребра, иначе
// профиль вывернется. При превышении — ошибка, а не молчаливый результат.
func RoundCorner(prev, corner, next Point3, radius float64, segments int) ([]Point3, error) {
	if radius <= Precision {
		return nil, fmt.Errorf("geometry: round corner radius must be positive, got %v", radius)
	}
	if segments < 1 {
		segments = 1
	}

	toPrev := prev.Sub(corner)
	toNext := next.Sub(corner)
	prevLen := math.Sqrt(toPrev.X*toPrev.X + toPrev.Y*toPrev.Y + toPrev.Z*toPrev.Z)
	nextLen := math.Sqrt(toNext.X*toNext.X + toNext.Y*toNext.Y + toNext.Z*toNext.Z)
	if prevLen <= Precision || nextLen <= Precision {
		return nil, fmt.Errorf("geometry: round corner needs two non-degenerate edges")
	}

	u := toPrev.Scale(1 / prevLen) // единичный луч к предыдущей вершине
	v := toNext.Scale(1 / nextLen) // единичный луч к следующей вершине

	cosAlpha := u.Dot(v)
	if cosAlpha <= -1+Precision {
		return nil, fmt.Errorf("geometry: round corner needs an angle (edges are opposite)")
	}
	if cosAlpha >= 1-Precision {
		return nil, fmt.Errorf("geometry: round corner needs an angle (edges are collinear)")
	}
	alpha := math.Acos(cosAlpha) // угол при вершине, 0..π
	half := alpha / 2
	if half <= Precision || math.Pi-half <= Precision {
		return nil, fmt.Errorf("geometry: round corner angle is degenerate")
	}

	// Расстояние от угла до точки касания и радиус ограничения по ребру.
	tangent := radius / math.Tan(half)
	if tangent > prevLen+Precision {
		return nil, fmt.Errorf("geometry: round corner radius %v does not fit edge of length %v", radius, prevLen)
	}
	if tangent > nextLen+Precision {
		return nil, fmt.Errorf("geometry: round corner radius %v does not fit edge of length %v", radius, nextLen)
	}

	// Центр дуги на биссектрисе угла.
	bisector, ok := u.Add(v).Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: round corner needs non-collinear edges")
	}
	center := corner.Add(bisector.Scale(radius / math.Sin(half)))

	// Базис на плоскости дуги: e1 — от центра к первой точке касания,
	// e2 — поворот на 90° в сторону следующей точки касания.
	t1 := corner.Add(u.Scale(tangent))
	r1, ok := t1.Sub(center).Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: round corner is degenerate")
	}
	r2, ok := next.Sub(center).Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: round corner is degenerate")
	}
	// Перпендикуляр к e1 в плоскости (e1, r2): так, чтобы дуга шла к t2.
	var e2 Vector3
	cross := r1.Cross(r2)
	if cross.Norm() <= Precision {
		return nil, fmt.Errorf("geometry: round corner needs non-collinear edges")
	}
	e2, ok = r1.Cross(cross).Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: round corner is degenerate")
	}
	if e2.Dot(r2) < 0 {
		e2 = e2.Scale(-1)
	}

	// Дуга от t1 (θ=0) до t2: угол поворота = π − α.
	arcAngle := math.Pi - alpha
	points := make([]Point3, 0, segments-1)
	for i := 1; i < segments; i++ {
		theta := arcAngle * float64(i) / float64(segments)
		points = append(points, center.
			Add(r1.Scale(radius*math.Cos(theta))).
			Add(e2.Scale(radius*math.Sin(theta))))
	}
	return points, nil
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

// Area возвращает площадь n-угольника (формула Шнайрера) В ЕГО ПЛОСКОСТИ.
//
// Было: площадь считалась в XY, то есть для профиля, лежащего в другой
// плоскости (вертикальная грань, наклонная косоура), давала произвольное
// число. Теперь площадь считается в базисе плоскости n.Normal — это та же
// формула, но в повёрнутых координатах.
func (n *NGon) Area() float64 {
	if len(n.Vertices) < 3 {
		return 0
	}
	u, v := basisVectors(n.Normal)
	// Проецируем вершины в базис (u, v) — ортонормированный, площадь сохраняется.
	area := 0.0
	var px, py float64
	for i := range n.Vertices {
		d := n.Vertices[i].Sub(Point3{}) // смещение от нуля не влияет на площадь
		cx := d.X*u.X + d.Y*u.Y + d.Z*u.Z
		cy := d.X*v.X + d.Y*v.Y + d.Z*v.Z
		if i > 0 {
			area += px*cy - cx*py
		}
		px, py = cx, cy
	}
	// Замыкаем контур на последнюю вершину.
	first := n.Vertices[0]
	fd := first.Sub(Point3{})
	fx := fd.X*u.X + fd.Y*u.Y + fd.Z*u.Z
	fy := fd.X*v.X + fd.Y*v.Y + fd.Z*v.Z
	area += px*fy - fx*py
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
