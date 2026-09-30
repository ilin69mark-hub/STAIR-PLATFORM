package geometry

import "fmt"

// Mesh — полигональная сетка для отображения (ENG-GEO-0008).
// Mesh является производной величиной: он никогда не является источником
// истины геометрии и всегда пересчитывается из параметрической модели.
type Mesh struct {
	Vertices  []Point3 `json:"Vertices"`
	Triangles [][3]int `json:"Triangles"`
	// PartRanges — диапазоны треугольников по телам с их ролями
	// (этап 1 «студийный 3D»). Позволяет 3D-вьюверу красить ступени,
	// косоуры, площадку и перила разными материалами, не раздувая payload
	// (роль известна в Solid.Role(), терялась при сборке плоской сетки).
	// Аддитивно: старые потребители читают только Vertices/Triangles.
	PartRanges []PartRange `json:"PartRanges,omitempty"`
	// UV — текстурные координаты, по одной на вершину Vertices (длина
	// совпадает). Без них текстура не натягивается: все вершины получают
	// uv=(0,0) и семплят один пиксель, поэтому материал выглядит плоским
	// цветом, сколько бы карт ни грузилось.
	//
	// Координаты в МЕТРАХ, а не в 0..1: масштаб текстуры тогда задаётся
	// масштабом повтора в материале (materials.ts repeatFor), и один и тот же
	// рисунок одинаково ложится на проступь, косоур и поручень.
	//
	// Аддитивно: если поле пустое, вьювер считает, что UV нет, и рисует
	// процедурный материал — поведение прежнее.
	UV []Point2 `json:"UV,omitempty"`
}

// Point2 — текстурная координата.
type Point2 struct {
	U float64 `json:"U"`
	V float64 `json:"V"`
}

// SetUV задаёт текстурные координаты, проверяя длину по числу вершин.
func (m *Mesh) SetUV(uv []Point2) error {
	if len(uv) != len(m.Vertices) {
		return fmt.Errorf("geometry: uv count %d must match vertex count %d", len(uv), len(m.Vertices))
	}
	m.UV = uv
	return nil
}

// PartRange — диапазон [Start,End) треугольников одного тела с ролью
// (например "tread", "stringer", "landing", "railing_post").
type PartRange struct {
	Solid int    `json:"Solid"`
	Role  string `json:"Role"`
	Start int    `json:"Start"`
	End   int    `json:"End"`
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
