package geometry

import (
	"fmt"
	"math"
	"sort"
)

// TriangulateWithHoles триангулирует многоугольник с отверстиями методом
// разложения на вертикальные полосы.
//
// outer — внешний контур, inner — контуры отверстий. Ориентация контуров
// значения не имеет. Все контуры должны лежать в одной плоскости.
//
// ПОЧЕМУ НЕ МОСТИКИ: классический способ (мостик от отверстия + отрезание
// ушей) на контурах с совпадающими вершинами даёт «перевёрнутые» треугольники
// — отверстие заливается. Полосы такого класса ошибок не имеют.
//
// АЛГОРИТМ. Собираются все X вершин; между соседними X нарезаются полосы.
// Внутри полосы нет вершин, поэтому все пересекающие её рёбра прямолинейны, и
// граница области на вертикали — ступенчатая функция из отрезков. Обход
// снизу вверх даёт пары «вход/выход»: между первой и второй чётками —
// область тела, со второй по третью — отверстие, и так далее (правило
// чётности). Каждая пара ограничивает трапецию, которая и триангулируется
// двумя треугольниками.
//
// Возвращает СВОИ точки: углы трапеций лежат на рёбрах, а не в исходных
// вершинах, поэтому ссылаться на входные точки нельзя.
func TriangulateWithHoles(outer []Point3, inner [][]Point3) ([]Point3, [][3]int, error) {
	if len(outer) < 3 {
		return nil, nil, fmt.Errorf("geometry: triangulation requires at least 3 outer points, got %d", len(outer))
	}
	normal, ok := polygonNormal(outer)
	if !ok {
		return nil, nil, fmt.Errorf("geometry: triangulation requires a polygon, points are collinear")
	}
	rings := [][]Point3{outer}
	for i, hole := range inner {
		if len(hole) < 3 {
			return nil, nil, fmt.Errorf("geometry: hole %d requires at least 3 points, got %d", i, len(hole))
		}
		hn, ok := polygonNormal(hole)
		if !ok {
			return nil, nil, fmt.Errorf("geometry: hole %d is degenerate", i)
		}
		if d := math.Abs(hn.Dot(normal)); d < 1-1e-6 {
			return nil, nil, fmt.Errorf("geometry: hole %d is not coplanar with the outer contour (dot %.6f)", i, d)
		}
		rings = append(rings, hole)
	}

	// Работаем в 2D-проекции: обратное отображение в 3D в конце.
	flat := make([][2]float64, 0, 16)
	edges := make([][2]int, 0, 32) // индексы пар в flat
	for _, ring := range rings {
		p2 := project2D(ring, normal)
		base := len(flat)
		flat = append(flat, p2...)
		for i := 0; i < len(p2); i++ {
			edges = append(edges, [2]int{base + i, base + (i+1)%len(p2)})
		}
	}

	// Все X вершин — границы полос.
	xs := make([]float64, 0, len(flat))
	for _, p := range flat {
		xs = append(xs, p[0])
	}
	xs = uniqueSorted(xs)

	// Проекция отбрасывает одну координату; её значение берём из внешнего
	// контура — плоскость грани восстанавливается точно (для верхней крышки
	// это, например, Z = толщина, а не 0).
	base3 := droppedCoord(outer[0], normal)

	// Ёмкость с запасом: на каждую пару чёток в полосе — два треугольника
	// и три точки. Без предвыделения срез рос логарифмическими шагами.
	est := 6 * len(xs)
	pts := make([]Point3, 0, est)
	tris := make([][3]int, 0, 2*len(xs))
	addTri := func(a, b, c [2]float64) {
		i0 := len(pts)
		pts = append(pts,
			unproject2D(a, normal, base3),
			unproject2D(b, normal, base3),
			unproject2D(c, normal, base3))
		tris = append(tris, [3]int{i0, i0 + 1, i0 + 2})
	}

	// Ориентация: треугольники должны смотреть наружу грани. Наружу — та
	// сторона, куда смотрят грани оболочки; для плоской грани достаточно
	// знать нормаль контура: CCW в 2D-проекции соответствует +нормали
	// (project2D праворукая).
	flip := signedArea2(project2D(outer, normal)) < 0

	// Раскладка рёбер по полосам в CSR-виде. Без неё каждая полоса
	// просматривала ВСЕ рёбра, то есть O(ребра × полосы): на детали с
	// овалами (контур круга даёт десятки вершин) это было узким местом.
	csr := bandEdgeIndex(flat, edges, xs)

	var cs []bandEdge
	for i := 0; i+1 < len(xs); i++ {
		x0, x1 := xs[i], xs[i+1]
		if x1-x0 <= triangulationTolerance {
			continue
		}
		// Рёбра, перекрывающие всю полосу: внутри нет вершин, значит
		// отрезок прямолинеен и его концы лежат на границах полосы.
		cs = cs[:0]
		for _, ei := range csr.band(i) {
			a, b := flat[edges[ei][0]], flat[edges[ei][1]]
			da, db := b[0]-a[0], b[1]-a[1]
			cs = append(cs, bandEdge{y0: a[1] + (x0-a[0])*db/da, y1: a[1] + (x1-a[0])*db/da})
		}
		if len(cs) < 2 {
			continue
		}
		sortByMid(cs)
		// Чётность: внутри области между 1-й и 2-й чёткой, вне — между 2-й
		// и 3-й, и так далее.
		for k := 0; k+1 < len(cs); k += 2 {
			lo, hi := cs[k], cs[k+1]
			if math.Abs(lo.y0-hi.y0) <= triangulationTolerance &&
				math.Abs(lo.y1-hi.y1) <= triangulationTolerance {
				continue // вырожденная трапеция
			}
			bl := [2]float64{x0, lo.y0}
			br := [2]float64{x1, lo.y1}
			tr := [2]float64{x1, hi.y1}
			tl := [2]float64{x0, hi.y0}
			if flip {
				addTri(br, bl, tl)
				addTri(br, tl, tr)
			} else {
				addTri(bl, br, tr)
				addTri(bl, tr, tl)
			}
		}
	}
	if len(tris) == 0 {
		return nil, nil, fmt.Errorf("geometry: polygon with holes produced no triangles")
	}
	return pts, tris, nil
}

