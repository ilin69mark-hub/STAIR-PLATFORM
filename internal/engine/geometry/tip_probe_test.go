package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

func TestHolderTipProbe(t *testing.T) {
	parts := railHolderAt(ProductionQuality, 115, 841, 900, 1000, 0.75)
	for i, p := range parts {
		b := kerngeo.SolidBoundingBox(p)
		v, _ := kerngeo.Volume(p)
		t.Logf("part %d: Z %.1f..%.1f (h %.1f) XY %.1fx%.1f vol %.1f", i, b.Min.Z, b.Max.Z, (b.Max.Z - b.Min.Z), (b.Max.X - b.Min.X), (b.Max.Y - b.Min.Y), v)
	}
	cfg := *testConfig(t)
	cfg.Railing = engineering.RailingRight
	cfg.RailingHeight = mustLength(t, 900)
	cfg.RailingType = engineering.RailingTypeSteel
	decor, _ := BuildRailingDecor(&cfg)
	var tips int
	for _, s := range decor {
		b := kerngeo.SolidBoundingBox(s)
		if (b.Max.X-b.Min.X) < 25 && (b.Max.Y-b.Min.Y) < 25 && (b.Max.Z-b.Min.Z) < 20 && b.Max.Z > 950 {
			tips++
		}
	}
	t.Logf("мелких деталей наверху опоры: %d", tips)
}
