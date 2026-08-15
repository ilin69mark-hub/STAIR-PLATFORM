package geometry

import (
	"context"
	"strconv"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// benchConfig возвращает конфигурацию прямого марша с заданным числом
// ступеней (без t — для бенчмарка).
func benchConfig(stepCount int) *engineering.StairConfiguration {
	cfg, _ := engineering.NewStairConfiguration(
		mustLengthNaN(900), mustLengthNaN(2700), engineering.FlightStraight)
	cfg.StepCount = stepCount
	cfg.StepHeight = mustLengthNaN(180)
	cfg.TreadDepth = mustLengthNaN(270)
	cfg.StringerThickness = mustLengthNaN(50)
	cfg.StepThickness = mustLengthNaN(40)
	return cfg
}

func mustLengthNaN(mm float64) engineering.Length {
	l, _ := engineering.NewLength(mm)
	return l
}

func benchSpiralConfig(stepCount int) *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             mustLengthNaN(500),
		Height:            mustLengthNaN(2700),
		Flight:            engineering.FlightSpiral,
		StepCount:         stepCount,
		StepHeight:        mustLengthNaN(180),
		TreadDepth:        mustLengthNaN(265.291),
		StringerThickness: mustLengthNaN(50),
		StepThickness:     mustLengthNaN(40),
		OuterRadius:       mustLengthNaN(800),
	}
}

// BenchmarkGenerateStraight измеряет весь Geometry Stage конвейера для
// прямого марша (включая 4× избыточную триангуляцию — см. B2.2). Входы
// детерминированы, но результаты не разделяются между итерациями
// (анти-оптимизация для честного замера).
func BenchmarkGenerateStraight(b *testing.B) {
	for _, n := range []int{10, 15, 20} {
		cfg := benchConfig(n)
		b.Run("steps="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			var last *GenerationResult
			for i := 0; i < b.N; i++ {
				res, err := Generate(context.Background(), cfg)
				if err != nil {
					b.Fatal(err)
				}
				last = res
			}
			_ = last
		})
	}
}

// BenchmarkGenerateSpiral измеряет Geometry Stage для спиральной лестницы.
func BenchmarkGenerateSpiral(b *testing.B) {
	for _, n := range []int{10, 15, 20} {
		cfg := benchSpiralConfig(n)
		b.Run("steps="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			var last *GenerationResult
			for i := 0; i < b.N; i++ {
				res, err := Generate(context.Background(), cfg)
				if err != nil {
					b.Fatal(err)
				}
				last = res
			}
			_ = last
		})
	}
}

// BenchmarkToPreviewMesh изолирует построение preview-сетки: каждая грань
// модели триангулируется 4-й раз (после Validate/Volume/SurfaceArea) — см.
// B2.2 (разделение одной триангуляции на все потребители).
func BenchmarkToPreviewMesh(b *testing.B) {
	for _, n := range []int{15} {
		cfg := benchConfig(n)
		res, err := Generate(context.Background(), cfg)
		if err != nil {
			b.Fatal(err)
		}
		model := res.Model
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := ToPreviewMesh(model); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkBuildSpiralFlight измеряет только построение модели спирали
// (без валидации/измерений/mesh) — изолирует цикл экструзии ступеней,
// цель B3.1 параллелизации.
func BenchmarkBuildSpiralFlight(b *testing.B) {
	for _, n := range []int{10, 15, 20} {
		cfg := benchSpiralConfig(n)
		b.Run("steps="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			var last *kerngeo.Compound
			for i := 0; i < b.N; i++ {
				m, err := BuildSpiralFlight(cfg)
				if err != nil {
					b.Fatal(err)
				}
				last = m
			}
			_ = last
		})
	}
}
