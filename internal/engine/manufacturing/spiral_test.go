package manufacturing

import (
	"context"
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
)

// spiralConfig возвращает валидную конфигурацию спиральной лестницы
// (EDR-0007): H=2700, n=15, h=180, W=500, R=800 → r=300, st=40, T=50.
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

// TestManufactureSpiralParts проверяет декомпозицию спиральной модели:
// 1 колонна + 15 проступей.
func TestManufactureSpiralParts(t *testing.T) {
	cfg := spiralConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Parts) != 16 {
		t.Fatalf("parts = %d, want 16", len(pkg.Parts))
	}
	byKind := make(map[dommfg.PartKind]int)
	for _, p := range pkg.Parts {
		byKind[p.Kind]++
	}
	if byKind[dommfg.PartColumn] != 1 || byKind[dommfg.PartTread] != 15 {
		t.Fatalf("part kinds = %+v, want column 1 / tread 15", byKind)
	}
	for i, p := range pkg.Parts {
		if p.SolidIndex != i {
			t.Fatalf("part %q solid index = %d, want %d", p.Number, p.SolidIndex, i)
		}
	}
}

// TestManufactureSpiralColumn проверяет развёртку колонны: толщина косоура,
// длина H, ширина периметр 2πr (EDR-0007).
func TestManufactureSpiralColumn(t *testing.T) {
	cfg := spiralConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	col := findPart(pkg, "CLM-01")
	if col == nil {
		t.Fatal("column CLM-01 not found")
	}
	if !nearlyEqual(col.Thickness.Millimeters(), 50) {
		t.Errorf("column thickness = %v, want 50 (stringer thickness)", col.Thickness.Millimeters())
	}
	if !nearlyEqual(col.Length.Millimeters(), 2700) {
		t.Errorf("column length = %v, want 2700 (H)", col.Length.Millimeters())
	}
	circ := 2 * math.Pi * 300
	if !nearlyEqual(col.Width.Millimeters(), circ) {
		t.Errorf("column width = %v, want %v (2πr)", col.Width.Millimeters(), circ)
	}
	// материал назначен из каталога.
	if _, ok := mustMaterials(t).Find(col.Material); !ok {
		t.Fatalf("column material %q not in registry", col.Material)
	}
}

// TestManufactureSpiralBOM проверяет BOM спирали: колонна отдельной строкой
// «Column», проступи группой.
func TestManufactureSpiralBOM(t *testing.T) {
	cfg := spiralConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	var hasColumn bool
	for _, line := range pkg.BOM.Lines {
		if line.Description == "Column" && int(line.Quantity) == 1 {
			hasColumn = true
		}
	}
	if !hasColumn {
		t.Fatal("BOM must contain a single Column line")
	}
	// все детали попали в BOM.
	total := 0
	for _, line := range pkg.BOM.Lines {
		total += int(line.Quantity)
	}
	if total != 16 {
		t.Fatalf("BOM total quantity = %d, want 16", total)
	}
}

// TestManufactureSpiralNesting проверяет раскладку спиральной модели.
func TestManufactureSpiralNesting(t *testing.T) {
	cfg := spiralConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Nesting == nil {
		t.Fatal("package must contain a nesting result")
	}
	if int(pkg.Nesting.PartCount) != len(pkg.Parts) {
		t.Fatalf("nesting parts = %d, want %d", pkg.Nesting.PartCount, len(pkg.Parts))
	}
	if pkg.Nesting.Utilization <= 0 || pkg.Nesting.Utilization > 1 {
		t.Fatalf("utilization out of range: %v", pkg.Nesting.Utilization)
	}
	if err := pkg.Nesting.Validate(); err != nil {
		t.Fatalf("nesting must validate: %v", err)
	}
}

// TestManufactureSpiralDeterminism проверяет детерминизм производства.
func TestManufactureSpiralDeterminism(t *testing.T) {
	cfg := spiralConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Parts) != len(b.Parts) || len(a.BOM.Lines) != len(b.BOM.Lines) {
		t.Fatal("manufacturing result not deterministic")
	}
}
