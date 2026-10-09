package geometry

import (
	"fmt"
	"math"
)

// planarityTolerance — допустимое отклонение точки от плоскости профиля
// при экструзии (мм, ADR-0008).
const planarityTolerance = Precision * 100

// Extrude строит призму (B-Rep Solid) из плоского полигона профиля,
// вытягивая его вдоль направления dir на положительное расстояние dist
// (ENG-GEO-0007). Требования: профиль из ≥3 неколлинеарных точек в одной
// плоскости; направление не должно лежать в плоскости профиля (иначе тело
// вырождено); dist > 0. Грани оболочки ориентированы наружу: крышка в
// плоскости профиля имеет нормаль, противоположную направлению вытягивания,
// крышка на удалённом конце — по направлению. Результат детерминирован:
// порядок граней и вершин соответствует порядку точек профиля; вся
// геометрия вычисляется заново из входных параметров (BC-002).
func Extrude(profile []Point3, dir Vector3, dist float64) (*Solid, error) {
	if len(profile) < 3 {
		return nil, fmt.Errorf("geometry: profile must have at least 3 points, got %d", len(profile))
	}
	if dist <= Precision {
		return nil, fmt.Errorf("geometry: extrusion distance must be positive, got %v", dist)
	}
	normal, ok := polygonNormal(profile)
	if !ok {
		return nil, fmt.Errorf("geometry: profile points are collinear")
	}
	origin := profile[0]
	for _, p := range profile {
		if math.Abs(p.Sub(origin).Dot(normal)) > planarityTolerance {
			return nil, fmt.Errorf("geometry: profile is not planar")
		}
	}
	dirUnit, ok := dir.Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: extrusion direction must be non-zero")
	}
	// направление обязано иметь ненулевую компоненту вдоль нормали,
	// иначе вытягивание остаётся в плоскости профиля (тело вырождено).
	if math.Abs(dirUnit.Dot(normal)) <= Precision {
		return nil, fmt.Errorf("geometry: extrusion direction lies in the profile plane")
	}

	// Профиль приводим к ориентации, согласованной с направлением
	// вытягивания (CCW вокруг +dir): боковые грани при этом наружу,
	// независимо от исходного обхода профиля.
	prof := orientedProfile(profile, dirUnit)

	shift := dir.Scale(dist)
	top := make([]Point3, len(prof))
	for i, p := range prof {
		top[i] = p.Add(shift)
	}

	faces := make([]*Face, 0, len(prof)+2)
	// крышки ориентируем наружу: нижняя против направления вытягивания.
	faces = append(faces, polygonFace(orientedProfile(prof, dirUnit.Scale(-1))))
	faces = append(faces, polygonFace(orientedProfile(top, dirUnit)))
	for i := 0; i < len(prof); i++ {
		j := (i + 1) % len(prof)
		faces = append(faces, polygonFace([]Point3{prof[i], prof[j], top[j], top[i]}))
	}
	return NewSolid(NewShell(faces...)), nil
}

// orientedProfile возвращает копию точек профиля в порядке, при котором
// нормаль полигона сонаправлена outward. Если текущая ориентация профиля
// противоположна outward, порядок точек разворачивается.
func orientedProfile(points []Point3, outward Vector3) []Point3 {
	normal, ok := polygonNormal(points)
	if ok && normal.Dot(outward) < 0 {
		reversed := make([]Point3, len(points))
		for i := range points {
			reversed[i] = points[len(points)-1-i]
		}
		return reversed
	}
	return points
}

// polygonFace создаёт грань с замкнутым внешним контуром по точкам.
func polygonFace(points []Point3) *Face {
	return NewFace(ringWire(points))
}

// ringWire строит замкнутый провод по точкам кольца. Вершины и рёбра
// выделяются СЛАЙБАМИ: по одному new на вершину и на ребро грань детали с
// десятками точек (контур овального отверстия) давала сотни выделений, и на
// металлокаркасе это было самым горячим местом построения модели.
func ringWire(points []Point3) *Wire {
	n := len(points)
	if n == 0 {
		return NewWire()
	}
	verts := make([]Vertex, n)
	edgeSlab := make([]Edge, n)
	edges := make([]*Edge, n)
	for i, p := range points {
		verts[i] = Vertex{point: p}
		edgeSlab[i] = Edge{v1: &verts[i], v2: &verts[(i+1)%n]}
		edges[i] = &edgeSlab[i]
	}
	return &Wire{edges: edges}
}

// polygonNormal возвращает единичную нормаль плоскости полигона.
// ok=false, если точки коллинеарны или полигон вырожден.
func polygonNormal(points []Point3) (Vector3, bool) {
	origin := points[0]
	for i := 1; i < len(points)-1; i++ {
		n := points[i].Sub(origin).Cross(points[i+1].Sub(origin))
		if n.Norm() > Precision {
			return n.Normalized()
		}
	}
	return Vector3{}, false
}

