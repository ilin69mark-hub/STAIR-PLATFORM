//go:build ignore
// +build ignore

// Генератор 500 валидных конфигураций на каждый flight (2000 total).
// Запуск: go run ./scripts/generate-variations.go -n 500
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/geometry"
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

func mustLength(v float64) engineering.Length {
	l, err := engineering.NewLength(v)
	if err != nil {
		panic(err)
	}
	return l
}

var attemptsDebug int

func randomValid(flight string) (*Case, bool) {
	mats := []string{"STEEL-S235", "ALUM-5083", "WOOD-OAK"}
	mat := mats[rand.Intn(len(mats))]
	// width 300-1200 stratified
	W := 300 + rand.Float64()*900
	if rand.Float64() < 0.2 {
		W = 1500 + rand.Float64()*1500
	}
	Hmax := 6000.0
	if mat != "STEEL-S235" {
		Hmax = 4550
	}
	H := 1800 + rand.Float64()*(Hmax-1800)
	S := 600 + rand.Float64()*40
	comfortToStore := S
	h0 := 150 + rand.Float64()*50
	// step thickness per material
	var stepTh float64
	switch mat {
	case "STEEL-S235":
		stepTh = 3 + rand.Float64()*5
	case "ALUM-5083":
		stepTh = 2 + rand.Float64()*10
		if stepTh > 20 {
			stepTh = 2 + rand.Float64()*8
		}
	default:
		stepTh = 20 + rand.Float64()*20
	}
	stringerTh := 30 + rand.Float64()*30
	clearance := rand.Float64() * 5000
	railingH := 0.0
	if rand.Float64() < 0.8 {
		railingH = 900 + rand.Float64()*300
		if rand.Float64() < 0.1 {
			railingH = 0
		}
	}
	approach := 1000 + rand.Float64()*200
	var roomW, roomL float64
	if rand.Float64() < 0.5 {
		roomW = 3000 + rand.Float64()*2000
		roomL = 3000 + rand.Float64()*2000
	}
	// railing for straight
	railings := []string{"none", "left", "right", "both"}
	railing := railings[rand.Intn(len(railings))]
	directions := []string{"left", "right"}
	dir := directions[rand.Intn(len(directions))]
	spiralDirs := []string{"cw", "ccw"}
	spiralDir := spiralDirs[rand.Intn(len(spiralDirs))]

	// quick solver check
	n := int(math.Round(H / h0))
	if n < 1 {
		return nil, false
	}
	h := H / float64(n)
	// flight specific
	var Wp, Ld float64
	var n1 int
	var R float64
	switch flight {
	case "straight", "l_shape", "u_shape":
		b := S - 2*h
		if b <= 0 || b < 260 || b > 320 {
			return nil, false
		}
		alpha := math.Atan(h/b) * 180 / math.Pi
		if alpha < 30 || alpha > 45 {
			return nil, false
		}
		if flight == "l_shape" || flight == "u_shape" {
			Wp = math.Max(600, W) + rand.Float64()*(3000-math.Max(600, W))
			Ld = W
			if rand.Float64() < 0.5 {
				Ld = 600 + rand.Float64()*500
			}
			if Wp < W {
				return nil, false
			}
			n1 = 1 + rand.Intn(n-1)
			if n1 < 1 || n1 >= n {
				return nil, false
			}
		}
	case "spiral":
		// Для спирали фиксируем H 2700 и подбираем R чтобы попасть в коридор
		H = 2700
		n = 15
		h = H / float64(n) // 180
		if W < 500 || W > 700 {
			W = 500 + rand.Float64()*200
		}
		desiredBWalk := 270 + rand.Float64()*10 // 270-280 -> S2 630-640
		R = desiredBWalk*float64(n)/(2*math.Pi) + W/3
		if R > 5000 || R <= W {
			return nil, false
		}
		comfortToStore = 2*h + desiredBWalk
	default:
		return nil, false
	}

	// Build engineering config for validation
	var wL, hL engineering.Length
	wL = mustLength(W)
	hL = mustLength(H)
	var flightType engineering.FlightType
	switch flight {
	case "straight":
		flightType = engineering.FlightStraight
	case "l_shape":
		flightType = engineering.FlightLShape
	case "u_shape":
		flightType = engineering.FlightUShape
	case "spiral":
		flightType = engineering.FlightSpiral
	}
	var err error
	var stairCfg *engineering.StairConfiguration
	if flight == "spiral" {
		stairCfg = &engineering.StairConfiguration{Width: wL, Height: hL, Flight: flightType}
		err = nil
	} else {
		var err2 error
		stairCfg, err2 = engineering.NewStairConfiguration(wL, hL, flightType)
		if err2 != nil {
			return nil, false
		}
		err = err2
	}
	// set extra fields
	stairCfg.StepHeight = mustLength(h)
	stairCfg.StepThickness = mustLength(stepTh)
	stairCfg.StringerThickness = mustLength(stringerTh)
	stairCfg.Clearance = mustLength(clearance)
	if railingH > 0 {
		stairCfg.RailingHeight = mustLength(railingH)
	}
	stairCfg.ApproachSpace = mustLength(approach)
	if roomW > 0 {
		stairCfg.RoomWidth = mustLength(roomW)
		stairCfg.RoomLength = mustLength(roomL)
	}
	if flight == "straight" {
		switch railing {
		case "none":
			stairCfg.Railing = engineering.RailingNone
		case "left":
			stairCfg.Railing = engineering.RailingLeft
		case "right":
			stairCfg.Railing = engineering.RailingRight
		default:
			stairCfg.Railing = engineering.RailingBoth
		}
	}
	if flight == "l_shape" || flight == "u_shape" {
		stairCfg.LandingWidth = mustLength(Wp)
		stairCfg.LandingDepth = mustLength(Ld)
		stairCfg.LowerStepCount = n1
		if dir == "left" {
			stairCfg.Direction = engineering.TurnLeft
		} else {
			stairCfg.Direction = engineering.TurnRight
		}
		// per-segment railing
		var rs engineering.RailingSide
		switch railing {
		case "none":
			rs = engineering.RailingNone
		case "left":
			rs = engineering.RailingLeft
		case "right":
			rs = engineering.RailingRight
		default:
			rs = engineering.RailingBoth
		}
		stairCfg.RailingLower = rs
		stairCfg.RailingLanding = rs
		stairCfg.RailingUpper = rs
	}
	if flight == "spiral" {
		stairCfg.OuterRadius = mustLength(R)
		if spiralDir == "cw" {
			stairCfg.SpiralDirection = engineering.SpiralCW
		} else {
			stairCfg.SpiralDirection = engineering.SpiralCCW
		}
	}
	stairCfg.Material = mat

	// SolveChecked + validation + geometry
	prof := constraint.StandardProfile("std")
	var vr validation.Result
	switch flight {
	case "straight":
		_, vr, err = solver.SolveChecked(stairCfg, prof, S)
	case "l_shape":
		_, vr, err = solver.SolveCheckedLShape(stairCfg, prof, S)
	case "u_shape":
		_, vr, err = solver.SolveCheckedUShape(stairCfg, prof, S)
	case "spiral":
		_, vr, err = solver.SolveCheckedSpiral(stairCfg, prof)
	}
	if err != nil || vr.Blocking {
		if flight == "spiral" {
			// debug first few
			if attemptsDebug < 5 {
				fmt.Printf("spiral fail err=%v blocking=%v issues=%v W=%.0f R=%.0f n=%d h=%.0f\n", err, vr.Blocking, vr.Issues, W, R, n, h)
				attemptsDebug++
			}
		}
		return nil, false
	}
	// geometry
	gen, err := geometry.Generate(context.Background(), stairCfg)
	if err != nil {
		return nil, false
	}
	if gen.Mesh == nil || len(gen.Mesh.Vertices) == 0 {
		return nil, false
	}
	// room fit already in issues, but we already validated blocking false
	// consider valid
	_ = Wp
	_ = Ld
	_ = n1
	_ = R

	cs := &Case{
		Flight:              flight,
		WidthMM:             W,
		HeightMM:            H,
		Material:            mat,
		StepHeightMM:        h,
		StepThicknessMM:     stepTh,
		StringerThicknessMM: stringerTh,
		ClearanceMM:         clearance,
		RailingHeightMM:     railingH,
		Railing:             railing,
		ApproachSpaceMM:     approach,
		RoomWidthMM:         roomW,
		RoomLengthMM:        roomL,
		LandingWidthMM:      Wp,
		LandingDepthMM:      Ld,
		LowerStepCount:      n1,
		OuterRadiusMM:       R,
		SpiralDirection:     spiralDir,
		Direction:           dir,
		ComfortStepMM:       comfortToStore,
	}
	return cs, true
}

