package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

func TestPlateTiltProbe(t *testing.T) {
	const L, W, T, nos, off, d, gr = 70.0, 50.8, 3.0, 20.0, 15.0, 5.0, 44.2
	slope := 0.75
	p0 := kerngeo.NewPoint3(-L/2, 0, 0)
	p1 := kerngeo.NewPoint3(L/2, 0, slope*L)
	s := plateGrooved(p0, p1, W, T, off, d, gr, "probe")
	if s == nil {
		t.Fatal("nil")
	}
	b := kerngeo.SolidBoundingBox(s)
	t.Logf("bbox X %.1f..%.1f  Y %.1f..%.1f  Z %.1f..%.1f", b.Min.X, b.Max.X, b.Min.Y, b.Max.Y, b.Min.Z, b.Max.Z)
}
