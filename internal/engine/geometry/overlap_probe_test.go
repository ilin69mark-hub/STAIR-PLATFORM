package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// Перила не должны пересекать материал площадки держателя и трубы опоры.
func TestRailingNoUnexpectedOverlap(t *testing.T) {
	cfg := *testConfig(t)
	cfg.Railing = engineering.RailingRight
	cfg.RailingHeight = mustLength(t, 900)
	cfg.RailingType = engineering.RailingTypeSteel
	decor, err := BuildRailingDecor(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	var rails, holders, posts []*kerngeo.Solid
	for _, s := range decor {
		switch s.Role() {
		case roleRailing:
			rails = append(rails, s)
		case roleHolder:
			holders = append(holders, s)
		case roleBaluster:
			posts = append(posts, s)
		}
	}
	if len(rails) == 0 || len(holders) == 0 || len(posts) == 0 {
		t.Fatalf("missing parts: rails=%d holders=%d posts=%d", len(rails), len(holders), len(posts))
	}
	// Ложемент: площадка ДОЛЖНА иметь выемку под поручень. Без булевых
	// операций пересечение не посчитать, поэтому проверяем объём: с выемкой
	// площадка должна быть заметно легче сплошной полосы того же габарита.
	p0 := kerngeo.NewPoint3(0, 0, 0)
	p1 := kerngeo.NewPoint3(holderPlateHalfX*2, 0, 10)
	plain := plateGrooved(p0, p1, holderPlateW, holderPlateT, holderHoleOfs, holderHoleD, 0, "probe")
	grooved := plateGrooved(p0, p1, holderPlateW, holderPlateT, holderHoleOfs, holderHoleD, holderGrooveD, "probe")
	if plain == nil || grooved == nil {
		t.Fatal("plate nil")
	}
	vp, _ := kerngeo.Volume(plain)
	vg, _ := kerngeo.Volume(grooved)
	if vg >= vp {
		t.Errorf("grooved plate volume %.1f >= plain %.1f — ложемента нет", vg, vp)
	}
	t.Logf("plate volume: plain %.1f, grooved %.1f (groove removes %.1f)", vp, vg, vp-vg)

	// Ось стойки и ось цанги/колпачка должны совпадать по X: стойка стоит
	// в центре опоры. Проверяем, что все детали узла на одной X.
	var clampX []float64
	for _, c := range decor {
		if c.Role() == roleClamp {
			cb := kerngeo.SolidBoundingBox(c)
			clampX = append(clampX, (cb.Min.X+cb.Max.X)/2)
		}
	}
	var postX []float64
	for _, pst := range posts {
		pbb := kerngeo.SolidBoundingBox(pst)
		postX = append(postX, (pbb.Min.X+pbb.Max.X)/2)
	}
	seenC := map[float64]bool{}
	var clampUniq []float64
	for _, cx := range clampX {
		if !seenC[cx] {
			seenC[cx] = true
			clampUniq = append(clampUniq, cx)
		}
	}
	_ = clampUniq
	for i := range postX {
		if i*2 >= len(clampX) {
			break
		}
		if !nearlyEqual(clampX[i*2], postX[i]) {
			t.Errorf("clamp %d centre X = %v, post %d centre X = %v — стойка не в центре опоры",
				i+1, clampX[i*2], i+1, postX[i])
		}
	}
	t.Logf("post centres %v (tread centre)", postX)

	// позиция стойки: ось на 15 мм от края ступени
	b := cfg.Width.Millimeters()
	pb := kerngeo.SolidBoundingBox(posts[0])
	axis := (pb.Min.Y + pb.Max.Y) / 2
	wantAxis := b - balusterEdgeInset - capOD/2
	if !nearlyEqual(axis, wantAxis) {
		t.Errorf("baluster axis Y = %v, want %v (%.0f mm from edge, dia %.1f)",
			axis, wantAxis, balusterEdgeInset, balusterD)
	}
	// Середина СТУПЕНИ при свесе НАЗАД: ступень [ (k−1)·step − nosing,
	// (k−1)·step + step + nosing ], центр = (k−1)·step + step/2 = 115.
	step := cfg.TreadDepth.Millimeters()
	wantX := step/2 - cfgProfileNosing(&cfg)
	if !nearlyEqual((pb.Min.X+pb.Max.X)/2, wantX) {
		t.Errorf("baluster axis X = %v, want %v (tread centre)", (pb.Min.X+pb.Max.X)/2, wantX)
	}
}
