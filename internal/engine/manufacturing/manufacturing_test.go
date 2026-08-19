package manufacturing

import (
	"context"
	"math"
	"reflect"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
	kerngeo "stairplatform/internal/geometry"
)

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6
}

func mustLength(t *testing.T, mm float64) engineering.Length {
	t.Helper()
	l, err := engineering.NewLength(mm)
	if err != nil {
		t.Fatalf("NewLength(%v): %v", mm, err)
	}
	return l
}

// testConfig возвращает валидную конфигурацию прямого марша (H=2700,
// n=15, h=180, b=270, W=900, T=50, st=40).
func testConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.StepThickness = mustLength(t, 40)
	return cfg
}

func genResult(t *testing.T, cfg *engineering.StairConfiguration) *enggeo.GenerationResult {
	t.Helper()
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestManufactureParts(t *testing.T) {
	pkg, err := Manufacture(testConfig(t), genResult(t, testConfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Parts) != 32 {
		t.Fatalf("parts = %d, want 32", len(pkg.Parts))
	}
	// уникальные номера: 2 косоура, 15 проступей, 15 подступенков.
	seen := make(map[dommfg.PartNumber]bool)
	byKind := make(map[dommfg.PartKind]int)
	for _, p := range pkg.Parts {
		if seen[p.Number] {
			t.Fatalf("duplicate part number %q", p.Number)
		}
		seen[p.Number] = true
		byKind[p.Kind]++
	}
	if byKind[dommfg.PartStringer] != 2 || byKind[dommfg.PartTread] != 15 || byKind[dommfg.PartRiser] != 15 {
		t.Fatalf("part kinds = %+v, want stringer 2 / tread 15 / riser 15", byKind)
	}
	// трассировка к геометрии: индексы покрывают все 32 тела.
	for i, p := range pkg.Parts {
		if p.SolidIndex != i {
			t.Fatalf("part %q solid index = %d, want %d", p.Number, p.SolidIndex, i)
		}
	}
}

func TestManufacturePartDimensions(t *testing.T) {
	pkg, err := Manufacture(testConfig(t), genResult(t, testConfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	// косоур-гребёнка: 4077.7×2660×50 (седла на низ проступи, спинка до пола).
	if p := findPart(pkg, "STR-01"); p != nil {
		if !nearlyEqual(p.Length.Millimeters(), 4077.73501) || !nearlyEqual(p.Width.Millimeters(), 2660) ||
			!nearlyEqual(p.Thickness.Millimeters(), 50) {
			t.Fatalf("stringer dims = %v×%v×%v, want 4077.735×2660×50",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
	} else {
		t.Fatal("STR-01 not found")
	}
	// проступь: 900×310×40 (во всю ширину, глубина шага + толщина подступенка, тонок по Z).
	if p := findPart(pkg, "TRD-01"); p != nil {
		if !nearlyEqual(p.Length.Millimeters(), 900) || !nearlyEqual(p.Width.Millimeters(), 310) ||
			!nearlyEqual(p.Thickness.Millimeters(), 40) {
			t.Fatalf("tread dims = %v×%v×%v, want 900×310×40",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
	} else {
		t.Fatal("TRD-01 not found")
	}
	// подступенок: единое полотно во всю ширину (тонок по X, высота h−st):
	// 900×140×40.
	if p := findPart(pkg, "RSR-01"); p != nil {
		if !nearlyEqual(p.Length.Millimeters(), 900) || !nearlyEqual(p.Width.Millimeters(), 140) ||
			!nearlyEqual(p.Thickness.Millimeters(), 40) {
			t.Fatalf("riser dims = %v×%v×%v, want 900×140×40",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
	} else {
		t.Fatal("RSR-01 not found")
	}
	if p := findPart(pkg, "RSR-02"); p != nil {
		if !nearlyEqual(p.Length.Millimeters(), 900) || !nearlyEqual(p.Width.Millimeters(), 140) ||
			!nearlyEqual(p.Thickness.Millimeters(), 40) {
			t.Fatalf("riser 2 dims = %v×%v×%v, want 900×140×40",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
	} else {
		t.Fatal("RSR-02 not found")
	}
	// материал назначен каждому косоуру/проступи/подступенку из каталога.
	for _, p := range pkg.Parts {
		if _, ok := DefaultMaterialRegistry().Find(p.Material); !ok {
			t.Fatalf("part %q material %q not in registry", p.Number, p.Material)
		}
	}
}

func TestManufactureBOM(t *testing.T) {
	pkg, err := Manufacture(testConfig(t), genResult(t, testConfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.BOM.Lines) != 3 {
		t.Fatalf("BOM lines = %d, want 3", len(pkg.BOM.Lines))
	}
	// порядок строк детерминирован: косоуры, проступи, подступенки
	// (единое полотно 900×140).
	want := []struct {
		desc     string
		quantity int
		length   float64
		width    float64
	}{
		{"Stringer", 2, 4077.73501, 2660},
		{"Tread", 15, 900, 310},
		{"Riser", 15, 900, 140},
	}
	for i, w := range want {
		line := pkg.BOM.Lines[i]
		if line.Description != w.desc {
			t.Fatalf("line %d description = %q, want %q", i, line.Description, w.desc)
		}
		if int(line.Quantity) != w.quantity {
			t.Fatalf("line %d quantity = %d, want %d", i, int(line.Quantity), w.quantity)
		}
		if !nearlyEqual(line.Length.Millimeters(), w.length) || !nearlyEqual(line.Width.Millimeters(), w.width) {
			t.Fatalf("line %d dims = %v×%v, want %v×%v", i, line.Length.Millimeters(), line.Width.Millimeters(), w.length, w.width)
		}
		if line.MaterialCode != "STEEL-S235" {
			t.Fatalf("line %d material = %q, want STEEL-S235", i, line.MaterialCode)
		}
	}
	// карта раскроя повторяет строки BOM.
	if len(pkg.CutList.Items) != len(pkg.BOM.Lines) {
		t.Fatal("cut list must mirror BOM lines")
	}
	for i := range pkg.CutList.Items {
		it := pkg.CutList.Items[i]
		line := pkg.BOM.Lines[i]
		if it.PartNumber != line.PartNumber || it.Quantity != line.Quantity {
			t.Fatalf("cut item %d does not mirror BOM line %d", i, i)
		}
	}
}

func TestManufactureNoSteps(t *testing.T) {
	cfg := testConfig(t)
	cfg.StepThickness = mustLength(t, 0)
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Parts) != 2 {
		t.Fatalf("parts = %d, want 2 (stringers only)", len(pkg.Parts))
	}
	if len(pkg.BOM.Lines) != 1 || int(pkg.BOM.Lines[0].Quantity) != 2 {
		t.Fatalf("BOM = %+v, want single stringer line with quantity 2", pkg.BOM.Lines)
	}
}

func TestManufactureDeterminism(t *testing.T) {
	cfg := testConfig(t)
	gen := genResult(t, cfg)
	a, err := Manufacture(cfg, gen)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Manufacture(cfg, gen)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("manufacturing must be deterministic")
	}
}

func TestManufactureNesting(t *testing.T) {
	pkg, err := Manufacture(testConfig(t), genResult(t, testConfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Nesting == nil {
		t.Fatal("package must contain a nesting result")
	}
	if int(pkg.Nesting.PartCount) != len(pkg.Parts) {
		t.Fatalf("nesting parts = %d, want %d", pkg.Nesting.PartCount, len(pkg.Parts))
	}
	// n=15: 2 косоура (t=50, лист 6000×3000, по одному на лист → 2 листа);
	// проступи и полосы подступенков (t=40, лист 2500×1250) раскраиваются
	// вместе, проступи увеличены до 310 вглубь → 4 листа. Итого 6 листов.
	if len(pkg.Nesting.Sheets) != 6 {
		t.Fatalf("sheets = %d, want 6", len(pkg.Nesting.Sheets))
	}
	if pkg.Nesting.Utilization <= 0 || pkg.Nesting.Utilization > 1 {
		t.Fatalf("utilization out of range: %v", pkg.Nesting.Utilization)
	}
	if err := pkg.Nesting.Validate(); err != nil {
		t.Fatalf("nesting must validate: %v", err)
	}
}

func TestManufactureErrors(t *testing.T) {
	cfg := testConfig(t)
	gen := genResult(t, cfg)

	if _, err := Manufacture(nil, gen); err == nil {
		t.Fatal("nil config must be rejected")
	}
	if _, err := Manufacture(cfg, nil); err == nil {
		t.Fatal("nil geometry result must be rejected")
	}
	// геометрия с ошибками валидации отклоняется (BC-007).
	broken := &enggeo.GenerationResult{
		Model: gen.Model,
		Issues: []kerngeo.ValidationIssue{{
			Code: "GEO-SOLID-NON-POSITIVE-VOLUME", Severity: kerngeo.SeverityError,
			Message: "broken",
		}},
	}
	if _, err := Manufacture(cfg, broken); err == nil {
		t.Fatal("geometry with validation errors must be rejected")
	}
	// невалидная конфигурация отклоняется.
	bad := testConfig(t)
	bad.StepCount = 0
	if _, err := Manufacture(bad, gen); err == nil {
		t.Fatal("invalid config must be rejected")
	}
}

func findPart(pkg *dommfg.ManufacturingPackage, number dommfg.PartNumber) *dommfg.Part {
	for i := range pkg.Parts {
		if pkg.Parts[i].Number == number {
			return &pkg.Parts[i]
		}
	}
	return nil
}
