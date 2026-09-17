package geometry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

type Case struct {
	Flight              string  `json:"flight"`
	WidthMM             float64 `json:"width_mm"`
	HeightMM            float64 `json:"height_mm"`
	Material            string  `json:"material"`
	StepHeightMM        float64 `json:"step_height_mm"`
	StepThicknessMM     float64 `json:"step_thickness_mm"`
	StringerThicknessMM float64 `json:"stringer_thickness_mm"`
	ClearanceMM         float64 `json:"clearance_mm"`
	RailingHeightMM     float64 `json:"railing_height_mm"`
	Railing             string  `json:"railing"`
	ApproachSpaceMM     float64 `json:"approach_space_mm"`
	RoomWidthMM         float64 `json:"room_width_mm"`
	RoomLengthMM        float64 `json:"room_length_mm"`
	LandingWidthMM      float64 `json:"landing_width_mm"`
	LandingDepthMM      float64 `json:"landing_depth_mm"`
	LowerStepCount      int     `json:"lower_step_count"`
	OuterRadiusMM       float64 `json:"outer_radius_mm"`
	SpiralDirection     string  `json:"spiral_direction"`
	Direction           string  `json:"direction"`
	ComfortStepMM       float64 `json:"comfort_step_mm"`
}

func genMustLength(v float64) engineering.Length { l,_ := engineering.NewLength(v); return l }

func caseToConfig(c Case) *engineering.StairConfiguration {
	var wL, hL = genMustLength(c.WidthMM), genMustLength(c.HeightMM)
	var ft engineering.FlightType
	switch c.Flight {
	case "straight":
		ft = engineering.FlightStraight
	case "l_shape":
		ft = engineering.FlightLShape
	case "u_shape":
		ft = engineering.FlightUShape
	case "spiral":
		ft = engineering.FlightSpiral
	}
	var cfg *engineering.StairConfiguration
	if c.Flight == "spiral" {
		cfg = &engineering.StairConfiguration{Width: wL, Height: hL, Flight: ft}
	} else {
		cfg,_ = engineering.NewStairConfiguration(wL, hL, ft)
	}
	cfg.StepHeight = genMustLength(c.StepHeightMM)
	cfg.StepThickness = genMustLength(c.StepThicknessMM)
	cfg.StringerThickness = genMustLength(c.StringerThicknessMM)
	cfg.Clearance = genMustLength(c.ClearanceMM)
	if c.RailingHeightMM > 0 {
		cfg.RailingHeight = genMustLength(c.RailingHeightMM)
	}
	cfg.ApproachSpace = genMustLength(c.ApproachSpaceMM)
	if c.RoomWidthMM > 0 {
		cfg.RoomWidth = genMustLength(c.RoomWidthMM)
		cfg.RoomLength = genMustLength(c.RoomLengthMM)
	}
	cfg.Material = c.Material
	if c.Flight == "straight" {
		switch c.Railing {
		case "none":
			cfg.Railing = engineering.RailingNone
		case "left":
			cfg.Railing = engineering.RailingLeft
		case "right":
			cfg.Railing = engineering.RailingRight
		default:
			cfg.Railing = engineering.RailingBoth
		}
	}
	if c.Flight == "l_shape" || c.Flight == "u_shape" {
		cfg.LandingWidth = genMustLength(c.LandingWidthMM)
		cfg.LandingDepth = genMustLength(c.LandingDepthMM)
		cfg.LowerStepCount = c.LowerStepCount
		if c.Direction == "left" { cfg.Direction = engineering.TurnLeft } else { cfg.Direction = engineering.TurnRight }
		var rs engineering.RailingSide
		switch c.Railing {
		case "none":
			rs = engineering.RailingNone
		case "left":
			rs = engineering.RailingLeft
		case "right":
			rs = engineering.RailingRight
		default:
			rs = engineering.RailingBoth
		}
		cfg.RailingLower = rs
		cfg.RailingLanding = rs
		cfg.RailingUpper = rs
	}
	if c.Flight == "spiral" {
		cfg.OuterRadius = genMustLength(c.OuterRadiusMM)
		if c.SpiralDirection == "cw" { cfg.SpiralDirection = engineering.SpiralCW } else { cfg.SpiralDirection = engineering.SpiralCCW }
	}
	return cfg
}

func TestGeneratedBbox500(t *testing.T) {
	f, err := os.Open("../solver/testdata/cases-500.json")
	if err != nil {
		t.Skip("no generated cases")
	}
	defer f.Close()
	var cases []Case
	if err := json.NewDecoder(f).Decode(&cases); err != nil { t.Fatalf("decode: %v", err) }
	prof := constraint.StandardProfile("std")
	for i, cs := range cases {
		if i%4 != 0 { continue } // sample 500 from 2000 to keep <1s (500 cases)
		cs := cs
		t.Run(fmt.Sprintf("%s_%d", cs.Flight, i), func(t *testing.T) {
			t.Parallel()
			cfg := caseToConfig(cs)
			var vr validation.Result
			switch cs.Flight {
			case "straight":
				_, vr, _ = solver.SolveChecked(cfg, prof, cs.ComfortStepMM)
			case "l_shape":
				_, vr, _ = solver.SolveCheckedLShape(cfg, prof, cs.ComfortStepMM)
			case "u_shape":
				_, vr, _ = solver.SolveCheckedUShape(cfg, prof, cs.ComfortStepMM)
			case "spiral":
				_, vr, _ = solver.SolveCheckedSpiral(cfg, prof)
			}
			if vr.Blocking { t.Fatalf("blocking: %+v", vr.Issues) }
			gen, err := Generate(context.Background(), cfg)
			if err != nil { t.Fatalf("Generate err: %v", err) }
			if gen.Mesh == nil || len(gen.Mesh.Vertices)==0 { t.Fatal("empty mesh") }
			bb := gen.Measurement.BoundingBox
			if bb.Max.X <= bb.Min.X || bb.Max.Y <= bb.Min.Y { t.Fatalf("invalid bbox %+v", bb) }
		})
	}
}
