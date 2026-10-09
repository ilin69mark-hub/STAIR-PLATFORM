package geometry

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// TestModelBoundingBoxMatchesGenerate — инвариант: облегчённая точка входа
// для подбора вариантов обязана давать ТОТ ЖЕ габарит, что и полный Generate.
// Если габариты разойдутся, проверка вписываемости в вариациях начнёт считать
// по другой модели, и это не поймает ни один другой тест.
func TestModelBoundingBoxMatchesGenerate(t *testing.T) {
	cases := []struct {
		name   string
		flight engineering.FlightType
		mat    string
	}{
		{"steel straight", engineering.FlightStraight, "STEEL-S235"},
		{"wood straight", engineering.FlightStraight, "WOOD-OAK"},
		{"steel l", engineering.FlightLShape, "STEEL-S235"},
		{"steel u", engineering.FlightUShape, "STEEL-S235"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, _ := engineering.NewStairConfiguration(900, 2700, c.flight)
			if cfg == nil {
				t.Fatal("configuration is nil")
			}
			cfg.StepCount = 16
			cfg.StepHeight, _ = engineering.NewLength(175)
			cfg.TreadDepth, _ = engineering.NewLength(280)
			cfg.StringerThickness, _ = engineering.NewLength(8)
			cfg.StepThickness, _ = engineering.NewLength(40)
			cfg.LandingDepth, _ = engineering.NewLength(900)
			cfg.LandingWidth, _ = engineering.NewLength(900)
			if c.flight == engineering.FlightLShape || c.flight == engineering.FlightUShape {
				cfg.LowerStepCount = 9
			}
			cfg.Material = c.mat
			cfg.ApproachSpace, _ = engineering.NewLength(1000)
			if err := cfg.Validate(); err != nil {
				t.Fatalf("config: %v", err)
			}

			res, err := Generate(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			fast, err := ModelBoundingBox(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			got := res.Measurement.BoundingBox
			if fast != got {
				t.Fatalf("bbox = %+v, Generate = %+v", fast, got)
			}
		})
	}
}
