package manufacturing

import (
	"errors"
	"math"
	"reflect"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// rectCut — карта раскроя из одной позиции.
func rectCut(t *testing.T, material dommfg.MaterialCode, thickness, length, width float64, qty dommfg.Quantity) dommfg.CutList {
	t.Helper()
	return dommfg.CutList{Items: []dommfg.CutItem{{
		PartNumber: "P-1", MaterialCode: material,
		Thickness: mustLength(t, thickness),
		Length:    mustLength(t, length),
		Width:     mustLength(t, width),
		Quantity:  qty,
	}}}
}

func TestNestSingleSheet(t *testing.T) {
	// 12 проступей 800×270 на листе 2500×1250 (kerf 3): 4 ряда × 3 — один лист.
	cut := rectCut(t, "STEEL-S235", 40, 800, 270, 12)
	res, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(res.Sheets))
	}
	sheet := res.Sheets[0]
	if int(res.PartCount) != 12 || len(sheet.Placed) != 12 {
		t.Fatalf("placed = %d, want 12", res.PartCount)
	}
	if sheet.MaterialCode != "STEEL-S235" || sheet.Thickness.Millimeters() != 40 {
		t.Fatalf("sheet material/thickness = %q/%v", sheet.MaterialCode, sheet.Thickness.Millimeters())
	}
	assertNoOverlap(t, sheet)
	if res.Utilization <= 0 || res.Utilization > 1 {
		t.Fatalf("utilization out of range: %v", res.Utilization)
	}
	if err := res.Validate(); err != nil {
		t.Fatalf("nesting must validate: %v", err)
	}
}

func TestNestMultipleSheets(t *testing.T) {
	// 2 косоура 4050×2750: по одному на лист 6000×3000 → 2 листа.
	cut := rectCut(t, "STEEL-S235", 50, 4050, 2750, 2)
	res, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sheets) != 2 {
		t.Fatalf("sheets = %d, want 2", len(res.Sheets))
	}
	for _, s := range res.Sheets {
		if !nearlyEqual(s.Length.Millimeters(), 6000) || !nearlyEqual(s.Width.Millimeters(), 3000) {
			t.Fatalf("sheet dims = %v×%v, want 6000×3000", s.Length.Millimeters(), s.Width.Millimeters())
		}
		assertNoOverlap(t, s)
	}
}

func TestNestPicksSmallestSheet(t *testing.T) {
	// 1 деталь 800×270: выбирается лист 2500×1250, а не 6000×3000.
	cut := rectCut(t, "STEEL-S235", 40, 800, 270, 1)
	res, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sheets) != 1 {
		t.Fatal("single part must use a single sheet")
	}
	if !nearlyEqual(res.Sheets[0].Length.Millimeters(), 2500) || !nearlyEqual(res.Sheets[0].Width.Millimeters(), 1250) {
		t.Fatalf("sheet dims = %v×%v, want smallest fitting 2500×1250",
			res.Sheets[0].Length.Millimeters(), res.Sheets[0].Width.Millimeters())
	}
}

func TestNestDeterminism(t *testing.T) {
	cut := dommfg.CutList{Items: []dommfg.CutItem{
		{PartNumber: "TRD", MaterialCode: "STEEL-S235", Thickness: mustLength(t, 40), Length: mustLength(t, 800), Width: mustLength(t, 270), Quantity: 15},
		{PartNumber: "RSR", MaterialCode: "STEEL-S235", Thickness: mustLength(t, 40), Length: mustLength(t, 800), Width: mustLength(t, 180), Quantity: 15},
		{PartNumber: "STR", MaterialCode: "STEEL-S235", Thickness: mustLength(t, 50), Length: mustLength(t, 4050), Width: mustLength(t, 2750), Quantity: 2},
	}}
	a, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("nesting must be deterministic")
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("nesting must validate: %v", err)
	}
}

func TestNestErrors(t *testing.T) {
	if _, err := Nest(dommfg.CutList{}, DefaultStockSheetRegistry(), DefaultKerf); err == nil {
		t.Fatal("empty cut list must be rejected")
	}
	if _, err := Nest(rectCut(t, "STEEL-S235", 40, 800, 270, 1), nil, DefaultKerf); err == nil {
		t.Fatal("nil registry must be rejected")
	}
	if _, err := Nest(rectCut(t, "STEEL-S235", 40, 800, 270, 1), DefaultStockSheetRegistry(), -1); err == nil {
		t.Fatal("negative kerf must be rejected")
	}
	// материал без листов в каталоге.
	if _, err := Nest(rectCut(t, "TITAN-X", 40, 800, 270, 1), DefaultStockSheetRegistry(), DefaultKerf); err == nil {
		t.Fatal("material without sheets must be rejected")
	}
	// деталь крупнее любого листа каталога.
	if _, err := Nest(rectCut(t, "STEEL-S235", 40, 12000, 12000, 1), DefaultStockSheetRegistry(), DefaultKerf); err == nil {
		t.Fatal("part larger than any sheet must be rejected")
	}
}

