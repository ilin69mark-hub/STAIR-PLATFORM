package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

func TestPostAssemblyCentreProbe(t *testing.T) {
	cfg := *testConfig(t)
	cfg.Railing = engineering.RailingRight
	cfg.RailingHeight = mustLength(t, 900)
	cfg.RailingType = engineering.RailingTypeSteel
	decor, err := BuildRailingDecor(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	// первая опора: все детали узла в окрестности её оси
	first := 1e9
	for _, s := range decor {
		b := kerngeo.SolidBoundingBox(s)
		if b.Min.X < first && b.Max.X < 600 {
			first = b.Min.X
		}
	}
	var near []*kerngeo.Solid
	for _, s := range decor {
		b := kerngeo.SolidBoundingBox(s)
		// Только детали ОПОРЫ: ригели и поручень тянутся вдоль марша и
		// ломают bbox узла.
		switch s.Role() {
		case roleBaluster, roleClamp, roleHolder:
			if b.Max.X < first+400 {
				near = append(near, s)
			}
		}
	}
	xmin, xmax := 1e9, -1e9
	for _, s := range near {
		b := kerngeo.SolidBoundingBox(s)
		if b.Min.X < xmin {
			xmin = b.Min.X
		}
		if b.Max.X > xmax {
			xmax = b.Max.X
		}
	}
	t.Logf("узел первой опоры: X %.1f..%.1f, центр %.1f (деталей %d)", xmin, xmax, (xmin+xmax)/2, len(near))
}
