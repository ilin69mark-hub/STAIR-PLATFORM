package geometry

import "math"

// Детектор взаимопроникновения тел (S-171).
//
// ЗАЧЕМ. В модели лестницы тела проникают друг в друга СОЗНАТЕЛЬНО: проступь
// входит в паз косоура, это соединение, а не дефект. Но из-за этого модель
// перестаёт быть двух-manifold-ной, и по геометрии нельзя понять, где именно
// происходит стык и насколько он глубок. Измерено на лестнице 2700 мм:
// 29 рёбер принадлежат более чем двум треугольникам — это и есть места
// стыков, но КАКИЕ — неизвестно.
//
// Этот пакет не решает, что со стыком делать (это задача булевых операций,
// см. ENG-GEO-0011). Он даёт карту: какие пары тел пересекаются, сколько
// точек тела A лежит внутри тела B и насколько перекрываются габариты.
// Без такой карты выбор библиотеки — предположение.
//
// Метод: точка внутри замкнутой треугольной оболочки определяется методом
// лучей (нечётное число пересечений = внутри). Для надёжности берётся
// несколько фиксированных направлений и голосуется: луч может попасть
// ровно в ребро или вершину. Направления ЗАФИКСИРОВАНЫ константами —
// результат детерминирован (ADR-0007).

// Penetration — измеренное взаимопроникновение двух тел.
type Penetration struct {
	SolidA, SolidB int     // индексы тел в порядке PartRanges
	RoleA, RoleB   string  // роли тел ("tread", "stringer", …)
	PointsInB      int     // сколько вершин тела A лежит внутри тела B
	PointsInA      int     // сколько вершин тела B лежит внутри тела A
	OverlapX       float64 // перекрытие габаритов, мм
	OverlapY       float64
	OverlapZ       float64
}

// Глубина — меньшая сторона перекрытия габаритов. Это НЕ глубина
// пересечения тел, а нижняя оценка: настоящая глубина требует булевых
// операций. Её достаточно, чтобы отличить «ступень на 1 мм зашла в косоур»
// от «косоур проглотил половину марша».
func (p Penetration) Depth() float64 {
	d := p.OverlapX
	if p.OverlapY < d {
		d = p.OverlapY
	}
	if p.OverlapZ < d {
		d = p.OverlapZ
	}
	return d
}

// rayDirs — фиксированные направления лучей. Растровые, взяты с
// взаимно простыми дробями: вероятность попасть в ребро кратно ниже, чем у
// «красивых» направлений вроде (1,1,1).
var rayDirs = [5][3]float64{
	{0.5773502691896258, 0.5773502691896258, 0.5773502691896258},
	{-0.8017837257372732, 0.5345224838248488, 0.2672612419124244},
	{0.2672612419124244, -0.8017837257372732, 0.5345224838248488},
	{0.5345224838248488, 0.2672612419124244, -0.8017837257372732},
	{-0.3713906763541037, 0.5570860145311556, 0.7427813527082075},
}

// rayHitsTriangle — пересекает ли луч отрезок orig+t·dir (t>0) с треугольником.
// Алгоритм Мёллера — Трамбора. Возвращает true при пересечении.
func rayHitsTriangle(orig Point3, dir Vector3, v0, v1, v2 Point3) bool {
	const eps = 1e-12
	edge1 := Vector3{v1.X - v0.X, v1.Y - v0.Y, v1.Z - v0.Z}
	edge2 := Vector3{v2.X - v0.X, v2.Y - v0.Y, v2.Z - v0.Z}
	pvec := Vector3{dir.Y*edge2.Z - dir.Z*edge2.Y, dir.Z*edge2.X - dir.X*edge2.Z, dir.X*edge2.Y - dir.Y*edge2.X}
	det := edge1.X*pvec.X + edge1.Y*pvec.Y + edge1.Z*pvec.Z
	if math.Abs(det) < eps {
		return false // луч параллелен плоскости треугольника
	}
	inv := 1 / det
	tvec := Vector3{orig.X - v0.X, orig.Y - v0.Y, orig.Z - v0.Z}
	u := (tvec.X*pvec.X + tvec.Y*pvec.Y + tvec.Z*pvec.Z) * inv
	if u < 0 || u > 1 {
		return false
	}
	qvec := Vector3{tvec.Y*edge1.Z - tvec.Z*edge1.Y, tvec.Z*edge1.X - tvec.X*edge1.Z, tvec.X*edge1.Y - tvec.Y*edge1.X}
	v := (dir.X*qvec.X + dir.Y*qvec.Y + dir.Z*qvec.Z) * inv
	if v < 0 || u+v > 1 {
		return false
	}
	t := (edge2.X*qvec.X + edge2.Y*qvec.Y + edge2.Z*qvec.Z) * inv
	return t > eps
}

// pointInsideTriangles — лежит ли точка внутри замкнутой оболочки из
// заданных треугольников. Голосует по rayDirs: точка считается внутри, если
// большинство направлений дали нечётное число пересечений.
func pointInsideTriangles(p Point3, verts []Point3, tris [][3]int) bool {
	inside := 0
	used := 0
	for _, d := range rayDirs {
		dir := Vector3{d[0], d[1], d[2]}
		hits := 0
		for _, t := range tris {
			if rayHitsTriangle(p, dir, verts[t[0]], verts[t[1]], verts[t[2]]) {
				hits++
			}
		}
		used++
		if hits%2 == 1 {
			inside++
		}
		// Досрочный выход, когда уверенность уже набрана: голосование
		// останавливается, как только перевес стал доказательным.
		if used >= 3 && inside*2 > used {
			return true
		}
	}
	return used > 0 && inside*2 > used
}

