package variation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/solver"
)

type genCase struct {
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

func genMustLength(v float64) engineering.Length { l, _ := engineering.NewLength(v); return l }

// genSem ограничивает параллелизм generated-теста: 2000 безлимитных
// t.Parallel-подтестов под -race устраивают трэш детектора гонок
// (в CI — panic timeout 10m), а замер даёт ~0.12с/кейс под race —
// с 8 слотами весь прогон укладывается в ~1-2 мин.
var genSem = make(chan struct{}, 8)

func genCaseToConfig(c genCase) *engineering.StairConfiguration {
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
		cfg, _ = engineering.NewStairConfiguration(wL, hL, ft)
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
	cfg.Length = genMustLength(c.ComfortStepMM) // шаг комфорта для ForRoomFit
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
		if c.Direction == "left" {
			cfg.Direction = engineering.TurnLeft
		} else {
			cfg.Direction = engineering.TurnRight
		}
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
		if c.SpiralDirection == "cw" {
			cfg.SpiralDirection = engineering.SpiralCW
		} else {
			cfg.SpiralDirection = engineering.SpiralCCW
		}
	}
	cfg.Material = c.Material
	return cfg
}

// TestGeneratedRoomFit500 — полный ForRoomFit по всем 2000 кейсам
// (500/марш). Сначала SolveChecked доводит конфиг до решённого состояния
// (как в generate-variations.go), затем ForRoomFit не должен паниковать,
// а все отданные варианты обязаны быть Fits+PassesNorms с непустым Config.
func TestGeneratedRoomFit500(t *testing.T) {
	f, err := os.Open("../solver/testdata/cases-500.json")
	if err != nil {
		t.Skip("no generated cases")
	}
	defer func() { _ = f.Close() }()
	var cases []genCase
	if err := json.NewDecoder(f).Decode(&cases); err != nil {
		t.Fatalf("decode: %v", err)
	}
	prof := constraint.StandardProfile("std")
	ctx := context.Background()
	for i, cs := range cases {
		cs := cs
		t.Run(fmt.Sprintf("%s_%d", cs.Flight, i), func(t *testing.T) {
			t.Parallel()
			genSem <- struct{}{}
			defer func() { <-genSem }()
			cfg := genCaseToConfig(cs)
			// Доводим до решённого состояния (мутирует cfg через Apply).
			switch cs.Flight {
			case "straight":
				_, _, _ = solver.SolveChecked(cfg, prof, cs.ComfortStepMM)
			case "l_shape":
				_, _, _ = solver.SolveCheckedLShape(cfg, prof, cs.ComfortStepMM)
			case "u_shape":
				_, _, _ = solver.SolveCheckedUShape(cfg, prof, cs.ComfortStepMM)
			case "spiral":
				_, _, _ = solver.SolveCheckedSpiral(cfg, prof)
			}
			cfg.Length = genMustLength(cs.ComfortStepMM)
			vars := ForRoomFit(ctx, cfg, prof)
			for _, v := range vars {
				if !v.Fits {
					t.Errorf("variant %q not fitting", v.Title)
				}
				if !v.PassesNorms {
					t.Errorf("variant %q does not pass norms", v.Title)
				}
				if len(v.Config) == 0 {
					t.Errorf("variant %q has empty config", v.Title)
				}
			}
		})
	}
}
