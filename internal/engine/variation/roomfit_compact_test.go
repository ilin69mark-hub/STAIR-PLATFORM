package variation

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// S-P6: обогащение вариантов ForRoomFit. ВАЖНО: на тесных помещениях
// физически может помещаться только один тип (спираль) — движок обязан
// предложить его варианты, а не вернуть «вариантов нет» без причины.

// straightBase — типовая прямая лестница (H=2600, 15 ступеней) с уже
// согласованными с солвером параметрами (как после checkedSolve): при
// H=2600 и шаге комфорта 630 нормативная проступь ≥260 достижима только
// при n=15 (h≈173). «Сырой» StepHeight=180 давал бы n=14/h=185.7 → проступь
// 258.6 <260 → кандидаты ложно блокировались бы по норме.
func straightBase() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             1000,
		Height:            2600,
		Flight:            engineering.FlightStraight,
		Riser:             true,
		StepCount:         15,
		StepHeight:        173,
		TreadDepth:        283,
		Length:            630,
		StringerThickness: 40,
		StepThickness:     6,
		Clearance:         1900,
		RailingHeight:     900,
		ApproachSpace:     1000,
	}
}

// TestRoomFit_UpToTwoPerType — на одном типе возвращается до двух
// вариантов (ближайший к исходному И самый компактный), а не один.
func TestRoomFit_UpToTwoPerType(t *testing.T) {
	c := straightBase()
	c.RoomWidth = 2500
	c.RoomLength = 1600
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	if len(vars) != 2 {
		t.Fatalf("expected 2 variants (nearest+compact spiral), got %d", len(vars))
	}
	for i, v := range vars {
		if !v.Fits {
			t.Fatalf("variant %d не вписывается (fit=false): %s", i, v.Title)
		}
	}
}

// TestRoomFit_SpiralBase_OffersCompactD — для исходной спирали, не
// помещающейся в помещение, предлагается «D: спираль компактнее» (более
// узкий марш и меньший радиус) — ранние версии возвращали «вариантов нет».
func TestRoomFit_SpiralBase_OffersCompactD(t *testing.T) {
	cfg := &engineering.StairConfiguration{
		Width:             800,
		Height:            2600,
		Flight:            engineering.FlightSpiral,
		Riser:             true,
		StepCount:         15,
		StepHeight:        185,
		OuterRadius:       1400,
		Length:            630,
		StringerThickness: 40,
		StepThickness:     6,
		Clearance:         1900,
		RailingHeight:     900,
		ApproachSpace:     1000,
		RoomWidth:         2000,
		RoomLength:        1500,
	}
	vars := ForRoomFit(context.Background(), cfg, constraint.StandardProfile("STANDARD"))
	if len(vars) == 0 {
		t.Fatal("expected compact-spiral variants, got 0")
	}
	hasCompact := false
	for _, v := range vars {
		if strings.HasPrefix(v.Title, "D: спираль") {
			hasCompact = true
		}
		if !v.Fits {
			t.Fatalf("variant не вписывается: %s", v.Title)
		}
	}
	if !hasCompact {
		t.Fatalf("expected «D: спираль компактнее», got: %v", titlesOf(vars))
	}
}

// TestRoomFit_NarrowerSameType — узкий марш того же типа предлагается для
// слишком широкой исходной лестницы (длина влезает, ширины помещения мало).
func TestRoomFit_NarrowerSameType(t *testing.T) {
	c := straightBase()
	c.RoomWidth = 6000
	c.RoomLength = 850
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	if len(vars) == 0 {
		t.Fatal("expected variants, got 0")
	}
	hasNarrow := false
	for _, v := range vars {
		if strings.HasPrefix(v.Title, "D: сделать марш") {
			hasNarrow = true
		}
	}
	if !hasNarrow {
		t.Fatalf("expected «D: сделать марш уже», got: %v", titlesOf(vars))
	}
}

// TestRoomFit_NearFitFallback — если ни один вариант не вписывается строго,
// но есть кандидат с запасом менее 50 мм, он возвращается с пометкой
// «впритык» (ранние версии возвращали пустой список).
func TestRoomFit_NearFitFallback(t *testing.T) {
	c := straightBase()
	c.RoomWidth = 1650
	c.RoomLength = 1100
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	if len(vars) == 0 {
		t.Fatal("expected near-fit fallback variant, got 0")
	}
	if !strings.Contains(vars[0].Summary, "впритык") {
		t.Fatalf("expected «впритык» note in summary, got: %s", vars[0].Summary)
	}
}

// TestRoomFit_LShapeTightRoom_AtLeastSpiral — в тесном помещении хотя бы
// компактный вариант спирали предлагается (раньше — только один).
func TestRoomFit_LShapeTightRoom_AtLeastSpiral(t *testing.T) {
	c := straightBase()
	c.Flight = engineering.FlightLShape
	c.Height = 2700
	c.StepHeight = 175
	c.StepCount = 16
	c.LowerStepCount = 8
	c.LandingWidth = 1000
	c.LandingDepth = 1000
	c.RoomWidth = 2500
	c.RoomLength = 1800
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	if len(vars) == 0 {
		t.Fatal("expected at least a spiral variant, got 0")
	}
	if strings.HasPrefix(vars[0].Title, "D:") {
		t.Fatalf("expected cross-type spiral FIRST, got: %v", titlesOf(vars))
	}
}

func titlesOf(vars []validation.Variation) []string {
	out := make([]string, 0, len(vars))
	for _, v := range vars {
		out = append(out, v.Title)
	}
	return out
}
