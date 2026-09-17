package geometry

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

func TestExtraStringerExtent(t *testing.T) {
	if got := StringerExtent(2700, 40); got != 2660 {
		t.Fatalf("StringerExtent = %v, want 2660", got)
	}
}

func TestExtraBuildUShapeFlightDispatchWinder(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.TurnKind = engineering.TurnWinder
	cfg.WinderCount = 3
	m, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Solids()) != 31 {
		t.Fatalf("solids = %d, want 31", len(m.Solids()))
	}
}

func TestExtraBuildUShapeWinderWrongFlight(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.Flight = engineering.FlightStraight
	if _, err := BuildUShapeWinderFlight(cfg); err == nil {
		t.Fatal("straight flight must be rejected")
	}
}

func TestExtraBuildUShapeWinderWrongTurn(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.TurnKind = engineering.TurnPlatform
	if _, err := BuildUShapeWinderFlight(cfg); err == nil {
		t.Fatal("platform turn kind must be rejected")
	}
}

func TestExtraBuildUShapeWinderTurnLeft(t *testing.T) {
	cfg := ushapeConfig(t)
	cfg.TurnKind = engineering.TurnWinder
	cfg.WinderCount = 3
	cfg.Direction = engineering.TurnLeft
	m, err := BuildUShapeWinderFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Solids()) != 31 {
		t.Fatalf("solids = %d, want 31", len(m.Solids()))
	}
}

func TestExtraBuildWindersErrors(t *testing.T) {
	p1 := kerngeo.NewPoint3(0, 900, 1080)
	p2 := kerngeo.NewPoint3(0, 1900, 1620)
	if _, err := buildWinders(900, 180, 40, 6, 2, p1, p2); err == nil {
		t.Fatal("winder count < 3 must be rejected")
	}
	if _, err := buildWinders(900, 180, 40, 6, 3, p2, p2); err == nil {
		t.Fatal("degenerate pivot must be rejected")
	}
}

func TestExtraStringerProfileClamps(t *testing.T) {
	// fx < 0 (отрицательная толщина ступени): фронтальная точка зажимается в 0.
	pts := stringerProfile(15, 270, 180, -1000, 0, 50)
	last := pts[len(pts)-1]
	if last.X != 0 || last.Z != 0 {
		t.Fatalf("fx<0: last point = %v, want (0,0)", last)
	}
	// fx > n·b: зажим на x = n·b.
	pts = stringerProfile(1, 270, 300, 400, 0, 50)
	last = pts[len(pts)-1]
	if last.X != 270 {
		t.Fatalf("fx>n·b: last point X = %v, want 270", last.X)
	}
}
