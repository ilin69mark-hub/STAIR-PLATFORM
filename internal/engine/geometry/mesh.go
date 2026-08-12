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
	if model == nil {
		return nil, fmt.Errorf("geometry: model is required")
	}
	mesh := &kerngeo.Mesh{}
	for _, solid := range model.Solids() {
		for _, shell := range solid.Shells() {
			for _, face := range shell.Faces() {
				pts, err := wirePoints(face.Outer())
				if err != nil {
					return nil, fmt.Errorf("geometry: face wire: %w", err)
				}
				tris, err := kerngeo.Triangulate(pts)
				if err != nil {
					return nil, fmt.Errorf("geometry: face triangulation: %w", err)
				}
				base := len(mesh.Vertices)
				mesh.Vertices = append(mesh.Vertices, pts...)
				for _, tr := range tris {
					if err := mesh.AddTriangle(base+tr[0], base+tr[1], base+tr[2]); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return mesh, nil
}

// wirePoints собирает точки замкнутого контура грани в порядке рёбер.
func wirePoints(w *kerngeo.Wire) ([]kerngeo.Point3, error) {
	edges := w.Edges()
	if len(edges) < 3 {
		return nil, fmt.Errorf("wire must have at least 3 edges")
	}
	pts := make([]kerngeo.Point3, 0, len(edges))
	for _, e := range edges {
		v1, _ := e.Endpoints()
		pts = append(pts, v1.Point())
	}
	return pts, nil
}
