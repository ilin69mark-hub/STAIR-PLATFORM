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
		for _, shell := range solid.Shells() {
			for _, face := range shell.Faces() {
				t := tess.Face(face.Outer())
				if t.WireErr != nil {
					return nil, fmt.Errorf("geometry: face wire: %w", t.WireErr)
				}
				if t.TrisErr != nil {
					return nil, fmt.Errorf("geometry: face triangulation: %w", t.TrisErr)
				}
				base := len(mesh.Vertices)
				mesh.Vertices = append(mesh.Vertices, t.Points...)
				for _, tr := range t.Tris {
					if err := mesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return mesh, nil
}
