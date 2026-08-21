package manufacturing

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
)

// ushapeConfig возвращает валидную конфигурацию П-образной лестницы
// (H=2700, n=15, n1=6, h=180, b=270, W=900, Wp=1000, T=50, st=40).
func ushapeConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightUShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.StepThickness = mustLength(t, 40)
	cfg.LowerStepCount = 6
	cfg.LandingWidth = mustLength(t, 1000)
	return cfg
}

// TestManufactureUShapeParts проверяет, что П-образная модель декомпозируется
// по семантическим ролям (BC-007): косоуры развёрнутого на 180° верхнего
// марша не классифицируются как подступенки, площадка — как проступь.
func TestManufactureUShapeParts(t *testing.T) {
	cfg := ushapeConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	// 4 косоура (2 нижних + 2 верхних), 16 проступей (15 ступеней + площадка),
	// 15 подступенков (полотно на ступень). Итого 35 деталей.
	if len(pkg.Parts) != 35 {
		t.Fatalf("parts = %d, want 35", len(pkg.Parts))
	}
	byKind := make(map[dommfg.PartKind]int)
	for _, p := range pkg.Parts {
		byKind[p.Kind]++
	}
	if byKind[dommfg.PartStringer] != 4 || byKind[dommfg.PartTread] != 16 || byKind[dommfg.PartRiser] != 15 {
		t.Fatalf("part kinds = %+v, want stringer 4 / tread 16 / riser 15", byKind)
	}
	// трассировка к геометрии: индексы покрывают все 35 тел.
	for i, p := range pkg.Parts {
		if p.SolidIndex != i {
			t.Fatalf("part %q solid index = %d, want %d", p.Number, p.SolidIndex, i)
		}
	}
}

// TestManufactureUShapeDimensions проверяет габариты деталей П-марша:
// нижний косоур 1620×1130×50, верхний 2430×1670×50, проступи 850×310×40
// (марш сужен на flightSideInsetMM), подступенки 850×140×40, площадка 1800×900×40.
func TestManufactureUShapeDimensions(t *testing.T) {
	cfg := ushapeConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		number dommfg.PartNumber
		l, wd  float64
	}{
		{"STR-01", 1647.73501, 1040},
		{"STR-02", 1647.73501, 1040},
		{"STR-03", 2457.73501, 1580},
		{"STR-04", 2457.73501, 1580},
		{"TRD-01", 850, 310},
		{"TRD-07", 1800, 900},
		{"TRD-08", 850, 310},
		{"TRD-16", 850, 310},
		{"RSR-01", 850, 140},
		{"RSR-02", 850, 140},
		{"RSR-15", 850, 140},
	}
	for _, c := range cases {
		p := findPart(pkg, c.number)
		if p == nil {
			t.Fatalf("part %q not found", c.number)
		}
		if !nearlyEqual(p.Length.Millimeters(), c.l) || !nearlyEqual(p.Width.Millimeters(), c.wd) {
			t.Fatalf("part %q dims = %v×%v, want %v×%v",
				c.number, p.Length.Millimeters(), p.Width.Millimeters(), c.l, c.wd)
		}
	}
	// толщины: косоуры 50, проступи/подступенки/площадка 40.
	for _, p := range pkg.Parts {
		want := 50.0
		if p.Kind != dommfg.PartStringer {
			want = 40.0
		}
		if !nearlyEqual(p.Thickness.Millimeters(), want) {
			t.Fatalf("part %q thickness = %v, want %v", p.Number, p.Thickness.Millimeters(), want)
		}
	}
	// материал назначен каждому косоуру/проступи/подступенку из каталога.
	for _, p := range pkg.Parts {
		if _, ok := DefaultMaterialRegistry().Find(p.Material); !ok {
			t.Fatalf("part %q material %q not in registry", p.Number, p.Material)
		}
	}
}

// TestManufactureUShapeBOM проверяет группировку BOM: косоуры двух размеров
// (1620×1130 и 2430×1670) — отдельные строки, проступи и полосы подступенков
// объединяются по размерам, площадка — отдельная проступь.
func TestManufactureUShapeBOM(t *testing.T) {
	cfg := ushapeConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		desc     string
		quantity int
		length   float64
		width    float64
	}{
		{"Stringer", 2, 1647.73501, 1040},
		{"Tread", 15, 850, 310},

		{"Riser", 15, 850, 140},
		{"Tread", 1, 1800, 900},
		{"Stringer", 2, 2457.73501, 1580},
	}
	if len(pkg.BOM.Lines) != len(want) {
		t.Fatalf("BOM lines = %d, want %d", len(pkg.BOM.Lines), len(want))
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
			t.Fatalf("line %d dims = %v×%v, want %v×%v", i,
				line.Length.Millimeters(), line.Width.Millimeters(), w.length, w.width)
		}
	}
	// карта раскроя повторяет строки BOM.
	if len(pkg.CutList.Items) != len(pkg.BOM.Lines) {
		t.Fatal("cut list must mirror BOM lines")
	}
}

// TestManufactureUShapeNesting проверяет, что раскладка по листам для
// П-марша корректна и валидна.
func TestManufactureUShapeNesting(t *testing.T) {
	cfg := ushapeConfig(t)
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
