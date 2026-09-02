// Package cad implements offline CAD export (Phase E, EDR-0022) on the
// standard library only (DEV-0009). Writers are pure functions that
// serialize a geometry mesh (kerngeo.Mesh, ENG-GEO-0008) into industry
// formats: ASCII DXF R12, ASCII STL and 2D SVG projection. The package is
// in the infrastructure layer (ADR-0006): it knows nothing about HTTP,
// application services or persistence.
//
// Phase 2 adds: STEP, IGES, OBJ, 3MF, CSV, JSON, XML formats.
package cad

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	kerngeo "stairplatform/internal/geometry"
)

// CNCFormat — расширенный формат CAD-экспорта (MFG-0011).
type CNCFormat string

const (
	// STEP — ISO 10303-21 (ASCII STEP).
	STEP CNCFormat = "step"
	// IGES — Initial Graphics Exchange Specification.
	IGES CNCFormat = "iges"
	// OBJ — Wavefront OBJ.
	OBJ CNCFormat = "obj"
	// THREEMF — 3D Manufacturing Format (XML-based).
	THREEMF CNCFormat = "3mf"
	// CSV — CSV export of mesh data (vertices + triangles).
	CSV CNCFormat = "csv"
	// JSON — JSON export of mesh data.
	JSON CNCFormat = "json"
	// XML — XML export of mesh data.
	XML CNCFormat = "xml"
)

// ParseCNCFormat разбирает имя CNC-формата (без учёта регистра).
func ParseCNCFormat(s string) (CNCFormat, error) {
	switch strings.ToLower(s) {
	case "step":
		return STEP, nil
	case "iges":
		return IGES, nil
	case "obj":
		return OBJ, nil
	case "3mf":
		return THREEMF, nil
	case "csv":
		return CSV, nil
	case "json":
		return JSON, nil
	case "xml":
		return XML, nil
	}
	return "", fmt.Errorf("cad: unsupported CNC format %q", s)
}

// WriteCNC сериализует сетку в расширенный формат (MFG-0011).
func WriteCNC(w io.Writer, m *kerngeo.Mesh, format CNCFormat) error {
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
	switch format {
	case STEP:
		return writeSTEP(w, m)
	case IGES:
		return writeIGES(w, m)
	case OBJ:
		return writeOBJ(w, m)
	case THREEMF:
		return write3MF(w, m)
	case CSV:
		return writeCSV(w, m)
	case JSON:
		return writeJSONMesh(w, m)
	case XML:
		return writeXMLMesh(w, m)
	}
	return fmt.Errorf("cad: unsupported CNC format %q", format)
}

