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
	verts := make([]*Vertex, len(points))
	for i, p := range points {
		verts[i] = NewVertex(p)
	}
	edges := make([]*Edge, len(points))
	for i := 0; i < len(points); i++ {
		edges[i] = NewEdge(verts[i], verts[(i+1)%len(points)])
	}
	return NewFace(NewWire(edges...))
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
