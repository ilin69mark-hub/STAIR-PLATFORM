package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

func TestTreadGeometryProbe(t *testing.T) {
	cfg := testConfig(t)
	b := cfg.TreadDepth.Millimeters()
	st := cfg.StepThickness.Millimeters()
	t.Logf("tread_depth(b)=%.0f step_thickness=%.0f", b, st)
	comp, err := BuildStraightFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range comp.Solids() {
		if s.Role() != "tread" {
			continue
		}
		n++
		if n > 3 {
			break
		}
		bb := kerngeo.SolidBoundingBox(s)
		t.Logf("tread %d: X %.1f..%.1f (длина %.1f), Y %.1f..%.1f, Z %.1f..%.1f",
			n, bb.Min.X, bb.Max.X, (bb.Max.X-bb.Min.X), bb.Min.Y, bb.Max.Y, bb.Min.Z, bb.Max.Z)
	}
	t.Logf("всего ступеней: %d", n)
}
