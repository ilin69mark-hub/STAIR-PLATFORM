package geometry

import "testing"

func TestBalusterStepsProbe(t *testing.T) {
	for _, b := range []float64{270, 290} {
		for _, n := range []int{8, 12, 16, 20} {
			st := balusterSteps(n, b)
			var gaps []float64
			for i := 1; i < len(st); i++ {
				gaps = append(gaps, float64(st[i]-st[i-1])*b)
			}
			t.Logf("n=%2d b=%.0f -> steps %v gaps %v", n, b, st, gaps)
		}
	}
}