// solidView — треугольники и вершины одного тела внутри общей сетки.
type solidView struct {
	index int
	role  string
	tris  [][3]int
	verts []Point3 // копия вершин тела, индексы локальные
	bbMin Point3
	bbMax Point3
}

func (s *solidView) bounds() {
	s.bbMin = Point3{math.Inf(1), math.Inf(1), math.Inf(1)}
	s.bbMax = Point3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, v := range s.verts {
		s.bbMin = minP(s.bbMin, v)
		s.bbMax = maxP(s.bbMax, v)
	}
}

func minP(a, b Point3) Point3 {
	return Point3{math.Min(a.X, b.X), math.Min(a.Y, b.Y), math.Min(a.Z, b.Z)}
}

func maxP(a, b Point3) Point3 {
	return Point3{math.Max(a.X, b.X), math.Max(a.Y, b.Y), math.Max(a.Z, b.Z)}
}

// viewsFromPartRanges разбирает сетку на тела по PartRanges. Без
// PartRanges вернуть пустой результат: раскладку тогда делать не на чем.
func viewsFromPartRanges(m *Mesh) []solidView {
	if m == nil || len(m.PartRanges) == 0 {
		return nil
	}
	views := make([]solidView, 0, len(m.PartRanges))
	for _, pr := range m.PartRanges {
		if pr.Start < 0 || pr.End > len(m.Triangles) || pr.Start >= pr.End {
			continue
		}
		// Собираем вершины тела и перенумеровываем треугольники.
		local := make(map[int]int)
		v := solidView{index: pr.Solid, role: pr.Role}
		remap := func(idx int) int {
			if li, ok := local[idx]; ok {
				return li
			}
			li := len(v.verts)
			local[idx] = li
			v.verts = append(v.verts, m.Vertices[idx])
			return li
		}
		for _, t := range m.Triangles[pr.Start:pr.End] {
			v.tris = append(v.tris, [3]int{remap(t[0]), remap(t[1]), remap(t[2])})
		}
		if len(v.verts) == 0 {
			continue
		}
		v.bounds()
		views = append(views, v)
	}
	return views
}

// FindPenetrations возвращает все пары тел, которые проникают друг в друга,
// вместе с измеренной глубиной. Пары отсортированы по убыванию Depth, затем
// по ролям — результат детерминирован (ADR-0007).
//
// Тела без PartRanges не разбираются и пропускаются: вызывающий обязан
// передать сетку, собранную geometry engine.
func FindPenetrations(m *Mesh) []Penetration {
	views := viewsFromPartRanges(m)
	if len(views) < 2 {
		return nil
	}
	var out []Penetration
	for i := 0; i < len(views); i++ {
		for j := i + 1; j < len(views); j++ {
			a, b := &views[i], &views[j]
			ox := overlap1D(a.bbMin.X, a.bbMax.X, b.bbMin.X, b.bbMax.X)
			oy := overlap1D(a.bbMin.Y, a.bbMax.Y, b.bbMin.Y, b.bbMax.Y)
			oz := overlap1D(a.bbMin.Z, a.bbMax.Z, b.bbMin.Z, b.bbMax.Z)
			if ox <= 0 || oy <= 0 || oz <= 0 {
				continue // габариты не пересекаются — тела не могут пересекаться
			}
			p := Penetration{
				SolidA: a.index, SolidB: b.index,
				RoleA: a.role, RoleB: b.role,
				OverlapX: ox, OverlapY: oy, OverlapZ: oz,
			}
			p.PointsInB = countInside(a.verts, b.verts, b.tris)
			p.PointsInA = countInside(b.verts, a.verts, a.tris)
			if p.PointsInB == 0 && p.PointsInA == 0 {
				continue // габариты задевают, но ни одна вершина не внутри
			}
			out = append(out, p)
		}
	}
	sortPenetrations(out)
	return out
}

func overlap1D(aMin, aMax, bMin, bMax float64) float64 {
	lo := math.Max(aMin, bMin)
	hi := math.Min(aMax, bMax)
	return hi - lo
}

func countInside(pts, shellVerts []Point3, shellTris [][3]int) int {
	n := 0
	for _, p := range pts {
		if pointInsideTriangles(p, shellVerts, shellTris) {
			n++
		}
	}
	return n
}

func sortPenetrations(p []Penetration) {
	// Сортировка вставками: элементов десятки, полная сортировка избыточна.
	for i := 1; i < len(p); i++ {
		x := p[i]
		j := i - 1
		for j >= 0 && penetrationLess(x, p[j]) {
			p[j+1] = p[j]
			j--
		}
		p[j+1] = x
	}
}

func penetrationLess(a, b Penetration) bool {
	da, db := a.Depth(), b.Depth()
	if da != db {
		return da > db
	}
	if a.RoleA != b.RoleA {
		return a.RoleA < b.RoleA
	}
	if a.RoleB != b.RoleB {
		return a.RoleB < b.RoleB
	}
	if a.PointsInB != b.PointsInB {
		return a.PointsInB > b.PointsInB
	}
	return a.PointsInA > b.PointsInA
}
