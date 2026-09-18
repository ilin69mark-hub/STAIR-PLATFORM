package variation

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

const extraAngleSetName = "extra-angle-norms"

func extraStraightCfg() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             900,
		Height:            3000,
		Flight:            engineering.FlightStraight,
		Riser:             true,
		StepCount:         16,
		StepHeight:        180,
		TreadDepth:        270,
		StringerThickness: 40,
		StepThickness:     40,
		Clearance:         2100,
		RailingHeight:     900,
	}
}

func extraFlightCfg(flight engineering.FlightType) *engineering.StairConfiguration {
	c := extraStraightCfg()
	c.Flight = flight
	c.StepCount = 15
	c.LandingWidth = 1100
	c.LandingDepth = 1100
	c.LowerStepCount = 7
	c.OuterRadius = 1300
	return c
}

func TestExtraForAngle_NoHeight(t *testing.T) {
	cfg := extraStraightCfg()
	cfg.Height = 0
	if v := ForAngle(context.Background(), cfg, constraint.StandardProfile(extraAngleSetName)); v != nil {
		t.Fatalf("expected nil, got %d variants", len(v))
	}
}

func TestExtraForAngle_NoCandidates(t *testing.T) {
	cfg := extraStraightCfg()
	cfg.Height = 20000
	if v := ForAngle(context.Background(), cfg, constraint.StandardProfile(extraAngleSetName)); v != nil {
		t.Fatalf("expected nil, got %d variants", len(v))
	}
}

func TestExtraForAngle_ChooseAll(t *testing.T) {
	cfg := extraStraightCfg()
	cfg.Height = 1000
	vars := ForAngle(context.Background(), cfg, constraint.StandardProfile(extraAngleSetName))
	if len(vars) == 0 {
		t.Fatalf("expected variants, got 0")
	}
}

func TestExtraForAngle_Flights(t *testing.T) {
	for _, flight := range []engineering.FlightType{engineering.FlightLShape, engineering.FlightUShape, engineering.FlightSpiral} {
		t.Run(string(flight), func(t *testing.T) {
			cfg := extraFlightCfg(flight)
			if flight == engineering.FlightSpiral {
				cfg.Width = 500
			}
			vars := ForAngle(context.Background(), cfg, constraint.StandardProfile(extraAngleSetName))
			if len(vars) == 0 {
				t.Fatalf("expected variants, got 0")
			}
		})
	}
}

func TestExtraChDegFromS(t *testing.T) {
	if got := chDegFromS(200, 100); got != 0 {
		t.Fatalf("chDegFromS = %v, want 0", got)
	}
}

