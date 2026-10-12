package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

func TestSocketProbe(t *testing.T) {
	parts := railHolderAt(ProductionQuality, 115, 841, 900, 0.75)
	post := tubeAlong(ProductionQuality,
		kerngeo.NewPoint3(115, 841, 0), kerngeo.NewPoint3(115, 841, 900),
		balusterD, balusterWall, roleBaluster)
	if post == nil {
		t.Fatal("post nil")
	}
	pb := kerngeo.SolidBoundingBox(post)
	t.Logf("стойка: XY %.1fx%.1f", (pb.Max.X - pb.Min.X), (pb.Max.Y - pb.Min.Y))
	for i, p := range parts {
		b := kerngeo.SolidBoundingBox(p)
		t.Logf("part %d: XY %.1f  Z %.0f..%.0f", i, (pb.Max.X - pb.Min.X), b.Min.Z, b.Max.Z)
	}
}
