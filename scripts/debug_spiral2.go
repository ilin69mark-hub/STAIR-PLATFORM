//go:build ignore
package main

import (
	"fmt"
	"math"
	"math/rand"
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/solver"
)

func mustLength(v float64) engineering.Length { l,_ := engineering.NewLength(v); return l }

func main() {
	for i:=0;i<5;i++{
		W := 500 + rand.Float64()*200
		H := 2700.0
		n := 15
		h := 180.0
		desiredBWalk := 270 + rand.Float64()*10
		R := desiredBWalk*float64(n)/(2*math.Pi) + W/3
		fmt.Printf("try W=%.0f R=%.0f bWalk desired %.0f => R %.0f\n", W, R, desiredBWalk, R)
		cfg,_ := engineering.NewStairConfiguration(mustLength(W), mustLength(H), engineering.FlightSpiral)
		cfg.StepHeight = mustLength(h)
		cfg.StepThickness = mustLength(6)
		cfg.StringerThickness = mustLength(50)
		cfg.Clearance = mustLength(2000)
		cfg.RailingHeight = mustLength(900)
		cfg.ApproachSpace = mustLength(1000)
		cfg.OuterRadius = mustLength(R)
		cfg.SpiralDirection = engineering.SpiralCW
		cfg.Material = "STEEL-S235"
		prof := constraint.StandardProfile("std")
		_, vr, err := solver.SolveCheckedSpiral(cfg, prof)
		fmt.Printf(" err=%v blocking=%v issues=%v\n", err, vr.Blocking, vr.Issues)
		if err==nil && !vr.Blocking {
			fmt.Println("valid!")
			break
		}
	}
}