func TestExtraAngleVariant_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("straight_solve_err", func(t *testing.T) {
		cfg := extraStraightCfg()
		if _, ok := angleVariant(ctx, cfg, cfg.Height, 1000, 640, 1, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("lshape_small_n", func(t *testing.T) {
		cfg := extraFlightCfg(engineering.FlightLShape)
		if _, ok := angleVariant(ctx, cfg, cfg.Height, 200, 600, 1, nil, "x"); !ok {
			t.Fatal("expected success")
		}
	})

	t.Run("lshape_null_landing", func(t *testing.T) {
		cfg := extraFlightCfg(engineering.FlightLShape)
		cfg.Width = 0
		cfg.LandingWidth = 0
		if _, ok := angleVariant(ctx, cfg, cfg.Height, 200, 600, 4, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("ushape_null_landing", func(t *testing.T) {
		cfg := extraFlightCfg(engineering.FlightUShape)
		cfg.Width = 0
		cfg.LandingWidth = 0
		if _, ok := angleVariant(ctx, cfg, cfg.Height, 200, 600, 4, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("spiral_narrow", func(t *testing.T) {
		cfg := extraFlightCfg(engineering.FlightSpiral)
		cfg.Width = 2000
		if _, ok := angleVariant(ctx, cfg, cfg.Height, 200, 640, 2, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("spiral_default_width", func(t *testing.T) {
		cfg := extraFlightCfg(engineering.FlightSpiral)
		cfg.Width = 0
		angleVariant(ctx, cfg, cfg.Height, 200, 640, 15, nil, "x")
	})

	t.Run("spiral_solve_err", func(t *testing.T) {
		cfg := extraFlightCfg(engineering.FlightSpiral)
		cfg.Width = 800
		if _, ok := angleVariant(ctx, cfg, cfg.Height, 200, 640, 15, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("bogus_flight", func(t *testing.T) {
		cfg := extraStraightCfg()
		cfg.Flight = "bogus"
		if _, ok := angleVariant(ctx, cfg, cfg.Height, 200, 600, 4, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})
}

func TestExtraTryAngleGenerate_Branches(t *testing.T) {
	ctx := context.Background()
	t.Run("generate_err", func(t *testing.T) {
		cfg := extraStraightCfg()
		cfg.Flight = "bogus"
		if _, ok := tryAngleGenerate(ctx, cfg, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("angle_out_of_range", func(t *testing.T) {
		cfg := extraStraightCfg()
		cfg.Angle = engineering.Angle(0.2) // ~11.5°
		if _, ok := tryAngleGenerate(ctx, cfg, nil, "x"); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("blocking", func(t *testing.T) {
		cfg := extraStraightCfg()
		cfg.Angle = engineering.Angle(0.588) // ~33.7°
		cfg.StepHeight = 210                 // GEO-STEP-HEIGHT error → build blocks
		set := constraint.StandardProfile(extraAngleSetName)
		if _, ok := tryAngleGenerate(ctx, cfg, set, "x"); ok {
			t.Fatal("expected false")
		}
	})
}

func TestExtraFromSuggestions_Edges(t *testing.T) {
	if v := FromSuggestions(&validation.Issue{Code: "X"}, extraStraightCfg()); v != nil {
		t.Fatalf("expected nil, got %d", len(v))
	}

	cfg := extraFlightCfg(engineering.FlightLShape)
	issue := &validation.Issue{
		Code: "GEO-STEP",
		Suggestions: []validation.Suggestion{{
			StepCount: 16, StepHeightMm: 180, TreadDepthMm: 270, AngleDeg: 33, LowerStepCount: 6,
		}},
	}
	if v := FromSuggestions(issue, cfg); len(v) != 1 {
		t.Fatalf("L-shape: got %d, want 1", len(v))
	}

	sp := &validation.Issue{
		Code: "GEO-SPIRAL",
		Suggestions: []validation.Suggestion{{
			StepCount: 15, StepHeightMm: 180, TreadDepthMm: 270, AngleDeg: 30,
			WidthMm: 900, OuterRadiusMm: 1300,
		}},
	}
	if v := FromSuggestions(sp, extraFlightCfg(engineering.FlightSpiral)); len(v) != 1 {
		t.Fatalf("spiral: got %d, want 1", len(v))
	}
}

func TestExtraSteeper_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("impossible_target", func(t *testing.T) {
		cfg := extraStraightCfg()
		if _, ok := steeperVariant(ctx, cfg, cfg.Height, 500, nil, 20000, 20000, fitEps); ok {
			t.Fatal("expected false")
		}
	})

	t.Run("bogus_flight", func(t *testing.T) {
		cfg := extraStraightCfg()
		cfg.Flight = "bogus"
		if _, ok := steeperVariant(ctx, cfg, cfg.Height, 630, nil, 20000, 20000, fitEps); ok {
			t.Fatal("expected false")
		}
	})
}

func TestExtraSmallerLanding_NoWidth(t *testing.T) {
	cfg := extraFlightCfg(engineering.FlightLShape)
	cfg.Width = 0
	if _, ok := smallerLandingVariant(context.Background(), cfg, nil, 20000, 20000, fitEps); ok {
		t.Fatal("expected false")
	}
}

func TestExtraOtherTypeVariants_Bogus(t *testing.T) {
	cfg := extraStraightCfg()
	cfg.Flight = "bogus"
	otherTypeVariants(context.Background(), cfg, cfg.Height, 630, nil, 20000, 20000, fitEps)
}

func TestExtraBuildOtherType_Defaults(t *testing.T) {
	ctx := context.Background()

	t.Run("zero_width", func(t *testing.T) {
		cfg := extraStraightCfg()
		cfg.Width = 0
		cfg.StepHeight = 250
		if vs := buildOtherTypes(ctx, cfg, engineering.FlightStraight, cfg.Height, 630, nil, 30000, 30000, fitEps); len(vs) == 0 {
			t.Fatal("expected success")
		}
	})

	t.Run("zero_step_height", func(t *testing.T) {
		cfg := extraStraightCfg()
		cfg.Width = 400
		cfg.StepHeight = 0
		if vs := buildOtherTypes(ctx, cfg, engineering.FlightStraight, cfg.Height, 630, nil, 30000, 30000, fitEps); len(vs) == 0 {
			t.Fatal("expected success")
		}
	})
}

func TestExtraCollectOtherType_MinStepHeight(t *testing.T) {
	cfg := extraStraightCfg()
	cfg.Width = 400
	cfg.StepHeight = 100
	cands := collectOtherType(context.Background(), cfg, engineering.FlightStraight, cfg.Height, 630, nil, 30000, 30000, fitEps)
	if len(cands) == 0 {
		t.Fatalf("expected candidates, got 0")
	}
}

func TestExtraTryOtherAt_Errors(t *testing.T) {
	ctx := context.Background()
	cfg := extraStraightCfg()

	if _, _, ok := tryOtherAt(ctx, cfg, engineering.FlightStraight, cfg.Height, 630, nil, 30000, 30000, fitEps, 0, 900, 0); ok {
		t.Fatal("straight: expected false")
	}
	if _, _, ok := tryOtherAt(ctx, cfg, engineering.FlightLShape, 100, 630, nil, 30000, 30000, fitEps, 200, 900, 0); ok {
		t.Fatal("L-shape: expected false")
	}
	if _, _, ok := tryOtherAt(ctx, cfg, engineering.FlightUShape, 100, 630, nil, 30000, 30000, fitEps, 200, 900, 0); ok {
		t.Fatal("U-shape: expected false")
	}
	if _, _, ok := tryOtherAt(ctx, cfg, engineering.FlightSpiral, cfg.Height, 630, nil, 30000, 30000, fitEps, 200, 900, 100); ok {
		t.Fatal("spiral R<=w: expected false")
	}
	if _, _, ok := tryOtherAt(ctx, cfg, "bogus", cfg.Height, 630, nil, 30000, 30000, fitEps, 200, 900, 0); ok {
		t.Fatal("bogus: expected false")
	}
}

func TestExtraTryGenerate_GenerateErr(t *testing.T) {
	cfg := extraStraightCfg()
	cfg.Flight = "bogus"
	if _, ok := tryGenerate(context.Background(), cfg, nil, 30000, 30000, fitEps, "t", "d"); ok {
		t.Fatal("expected false")
	}
}

func TestExtraDedupe_Duplicates(t *testing.T) {
	in := []validation.Variation{
		{ID: "a", Config: map[string]string{"Width": "900"}},
		{ID: "b", Config: map[string]string{"Width": "900"}},
		{ID: "c", Config: map[string]string{"Width": "1000"}},
	}
	out := dedupe(in)
	if len(out) != 2 {
		t.Fatalf("dedupe = %d, want 2", len(out))
	}
}

func TestExtraForRoomFit_UShape(t *testing.T) {
	cfg := testUShapeConfig()
	cfg.StepHeight = 200
	cfg.RoomWidth = 6000
	cfg.RoomLength = 6000
	vars := ForRoomFit(context.Background(), cfg, nil)
	if len(vars) == 0 {
		t.Fatalf("expected variants, got 0")
	}
}

func TestExtraForRoomFit_Spiral(t *testing.T) {
	cfg := testSpiralConfig()
	cfg.StepHeight = 200
	cfg.Width = 500
	cfg.RoomWidth = 6000
	cfg.RoomLength = 6000
	ForRoomFit(context.Background(), cfg, nil)
}
