package cad

import (
	"fmt"
	"io"
	"math"
	"sort"

	kerngeo "stairplatform/internal/geometry"
)

// writeSVG сериализует ортографическую проекцию сетки на XY-плоскость
// (вид сверху) в SVG (EDR-0022 §3.3). Отрисовка — wireframe: уникальные
// рёбра всех граней как линии. Y-ось SVG направлена вниз, поэтому проекция
// инвертирует Y для привычной декартовой ориентации. viewBox строится по
// bounding box XY-проекции с отступом 4 мм.
func writeSVG(w io.Writer, m *kerngeo.Mesh) error {
	edges := collectEdges(m)
	sortEdges(edges)

	minX, minY, maxX, maxY := bbox2D(m)
	pad := 4.0
	vb := fmt.Sprintf("%.6f %.6f %.6f %.6f", minX-pad, minY-pad,
		(maxX-minX)+2*pad, (maxY-minY)+2*pad)

	if _, err := io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n"); err != nil {
		return err
	}
	header := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s" width="800" height="800">`+"\n",
		vb)
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if _, err := io.WriteString(w, `<g fill="none" stroke="#1f2937" stroke-width="0.5">`+"\n"); err != nil {
		return err
	}
	for _, e := range edges {
		ax, ay := project(e.a.X, e.a.Y)
		bx, by := project(e.b.X, e.b.Y)
		line := fmt.Sprintf("<line x1=\"%.6f\" y1=\"%.6f\" x2=\"%.6f\" y2=\"%.6f\"/>\n",
			ax, ay, bx, by)
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "</g>\n</svg>\n"); err != nil {
		return err
	}
	return nil
}

// project отображает декартовы координаты в SVG-координаты (Y инвертирован).
func project(x, y float64) (float64, float64) {
	return x, -y
}

type edge struct{ a, b kerngeo.Point3 }

// collectEdges собирает уникальные рёбра по парам индексов вершин
// (a<b — направление не важно, грань учитывается один раз).
func collectEdges(m *kerngeo.Mesh) []edge {
	seen := make(map[[2]int]struct{}, len(m.Triangles)*3)
	var edges []edge
	for _, t := range m.Triangles {
		pairs := [][2]int{{t[0], t[1]}, {t[1], t[2]}, {t[2], t[0]}}
		for _, p := range pairs {
			if p[0] > p[1] {
				p[0], p[1] = p[1], p[0]
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			edges = append(edges, edge{m.Vertices[p[0]], m.Vertices[p[1]]})
		}
	}
	return edges
}

func sortEdges(e []edge) {
	sort.Slice(e, func(i, j int) bool {
		if e[i].a.X != e[j].a.X {
			return e[i].a.X < e[j].a.X
		}
		if e[i].a.Y != e[j].a.Y {
			return e[i].a.Y < e[j].a.Y
		}
		if e[i].a.Z != e[j].a.Z {
			return e[i].a.Z < e[j].a.Z
		}
		if e[i].b.X != e[j].b.X {
			return e[i].b.X < e[j].b.X
		}
		if e[i].b.Y != e[j].b.Y {
			return e[i].b.Y < e[j].b.Y
		}
		return e[i].b.Z < e[j].b.Z
	})
}

// bbox2D возвращает bounding box XY-проекции всех вершин (вид сверху).
func bbox2D(m *kerngeo.Mesh) (minX, minY, maxX, maxY float64) {
	if len(m.Vertices) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY = math.MaxFloat64, math.MaxFloat64
	maxX, maxY = -math.MaxFloat64, -math.MaxFloat64
	for _, v := range m.Vertices {
		minX = math.Min(minX, v.X)
		maxX = math.Max(maxX, v.X)
		minY = math.Min(minY, v.Y)
		maxY = math.Max(maxY, v.Y)
	}
	return
}
