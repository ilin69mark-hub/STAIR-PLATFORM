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

// Weld сливает вершины с ОДИНАКОВЫМИ координатами: если две вершины стоят
// в одной точке, остаётся одна, а все треугольники переезжают на её индекс.
//
// Зачем: грани тела тесселируются независимо, поэтому общая вершина
// попадает в меш по разу для каждой грани. Измерено на лестнице 2700 мм:
// 1116 вершин при 314 уникальных — 72 % дублей. В STL и DXF эти дубли
// уходят в файл дословно, раздувая его вчетверо по числу координат
// (S-175).
//
// Сливает ТОЛЬКО побитово равные координаты, без допуска и без сетки.
// Это не компромисс, а измеренный факт: поиск пар «в одной ячейке
// 1e-6 мм, но не равных побитово» даёт ноль. Разные грани получают одну и
// ту же точку из одной и той же вершины B-Rep, поэтому float64 совпадает
// бит в бит. Сетка с допуском дала бы ошибки на границах ячеек и риск
// слить разные точки — здесь этого нет.
//
// Текстуры: UV слить нельзя — они считаются на каждую грань по её нормали
// (triplanarUV), и у склеенной вершины останется UV только первой грани.
// Поэтому результат БЕЗ UV. Вызывать там, где UV не нужен (экспорт CAD).
// Меш для 3D-вьювера не сваривается: там UV важен.
//
// Возвращает новый меш и число слитых вершин.
func (m *Mesh) Weld() (*Mesh, int) {
	if m == nil || len(m.Vertices) == 0 {
		return m, 0
	}
	type pos [3]float64
	idx := make(map[pos]int, len(m.Vertices))
	out := &Mesh{
		Vertices:   make([]Point3, 0, len(m.Vertices)),
		Triangles:  make([][3]int, 0, len(m.Triangles)),
		PartRanges: m.PartRanges,
	}
	for _, v := range m.Vertices {
		k := pos{v.X, v.Y, v.Z}
		if _, ok := idx[k]; ok {
			continue
		}
		idx[k] = len(out.Vertices)
		out.Vertices = append(out.Vertices, v)
	}
	if len(out.Vertices) == len(m.Vertices) {
		return m, 0 // совпало всё — отдаём исходный меш, копия не нужна
	}
	for _, t := range m.Triangles {
		a, aok := idx[pos{m.Vertices[t[0]].X, m.Vertices[t[0]].Y, m.Vertices[t[0]].Z}]
		b, bok := idx[pos{m.Vertices[t[1]].X, m.Vertices[t[1]].Y, m.Vertices[t[1]].Z}]
		c, cok := idx[pos{m.Vertices[t[2]].X, m.Vertices[t[2]].Y, m.Vertices[t[2]].Z}]
		if !aok || !bok || !cok {
			// Недостижимо при корректном индексе; лучше пустым треугольником,
			// чем обращением за границу в худшем виде.
			continue
		}
		out.Triangles = append(out.Triangles, [3]int{a, b, c})
	}
	return out, len(m.Vertices) - len(out.Vertices)
}

// Point2 — текстурная координата.
type Point2 struct {
	U float64 `json:"U"`
	V float64 `json:"V"`
}

// NewMesh создаёт меш с заранее выделенными срезами под nv вершин, nuv
// текстурных координат и ntr треугольников. Сборка меша модели знает итог
// заранее, поэтому рост срезов логарифмическими шагами не нужен.
func NewMesh(nv, nuv, ntr int) *Mesh {
	m := &Mesh{}
	if nv > 0 {
		m.Vertices = make([]Point3, 0, nv)
	}
	if nuv > 0 {
		m.UV = make([]Point2, 0, nuv)
	}
	if ntr > 0 {
		m.Triangles = make([][3]int, 0, ntr)
	}
	return m
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