func TestNestFeasibilityError(t *testing.T) {
	cut := rectCut(t, "STEEL-S235", 50, 12000, 9000, 1)
	_, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	var feas *FeasibilityError
	if !errors.As(err, &feas) {
		t.Fatalf("want *FeasibilityError, got %T: %v", err, err)
	}
	if feas.PartLen != 12000 || feas.PartWid != 9000 || feas.Kerf != DefaultKerf {
		t.Fatalf("feasibility fields = %+v, want len=12000 wid=9000 kerf=%v", feas, DefaultKerf)
	}
}

func TestSheetFeasible(t *testing.T) {
	reg := DefaultStockSheetRegistry()
	cases := []struct {
		name   string
		rects  []Rect
		expect bool
	}{
		{"косоур 4710×3050 (H=3000) влезает в 8000×4600", []Rect{{4710, 3050}}, true},
		{"косоур 4050×2750 влезает", []Rect{{4050, 2750}}, true},
		{"проступи 800×270 влезают", []Rect{{800, 270}}, true},
		{"поворот граней не мешает", []Rect{{3050, 4710}}, true},
		{"худший косоур H=6000 (10200×6050) влезает в 10400×6200", []Rect{{10200, 6050}}, true},
		{"над max-листом по длине (10500×6300)", []Rect{{10500, 6300}}, false},
		{"над max-листом по ширине (12000×9000)", []Rect{{12000, 9000}}, false},
	}
	for _, tc := range cases {
		if got := SheetFeasible(reg, "STEEL-S235", DefaultKerf, tc.rects...); got != tc.expect {
			t.Errorf("%s: SheetFeasible = %v, want %v", tc.name, got, tc.expect)
		}
	}
}

func TestLargestStockSheet(t *testing.T) {
	l, w, ok := LargestStockSheet(DefaultStockSheetRegistry(), "STEEL-S235")
	if !ok || l != 10400 || w != 6200 {
		t.Fatalf("largest steel sheet = %v×%v ok=%v, want 10400×6200", l, w, ok)
	}
	if _, _, ok := LargestStockSheet(DefaultStockSheetRegistry(), "TITAN-X"); ok {
		t.Fatal("unknown material must report ok=false")
	}
}

// materialCases — сценарии min/средний/max/over-max для каждого материала.
// Деталь выбирается так, чтобы покрываться ровно целевым листом каталога.
var materialCases = []struct {
	name     string
	material dommfg.MaterialCode
	parts    []struct {
		label      string
		length     float64
		width      float64
		feasible   bool
		expectL    float64 // ожидаемый лист (0 — не проверяется)
		expectW    float64
	}
}{
	{
		name:     "steel",
		material: "STEEL-S235",
		parts: []struct {
			label      string
			length     float64
			width      float64
			feasible   bool
			expectL    float64
			expectW    float64
		}{
			{"min лист 2500×1250: деталь 2400×1200", 2400, 1200, true, 2500, 1250},
			{"мид лист 8000×4600: деталь 7900×4500", 7900, 4500, true, 8000, 4600},
			{"max лист 10400×6200: деталь 10300×6100", 10300, 6100, true, 10400, 6200},
			{"регресс H=3000: косоур 4710×3050 → 8000×4600", 4710, 3050, true, 8000, 4600},
			{"over-max: 10500×6300", 10500, 6300, false, 0, 0},
		},
	},
	{
		name:     "aluminum",
		material: "ALUM-5083",
		parts: []struct {
			label      string
			length     float64
			width      float64
			feasible   bool
			expectL    float64
			expectW    float64
		}{
			{"единственный лист 3000×1500: деталь 2900×1400", 2900, 1400, true, 3000, 1500},
			{"плита 6000×3000: деталь 5900×2900", 5900, 2900, true, 6000, 3000},
			{"плита 9000×4600: деталь 8900×4500", 8900, 4500, true, 9000, 4600},
			{"over-max алюминий: 9001×4601", 9001, 4601, false, 0, 0},
		},
	},
	{
		name:     "wood",
		material: "WOOD-OAK",
		parts: []struct {
			label      string
			length     float64
			width      float64
			feasible   bool
			expectL    float64
			expectW    float64
		}{
			{"единственный лист 2500×600: деталь 2400×590", 2400, 590, true, 2500, 600},
			{"плита 2500×1250: деталь 2480×1240", 2480, 1240, true, 2500, 1250},
			{"плита 6000×3000: деталь 5900×2900", 5900, 2900, true, 6000, 3000},
			{"плита 9000×4600: деталь 8900×4500", 8900, 4500, true, 9000, 4600},
			{"over-max дерево: 9001×4601", 9001, 4601, false, 0, 0},
		},
	},
}

