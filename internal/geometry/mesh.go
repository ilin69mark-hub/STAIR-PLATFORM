package geometry

import "fmt"

// Mesh — полигональная сетка для отображения (ENG-GEO-0008).
// Mesh является производной величиной: он никогда не является источником
// истины геометрии и всегда пересчитывается из параметрической модели.
type Mesh struct {
	Vertices  []Point3  `json:"Vertices"`
	Triangles [][3]int `json:"Triangles"`
}

// AddTriangle добавляет треугольник из индексов вершин; индекс вне
// диапазона запрещён.
func (m *Mesh) AddTriangle(a, b, c int) error {
	if a < 0 || b < 0 || c < 0 || a >= len(m.Vertices) || b >= len(m.Vertices) || c >= len(m.Vertices) {
		return fmt.Errorf("geometry: triangle index out of range")
	}
	m.Triangles = append(m.Triangles, [3]int{a, b, c})
	return nil
}
