package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

func TestExtraRailingSidesUnknown(t *testing.T) {
	if sides := railingSides("bogus", 900); sides != nil {
		t.Fatalf("unknown side must yield nil, got %v", sides)
	}
}

func TestExtraLandingInsetOffEdge(t *testing.T) {
	p := kerngeo.NewPoint3(5, 5, 10)
	got := landingInset(p, 0, 900, 1000, 20)
	if got != p {
		t.Fatalf("off-edge point must be unchanged, got %v", got)
	}
}

func TestExtraFlipSideUnknown(t *testing.T) {
	if got := flipSide("bogus"); got != "bogus" {
		t.Fatalf("unknown side must pass through, got %v", got)
	}
}

func TestExtraRailAlongDegenerate(t *testing.T) {
	a := kerngeo.NewPoint3(100, 200, 300)
	if s := railAlong(a, a); s != nil {
		t.Fatal("degenerate segment must yield nil")
	}
}

func TestExtraRailAlongVertical(t *testing.T) {
	// Вертикальный сегмент: направление ширины wd=(−u.Y,u.X,0) вырождено
	// → фолбэк wd=(1,0,0).
	s := railAlong(kerngeo.NewPoint3(0, 0, 0), kerngeo.NewPoint3(0, 0, 1000))
	if s == nil {
		t.Fatal("vertical rail must be built")
	}
	if s.Role() != roleRailing {
		t.Fatalf("role = %s", s.Role())
	}
}

func TestExtraRailAlongMainPath(t *testing.T) {
	s := railAlong(kerngeo.NewPoint3(0, 0, 0), kerngeo.NewPoint3(2700, 0, 1800))
	if s == nil {
		t.Fatal("inclined rail must be built")
	}
}

func TestExtraBalusterAtZeroHeight(t *testing.T) {
	if s := balusterAt(10, 10, 0, 0); s != nil {
		t.Fatal("zero height must yield nil")
	}
}

func TestExtraBalusterAtSlopedZeroHeight(t *testing.T) {
	if s := balusterAtSloped(10, 10, 0, 0, 0.2); s != nil {
		t.Fatal("zero height must yield nil")
	}
}

func TestExtraLandingRailingNoHeight(t *testing.T) {
	if sols := landingRailingSolids(900, 1000, 0, 1080, 270, 0, false, false, engineering.RailingBoth); sols != nil {
		t.Fatalf("zero rail height must yield nil, got %d", len(sols))
	}
}

func TestExtraWinderRailing(t *testing.T) {
	if sols := winderRailingSolids(900, 180, 0, 6, 3, 1620, 1000, false); sols != nil {
		t.Fatalf("zero rail height must yield nil, got %d", len(sols))
	}
	for _, left := range []bool{false, true} {
		sols := winderRailingSolids(900, 180, 900, 6, 3, 1620, 1000, left)
		if len(sols) == 0 {
			t.Fatalf("left=%v: expected winder rail solids, got 0", left)
		}
	}
}

func TestExtraBuildRailingDecorWinder(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.TurnKind = engineering.TurnWinder
	cfg.WinderCount = 3
	cfg.RailingHeight = mustLength(t, 900)
	cfg.RailingLower = engineering.RailingBoth
	cfg.RailingUpper = engineering.RailingBoth
	cfg.RailingLanding = engineering.RailingRight
	sols, err := BuildRailingDecor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(sols) == 0 {
		t.Fatal("expected railing solids, got 0")
	}
}

func TestExtraBuildRailingDecorUnknownFlight(t *testing.T) {
	cfg := testConfig(t)
	cfg.Flight = "bogus"
	cfg.RailingHeight = mustLength(t, 900)
	if _, err := BuildRailingDecor(cfg); err == nil {
		t.Fatal("unknown flight must be rejected")
	}
}
