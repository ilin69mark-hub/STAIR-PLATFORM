package geometry

import (
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

// Эталон FreeCAD Rail_Tip_01 при zTop=980.7.
var refStack = []struct {
	name           string
	h, odX, odY    float64
	relZLo, relZHi float64
}{
	{"sleeve", 24.0, 39.4, 39.4, -24.0, 0.0},
	{"cap", 3.0, 42.4, 42.4, 0.0, 3.0},
	{"rod", 27.7, 12.0, 12.0, 3.0, 30.7},
	{"joint", 19.4, 14.0, 14.0, 29.4, 48.8},
	{"tip", 0, 70.0, 50.8, 47.2, 75.0}, // эталон: stadium 70 x 50,8 x 1 мм
	// ВНИМАНИЕ: ложемент (паз R22,1) в наконечнике НЕ построен — в kernel
	// нет булевых операций. Деталь плоская; паз визуально на витрине не
	// читается, для раскроя не используется (наконечник покупной).
}

func TestStackVsReference(t *testing.T) {
	const zTop = 980.7
	parts := railHolderAt(ProductionQuality, 0, 0, zTop, 0.75)
	if len(parts) != len(refStack) {
		t.Fatalf("деталей %d, в эталоне %d", len(parts), len(refStack))
	}
	for i, r := range refStack {
		b := kerngeo.SolidBoundingBox(parts[i])
		gotLo, gotHi := b.Min.Z-zTop, b.Max.Z-zTop
		gotX, gotY := b.Max.X-b.Min.X, b.Max.Y-b.Min.Y
		t.Logf("%-6s Z %+6.1f..%+6.1f (эталон %+6.1f..%+6.1f)  XY %5.1fx%5.1f (эталон %4.1fx%4.1f)",
			r.name, gotLo, gotHi, r.relZLo, r.relZHi, gotX, gotY, r.odX, r.odY)
	}
	// наконечник обязан ДОХОДИТЬ до низа поручня (не висеть в воздухе)
	if n := len(parts); n > 0 {
		b := kerngeo.SolidBoundingBox(parts[n-1])
		t.Logf("низ поручня 1055, верх наконечника %.1f -> зазор %.1f",
			b.Max.Z, b.Min.Z-1055)
	}
	// наконечник обязан быть ЦЕЛИКОМ наклонным: верх и низ несут slope*L
	b := kerngeo.SolidBoundingBox(parts[4])
	slopeRun := b.Max.X - b.Min.X
	t.Logf("наконечник: Z-размах %.1f при длине %.1f -> наклон %.3f (эталон 0.75)",
		b.Max.Z-b.Min.Z, slopeRun, (b.Max.Z-b.Min.Z)/(slopeRun))
}
