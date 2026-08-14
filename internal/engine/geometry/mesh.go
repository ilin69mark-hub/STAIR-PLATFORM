package geometry

import (
	"fmt"

	kerngeo "stairplatform/internal/geometry"
)

// ToPreviewMesh строит полигональную сетку для отображения модели
// (ENG-GEO-0008): каждая грань B-Rep модели триангулируется. Mesh
// является производной величиной и никогда не является источником истины
// геометрии; источник — параметрическая модель.
func ToPreviewMesh(model *kerngeo.Compound) (*kerngeo.Mesh, error) {
	return ToPreviewMeshCached(model, kerngeo.NewTessellationCache())
}

// ToPreviewMeshCached — вариант ToPreviewMesh, разделяющий кеш триангуляций
// с валидацией и измерениями (EM-06, B2.2): каждая грань триангулируется
// один раз за вызов Generate.
func ToPreviewMeshCached(model *kerngeo.Compound, tess *kerngeo.TessellationCache) (*kerngeo.Mesh, error) {
	if model == nil {
		return nil, fmt.Errorf("geometry: model is required")
	}
	mesh := &kerngeo.Mesh{}
	for _, solid := range model.Solids() {
		verts, tris, err := meshSolid(solid, tess)
		if err != nil {
			return nil, err
		}
		base := len(mesh.Vertices)
		mesh.Vertices = append(mesh.Vertices, verts...)
		for _, tr := range tris {
			if err := mesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
				return nil, err
			}
		}
	}
	return mesh, nil
}

// meshSolid собирает вершины и треугольники (локальные индексы) одной грани
// тела Solid с разделяемым кешем триангуляций. Локальные индексы
// позволяют собирать mesh из параллельных результатов (result-slot, EM-06).
func meshSolid(solid *kerngeo.Solid, tess *kerngeo.TessellationCache) ([]kerngeo.Point3, [][3]int, error) {
	var verts []kerngeo.Point3
	var tris [][3]int
	for _, shell := range solid.Shells() {
		for _, face := range shell.Faces() {
			t := tess.Face(face.Outer())
			if t.WireErr != nil {
				return nil, nil, fmt.Errorf("geometry: face wire: %w", t.WireErr)
			}
			if t.TrisErr != nil {
				return nil, nil, fmt.Errorf("geometry: face triangulation: %w", t.TrisErr)
			}
			base := len(verts)
			verts = append(verts, t.Points...)
			for _, tr := range t.Tris {
				tris = append(tris, [3]int{base + tr[0], base + tr[1], base + tr[2]})
			}
		}
	}
	return verts, tris, nil
}
