package engineering

import (
	"strings"
	"testing"
)

func TestTurnKindValid(t *testing.T) {
	if !TurnPlatform.Valid() || !TurnWinder.Valid() || !TurnKind("").Valid() {
		t.Fatal("platform, winder and empty turn kinds must be valid")
	}
	if TurnKind("spiral").Valid() {
		t.Fatal("unknown turn kind must be rejected")
	}
}

func TestStairConfigurationValidateErrors(t *testing.T) {
	base := &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6}

	cases := []struct {
		name string
		cfg  *StairConfiguration
		want string
	}{
		{"zero width", &StairConfiguration{Width: 0, Height: 2700, Flight: FlightStraight, StepCount: 6}, "width must be positive"},
		{"zero height", &StairConfiguration{Width: 900, Height: 0, Flight: FlightStraight, StepCount: 6}, "height must be positive"},
		{"empty flight", &StairConfiguration{Width: 900, Height: 2700, Flight: "", StepCount: 6}, "flight type is required"},
		{"zero steps", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 0}, "step count must be positive"},
		{"negative stringer thickness", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, StringerThickness: -1}, "stringer thickness must not be negative"},
		{"negative step thickness", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, StepThickness: -1}, "step thickness must not be negative"},
		{"negative tread depth", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, TreadDepth: -1}, "tread depth must not be negative"},
		{"negative clearance", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, Clearance: -1}, "clearance must not be negative"},
		{"negative railing height", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, RailingHeight: -1}, "railing height must not be negative"},
		{"negative stringer length", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, StringerLength: -1}, "stringer length must not be negative"},
		{"negative lower step count", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, LowerStepCount: -1}, "lower step count must not be negative"},
		{"negative landing width", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, LandingWidth: -1}, "landing width must not be negative"},
		{"negative landing depth", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, LandingDepth: -1}, "landing depth must not be negative"},
		{"negative room width", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, RoomWidth: -1}, "room width must not be negative"},
		{"negative room length", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, RoomLength: -1}, "room length must not be negative"},
		{"negative approach space", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, ApproachSpace: -1}, "approach space must not be negative"},
		{"approach space too small", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, ApproachSpace: 999}, "approach space must be within"},
		{"approach space too large", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, ApproachSpace: 1300}, "approach space must be within"},
		{"invalid turn kind", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightUShape, StepCount: 8, TurnKind: TurnKind("bogus")}, "invalid turn kind"},
		{"winder count too low", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightUShape, StepCount: 8, TurnKind: TurnWinder, WinderCount: 2}, "winder count must be at least 3"},
		{"winder no well width", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightUShape, StepCount: 8, TurnKind: TurnWinder, WinderCount: 3}, "well width must be positive"},
		{"winder lower steps zero", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightUShape, StepCount: 8, TurnKind: TurnWinder, WinderCount: 3, LandingWidth: 200}, "lower step count must be in"},
		{"winder lower steps exceed", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightUShape, StepCount: 8, TurnKind: TurnWinder, WinderCount: 3, LandingWidth: 200, LowerStepCount: 8}, "lower step count must be in"},
		{"platform landing width narrow", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightLShape, StepCount: 8, LandingWidth: 800, LowerStepCount: 4}, "landing width must be at least"},
		{"platform landing depth shallow", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightLShape, StepCount: 8, LandingWidth: 1000, LandingDepth: 800, LowerStepCount: 4}, "landing depth must be at least"},
		{"platform lower steps zero", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightLShape, StepCount: 8, LandingWidth: 1000, LowerStepCount: 0}, "lower step count must be in"},
		{"platform lower steps exceed", &StairConfiguration{Width: 900, Height: 2700, Flight: FlightLShape, StepCount: 8, LandingWidth: 1000, LowerStepCount: 8}, "lower step count must be in"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q must contain %q", err, tc.want)
			}
		})
	}
	_ = base
}

func TestStairConfigurationValidateValid(t *testing.T) {
	straight := &StairConfiguration{Width: 900, Height: 2700, Flight: FlightStraight, StepCount: 6, ApproachSpace: 1000}
	if err := straight.Validate(); err != nil {
		t.Fatalf("straight must be valid: %v", err)
	}
	platform := &StairConfiguration{
		Width: 900, Height: 2700, Flight: FlightLShape, StepCount: 8,
		LandingWidth: 1000, LandingDepth: 1000, LowerStepCount: 4,
	}
	if err := platform.Validate(); err != nil {
		t.Fatalf("platform l_shape must be valid: %v", err)
	}
	winder := &StairConfiguration{
		Width: 900, Height: 2700, Flight: FlightUShape, StepCount: 10,
		TurnKind: TurnWinder, WinderCount: 4, LandingWidth: 200, LowerStepCount: 5,
	}
	if err := winder.Validate(); err != nil {
		t.Fatalf("winder u_shape must be valid: %v", err)
	}
}
