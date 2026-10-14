package solver

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
)

// BenchmarkSolve* документируют O(1) closed-form решатель (EM-06): каждый
// тип марша — одна прямая формула, без поиска/итераций. Используется как
// точка отсчёта относительно Geometry/Manufacturing, доминирующих в
// конвейере.
func BenchmarkSolveStraight(b *testing.B) {
	c := benchConfig()
	b.ReportAllocs()
	var last FlightResult
	for i := 0; i < b.N; i++ {
		f, _, err := SolveChecked(c, constraint.StandardProfile("STANDARD"), DefaultComfortStep)
		if err != nil {
			b.Fatal(err)
		}
		last = f
	}
	_ = last
}

func BenchmarkSolveLShape(b *testing.B) {
	c := benchConfig()
	c.Flight = engineering.FlightLShape
	c.LandingWidth = mustLengthB(1000)
	c.LowerStepCount = 6
	b.ReportAllocs()
	var last *LShapeResult
	for i := 0; i < b.N; i++ {
		r, _, err := SolveCheckedLShape(c, constraint.StandardProfile("STANDARD"), DefaultComfortStep)
		if err != nil {
			b.Fatal(err)
		}
		last = &r
	}
	_ = last
}

func benchConfig() *engineering.StairConfiguration {
	c, _ := engineering.NewStairConfiguration(
		mustLengthB(900), mustLengthB(2700), engineering.FlightStraight)
	c.StepHeight = mustLengthB(180)
	c.TreadDepth = mustLengthB(270)
	c.StringerThickness = mustLengthB(50)
	c.StepThickness = mustLengthB(40)
	return c
}

func mustLengthB(mm float64) engineering.Length {
	l, _ := engineering.NewLength(mm)
	return l
}
