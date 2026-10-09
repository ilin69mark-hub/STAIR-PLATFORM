package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

func TestTubeAlongIsHollow(t *testing.T) {
	a := kerngeo.NewPoint3(0, 0, 0)
	b := kerngeo.NewPoint3(0, 0, 100)
	s := tubeAlong(a, b, 42.4, 1.5, "probe")
	if s == nil {
		t.Fatal("tubeAlong returned nil")
	}
	v, err := kerngeo.Volume(s)
	if err != nil {
		t.Fatalf("volume: %v", err)
	}
	solid := 3.14159265 * 21.2 * 21.2 * 100
	hollow := 3.14159265 * (21.2*21.2 - 19.7*19.7) * 100
	if v > solid*0.9 {
		t.Fatalf("volume %.1f too close to SOLID cylinder %.1f — полость не построена", v, solid)
	}
	if v < hollow*0.85 || v > hollow*1.15 {
		t.Fatalf("volume %.1f, want near hollow %.1f", v, hollow)
	}
	t.Logf("volume %.1f, solid would be %.1f, hollow %.1f", v, solid, hollow)
}

func TestRigelIsHollow(t *testing.T) {
	a := kerngeo.NewPoint3(0, 0, 0)
	b := kerngeo.NewPoint3(1200, 0, 0)
	s := tubeAlong(a, b, rigelD, rigelWall, roleRigel)
	if s == nil {
		t.Fatal("rigel nil")
	}
	v, err := kerngeo.Volume(s)
	if err != nil {
		t.Fatalf("volume: %v", err)
	}
	solid := 3.14159265 * 6 * 6 * 1200
	hollow := 3.14159265 * (36 - 25) * 1200
	if v > solid*0.9 {
		t.Fatalf("rigel volume %.1f looks solid (solid %.1f)", v, solid)
	}
	if v < hollow*0.8 || v > hollow*1.2 {
		t.Fatalf("rigel volume %.1f, want near hollow %.1f", v, hollow)
	}
	t.Logf("rigel volume %.1f, solid %.1f, hollow %.1f", v, solid, hollow)
}
