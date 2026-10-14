package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
)

// Стальное ограждение не должно содержать стекла, стеклянное — ригелей.
// Это разделение типов, а не вариантов заполнения.
func TestRailingTypesDoNotMix(t *testing.T) {
	base := *testConfig(t)
	base.Railing = engineering.RailingRight
	base.RailingHeight = mustLength(t, 900)

	steel := base
	steel.RailingType = engineering.RailingTypeSteel
	sd, err := BuildRailingDecor(&steel)
	if err != nil {
		t.Fatalf("steel: %v", err)
	}
	if n := len(solidsByRole(sd, roleRailingGlass)); n != 0 {
		t.Errorf("steel railing has %d glass panels, want 0", n)
	}
	if r := len(solidsByRole(sd, roleRigel)); r == 0 {
		t.Error("steel railing has no rigels")
	}
	if h := len(solidsByRole(sd, roleHolder)); h == 0 {
		t.Error("steel railing has no holders")
	}

	gl := base
	gl.RailingType = engineering.RailingTypeGlass
	gd, err := BuildRailingDecor(&gl)
	if err != nil {
		t.Fatalf("glass: %v", err)
	}
	if r := len(solidsByRole(gd, roleRigel)); r != 0 {
		t.Errorf("glass railing has %d rigels, want 0", r)
	}
	if p := len(solidsByRole(gd, roleRailingGlass)); p == 0 {
		t.Error("glass railing has no glass panels")
	}

	// пустой тип = стальное
	empty := base
	empty.RailingType = ""
	ed, err := BuildRailingDecor(&empty)
	if err != nil {
		t.Fatalf("empty type: %v", err)
	}
	if len(solidsByRole(ed, roleRailingGlass)) != 0 {
		t.Error("empty railing type must default to steel, but glass was produced")
	}
}