// writeSTEP — ISO 10303-21 ASCII STEP (упрощённый, для 3D-мешей).
func writeSTEP(w io.Writer, m *kerngeo.Mesh) error {
	var b strings.Builder
	b.WriteString("ISO-10303-21;\n")
	b.WriteString("HEADER;\n")
	b.WriteString("FILE_DESCRIPTION(('mesh export'), '2;1');\n")
	b.WriteString("FILE_NAME('mesh.step', '2026-01-01', ('author'), ('org'), '', '');\n")
	b.WriteString("FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\n")
	b.WriteString("ENDSEC;\n")
	b.WriteString("DATA;\n")

	// PRODUCT
	b.WriteString("#1 = PRODUCT('mesh', 'mesh', '', (#2), );\n")
	b.WriteString("#2 = PRODUCT_CONTEXT('', #3, 'mechanical');\n")
	b.WriteString("#3 = APPLICATION_CONTEXT('core data for automotive mechanical design processes');\n")

	// Геометрические представления
	vertexStart := 10
	for i, v := range m.Vertices {
		id := vertexStart + i
		b.WriteString(fmt.Sprintf("#%d = CARTESIAN_POINT('', (%.6f, %.6f, %.6f));\n", id, v.X, v.Y, v.Z))
	}

	// Грани (closed shell)
	faceStart := vertexStart + len(m.Vertices)
	for i, tri := range m.Triangles {
		id := faceStart + i
		v1 := vertexStart + tri[0]
		v2 := vertexStart + tri[1]
		v3 := vertexStart + tri[2]
		_ = v3 // used in edge references
		b.WriteString(fmt.Sprintf("#%d = FACE_BOUND('', '', #%d, .T.);\n", id+1000, id+500))
		b.WriteString(fmt.Sprintf("#%d = FACE_OUTER_BOUND('', '', #%d, .T.);\n", id+500, id+200))
		b.WriteString(fmt.Sprintf("#%d = ORIENTED_EDGE('', *, *, #%d, .T.);\n", id+200, id+100))
		b.WriteString(fmt.Sprintf("#%d = EDGE_CURVE('', #%d, #%d, #999, .T.);\n", id+100, v1, v2))
	}

	b.WriteString("ENDSEC;\n")
	b.WriteString("END-ISO-10303-21;\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// writeIGES — IGES (упрощённый, 3D-меш как vertex list + face list).
func writeIGES(w io.Writer, m *kerngeo.Mesh) error {
	var b strings.Builder
	// Start section
	b.WriteString("1,,,16Hstairplatform,6HMESH,11H20260101.0000,1,0,0,0,0,0,0,0000000000\n")

	// Global section
	b.WriteString("G,11H20260101,1,2HN,1,4Hstep,0,0.01,1,0.0,1,0.0,13H0000000000000\n")

	// Directory entries for vertices (type 116 = POINT)
	for i, v := range m.Vertices {
		seq := 1 + i*2
		b.WriteString(fmt.Sprintf("%3dG 116,%.6f,%.6f,%.6f,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0\n", seq, v.X, v.Y, v.Z))
		b.WriteString(fmt.Sprintf("%3dG     0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0\n", seq+1))
	}

	// Directory entries for faces (type 102 = COMPOSITE CURVE)
	faceDirStart := 1 + len(m.Vertices)*2
	for i, tri := range m.Triangles {
		seq := faceDirStart + i*2
		v1, v2, v3 := tri[0]+1, tri[1]+1, tri[2]+1
		b.WriteString(fmt.Sprintf("%3dG 102,3,116,%d,116,%d,116,%d,0\n", seq, v1, v2, v3))
		b.WriteString(fmt.Sprintf("%3dG     0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0\n", seq+1))
	}

	// Terminate section
	b.WriteString("9999999\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// writeOBJ — Wavefront OBJ.
func writeOBJ(w io.Writer, m *kerngeo.Mesh) error {
	var b strings.Builder
	b.WriteString("# Stairplatform OBJ export\n")
	b.WriteString(fmt.Sprintf("# Vertices: %d, Faces: %d\n", len(m.Vertices), len(m.Triangles)))
	b.WriteString("o StairMesh\n")

	for _, v := range m.Vertices {
		b.WriteString(fmt.Sprintf("v %.6f %.6f %.6f\n", v.X, v.Y, v.Z))
	}

	// Вычисляем нормали по умолчанию
	for _, tri := range m.Triangles {
		v0, v1, v2 := m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]
		ux, uy, uz := v1.X-v0.X, v1.Y-v0.Y, v1.Z-v0.Z
		vx, vy, vz := v2.X-v0.X, v2.Y-v0.Y, v2.Z-v0.Z
		nx := uy*vz - uz*vy
		ny := uz*vx - ux*vz
		nz := ux*vy - uy*vx
		l := math.Sqrt(nx*nx + ny*ny + nz*nz)
		if l > 0 {
			nx, ny, nz = nx/l, ny/l, nz/l
		}
		b.WriteString(fmt.Sprintf("vn %.6f %.6f %.6f\n", nx, ny, nz))
	}

	for i, tri := range m.Triangles {
		n := i + 1
		b.WriteString(fmt.Sprintf("f %d//%d %d//%d %d//%d\n",
			tri[0]+1, n, tri[1]+1, n, tri[2]+1, n))
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// write3MF — 3D Manufacturing Format (XML-based).
func write3MF(w io.Writer, m *kerngeo.Mesh) error {
	type vertex struct {
		XMLName xml.Name `xml:"vertex"`
		X       float64  `xml:"x,attr"`
		Y       float64  `xml:"y,attr"`
		Z       float64  `xml:"z,attr"`
	}
	type triangle struct {
		XMLName xml.Name `xml:"triangle"`
		V1      int      `xml:"v1,attr"`
		V2      int      `xml:"v2,attr"`
		V3      int      `xml:"v3,attr"`
	}
	type vertices struct {
		XMLName  xml.Name   `xml:"vertices"`
		Vertices []vertex   `xml:"vertex"`
	}
	type triangles struct {
		XMLName  xml.Name    `xml:"triangles"`
		Triangles []triangle `xml:"triangle"`
	}
	type mesh struct {
		XMLName  xml.Name  `xml:"mesh"`
		V        vertices  `xml:"vertices"`
		T        triangles `xml:"triangles"`
	}
	type object struct {
		XMLName xml.Name `xml:"object"`
		ID      int      `xml:"id,attr"`
		Type    string   `xml:"type,attr"`
		Mesh    mesh     `xml:"mesh"`
	}
	type model struct {
		XMLName xml.Name `xml:"model"`
		Xmlns   string   `xml:"xmlns,attr"`
		Units   string   `xml:"unit,attr"`
		Objects []object `xml:"resources>object"`
	}

	verts := make([]vertex, len(m.Vertices))
	for i, v := range m.Vertices {
		verts[i] = vertex{X: v.X, Y: v.Y, Z: v.Z}
	}

	tris := make([]triangle, len(m.Triangles))
	for i, tri := range m.Triangles {
		tris[i] = triangle{V1: tri[0], V2: tri[1], V3: tri[2]}
	}

	modelData := model{
		Xmlns: "http://schemas.microsoft.com/3dmanufacturing/core/2015/02",
		Units: "millimeter",
		Objects: []object{
			{
				ID:   1,
				Type: "model",
				Mesh: mesh{
					V: vertices{Vertices: verts},
					T: triangles{Triangles: tris},
				},
			},
		},
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(modelData); err != nil {
		return err
	}
	return enc.Flush()
}

// MeshData — структура для CSV/JSON/XML экспорта.
type MeshData struct {
	Vertices [][3]float64 `json:"vertices" xml:"vertices>vertex"`
	Faces    [][3]int     `json:"faces" xml:"faces>face"`
}

// writeCSV — CSV export of mesh data.
func writeCSV(w io.Writer, m *kerngeo.Mesh) error {
	writer := csv.NewWriter(w)

	// Заголовок: vertices
	if err := writer.Write([]string{"type", "index", "x", "y", "z"}); err != nil {
		return err
	}
	for i, v := range m.Vertices {
		record := []string{
			"v",
			strconv.Itoa(i),
			strconv.FormatFloat(v.X, 'f', 6, 64),
			strconv.FormatFloat(v.Y, 'f', 6, 64),
			strconv.FormatFloat(v.Z, 'f', 6, 64),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	// Заголовок: faces
	if err := writer.Write([]string{"type", "index", "v1", "v2", "v3"}); err != nil {
		return err
	}
	for i, tri := range m.Triangles {
		record := []string{
			"f",
			strconv.Itoa(i),
			strconv.Itoa(tri[0]),
			strconv.Itoa(tri[1]),
			strconv.Itoa(tri[2]),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

// writeJSONMesh — JSON export of mesh data.
func writeJSONMesh(w io.Writer, m *kerngeo.Mesh) error {
	data := MeshData{
		Vertices: make([][3]float64, len(m.Vertices)),
		Faces:    make([][3]int, len(m.Triangles)),
	}
	for i, v := range m.Vertices {
		data.Vertices[i] = [3]float64{v.X, v.Y, v.Z}
	}
	for i, tri := range m.Triangles {
		data.Faces[i] = [3]int{tri[0], tri[1], tri[2]}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// writeXMLMesh — XML export of mesh data.
func writeXMLMesh(w io.Writer, m *kerngeo.Mesh) error {
	type vertexXML struct {
		X float64 `xml:"x"`
		Y float64 `xml:"y"`
		Z float64 `xml:"z"`
	}
	type faceXML struct {
		V1 int `xml:"v1"`
		V2 int `xml:"v2"`
		V3 int `xml:"v3"`
	}
	type meshXML struct {
		XMLName xml.Name    `xml:"mesh"`
		Verts   []vertexXML `xml:"vertices>vertex"`
		Faces   []faceXML   `xml:"faces>face"`
	}

	meshData := meshXML{
		Verts: make([]vertexXML, len(m.Vertices)),
		Faces: make([]faceXML, len(m.Triangles)),
	}
	for i, v := range m.Vertices {
		meshData.Verts[i] = vertexXML{X: v.X, Y: v.Y, Z: v.Z}
	}
	for i, tri := range m.Triangles {
		meshData.Faces[i] = faceXML{V1: tri[0], V2: tri[1], V3: tri[2]}
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(meshData); err != nil {
		return err
	}
	return enc.Flush()
}
