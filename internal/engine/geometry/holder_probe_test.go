package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

func TestRailHolderHasAllParts(t *testing.T) {
	parts := railHolderAt(ProductionQuality, 100, 878.8, 900, 0.75)
	seen := map[string]int{}
	for _, p := range parts {
		if p == nil {
			t.Fatal("nil part in holder")
		}
		v, err := kerngeo.Volume(p)
		if err != nil || v <= 0 {
			t.Fatalf("bad volume %v err %v", v, err)
		}
		seen[p.Role()]++
	}
	// втулка, крышка, шпилька, корпус шарнира, площадка
	if len(parts) != 5 {
		t.Fatalf("holder parts = %d, want 5", len(parts))
	}
	t.Logf("holder parts: %d, roles %v", len(parts), seen)
}

func TestRailEndCapsAreInsideRail(t *testing.T) {
	a := kerngeo.NewPoint3(0, 900, 900)
	b := kerngeo.NewPoint3(4000, 900, 900)
	caps := railEndCaps(ProductionQuality, a, b)
	if len(caps) != 2 {
		t.Fatalf("end caps = %d, want 2", len(caps))
	}
	for i, c := range caps {
		bb := kerngeo.SolidBoundingBox(c)
		if bb.Min.X < a.X-0.01 || bb.Max.X > b.X+0.01 {
			t.Errorf("cap %d X [%v,%v] not inside rail span", i, bb.Min.X, bb.Max.X)
		}
	}
}
