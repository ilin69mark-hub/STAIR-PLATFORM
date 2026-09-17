package geometry

import (
	"math"
	"testing"
)

func TestTransformRotateX(t *testing.T) {
	rx := RotateX(math.Pi / 2)
	p := rx.Apply(NewPoint3(0, 1, 0))
	if !nearlyEqual(p.Z, 1) || !nearlyEqual(p.Y, 0) {
		t.Fatalf("rotateX(90°) applied to (0,1,0) = %+v", p)
	}
	if b := rx.Apply(rx.Apply(NewPoint3(0, 2, 0))); !nearlyEqual(b.Y, -2) || !nearlyEqual(b.Z, 0) {
		t.Fatalf("rotateX(90°) twice = %+v", b)
	}
}

func TestTransformRotateY(t *testing.T) {
	ry := RotateY(math.Pi / 2)
	p := ry.Apply(NewPoint3(0, 0, 1))
	if !nearlyEqual(p.X, 1) || !nearlyEqual(p.Z, 0) {
		t.Fatalf("rotateY(90°) applied to (0,0,1) = %+v", p)
	}
}

func TestTransformRotateZCardinalSnap(t *testing.T) {
	r0 := RotateZ(0)
	if r0 != Identity() {
		t.Fatalf("rotateZ(0) must be identity, got %+v", r0)
	}
	r90 := RotateZ(math.Pi / 2)
	if p := r90.Apply(NewPoint3(1, 0, 0)); !nearlyEqual(p.X, 0) || !nearlyEqual(p.Y, 1) {
		t.Fatalf("rotateZ(90°) applied to (1,0,0) = %+v", p)
	}
	if p := r90.Apply(NewPoint3(0, 1, 0)); !nearlyEqual(p.X, -1) || !nearlyEqual(p.Y, 0) {
		t.Fatalf("rotateZ(90°) applied to (0,1,0) = %+v", p)
	}
	r180 := RotateZ(math.Pi)
	if p := r180.Apply(NewPoint3(1, 0, 0)); !nearlyEqual(p.X, -1) || !nearlyEqual(p.Y, 0) {
		t.Fatalf("rotateZ(180°) applied to (1,0,0) = %+v", p)
	}
}

func TestNearUnit(t *testing.T) {
	if !nearUnit(0) || !nearUnit(1) || !nearUnit(-1) {
		t.Fatal("0, ±1 must be near unit")
	}
	if nearUnit(0.5) {
		t.Fatal("0.5 must not be near unit")
	}
}

func TestTransformInversePivotSwap(t *testing.T) {
	m := Transform{
		{0, 1, 0, 0},
		{1, 0, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
	}
	inv, err := m.Inverse()
	if err != nil {
		t.Fatal(err)
	}
	if inv != m {
		t.Fatalf("swap matrix must be self-inverse, got %+v", inv)
	}
}
