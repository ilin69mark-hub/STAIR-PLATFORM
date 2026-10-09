package variation

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// Обогащение вариантов ForRoomFit.
//
// Последствие удаления винтового марша (решение владельца 2026-10-06): на
// тесных помещениях самым компактным типом остался П-образный, и помещения,
// куда раньше влезала только спираль, теперь честно дают «вариантов нет».
// Раньше эти тесты держались именно на спирали — она была единственным
// ответом для комнат 1650×1100 и 2500×1800 мм.

// fitTolMM — запас, при котором вариант помечается «впритык» (см.
// tryGenerateMeta: summary с «впритык» появляется при tol > fitEps).

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
	c.RoomWidth = 4000
	c.RoomLength = 1100
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	// Комната 4000×1100 влезает для П-образного марша (габарит 3526×1100) и
	// не влезает ни для L, ни для прямого, поэтому предлагаются два варианта
	// одного типа — ближайший и самый компактный.
	if len(vars) != 2 {
		t.Fatalf("expected 2 variants (nearest + compact), got %d", len(vars))
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
	// Комната 3240×850: лучший кандидат — П-образный марш габаритом
	// 3276×600 мм, то есть шире комнаты на 36 мм. Это в пределах
	// допустимого запаса, поэтому вариант предлагается с честной пометкой.
	c.RoomWidth = 3240
	c.RoomLength = 850
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	if len(vars) == 0 {
		t.Fatal("expected near-fit fallback variant, got 0")
	}
	if !strings.Contains(vars[0].Summary, "впритык") {
		t.Fatalf("expected «впритык» note in summary, got: %s", vars[0].Summary)
	}
}

// TestRoomFit_LShapeTightRoom_AtLeastCompactUShape — в тесном помещении, где
// сам L-марш не помещается, предлагается смена типа на более компактный
// П-образный, а не сужение того же L («D:»).
func TestRoomFit_LShapeTightRoom_AtLeastCompactUShape(t *testing.T) {
	c := straightBase()
	c.Flight = engineering.FlightLShape
	c.Height = 2700
	c.StepHeight = 175
	c.StepCount = 16
	c.LowerStepCount = 8
	c.LandingWidth = 1000
	c.LandingDepth = 1000
	c.RoomWidth = 3400
	c.RoomLength = 1800
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	if len(vars) == 0 {
		t.Fatal("expected at least one cross-type variant, got 0")
	}
	if strings.HasPrefix(vars[0].Title, "D:") {
		t.Fatalf("expected cross-type change FIRST, got: %v", titlesOf(vars))
	}
}

// TestRoomFit_NoTypeFits_ReturnsNothing — комната 2500×1800 мм раньше
// влезала только под спираль: винтовой марш диаметром ~2.6 м был самым
// компактным из четырёх. После его удаления такого типа больше нет, и
// движок честно возвращает пустой список, а не выдумывает невписывающийся
// вариант. Тест фиксирует именно это следствие решения владельца.
func TestRoomFit_NoTypeFits_ReturnsNothing(t *testing.T) {
	c := straightBase()
	c.RoomWidth = 2500
	c.RoomLength = 1800
	vars := ForRoomFit(context.Background(), c, constraint.StandardProfile("STANDARD"))
	if len(vars) != 0 {
		t.Fatalf("room that only the removed spiral could fit must yield no variants, got %d: %v",
			len(vars), titlesOf(vars))
	}
}

func titlesOf(vars []validation.Variation) []string {
	out := make([]string, 0, len(vars))
	for _, v := range vars {
		out = append(out, v.Title)
	}
	return out
}
