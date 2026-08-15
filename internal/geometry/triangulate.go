package geometry

import (
	"fmt"
	"math"
)

// triangulationTolerance — допуск 2D-операций триангуляции.
const triangulationTolerance = 1e-6

// Triangulate триангулирует простой (не обязательно выпуклый) плоский
// полигон методом отрезания ушей (ear clipping). Порядок точек задаёт
// контур полигона; ориентация значения не имеет — она нормализуется.
// Возвращает индексы треугольников в порядке обхода; ошибка при менее
// чем 3 точках, коллинеарных точках или неразрешимом контуре.
func Triangulate(points []Point3) ([][3]int, error) {
	n := len(points)
	if n < 3 {
		return nil, fmt.Errorf("geometry: triangulation requires at least 3 points, got %d", n)
	}
	normal, ok := polygonNormal(points)
	if !ok {
		return nil, fmt.Errorf("geometry: triangulation requires a polygon, points are collinear")
	}
	pts := project2D(points, normal)
	idx := make([]int, n)
	for i := range pts {
		idx[i] = i
	}
	// Алгоритм требует CCW-контур; если входной контур CW, разворачиваем
	// вместе с таблицей индексов, а после триангуляции восстанавливаем
	// исходную ориентацию треугольников (rev).
	rev := false
	if signedArea2(pts) < 0 {
		rev = true
		for l, r := 0, len(pts)-1; l < r; l, r = l+1, r-1 {
			pts[l], pts[r] = pts[r], pts[l]
			idx[l], idx[r] = idx[r], idx[l]
		}
	}

	prev := make([]int, n)
	next := make([]int, n)
	for i := 0; i < n; i++ {
		prev[i] = (i + n - 1) % n
		next[i] = (i + 1) % n
	}

	// Reflex-список (B2, EDR-0033): для CCW-контура вершина рефлексная,
	// если поворот в ней не левый. Теорема: если ухо (a,b,c) содержит
	// вершину, то содержит reflex-вершину — поэтому isEar проверяет только
	// reflex-вершины вместо всех (O(n²) вместо O(n³) в худшем случае).
	// После среза уха reflex-статус меняется только у двух соседей.
	removed := make([]bool, n)
	reflex := make([]bool, n)
	for i := 0; i < n; i++ {
		reflex[i] = isReflexVertex(pts, prev[i], i, next[i])
	}

	tris := make([][3]int, 0, n-2)
	remaining := n
	guard := 0
	i := 0
	for remaining > 3 {
		guard++
		// прерывание зацикливания на неразрешимом контуре.
		if guard > n*n {
			return nil, fmt.Errorf("geometry: polygon is not simple")
		}
		a, b, c := prev[i], i, next[i]
		if isEar(pts, removed, reflex, a, b, c) {
			tris = append(tris, [3]int{idx[a], idx[b], idx[c]})
			removed[b] = true
			next[a] = c
			prev[c] = a
			// Изменились только соседи срезанной вершины.
			reflex[a] = isReflexVertex(pts, prev[a], a, next[a])
			reflex[c] = isReflexVertex(pts, prev[c], c, next[c])
			remaining--
			i = a
			continue
		}
		i = next[i]
	}
	tris = append(tris, [3]int{idx[prev[i]], idx[i], idx[next[i]]})
	// Восстанавливаем исходную ориентацию: алгоритм всегда выдаёт
	// CCW-треугольники в 2D-проекции, а при CW-входе это противоположно
	// обходу входного контура.
	if rev {
		for i := range tris {
			tris[i] = [3]int{tris[i][0], tris[i][2], tris[i][1]}
		}
	}
	return tris, nil
}

// isEar проверяет, что вершина b является ухом: выпуклая и треугольник
// (a,b,c) не содержит других вершин полигона. Благодаря reflex-списку
// достаточно проверить reflex-вершины (B2, EDR-0033 §3.4).
func isEar(p [][2]float64, removed, reflex []bool, a, b, c int) bool {
	ab := [2]float64{p[b][0] - p[a][0], p[b][1] - p[a][1]}
	bc := [2]float64{p[c][0] - p[b][0], p[c][1] - p[b][1]}
	if cross2(ab, bc) <= triangulationTolerance {
		return false
	}
	for i := range p {
		if removed[i] || !reflex[i] || i == a || i == b || i == c {
			continue
		}
		if pointInTriangle(p[i], p[a], p[b], p[c]) {
			return false
		}
	}
	return true
}

// isReflexVertex проверяет, что вершина cur контура рефлексная (не левый
// поворот) в CCW-полигоне.
func isReflexVertex(p [][2]float64, prev, cur, next int) bool {
	ab := [2]float64{p[cur][0] - p[prev][0], p[cur][1] - p[prev][1]}
	bc := [2]float64{p[next][0] - p[cur][0], p[next][1] - p[cur][1]}
	return cross2(ab, bc) <= triangulationTolerance
}

// pointInTriangle проверяет попадание точки в треугольник (включая
// границу) для контура против часовой стрелки.
func pointInTriangle(p, a, b, c [2]float64) bool {
	ab := [2]float64{b[0] - a[0], b[1] - a[1]}
	bc := [2]float64{c[0] - b[0], c[1] - b[1]}
	ca := [2]float64{a[0] - c[0], a[1] - c[1]}
	pa := [2]float64{p[0] - a[0], p[1] - a[1]}
	pb := [2]float64{p[0] - b[0], p[1] - b[1]}
	pc := [2]float64{p[0] - c[0], p[1] - c[1]}
	return cross2(ab, pa) >= -triangulationTolerance &&
		cross2(bc, pb) >= -triangulationTolerance &&
		cross2(ca, pc) >= -triangulationTolerance
}

// project2D проецирует точки на плоскость, отбрасывая координату оси
// с максимальной компонентой нормали (устойчивая проекция). Проекция
// праворукая: знак 2D-площади совпадает с ориентацией полигона в 3D
// (для оси Y плоскость XZ леворукая, поэтому Z разворачивается).
func project2D(points []Point3, normal Vector3) [][2]float64 {
	axis := 0
	if math.Abs(normal.Y) > math.Abs(normal.X) {
		axis = 1
	}
	if math.Abs(normal.Z) > math.Abs(normal.Y) && math.Abs(normal.Z) > math.Abs(normal.X) {
		axis = 2
	}
	pts := make([][2]float64, len(points))
	for i, p := range points {
		switch axis {
		case 0:
			pts[i] = [2]float64{p.Y, p.Z}
		case 1:
			pts[i] = [2]float64{p.X, -p.Z}
		default:
			pts[i] = [2]float64{p.X, p.Y}
		}
	}
	return pts
}

// signedArea2 возвращает удвоенную знаковую площадь полигона.
func signedArea2(p [][2]float64) float64 {
	var s float64
	for i := 0; i < len(p); i++ {
		j := (i + 1) % len(p)
		s += p[i][0]*p[j][1] - p[j][0]*p[i][1]
	}
	return s / 2
}

func cross2(a, b [2]float64) float64 { return a[0]*b[1] - a[1]*b[0] }
