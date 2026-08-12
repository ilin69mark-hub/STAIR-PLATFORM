package manufacturing

import (
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
