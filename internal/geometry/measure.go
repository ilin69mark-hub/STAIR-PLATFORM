package geometry

import (
	"fmt"
)

// BoundingBox возвращает BBox всей модели — минимальный охват всех вершин
// всех тел (ENG-GEO-0013). Детерминированно: порядок тел Compound.
func BoundingBox(model *Compound) BBox {
	if model == nil {
		return BBox{}
	}
	bbox := BBox{}
	first := true
	for _, solid := range model.Solids() {
		for _, v := range solidPoints(solid) {
			if first {
				bbox = NewBBox(v)
				first = false
				continue
			}
			b := NewBBox(v)
			bbox = bbox.Union(b)
		}
	}
	return bbox
}

// SolidCount возвращает число твёрдых тел в модели.
func SolidCount(model *Compound) int {
	if model == nil {
		return 0
	}
	return len(model.Solids())
}

// solidPoints собирает все вершины всех граней тела (с дубликатами,
// в порядке обхода). Только для измерений.
func solidPoints(solid *Solid) []Point3 {
	var pts []Point3
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			wire, _ := contourPoints(face.Outer())
			pts = append(pts, wire...)
		}
	}
	return pts
}

// Volume возвращает объём замкнутого твёрдого тела (мм³) по формуле Гаусса:
// знаковая сумма тетраэдров от начала координат на каждой триангулированной
// грани оболочки (ENG-GEO-0013). Требуется согласованная наружная ориентация
// граней; результат положителен для корректного тела.
func Volume(solid *Solid) (float64, error) {
	shells := solid.Shells()
	if len(shells) == 0 {
		return 0, fmt.Errorf("geometry: solid has no shells")
	}
	var vol float64
	for _, shell := range shells {
		for _, face := range shell.Faces() {
			wire, closed := contourPoints(face.Outer())
			if !closed {
				return 0, fmt.Errorf("geometry: open face contour")
			}
			tris, err := Triangulate(wire)
			if err != nil {
				return 0, err
			}
			for _, tr := range tris {
				vol += tetraVolume(wire[tr[0]], wire[tr[1]], wire[tr[2]])
			}
		}
	}
	return vol, nil
}

// SurfaceArea возвращает площадь поверхности тела (мм²) как сумму площадей
// всех триангулированных граней оболочки (ENG-GEO-0013).
func SurfaceArea(solid *Solid) (float64, error) {
	shells := solid.Shells()
	if len(shells) == 0 {
		return 0, fmt.Errorf("geometry: solid has no shells")
	}
	var area float64
	for _, shell := range shells {
		for _, face := range shell.Faces() {
			wire, closed := contourPoints(face.Outer())
			if !closed {
				return 0, fmt.Errorf("geometry: open face contour")
			}
			tris, err := Triangulate(wire)
			if err != nil {
				return 0, err
			}
			for _, tr := range tris {
				area += triangleArea(wire[tr[0]], wire[tr[1]], wire[tr[2]])
			}
		}
	}
	return area, nil
}

// triangleArea возвращает площадь треугольника по длине векторного
// произведения двух его сторон.
func triangleArea(a, b, c Point3) float64 {
	return 0.5 * b.Sub(a).Cross(c.Sub(a)).Norm()
}

// tetraVolume возвращает знаковый объём тетраэдра (0, a, b, c):
// a·(b×c)/6. Для наружно ориентированной грани вклад положителен.
func tetraVolume(a, b, c Point3) float64 {
	va := Vector3(a)
	vb := Vector3(b)
	vc := Vector3(c)
	return va.Dot(vb.Cross(vc)) / 6
}
