package cad

import (
	"bytes"
	"strings"
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

// testMesh строит простую сетку из двух треугольников (квадрат в XY на Z=0).
func testMesh(t *testing.T) *kerngeo.Mesh {
	t.Helper()
	m := &kerngeo.Mesh{
		Vertices: []kerngeo.Point3{
			kerngeo.NewPoint3(0, 0, 0),
			kerngeo.NewPoint3(10, 0, 0),
			kerngeo.NewPoint3(10, 10, 0),
			kerngeo.NewPoint3(0, 10, 0),
		},
	}
	for _, tr := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
		if err := m.AddTriangle(tr[0], tr[1], tr[2]); err != nil {
			t.Fatalf("AddTriangle: %v", err)
		}
	}
	return m
}

func TestWriteDXF(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testMesh(t), DXF); err != nil {
		t.Fatalf("Write(DXF): %v", err)
	}
	s := buf.String()
	for _, want := range []string{
		"0\nSECTION", "2\nHEADER", "$ACADVER", "1\nAC1009",
		"2\nENTITIES", "0\n3DFACE", "0\nEOF",
		"10\n0.000000", "11\n10.000000",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("DXF lacks %q\n%s", want, s)
		}
	}
	if got := strings.Count(s, "0\n3DFACE"); got != 2 {
		t.Errorf("3DFACE count = %d, want 2", got)
	}
	// Последний угол 4-й === 3-й (13..33 == 12..32) для треугольника.
	for _, pair := range []string{
		"12\n10.000000\n22\n10.000000", "13\n10.000000\n23\n10.000000",
		"32\n0.000000", "33\n0.000000",
	} {
		if !strings.Contains(s, pair) {
			t.Errorf("DXF lacks closure pair %q\n%s", pair, s)
		}
	}
}

func TestWriteSTL(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testMesh(t), STL); err != nil {
		t.Fatalf("Write(STL): %v", err)
	}
	s := buf.String()
	if !strings.HasPrefix(s, "solid STAIR\n") {
		t.Errorf("STL must start with solid STAIR, got %q", s[:16])
	}
	if !strings.HasSuffix(s, "endsolid STAIR\n") {
		t.Errorf("STL must end with endsolid STAIR")
	}
	if got := strings.Count(s, "facet normal"); got != 2 {
		t.Errorf("facet count = %d, want 2", got)
	}
	// Квадрат лежит в XY: нормаль обоих треугольников (0,0,1).
	if !strings.Contains(s, "facet normal 0.000000 0.000000 1.000000") {
		t.Errorf("expected normal (0,0,1), got:\n%s", s)
	}
	for _, v := range []string{
		"vertex 0.000000 0.000000 0.000000",
		"vertex 10.000000 10.000000 0.000000",
	} {
		if !strings.Contains(s, v) {
			t.Errorf("STL lacks vertex %q", v)
		}
	}
	// Правильный порядок граней: 6 вершин на каждый facet.
	if got := strings.Count(s, "vertex"); got != 6 {
		t.Errorf("vertex count = %d, want 6", got)
	}
}

func TestWriteSVG(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testMesh(t), SVG); err != nil {
		t.Fatalf("Write(SVG): %v", err)
	}
	s := buf.String()
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<svg xmlns="http://www.w3.org/2000/svg"`,
		`viewBox=`,
		`<line x1=` + `"0.000000"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG lacks %q", want)
		}
	}
	// Квадрат 0..10, разбитый диагональю на 2 грани: 4 внешних + 1 диагональ = 5.
	if got := strings.Count(s, "<line"); got != 5 {
		t.Errorf("line count = %d, want 5\n%s", got, s)
	}
	if !strings.Contains(s, "</svg>") {
		t.Errorf("SVG lacks closing tag")
	}
}

func TestWriteEmptyMesh(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, &kerngeo.Mesh{}, DXF); err != ErrEmptyMesh {
		t.Errorf("empty mesh: err = %v, want ErrEmptyMesh", err)
	}
	if err := Write(&buf, nil, STL); err != ErrEmptyMesh {
		t.Errorf("nil mesh: err = %v, want ErrEmptyMesh", err)
	}
}

func TestWriteOutOfRangeIndex(t *testing.T) {
	m := &kerngeo.Mesh{
		Vertices:  []kerngeo.Point3{kerngeo.NewPoint3(0, 0, 0)},
		Triangles: [][3]int{{0, 1, 0}},
	}
	var buf bytes.Buffer
	if err := Write(&buf, m, DXF); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("out-of-range: err = %v, want index error", err)
	}
}

func TestWriteUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, testMesh(t), Format("obj")); err == nil {
		t.Errorf("unknown format: expected error")
	}
}

func TestParseFormat(t *testing.T) {
	for in, want := range map[string]Format{
		"dxf": DXF, "DXF": DXF,
		"stl": STL, "STL": STL,
		"svg": SVG, "SVG": SVG,
	} {
		got, err := ParseFormat(in)
		if err != nil {
			t.Errorf("ParseFormat(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseFormat(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "obj", "dxf;svg"} {
		if _, err := ParseFormat(bad); err == nil {
			t.Errorf("ParseFormat(%q): expected error", bad)
		}
	}
}

func TestFormatMetadata(t *testing.T) {
	if DXF.MIME() != "application/dxf" || DXF.Extension() != ".dxf" {
		t.Errorf("DXF metadata wrong: %s %s", DXF.MIME(), DXF.Extension())
	}
	if STL.MIME() != "model/stl" || STL.Extension() != ".stl" {
		t.Errorf("STL metadata wrong")
	}
	if SVG.MIME() != "image/svg+xml" || SVG.Extension() != ".svg" {
		t.Errorf("SVG metadata wrong")
	}
}
