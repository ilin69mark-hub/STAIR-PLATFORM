// Package cad implements offline CAD export (Phase E, EDR-0022) on the
// standard library only (DEV-0009). Writers are pure functions that
// serialize a geometry mesh (kerngeo.Mesh, ENG-GEO-0008) into industry
// formats: ASCII DXF R12, ASCII STL and 2D SVG projection. The package is
// in the infrastructure layer (ADR-0006): it knows nothing about HTTP,
// application services or persistence.
package cad

import (
	"errors"
	"fmt"
	"io"

	kerngeo "stairplatform/internal/geometry"
)

// Format — формат CAD-экспорта (EDR-0022 §3.2).
type Format string

const (
	// DXF — AutoCAD DXF R12 ASCII (3DFACE).
	DXF Format = "dxf"
	// STL — ASCII STL (нормали по правилу правой тройки).
	STL Format = "stl"
	// SVG — двумерная ортографическая проекция (вид сверху, XY-плоскость).
	SVG Format = "svg"
)

// MIME возвращает Content-Type для формата.
func (f Format) MIME() string {
	switch f {
	case DXF:
		return "application/dxf"
	case STL:
		return "model/stl"
	case SVG:
		return "image/svg+xml"
	}
	return "application/octet-stream"
}

// Extension возвращает расширение файла (с точкой).
func (f Format) Extension() string {
	return "." + string(f)
}

// ParseFormat разбирает имя формата из query-параметра (без учёта регистра).
func ParseFormat(s string) (Format, error) {
	switch s {
	case "dxf", "DXF":
		return DXF, nil
	case "stl", "STL":
		return STL, nil
	case "svg", "SVG":
		return SVG, nil
	}
	return "", fmt.Errorf("cad: unsupported format %q (supported: dxf, stl, svg, step, iges, obj, 3mf, csv, json, xml)", s)
}

// ErrEmptyMesh — сетка не содержит вершин/граней.
var ErrEmptyMesh = errors.New("cad: empty mesh")

// Merge объединяет несколько сеток в одну (DOM-003).
//
// Нужен потому, что перила строятся ОТДЕЛЬНЫМ телом и в основной меш
// лестницы не входят (RailingMesh в geometry.Generate). Экспорт без слияния
// терял перила: пользователь задавал их, видел в 3D, а в DXF/STL/SVG их не
// было. Merge сдвигает индексы треугольников на текущий размер буфера
// вершин и склеивает PartRanges с корректным сдвигом Start.
func Merge(meshes ...*kerngeo.Mesh) *kerngeo.Mesh {
	out := &kerngeo.Mesh{}
	for _, m := range meshes {
		if m == nil || len(m.Vertices) == 0 {
			continue
		}
		vertexBase := len(out.Vertices)
		triBase := len(out.Triangles)
		out.Vertices = append(out.Vertices, m.Vertices...)
		for _, t := range m.Triangles {
			out.Triangles = append(out.Triangles, [3]int{vertexBase + t[0], vertexBase + t[1], vertexBase + t[2]})
		}
		for _, pr := range m.PartRanges {
			out.PartRanges = append(out.PartRanges, kerngeo.PartRange{
				Solid: pr.Solid,
				Role:  pr.Role,
				Start: triBase + pr.Start,
				End:   triBase + pr.End,
			})
		}
	}
	return out
}

// Write сериализует сетку в выбранный формат (EDR-0022 §3.3).
func Write(w io.Writer, m *kerngeo.Mesh, f Format) error {
	if m == nil || len(m.Vertices) == 0 || len(m.Triangles) == 0 {
		return ErrEmptyMesh
	}
	for _, t := range m.Triangles {
		for _, idx := range t {
			if idx < 0 || idx >= len(m.Vertices) {
				return fmt.Errorf("cad: triangle index %d out of range (vertices %d)", idx, len(m.Vertices))
			}
		}
	}
	switch f {
	case DXF:
		return writeDXF(w, m)
	case STL:
		return writeSTL(w, m)
	case SVG:
		return writeSVG(w, m)
	}
	return fmt.Errorf("cad: unsupported format %q", f)
}
