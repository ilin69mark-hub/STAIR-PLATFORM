package solver

import "testing"

func TestInputHelpersCoverage(t *testing.T) {
	_ = (&InputError{Message: "x"}).Error()
	_ = inputError("c", "f", "m", "g", "fix")
	_ = rangeError("c", "f", 1, 0, 2, "m", "g", "fix")
	_ = minError("c", "f", 1, 0, "m", "g", "fix")
	_ = riseInputError()
	_ = riserInputError()
	_ = widthInputError()
	_ = noFlightInputError(100)
	_ = comfortInputError(700)
	_ = treadPositiveError(-1)
	_ = landingPositiveError()
	_ = lowerStepInputError(5, 15)
	_ = winderCountInputError(2)
	_ = upperStepInputError(15, 5)
	_ = landingNarrowError(500, 900)
	_ = radiusNarrowError(400, 900)
	_ = spiralTreadInputError("inner", 50, 100, 0)
	_ = spiralTreadInputError("outer", 200, 100, 0)
	_ = spiralTreadInputError("walk", 300, 260, 320)
	_ = spiralTreadInputError("walk", 300, 260, 320)
}
