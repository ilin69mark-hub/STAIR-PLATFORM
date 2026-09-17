package geometry

import (
	"math"
	"testing"
)

func TestValidateArcErrorPaths(t *testing.T) {
	badRadius := &Arc{Center: Point3{}, Radius: 0, Normal: Vector3{X: 0, Y: 0, Z: 1}, Start: 0, End: math.Pi}
	if !hasIssue(ValidateArc(badRadius), "GEO-ARC-RADIUS") {
		t.Fatal("zero radius must be flagged")
	}
	badNormal := &Arc{Center: Point3{}, Radius: 100, Normal: Vector3{}, Start: 0, End: math.Pi}
	if !hasIssue(ValidateArc(badNormal), "GEO-ARC-NORMAL") {
		t.Fatal("zero normal must be flagged")
	}
	badAngles := &Arc{Center: Point3{}, Radius: 100, Normal: Vector3{X: 0, Y: 0, Z: 1}, Start: math.Pi, End: math.Pi}
	if !hasIssue(ValidateArc(badAngles), "GEO-ARC-ANGLES") {
		t.Fatal("equal angles must be flagged")
	}
}

func TestValidateBezierErrorPaths(t *testing.T) {
	issues := ValidateBezier(nil)
	if len(issues) != 1 || issues[0].Code != "GEO-BEZIER-MISSING" {
		t.Fatalf("nil bezier issues = %+v", issues)
	}
	p := Point3{X: 1, Y: 1, Z: 1}
	b := &Bezier{P0: p, P1: p, P2: p, P3: p}
	if !hasIssue(ValidateBezier(b), "GEO-BEZIER-DEGENERATE") {
		t.Fatal("coincident control points must be flagged")
	}
}

func TestValidateFilletErrorPaths(t *testing.T) {
	if issues := ValidateFillet(nil); len(issues) != 1 || issues[0].Code != "GEO-FILLET-MISSING" {
		t.Fatalf("nil fillet issues = %+v", issues)
	}
	badRadius := &Fillet{Edge: [2]Point3{{X: 0, Y: 0}, {X: 100, Y: 0}}, Radius: 0, Normal: Vector3{X: 0, Y: 0, Z: 1}}
	if !hasIssue(ValidateFillet(badRadius), "GEO-FILLET-RADIUS") {
		t.Fatal("zero radius must be flagged")
	}
	badNormal := &Fillet{Edge: [2]Point3{{X: 0, Y: 0}, {X: 100, Y: 0}}, Radius: 10, Normal: Vector3{}}
	if !hasIssue(ValidateFillet(badNormal), "GEO-FILLET-NORMAL") {
		t.Fatal("zero normal must be flagged")
	}
	degenerate := &Fillet{Edge: [2]Point3{{X: 5, Y: 5}, {X: 5, Y: 5}}, Radius: 10, Normal: Vector3{X: 0, Y: 0, Z: 1}}
	if !hasIssue(ValidateFillet(degenerate), "GEO-FILLET-EDGE") {
		t.Fatal("degenerate edge must be flagged")
	}
}

func TestValidateChamferErrorPaths(t *testing.T) {
	if issues := ValidateChamfer(nil); len(issues) != 1 || issues[0].Code != "GEO-CHAMFER-MISSING" {
		t.Fatalf("nil chamfer issues = %+v", issues)
	}
	badWidth := &Chamfer{Edge: [2]Point3{{X: 0, Y: 0}, {X: 100, Y: 0}}, Width: 0, Normal: Vector3{X: 0, Y: 0, Z: 1}}
	if !hasIssue(ValidateChamfer(badWidth), "GEO-CHAMFER-WIDTH") {
		t.Fatal("zero width must be flagged")
	}
	badNormal := &Chamfer{Edge: [2]Point3{{X: 0, Y: 0}, {X: 100, Y: 0}}, Width: 5, Normal: Vector3{}}
	if !hasIssue(ValidateChamfer(badNormal), "GEO-CHAMFER-NORMAL") {
		t.Fatal("zero normal must be flagged")
	}
	degenerate := &Chamfer{Edge: [2]Point3{{X: 5, Y: 5}, {X: 5, Y: 5}}, Width: 5, Normal: Vector3{X: 0, Y: 0, Z: 1}}
	if !hasIssue(ValidateChamfer(degenerate), "GEO-CHAMFER-EDGE") {
		t.Fatal("degenerate edge must be flagged")
	}
}

func TestValidateNGonErrorPaths(t *testing.T) {
	if issues := ValidateNGon(nil); len(issues) != 1 || issues[0].Code != "GEO-NGON-MISSING" {
		t.Fatalf("nil ngon issues = %+v", issues)
	}
	two := &NGon{Vertices: []Point3{{X: 0, Y: 0}, {X: 1, Y: 0}}, Normal: Vector3{X: 0, Y: 0, Z: 1}}
	if !hasIssue(ValidateNGon(two), "GEO-NGON-VERTICES") {
		t.Fatal("fewer than 3 vertices must be flagged")
	}
	zeroNormal := &NGon{
		Vertices: []Point3{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}},
		Normal:   Vector3{},
	}
	if !hasIssue(ValidateNGon(zeroNormal), "GEO-NGON-NORMAL") {
		t.Fatal("zero normal must be flagged")
	}
	mismatch := &NGon{
		Vertices: []Point3{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}},
		Normal:   Vector3{X: 0, Y: 0, Z: -1},
	}
	if !hasIssue(ValidateNGon(mismatch), "GEO-NGON-NORMAL-MISMATCH") {
		t.Fatal("opposite normal must be flagged")
	}
	collinear := &NGon{
		Vertices: []Point3{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 20, Y: 0}, {X: 30, Y: 0}},
		Normal:   Vector3{X: 0, Y: 0, Z: 1},
	}
	if !hasIssue(ValidateNGon(collinear), "GEO-NGON-COLLINEAR") {
		t.Fatal("collinear vertices must be flagged")
	}
}
