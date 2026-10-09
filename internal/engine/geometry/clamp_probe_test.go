package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

func TestClampAndCapPositiveVolume(t *testing.T) {
	x, y := 405.0, 878.8
	for _, c := range railClampAt(x, y, 180) {
		v, err := kerngeo.Volume(c)
		if err != nil {
			t.Fatalf("clamp volume err: %v", err)
		}
		t.Logf("clamp part %s vol %.1f", c.Role(), v)
		if v <= 0 {
			t.Fatalf("clamp part has non-positive volume %.1f", v)
		}
	}
	caps := railCapAt(x, y, 180)
	if len(caps) != 2 {
		t.Fatalf("cap parts = %d, want 2 (body + lid)", len(caps))
	}
	for i, cp := range caps {
		v, verr := kerngeo.Volume(cp)
		t.Logf("cap part %d vol %.1f err %v", i, v, verr)
		if v <= 0 {
			t.Fatalf("cap part %d volume %.1f", i, v)
		}
	}
}
