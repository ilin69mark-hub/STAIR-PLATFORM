package engineering

import "testing"

// TestFlightTypeValid (DOM-001, forensic 2026-09-24) — enum типа марша
// проверяется: неизвестные значения не проходят валидацию конфигурации.
func TestFlightTypeValid(t *testing.T) {
	valid := []FlightType{FlightStraight, FlightLShape, FlightUShape, FlightSpiral}
	for _, f := range valid {
		if !f.Valid() {
			t.Fatalf("%q must be valid", f)
		}
	}
	for _, f := range []FlightType{"", "diagonal", "STRAIGHT ", "lshape", "spiral2"} {
		if f.Valid() {
			t.Fatalf("%q must be invalid", f)
		}
	}
}

// TestValidateRejectsUnknownFlight — доменная валидация отвергает мусорный
// тип марша (раньше проходил и ломал конвейер ниже по стеку).
func TestValidateRejectsUnknownFlight(t *testing.T) {
	for _, f := range []FlightType{"diagonal", "lshape", "SPiral"} {
		c := StairConfiguration{Width: 900, Height: 2700, Flight: f, StepCount: 15}
		if err := c.Validate(); err == nil {
			t.Fatalf("Validate(%q) must fail", f)
		}
	}
	// Пустое значение — отдельный кейс с прежним сообщением.
	c := StairConfiguration{Width: 900, Height: 2700, Flight: "", StepCount: 15}
	if err := c.Validate(); err == nil {
		t.Fatal("empty flight must fail")
	}
}
