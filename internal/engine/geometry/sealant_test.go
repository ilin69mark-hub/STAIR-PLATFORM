package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
)

// Тесты площади герметизации (CONF-SEALANT).
//
// Проверяется главное: площадь берётся из РАЗМЕРОВ ОБЙМЫ ВЫРЕЗА, а не из
// габаритов лестницы. Если площадь посчитать по площади ступени, расход
// вырастет в разы, а если забыть убрать швы подступенка при Riser = false —
// герметик попадёт на несуществующие стыки.

func sealantCfg(riser bool) *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width: 900, Height: 2700, Flight: engineering.FlightStraight,
		StepCount: 15, StepHeight: 180, TreadDepth: 270,
		StringerThickness: 8, StepThickness: 5, Riser: riser,
		Clearance: 2000, RailingHeight: 900, ApproachSpace: 1000,
		Material: "STEEL-S235", TreadMaterial: "STEEL-S235",
		Comb: engineering.CombFrameDefaults(),
	}
}

// TestSealantAreasMatchCombSizes — площади стыков считаются по размерам
// пяток обоймы выреза.
//
// Для эталона (n = 15, b = 270, h = 180, st = 5, HeelWidth = 50,
// HeelInset = 5, HeelThickness = 4, HeelGap = 5, t = 8, StringerSpine = 300):
//
//	ступень ↔ пятка 1:  2 × 15 × (270−2×5) × 50   = 390 000 мм²
//	подступенок ↔ пятка 2: 2 × 14 × (180−4−2×5) × 50 = 232 400 мм²
//	подступенок №1 ↔ пластина: 2 × (175−8−10) × 40  =  12 560 мм²
//	торец косоура:   2 косоура × 2 торца × 300 × 8 =   9 600 мм²
func TestSealantAreasMatchCombSizes(t *testing.T) {
	a := SealantContactAreas(sealantCfg(true))
	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"ступень к пятке 1", a.TreadHeelsMM2, 390000},
		{"подступенок к пятке 2", a.RiserWebsMM2, 232400},
		{"подступенок №1 к пластине", a.RiserFirstWebsMM2, 12560},
		{"торец косоура", a.StringerEndsMM2, 9600},
		{"итого", a.TotalMM2(), 644560},
	}
	for _, c := range checks {
		if diff := c.got - c.want; diff > 0.5 || diff < -0.5 {
			t.Errorf("%s: площадь %.0f мм², want %.0f", c.name, c.got, c.want)
		}
	}
}

// TestSealantAreaScalesWithSteps — больше ступеней → больше швов. Расход
// обязан расти вместе с маршем, иначе на длинной лестнице герметика не
// хватит.
func TestSealantAreaScalesWithSteps(t *testing.T) {
	short := sealantCfg(true)
	short.StepCount, short.Height = 10, 1800
	long := sealantCfg(true)
	long.StepCount, long.Height = 20, 3600
	a1, a2 := SealantContactAreas(short), SealantContactAreas(long)
	if a2.TreadHeelsMM2 <= a1.TreadHeelsMM2 {
		t.Fatalf("площадь швов не выросла: %.0f → %.0f", a1.TreadHeelsMM2, a2.TreadHeelsMM2)
	}
	if a2.TotalMM2() <= 1.5*a1.TotalMM2() {
		t.Fatalf("рост числа ступеней не дал пропорционального роста площади: %.0f → %.0f", a1.TotalMM2(), a2.TotalMM2())
	}
}

// TestSealantWithoutRiserSkipsRiserJoints — без подступенка пятки 2 и
// пластина 1-й ступени не строятся. Герметик по несуществующим швам — это
// прямой переплаты, поэтому площади этих зон обязаны быть нулевыми.
func TestSealantWithoutRiserSkipsRiserJoints(t *testing.T) {
	a := SealantContactAreas(sealantCfg(false))
	if a.RiserWebsMM2 != 0 || a.RiserFirstWebsMM2 != 0 {
		t.Fatalf("без подступенка площади швов подступенка не должны считаться: пятка2=%v пластина=%v",
			a.RiserWebsMM2, a.RiserFirstWebsMM2)
	}
	if a.TreadHeelsMM2 <= 0 {
		t.Fatal("площадь шва ступень↔пятка должна считаться всегда")
	}
	if a.TotalMM2() >= SealantContactAreas(sealantCfg(true)).TotalMM2() {
		t.Fatal("без подступенка площадь должна быть меньше")
	}
}

// TestSealantNoStringerEndsForCombFrame — торец косоура считается только для
// пластинчатого каркаса. У гребёнки из дерева торцы профилируются по зубьям,
// и сечение пластины к ним отношения не имеет.
func TestSealantNoStringerEndsForCombFrame(t *testing.T) {
	wood := sealantCfg(true)
	wood.Material, wood.StringerThickness, wood.StepThickness = "WOOD-OAK", 60, 40
	if a := SealantContactAreas(wood); a.StringerEndsMM2 != 0 {
		t.Fatalf("для гребёнки торец косоура считаться не должен, получено %v мм²", a.StringerEndsMM2)
	}
}

// TestSealantEmptyConfigDoesNotPanic — пустая конфигурация возвращает нули,
// а не падает: расход вызывается на каждом шаге конвейера.
func TestSealantEmptyConfigDoesNotPanic(t *testing.T) {
	if a := SealantContactAreas(nil); a.TotalMM2() != 0 {
		t.Fatalf("nil-конфигурация: площадь %v, want 0", a.TotalMM2())
	}
	if a := SealantContactAreas(&engineering.StairConfiguration{}); a.TotalMM2() != 0 {
		t.Fatalf("пустая конфигурация: площадь %v, want 0", a.TotalMM2())
	}
}