// droppedAxis — индекс координаты, которую отбрасывает project2D.
func droppedAxis(normal Vector3) int {
	axis := 0
	if math.Abs(normal.Y) > math.Abs(normal.X) {
		axis = 1
	}
	if math.Abs(normal.Z) > math.Abs(normal.Y) && math.Abs(normal.Z) > math.Abs(normal.X) {
		axis = 2
	}
	return axis
}

// droppedCoord — значение координаты, отброшенной проекцией.
func droppedCoord(p Point3, normal Vector3) float64 {
	switch droppedAxis(normal) {
	case 0:
		return p.X
	case 1:
		return p.Y
	default:
		return p.Z
	}
}

// unproject2D возвращает 2D-точки обратно в пространство, восстанавливая
// отброшенную координату из base — так треугольник остаётся в плоскости
// грани, а не схлопывается в Z = 0.
func unproject2D(p [2]float64, normal Vector3, base float64) Point3 {
	switch droppedAxis(normal) {
	case 0:
		return Point3{X: base, Y: p[0], Z: p[1]}
	case 1:
		return Point3{X: p[0], Y: base, Z: -p[1]}
	default:
		return Point3{X: p[0], Y: p[1], Z: base}
	}
}

func uniqueSorted(v []float64) []float64 {
	sort.Float64s(v)
	out := make([]float64, 0, len(v))
	for _, x := range v {
		if len(out) > 0 && math.Abs(x-out[len(out)-1]) <= triangulationTolerance {
			continue
		}
		out = append(out, x)
	}
	return out
}

// bandEdge — ребро, пересекающее полосу: значения Y на её границах.
type bandEdge struct {
	y0, y1 float64
}

// edgeBands — индексы рёбер, пересекающих полосу, в CSR-разметке:
// offsets[i]..offsets[i+1] — диапазон в at для полосы i.
type edgeBands struct {
	offsets []int
	items   []int32
}

// band возвращает индексы рёбер, перекрывающих полосу i.
func (e *edgeBands) band(i int) []int32 {
	return e.items[e.offsets[i]:e.offsets[i+1]]
}

// bandEdgeIndex раскладывает НЕВЕРТИКАЛЬНЫЕ рёбра по полосам, которые они
// перекрывают. Полосы, перекрываемые ребром, идут подряд: ребро покрывает
// диапазон [firstBand, lastBand], поэтому раскладка строится двумя проходами
// (подсчёт, затем заполнение) за линейное время.
func bandEdgeIndex(flat [][2]float64, edges [][2]int, xs []float64) edgeBands {
	bands := len(xs) - 1
	if bands <= 0 || len(edges) == 0 {
		return edgeBands{}
	}
	// lo/hi по полосам для каждого ребра; -1 — ребро не используется
	// (вертикальное или не перекрывает ни одной полосы).
	lo := make([]int, len(edges))
	hi := make([]int, len(edges))
	offsets := make([]int, bands+1)
	for ei, e := range edges {
		a, b := flat[e[0]], flat[e[1]]
		da := b[0] - a[0]
		if math.Abs(da) <= triangulationTolerance {
			// Вертикальное ребро лежит на границе полосы — в interior
			// полосы его нет, и оно не даёт пересечения.
			lo[ei], hi[ei] = -1, -1
			continue
		}
		xlo, xhi := math.Min(a[0], b[0]), math.Max(a[0], b[0])
		// Полоса i = [xs[i], xs[i+1]] перекрыта ребром, если
		// xs[i] >= xlo-tol и xs[i+1] <= xhi+tol; границы сдвинуты на
		// допуск, как и в исходной проверке перекрытия.
		first := sort.Search(len(xs), func(i int) bool { return xs[i] >= xlo-triangulationTolerance })
		// j — первый индекс с xs[j] > xhi+tol; полоса i подходит, если
		// xs[i+1] <= xhi+tol, то есть i+1 < j.
		last := sort.Search(len(xs), func(i int) bool { return xs[i] > xhi+triangulationTolerance }) - 2
		if first < 0 || last >= bands || first > last {
			lo[ei], hi[ei] = -1, -1
			continue
		}
		lo[ei], hi[ei] = first, last
		for i := first; i <= last; i++ {
			offsets[i+1]++
		}
	}
	for i := 0; i < bands; i++ {
		offsets[i+1] += offsets[i]
	}
	total := offsets[bands]
	at := make([]int32, total)
	fill := make([]int, bands)
	copy(fill, offsets[:bands])
	for ei := range edges {
		if lo[ei] < 0 {
			continue
		}
		for i := lo[ei]; i <= hi[ei]; i++ {
			at[fill[i]] = int32(ei)
			fill[i]++
		}
	}
	return edgeBands{offsets: offsets, items: at}
}

func sortByMid(cs []bandEdge) {
	// Сортировка вставками по значению Y в середине полосы.
	for i := 1; i < len(cs); i++ {
		c := cs[i]
		mid := (c.y0 + c.y1) / 2
		j := i - 1
		for j >= 0 && (cs[j].y0+cs[j].y1)/2 > mid {
			cs[j+1] = cs[j]
			j--
		}
		cs[j+1] = c
	}
}
