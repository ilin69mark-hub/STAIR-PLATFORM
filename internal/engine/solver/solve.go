package solver

import (
	"math"

	"stairplatform/internal/domain/engineering"
)

// DefaultComfortStep — шаг комфорта S по умолчанию (EDR-0001 §4.3, 630 мм).
const DefaultComfortStep = 630.0

// ComfortStepMin / ComfortStepMax — нормативные границы шага комфорта (600–640 мм).
const (
	ComfortStepMin = 600.0
	ComfortStepMax = 640.0
)

// Solve рассчитывает прямой марш по EDR-0001 §4.
// H — высота подъёма, h0 — целевая высота ступени, s — шаг комфорта
// (по умолчанию DefaultComfortStep).
func Solve(H, h0 engineering.Length, s ...float64) (FlightResult, error) {
	var step float64
	if len(s) > 0 {
		step = s[0]
	} else {
		step = DefaultComfortStep
	}

	hm := H.Millimeters()
	if hm <= 0 {
		return FlightResult{}, riseInputError()
	}
	h0m := h0.Millimeters()
	if h0m <= 0 {
		return FlightResult{}, riserInputError()
	}
	if step < ComfortStepMin || step > ComfortStepMax {
		return FlightResult{}, comfortInputError(step)
	}

	// §4.1 число ступеней; §7 edge case: высота без участка.
	n := int(math.Round(hm / h0m))
	if n < 1 {
		return FlightResult{}, noFlightInputError(hm)
	}

	// §4.2 уточнённая высота ступени.
	h := hm / float64(n)
	// §4.3 проступь (невалидная геометрия при b <= 0).
	b := step - 2*h
	if b <= 0 {
		return FlightResult{}, treadPositiveError(b)
	}
	// §4.4 угол наклона.
	alpha := math.Atan(h / b)
	// §4.5 длина марша по горизонтали.
	L := float64(n) * b
	// §4.6 длина косоура.
	R := math.Sqrt(L*L + hm*hm)

	return FlightResult{
		StepCount:  n,
		StepHeight: engineering.Length(h),
		TreadDepth: engineering.Length(b),
		Run:        engineering.Length(L),
		Stringer:   engineering.Length(R),
		Angle:      engineering.Angle(alpha),
	}, nil
}