// ExtrudeWithHoles строит призму из плоского профиля с отверстиями: вытягивает
// его вдоль направления dir на расстояние dist. Отверстия проходят насквозь —
// крышки получают внутренние контуры, а стенки отверстий становятся гранями.
//
// Требования те же, что у Extrude, плюс каждый контур отверстия должен лежать
// в плоскости профиля. Порядок граней: крышка снизу, крышка сверху, боковые
// грани профиля, затем стенки отверстий по контурам. Ориентации не важны для
// топологии, но Volume/SurfaceArea требуют наружной ориентации, поэтому
// крышки и стенки ориентируются явно.
func ExtrudeWithHoles(profile []Point3, holes [][]Point3, dir Vector3, dist float64) (*Solid, error) {
	if len(holes) == 0 {
		return Extrude(profile, dir, dist)
	}
	if len(profile) < 3 {
		return nil, fmt.Errorf("geometry: profile must have at least 3 points, got %d", len(profile))
	}
	if dist <= Precision {
		return nil, fmt.Errorf("geometry: extrusion distance must be positive, got %v", dist)
	}
	normal, ok := polygonNormal(profile)
	if !ok {
		return nil, fmt.Errorf("geometry: profile points are collinear")
	}
	dirUnit, ok := dir.Normalized()
	if !ok {
		return nil, fmt.Errorf("geometry: extrusion direction must be non-zero")
	}
	if math.Abs(dirUnit.Dot(normal)) <= Precision {
		return nil, fmt.Errorf("geometry: extrusion direction lies in the profile plane")
	}
	shift := dir.Scale(dist)

	// Профиль приводим к обходу CCW вокруг +dir — боковые грани тогда смотрят
	// наружу, независимо от исходного порядка точек.
	prof := orientedProfile(profile, dirUnit)
	top := make([]Point3, len(prof))
	for i, p := range prof {
		top[i] = p.Add(shift)
	}

	innerB := make([][]Point3, len(holes)) // отверстия на нижней крышке
	innerT := make([][]Point3, len(holes)) // отверстия на верхней крышке
	for i, h := range holes {
		if len(h) < 3 {
			return nil, fmt.Errorf("geometry: hole %d must have at least 3 points, got %d", i, len(h))
		}
		hn, ok := polygonNormal(h)
		if !ok {
			return nil, fmt.Errorf("geometry: hole %d is degenerate", i)
		}
		if d := math.Abs(hn.Dot(normal)); d < 1-1e-6 {
			return nil, fmt.Errorf("geometry: hole %d is not coplanar with the profile (dot %.6f)", i, d)
		}
		// Крышки ориентированы противоположно друг другу, поэтому и обход
		// отверстия на них — противоположный. Внутренний контур ВЕРХНЕЙ
		// крышки лежит в плоскости верхней крышки, то есть сдвинут на shift:
		// иначе оба внутренних контура совпали бы и ребро получало три
		// грани вместо двух (проверка manifold).
		hs := make([]Point3, len(h))
		for j, p := range h {
			hs[j] = p.Add(shift)
		}
		innerB[i] = orientedProfile(h, normal.Scale(-1))
		innerT[i] = orientedProfile(hs, normal)
	}

	faces := make([]*Face, 0, 2+len(prof)+4*len(holes))
	faces = append(faces,
		// нижняя крышка смотрит против направления вытягивания
		polygonFaceWithHoles(orientedProfile(prof, dirUnit.Scale(-1)), innerB),
		polygonFaceWithHoles(top, innerT))
	for i := 0; i < len(prof); i++ {
		j := (i + 1) % len(prof)
		faces = append(faces, polygonFace([]Point3{prof[i], prof[j], top[j], top[i]}))
	}
	for _, h := range holes {
		// Ориентация стенок отверстия — ОДИН раз на весь контур, по
		// САМОМУ ДЛИННОМУ ребру: нормаль (b−a)×shift должна смотреть внутрь
		// отверстия. Покомпонентный признак по каждому ребру не годится —
		// короткое ребро, параллельное оси вытягивания, даёт численный шум и
		// разворот на одном ребре (в объём утекает ошибка).
		wall := orientedProfile(h, normal)
		flipWalls := holeWallsNeedFlip(wall, shift)
		for j := 0; j < len(wall); j++ {
			k := (j + 1) % len(wall)
			a, b := wall[j], wall[k]
			if flipWalls {
				faces = append(faces, polygonFace([]Point3{
					a, a.Add(shift), b.Add(shift), b,
				}))
			} else {
				faces = append(faces, polygonFace([]Point3{
					a, b, b.Add(shift), a.Add(shift),
				}))
			}
		}
	}
	return NewSolid(NewShell(faces...)), nil
}

// holeWallsNeedFlip — нужно ли развернуть стенки отверстия. Нормаль грани
// (b−a)×shift должна смотреть ВНУТРЬ отверстия (наружу от материала).
// Ориентир — самое длинное ребро контура: его середина и центр масс отверстия
// дают направление внутрь.
func holeWallsNeedFlip(wall []Point3, shift Vector3) bool {
	if len(wall) < 3 {
		return false
	}
	bestLen, ai, bi := 0.0, 0, 0
	for i := range wall {
		j := (i + 1) % len(wall)
		if d := wall[i].Distance(wall[j]); d > bestLen {
			bestLen, ai, bi = d, i, j
		}
	}
	if bestLen <= Precision {
		return false
	}
	a, b := wall[ai], wall[bi]
	n := Vector3(b.Sub(a)).Cross(shift)
	mid := Point3(a.Add(b.Sub(a).Scale(0.5)))
	inward := Vector3(centroid(wall).Sub(mid))
	// нормаль уже смотрит внутрь — разворот не нужен
	return n.Dot(inward) <= 0
}

// polygonFaceWithHoles создаёт грань с внешним контуром и отверстиями.
func polygonFaceWithHoles(outer []Point3, inner [][]Point3) *Face {
	f := polygonFace(outer)
	if len(inner) == 0 {
		return f
	}
	wires := make([]*Wire, 0, len(inner))
	for _, pts := range inner {
		if len(pts) < 3 {
			continue
		}
		wires = append(wires, ringWire(pts))
	}
	return NewFace(f.Outer(), wires...)
}

// centroid — центр масс контура (среднее по вершинам).
func centroid(pts []Point3) Point3 {
	var sum Vector3
	for _, p := range pts {
		sum = sum.Add(Vector3(p))
	}
	return Point3(sum.Scale(1 / float64(len(pts))))
}
