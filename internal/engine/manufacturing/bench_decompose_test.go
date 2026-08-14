package manufacturing

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	enggeo "stairplatform/internal/engine/geometry"
)

func mfgBenchConfig() *engineering.StairConfiguration {
	cfg, _ := engineering.NewStairConfiguration(
		mfgMustLength(&testing.B{}, 900), mfgMustLength(&testing.B{}, 2700), engineering.FlightStraight)
	cfg.StepCount = 15
	cfg.StepHeight = mfgMustLength(&testing.B{}, 180)
	cfg.TreadDepth = mfgMustLength(&testing.B{}, 270)
	cfg.StringerThickness = mfgMustLength(&testing.B{}, 50)
	cfg.StepThickness = mfgMustLength(&testing.B{}, 40)
	return cfg
}

func BenchmarkDecompose(b *testing.B) {
	gen, err := enggeo.Generate(mfgBenchConfig())
	if err != nil {
		b.Fatal(err)
	}
	cfg := mfgBenchConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := decompose(cfg, gen.Model); err != nil {
			b.Fatal(err)
		}
	}
}
