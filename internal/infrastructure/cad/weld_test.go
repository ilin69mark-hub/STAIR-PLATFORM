package cad

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

// heavyMesh — меш с заведомо большим числом дублей: каждый треугольник
// приносит свою копию трёх вершин, как это и делает meshSolid, собирая
// грани независимо.
func heavyMesh() *kerngeo.Mesh {
	var m kerngeo.Mesh
	for _, t := range [][3]kerngeo.Point3{
		{{X: 0}, {X: 10}, {X: 10, Y: 10}},
		{{X: 10}, {X: 20}, {X: 20, Y: 10}},
		{{X: 10, Y: 10}, {X: 20, Y: 10}, {X: 20, Y: 20}},
		{{X: 0}, {X: 10, Y: 10}, {X: 10, Z: 10}},
	} {
		base := len(m.Vertices)
		m.Vertices = append(m.Vertices, t[:]...)
		m.Triangles = append(m.Triangles, [3]int{base, base + 1, base + 2})
	}
	return &m
}

// countCSVVertices — сколько строк-вершин в CSV-экспорте.
func countCSVVertices(t *testing.T, raw []byte) int {
	t.Helper()
	r := csv.NewReader(bytes.NewReader(raw))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv: %v", err)
	}
	n := 0
	for _, row := range rows {
		if len(row) > 0 && row[0] == "v" {
			n++
		}
	}
	return n
}

// TestWriteCNCWeldsVertices — CNC-форматы перечисляют вершины списком,
// поэтому WriteCNC обязан сваривать сетку перед записью (S-175). Иначе в
// файл уходят дубли одной точки по разу на каждую грань.
//
// Проверяем не «меньше, чем раньше» (WriteCNC сваривает, сравнивать не с
// чем), а напрямую: число строк-вершин в CSV должно равняться числу
// УНИКАЛЬНЫХ позиций, а не числу вершин на входе.
func TestWriteCNCWeldsVertices(t *testing.T) {
	m := heavyMesh()
	welded, n := m.Weld()
	if n == 0 {
		t.Fatalf("подготовленный меш должен содержать дубли, слито %d", n)
	}

	var raw bytes.Buffer
	if err := WriteCNC(&raw, m, CSV); err != nil {
		t.Fatalf("WriteCNC(CSV): %v", err)
	}
	got := countCSVVertices(t, raw.Bytes())
	if got != len(welded.Vertices) {
		t.Errorf("в CSV %d вершин, в сваренном меше %d — сварка не применена",
			got, len(welded.Vertices))
	}
	if got == len(m.Vertices) {
		t.Errorf("в CSV %d вершин — столько же, сколько на входе (%d), сварки нет",
			got, len(m.Vertices))
	}
	t.Logf("вершин на входе %d, слито %d, в файле %d", len(m.Vertices), n, got)
}

// TestWriteCNCAllFormatsAfterWeld — после сварки все форматы обязаны
// записываться без ошибок, а число треугольников сохраняться.
func TestWriteCNCAllFormatsAfterWeld(t *testing.T) {
	m := heavyMesh()
	for _, f := range []CNCFormat{STEP, IGES, OBJ, THREEMF, CSV, JSON, XML} {
		var buf bytes.Buffer
		if err := WriteCNC(&buf, m, f); err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if buf.Len() == 0 {
			t.Errorf("%s: пустой вывод", f)
		}
	}
	// Треугольники сварка не трогает — иначе пропала бы геометрия.
	welded, _ := m.Weld()
	if len(welded.Triangles) != len(m.Triangles) {
		t.Errorf("треугольников %d, ожидалось %d", len(welded.Triangles), len(m.Triangles))
	}
}

// TestWriteWeldsWithoutBreakingFormats — DXF, STL и SVG пишут вершины
// инлайн на каждый треугольник, поэтому размер от сварки не меняется, но
// сама операция обязана быть безопасной и идемпотентной.
func TestWriteWeldsWithoutBreakingFormats(t *testing.T) {
	m := heavyMesh()
	for _, f := range []Format{DXF, STL, SVG} {
		var a, b bytes.Buffer
		if err := Write(&a, m, f); err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if err := Write(&b, m, f); err != nil {
			t.Errorf("%s (повтор): %v", f, err)
			continue
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Errorf("%s: повторная запись отличается — сварка не идемпотентна", f)
		}
		if !strings.Contains(a.String(), "ST") && a.Len() == 0 {
			t.Errorf("%s: пустой вывод", f)
		}
	}
}

// TestWriteCNCIndexedVerticesStayInRange — после сварки индексы
// треугольников обязаны вести в диапазон, иначе файл получит битые ссылки.
func TestWriteCNCIndexedVerticesStayInRange(t *testing.T) {
	m := heavyMesh()
	welded, _ := m.Weld()
	for _, t3 := range welded.Triangles {
		for _, idx := range t3 {
			if idx < 0 || idx >= len(welded.Vertices) {
				t.Fatalf("индекс %d вне диапазона [0,%d)", idx, len(welded.Vertices))
			}
		}
	}
	// И индексы в файле — валидные целые.
	var raw bytes.Buffer
	if err := WriteCNC(&raw, welded, CSV); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw.String(), strconv.Itoa(len(welded.Vertices)-1)) {
		t.Error("в CSV нет последнего индекса вершины")
	}
}
