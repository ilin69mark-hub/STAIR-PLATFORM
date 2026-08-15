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
	return "", fmt.Errorf("cad: unsupported format %q (supported: dxf, stl, svg)", s)
}

// ErrEmptyMesh — сетка не содержит вершин/граней.
var ErrEmptyMesh = errors.New("cad: empty mesh")

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
