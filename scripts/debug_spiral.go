//go:build ignore
package main

import (
	"context"
	"fmt"
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/geometry"
	"stairplatform/internal/engine/solver"
)

func mustLength(v float64) engineering.Length { l,_ := engineering.NewLength(v); return l }

func main() {
	W := 600.0
	H := 2700.0
	h := 180.0
	R := 900.0
	mat := "STEEL-S235"
	cfg, _ := engineering.NewStairConfiguration(mustLength(W), mustLength(H), engineering.FlightSpiral)
	cfg.StepHeight = mustLength(h)
	cfg.StepThickness = mustLength(6)
	cfg.StringerThickness = mustLength(50)
	cfg.Clearance = mustLength(2000)
	cfg.RailingHeight = mustLength(900)
	cfg.ApproachSpace = mustLength(1000)
	cfg.OuterRadius = mustLength(R)
	cfg.SpiralDirection = engineering.SpiralCW
	cfg.Material = mat
	prof := constraint.StandardProfile("std")
	_, vr, err := solver.SolveCheckedSpiral(cfg, prof)
	fmt.Printf("err=%v vr.Blocking=%v\n", err, vr.Blocking)
	for _, iss := range vr.Issues {
		fmt.Printf("issue %+v\n", iss)
	}
	gen, err := geometry.Generate(context.Background(), cfg)
	fmt.Printf("gen err=%v vertices %d\n", err, len(gen.Mesh.Vertices))
}