func main() {
	n := flag.Int("n", 500, "per flight")
	outGo := flag.String("out-go", "internal/engine/solver/testdata/cases-500.json", "go cases output")
	outJS := flag.String("out-js", "frontend-store/src/test/generated-cases-500.json", "js cases output")
	flag.Parse()

	flights := []string{"straight", "l_shape", "u_shape", "spiral"}
	all := make([]Case, 0, (*n)*len(flights))
	for _, fl := range flights {
		cnt := 0
		attempts := 0
		for cnt < *n && attempts < 200000 {
			attempts++
			cs, ok := randomValid(fl)
			if ok {
				all = append(all, *cs)
				cnt++
				if cnt%100 == 0 {
					fmt.Printf("%s %d/%d attempts %d\n", fl, cnt, *n, attempts)
				}
			}
		}
		if cnt < *n {
			log.Fatalf("flight %s only %d/%d after %d attempts", fl, cnt, *n, attempts)
		}
		fmt.Printf("flight %s done %d attempts %d\n", fl, *n, attempts)
	}
	// write go
	if err := os.MkdirAll(filepath.Dir(*outGo), 0755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*outGo)
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(all); err != nil {
		log.Fatal(err)
	}
	f.Close()
	fmt.Printf("wrote %d cases to %s\n", len(all), *outGo)
	// write js (same)
	if err := os.MkdirAll(filepath.Dir(*outJS), 0755); err != nil {
		log.Fatal(err)
	}
	f2, err := os.Create(*outJS)
	if err != nil {
		log.Fatal(err)
	}
	enc2 := json.NewEncoder(f2)
	enc2.SetIndent("", "  ")
	if err := enc2.Encode(all); err != nil {
		log.Fatal(err)
	}
	f2.Close()
	fmt.Printf("wrote %d cases to %s\n", len(all), *outJS)
}
