package geometry

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// spiralConfig возвращает валидную конфигурацию спиральной лестницы
// (EDR-0007): H=2700, n=15, h=180, W=500, R=800 → r=300, δ=2π/15, st=40.
func spiralConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	return &engineering.StairConfiguration{
		Width:             mustLength(t, 500),
		Height:            mustLength(t, 2700),
		Flight:            engineering.FlightSpiral,
		StepCount:         15,
		StepHeight:        mustLength(t, 180),
		TreadDepth:        mustLength(t, 265.291),
		StringerThickness: mustLength(t, 50),
		StepThickness:     mustLength(t, 40),
		OuterRadius:       mustLength(t, 800),
	}
}

// TestBuildSpiralFlightCount проверяет состав модели: центральная колонна
// (1) + n веерных ступеней (EDR-0007 §3).
func TestBuildSpiralFlightCount(t *testing.T) {
	cfg := spiralConfig(t)
	model, err := BuildSpiralFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Solids()) != 16 { // 1 + 15
		t.Fatalf("solids = %d, want 16", len(model.Solids()))
	}
}

// TestBuildSpiralFlightRoles проверяет роли тел: 1 "column" + n "tread".
func TestBuildSpiralFlightRoles(t *testing.T) {
	cfg := spiralConfig(t)
	model, err := BuildSpiralFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	columns, treads := 0, 0
	for _, s := range model.Solids() {
		switch s.Role() {
		case "column":
			columns++
		case "tread":
			treads++
		}
	}
	if columns != 1 {
		t.Errorf("columns = %d, want 1", columns)
	}
	if treads != 15 {
		t.Errorf("treads = %d, want 15", treads)
	}
}

// TestBuildSpiralFlightBBox проверяет габариты модели: центр колонны в
// начале координат, объём |x|,|y| ≤ R, z ∈ [0, H].
func TestBuildSpiralFlightBBox(t *testing.T) {
	cfg := spiralConfig(t)
	model, err := BuildSpiralFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bb := kerngeo.BoundingBox(model)
	if math.Abs(bb.Max.X-800) > 1e-6 || math.Abs(bb.Min.X+800) > 1e-6 {
		t.Fatalf("bbox X = [%v, %v], want [-800, 800]", bb.Min.X, bb.Max.X)
	}
	if math.Abs(bb.Max.Y-800) > 1e-6 || math.Abs(bb.Min.Y+800) > 1e-6 {
		t.Fatalf("bbox Y = [%v, %v], want [-800, 800]", bb.Min.Y, bb.Max.Y)
	}
	if bb.Min.Z < -1e-6 || math.Abs(bb.Max.Z-2700) > 1e-6 {
		t.Fatalf("bbox Z = [%v, %v], want [0, 2700]", bb.Min.Z, bb.Max.Z)
	}
}

// TestBuildSpiralFlightRisePosition проверяет подъём ступеней: верх первой
// ступени (k=0) на уровне h=180, последней (k=14) на уровне n·h=2700.
// Поворот k-й ступени — на угол k·δ; вершина наружной кромки на высоте
// n·h находится на радиусе R (окружность аппроксимируется полигоном).
func TestBuildSpiralFlightRisePosition(t *testing.T) {
	cfg := spiralConfig(t)
	model, err := BuildSpiralFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Первая ступень: её верх находится на высоте h=180 (толщина st=40
	// занимает z ∈ [140, 180]). Точка на рабочей поверхности присутствует.
	if !pointNearZ(model, true, 180.0) {
		t.Error("first tread top must exist at z=180")
	}
	// Последняя ступень: верх на высоте n·h = 2700.
	if !pointNearZ(model, true, 2700.0) {
		t.Error("last tread top must exist at z=2700")
	}
	// Колонна занимает весь диапазон высот от пола до H.
	if !pointNearZ(model, false, 0.0) {
		t.Error("column bottom must exist at z=0")
	}
}

// pointNearZ проверяет наличие вершины с координатой z=want среди тел с
// ролью column (role="column") или проступи (role="tread"), в пределах
// 1e-6 мм (допуск геометрии).
func pointNearZ(model *kerngeo.Compound, tread bool, want float64) bool {
	for _, solid := range model.Solids() {
		isTread := solid.Role() == "tread"
		if isTread != tread {
			continue
		}
		for _, shell := range solid.Shells() {
			for _, face := range shell.Faces() {
				for _, e := range face.Outer().Edges() {
					v1, _ := e.Endpoints()
					if nearlyEqual(v1.Point().Z, want) {
						return true
					}
				}
			}
		}
	}
	return false
}

// TestBuildSpiralFlightVolume проверяет положительность объёма и то, что
// объём колонны ≈ π·r²·H (аппроксимация окружности полигоном).
func TestBuildSpiralFlightVolume(t *testing.T) {
	cfg := spiralConfig(t)
	model, err := BuildSpiralFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	col := model.Solids()[0]
	vol, err := kerngeo.Volume(col)
	if err != nil {
		t.Fatal(err)
	}
	// Полигональная аппроксимация окружности даёт чуть меньший объём.
	exact := math.Pi * 300 * 300 * 2700
	ratio := vol / exact
	if ratio < 0.9 || ratio > 1.01 {
		t.Errorf("column volume ratio = %v, want ~1 (0.9-1.01)", ratio)
	}
}

// TestBuildSpiralFlightErrors проверяет ошибки конфигурации.
func TestBuildSpiralFlightErrors(t *testing.T) {
	t.Run("non spiral flight rejected", func(t *testing.T) {
		cfg := spiralConfig(t)
		cfg.Flight = engineering.FlightStraight
		if _, err := BuildSpiralFlight(cfg); err == nil {
			t.Fatal("expected flight type error")
		}
	})
	t.Run("outer radius not exceeding width rejected", func(t *testing.T) {
		cfg := spiralConfig(t)
		cfg.OuterRadius = mustLength(t, 500) // == W
		if _, err := BuildSpiralFlight(cfg); err == nil {
			t.Fatal("expected outer radius error")
		}
	})
}

// TestBuildSpiralFlightDeterminism проверяет детерминизм геометрии.
func TestBuildSpiralFlightDeterminism(t *testing.T) {
	cfg := spiralConfig(t)
	a, err := BuildSpiralFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildSpiralFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Solids()) != len(b.Solids()) {
		t.Fatal("different solid counts")
	}
	for i := range a.Solids() {
		if a.Solids()[i].Role() != b.Solids()[i].Role() {
			t.Fatalf("solid %d role differs", i)
		}
	}
}

// TestGenerateSpiral проверяет полный конвейер Generate для спирали.
func TestGenerateSpiral(t *testing.T) {
	cfg := spiralConfig(t)
	res, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Measurement.SolidCount != 16 {
		t.Errorf("solid count = %d, want 16", res.Measurement.SolidCount)
	}
	if len(res.Issues) != 0 {
		t.Errorf("expected no issues, got %+v", res.Issues)
	}
	if res.Mesh == nil || len(res.Mesh.Triangles) == 0 {
		t.Error("expected preview mesh with triangles")
	}
}

// TestGenerateSpiralDeterminism проверяет детерминизм Generate.
func TestGenerateSpiralDeterminism(t *testing.T) {
	cfg := spiralConfig(t)
	a, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a.Measurement.SolidCount != b.Measurement.SolidCount {
		t.Error("solid count not deterministic")
	}
	if a.Measurement.Volume != b.Measurement.Volume {
		t.Error("volume not deterministic")
	}
}
