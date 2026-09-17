package stair

import (
	"context"
	"testing"
)

// BenchmarkCalculateFlights — базовый замер полного конвейера расчёта для
// всех типов марша (P2: performance baseline).
func BenchmarkCalculateFlights(b *testing.B) {
	s := NewService()
	cases := []struct {
		name string
		cfg  Config
	}{
		{"straight", referenceConfig()},
		{"lshape", referenceLShapeConfig()},
		{"ushape", referenceUShapeConfig()},
		{"spiral", referenceSpiralConfig()},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.Calculate(context.Background(), c.cfg, Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkOptimizePrice — замер детерминированного поиска оптимума по цене
// (EDR-0032): один размер марша, три точки шага комфорта + повторный расчёт
// лучшего кандидата.
func BenchmarkOptimizePrice(b *testing.B) {
	s := NewService()
	cfg := referenceConfig()
	req := OptimizeRequest{
		Target:          TargetPrice,
		StepCountMin:    15,
		StepCountMax:    15,
		ComfortStepGrid: 20,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Optimize(context.Background(), cfg, Options{}, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOptimizeCost — тот же поиск по себестоимости (альтернативная
// целевая метрика).
func BenchmarkOptimizeCost(b *testing.B) {
	s := NewService()
	cfg := referenceLShapeConfig()
	req := OptimizeRequest{
		Target:          TargetCost,
		StepCountMin:    15,
		StepCountMax:    15,
		ComfortStepGrid: 20,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Optimize(context.Background(), cfg, Options{}, req); err != nil {
			b.Fatal(err)
		}
	}
}