func TestSheetFeasiblePerMaterial(t *testing.T) {
	reg := DefaultStockSheetRegistry()
	for _, tc := range materialCases {
		for _, p := range tc.parts {
			ok := SheetFeasible(reg, tc.material, DefaultKerf, Rect{Length: p.length, Width: p.width})
			if ok != p.feasible {
				t.Errorf("%s / %s: SheetFeasible = %v, want %v",
					tc.name, p.label, ok, p.feasible)
			}
		}
	}
}

func TestNestChoosesMinMidMaxSheetPerMaterial(t *testing.T) {
	for _, tc := range materialCases {
		for _, p := range tc.parts {
			if !p.feasible {
				continue
			}
			res, err := Nest(rectCut(t, tc.material, 40, p.length, p.width, 1),
				DefaultStockSheetRegistry(), DefaultKerf)
			if err != nil {
				t.Fatalf("%s / %s: Nest: %v", tc.name, p.label, err)
			}
			if len(res.Sheets) != 1 {
				t.Fatalf("%s / %s: sheets = %d, want 1", tc.name, p.label, len(res.Sheets))
			}
			s := res.Sheets[0]
			if p.expectL != 0 && !nearlyEqual(s.Length.Millimeters(), p.expectL) {
				t.Errorf("%s / %s: sheet length = %v, want %v",
					tc.name, p.label, s.Length.Millimeters(), p.expectL)
			}
			if p.expectW != 0 && !nearlyEqual(s.Width.Millimeters(), p.expectW) {
				t.Errorf("%s / %s: sheet width = %v, want %v",
					tc.name, p.label, s.Width.Millimeters(), p.expectW)
			}
		}
	}
}

// TestNestTallFlightStringer — худший косоур для H=6000 (прогон 10200 ×
// 6050) укладывается на лист 10400×6200 по одному на лист.
func TestNestTallFlightStringer(t *testing.T) {
	for _, qty := range []int{1, 2} {
		wantSheets := qty
		cut := rectCut(t, "STEEL-S235", 50, 10200, 6050, dommfg.Quantity(qty))
		res, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
		if err != nil {
			t.Fatalf("Nest qty=%d: %v", qty, err)
		}
		if len(res.Sheets) != wantSheets {
			t.Fatalf("qty=%d: sheets = %d, want %d", qty, len(res.Sheets), wantSheets)
		}
		for _, s := range res.Sheets {
			if !nearlyEqual(s.Length.Millimeters(), 10400) || !nearlyEqual(s.Width.Millimeters(), 6200) {
				t.Fatalf("qty=%d: sheet dims = %v×%v, want 10400×6200",
					qty, s.Length.Millimeters(), s.Width.Millimeters())
			}
			assertNoOverlap(t, s)
		}
	}
}

func TestLargestStockSheetPerMaterial(t *testing.T) {
	cases := []struct {
		material dommfg.MaterialCode
		length   float64
		width    float64
	}{
		{"STEEL-S235", 10400, 6200},
		{"ALUM-5083", 9000, 4600},
		{"WOOD-OAK", 9000, 4600},
	}
	for _, tc := range cases {
		l, w, ok := LargestStockSheet(DefaultStockSheetRegistry(), tc.material)
		if !ok || l != tc.length || w != tc.width {
			t.Errorf("%s: largest sheet = %v×%v ok=%v, want %v×%v",
				tc.material, l, w, ok, tc.length, tc.width)
		}
	}
}

func assertNoOverlap(t *testing.T, s dommfg.SheetLayout) {
	t.Helper()
	for i := 0; i < len(s.Placed); i++ {
		for j := i + 1; j < len(s.Placed); j++ {
			a, b := s.Placed[i], s.Placed[j]
			overlapX := a.X < b.X+b.Length.Millimeters() && b.X < a.X+a.Length.Millimeters()
			overlapY := a.Y < b.Y+b.Width.Millimeters() && b.Y < a.Y+a.Width.Millimeters()
			if overlapX && overlapY {
				t.Fatalf("parts %q and %q overlap on sheet", a.PartNumber, b.PartNumber)
			}
		}
	}
}

func TestNestAreaMetrics(t *testing.T) {
	cut := rectCut(t, "STEEL-S235", 40, 800, 270, 12)
	res, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf)
	if err != nil {
		t.Fatal(err)
	}
	wantPart := float64(12 * 800 * 270)
	wantSheet := float64(2500 * 1250)
	if math.Abs(res.PartArea-wantPart) > 1e-6 {
		t.Fatalf("part area = %v, want %v", res.PartArea, wantPart)
	}
	if math.Abs(res.SheetArea-wantSheet) > 1e-6 {
		t.Fatalf("sheet area = %v, want %v", res.SheetArea, wantSheet)
	}
	if math.Abs(res.WasteArea-(wantSheet-wantPart)) > 1e-6 {
		t.Fatalf("waste area = %v, want %v", res.WasteArea, wantSheet-wantPart)
	}
	if math.Abs(res.Utilization-wantPart/wantSheet) > 1e-6 {
		t.Fatalf("utilization = %v, want %v", res.Utilization, wantPart/wantSheet)
	}
}
