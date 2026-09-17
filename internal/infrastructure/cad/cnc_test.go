package cad

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCNCFormat(t *testing.T) {
	for _, c := range []struct {
		in   string
		want CNCFormat
	}{
		{"step", STEP}, {"STEP", STEP}, {"Step", STEP},
		{"iges", IGES}, {"IGES", IGES},
		{"obj", OBJ}, {"OBJ", OBJ},
		{"3mf", THREEMF}, {"3MF", THREEMF},
		{"csv", CSV}, {"CSV", CSV},
		{"json", JSON}, {"JSON", JSON},
		{"xml", XML}, {"XML", XML},
	} {
		got, err := ParseCNCFormat(c.in)
		if err != nil || got != c.want {
			t.Fatalf("ParseCNCFormat(%q)=%q err %v want %q", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "dxf", "stl", "bad"} {
		if _, err := ParseCNCFormat(bad); err == nil {
			t.Fatalf("want error for %q", bad)
		}
	}
}

func TestWriteCNCAllFormats(t *testing.T) {
	m := testMesh(t)
	table := []struct {
		fmt  CNCFormat
		want string
	}{
		{STEP, "ISO-10303-21"},
		{IGES, "16Hstairplatform"},
		{OBJ, "v "},
		{THREEMF, "http://schemas.microsoft.com/3dmanufacturing"},
		{CSV, "type,index"},
		{JSON, "\"vertices\""},
		{XML, "<mesh>"},
	}
	for _, tc := range table {
		var buf bytes.Buffer
		if err := WriteCNC(&buf, m, tc.fmt); err != nil {
			t.Fatalf("WriteCNC(%s): %v", tc.fmt, err)
		}
		if !strings.Contains(buf.String(), tc.want) {
			t.Fatalf("WriteCNC(%s) lacks %q: %q", tc.fmt, tc.want, buf.String()[:200])
		}
	}
}

func TestWriteCNCEmptyAndRange(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCNC(&buf, nil, STEP); err != ErrEmptyMesh {
		t.Fatalf("nil want ErrEmptyMesh got %v", err)
	}
	// placeholder valid mesh already tested above
	_ = testMesh(t)
	m := testMesh(t)
	m.Triangles[0] = [3]int{0, 1, 99}
	if err := WriteCNC(&buf, m, JSON); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("want out of range, got %v", err)
	}
}

func TestWriteCNCUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCNC(&buf, testMesh(t), CNCFormat("dxf")); err == nil {
		t.Fatal("want error for unknown CNC format")
	}
}

func TestWriteCNCObjDetails(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCNC(&buf, testMesh(t), OBJ); err != nil {
		t.Fatalf("OBJ: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "vn ") {
		t.Fatal("OBJ lacks vn")
	}
	if strings.Count(s, "f ") != 2 {
		t.Fatalf("OBJ face count want 2, got %d", strings.Count(s, "f "))
	}
}

func TestWriteCNCCSVDetails(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCNC(&buf, testMesh(t), CSV); err != nil {
		t.Fatalf("CSV: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "type,index,x,y,z") {
		t.Fatal("CSV lacks vertex header")
	}
	if !strings.Contains(s, "type,index,v1,v2,v3") {
		t.Fatal("CSV lacks face header")
	}
}
