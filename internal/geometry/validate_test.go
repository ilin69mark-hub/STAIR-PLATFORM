package geometry

import (
	"reflect"
	"testing"
)

func hasIssue(issues []ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestValidateValidBox(t *testing.T) {
	if issues := Validate(boxSolid()); len(issues) != 0 {
		t.Fatalf("valid box must pass validation, got %+v", issues)
	}
}

func TestValidateValidStringer(t *testing.T) {
	if issues := Validate(stringerLikeSolid()); len(issues) != 0 {
		t.Fatalf("valid stringer must pass validation, got %+v", issues)
	}
}

func TestValidateNil(t *testing.T) {
	issues := Validate(nil)
	if len(issues) != 1 || issues[0].Code != "GEO-SOLID-MISSING" {
		t.Fatalf("issues = %+v, want GEO-SOLID-MISSING", issues)
	}
	if issues[0].Severity != SeverityError {
		t.Fatal("missing solid must be an error")
	}
}

func TestValidateEmptySolid(t *testing.T) {
	issues := Validate(NewSolid())
	if len(issues) != 1 || issues[0].Code != "GEO-SOLID-EMPTY" {
		t.Fatalf("issues = %+v, want GEO-SOLID-EMPTY", issues)
	}
}

func TestValidateOpenWire(t *testing.T) {
	// грань с 3 рёбрами, но незамкнутым контуром: конец не совпадает
	// с началом.
	v0 := NewVertex(NewPoint3(0, 0, 0))
	v1 := NewVertex(NewPoint3(10, 0, 0))
	v2 := NewVertex(NewPoint3(10, 10, 0))
	v3 := NewVertex(NewPoint3(20, 10, 0))
	open := NewWire(NewEdge(v0, v1), NewEdge(v1, v2), NewEdge(v2, v3))
	solid := NewSolid(NewShell(NewFace(open)))
	issues := Validate(solid)
	if !hasIssue(issues, "GEO-FACE-OPEN-WIRE") {
		t.Fatalf("open wire must be flagged, got %+v", issues)
	}
}

func TestValidateNonManifold(t *testing.T) {
	// одна грань-треугольник: каждое ребро входит ровно в 1 грань (не 2).
	v0 := NewVertex(NewPoint3(0, 0, 0))
	v1 := NewVertex(NewPoint3(10, 0, 0))
	v2 := NewVertex(NewPoint3(0, 10, 0))
	w := NewWire(NewEdge(v0, v1), NewEdge(v1, v2), NewEdge(v2, v0))
	solid := NewSolid(NewShell(NewFace(w)))
	issues := Validate(solid)
	if !hasIssue(issues, "GEO-SOLID-NON-MANIFOLD") {
		t.Fatal("single-face solid must be non-manifold")
	}
	if !hasIssue(issues, "GEO-SOLID-NON-POSITIVE-VOLUME") {
		t.Fatal("degenerate (zero-volume) solid must be flagged")
	}
}

func TestValidateDeterminism(t *testing.T) {
	a := Validate(boxSolid())
	b := Validate(boxSolid())
	if !reflect.DeepEqual(a, b) {
		t.Fatal("validation must be deterministic")
	}
	// issues отсортированы по Code (single-face solid даёт несколько issues).
	v0 := NewVertex(NewPoint3(0, 0, 0))
	v1 := NewVertex(NewPoint3(10, 0, 0))
	v2 := NewVertex(NewPoint3(0, 10, 0))
	solid := NewSolid(NewShell(NewFace(NewWire(
		NewEdge(v0, v1), NewEdge(v1, v2), NewEdge(v2, v0)))))
	issues := Validate(solid)
	if len(issues) < 2 {
		t.Fatalf("expected multiple issues, got %+v", issues)
	}
	for i := 1; i < len(issues); i++ {
		if issues[i-1].Code > issues[i].Code {
			t.Fatalf("issues not sorted: %+v", issues)
		}
	}
}
