package manufacturing

import (
	"testing"

	"stairplatform/internal/domain/engineering"
)

func testSheet(t *testing.T, material MaterialCode, length, width float64) *StockSheet {
	t.Helper()
	return &StockSheet{MaterialCode: material, Length: engineering.Length(length), Width: engineering.Length(width)}
}

func TestStockSheetValidate(t *testing.T) {
	if err := testSheet(t, "S", 2500, 1250).Validate(); err != nil {
		t.Fatalf("valid sheet must pass: %v", err)
	}
	if err := (&StockSheet{}).Validate(); err == nil {
		t.Fatal("empty sheet must be rejected")
	}
	if err := testSheet(t, "", 2500, 1250).Validate(); err == nil {
		t.Fatal("sheet without material must be rejected")
	}
	if err := testSheet(t, "S", 0, 1250).Validate(); err == nil {
		t.Fatal("non-positive length must be rejected")
	}
	if err := testSheet(t, "S", 1000, 2000).Validate(); err == nil {
		t.Fatal("length below width must be rejected")
	}
}

func TestStockSheetRegistry(t *testing.T) {
	reg, err := NewStockSheetRegistry(
		testSheet(t, "S", 2500, 1250),
		testSheet(t, "S", 6000, 3000),
		testSheet(t, "A", 3000, 1500),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewStockSheetRegistry(testSheet(t, "S", 2500, 1250), testSheet(t, "S", 2500, 1250)); err == nil {
		t.Fatal("duplicate sheet must be rejected")
	}
	if _, err := NewStockSheetRegistry(&StockSheet{}); err == nil {
		t.Fatal("invalid sheet must be rejected")
	}
	// детерминированный порядок листов для материала.
	sheets := reg.SheetsFor("S")
	if len(sheets) != 2 || sheets[0].Length.Millimeters() != 2500 || sheets[1].Length.Millimeters() != 6000 {
		t.Fatalf("sheet order must be deterministic: %+v", sheets)
	}
	if len(reg.SheetsFor("missing")) != 0 {
		t.Fatal("unknown material must have no sheets")
	}
}

func validNesting() *NestingResult {
	return &NestingResult{
		Sheets: []SheetLayout{{
			MaterialCode: "M", Thickness: engineering.Length(40),
			Length: engineering.Length(2500), Width: engineering.Length(1250),
			Placed: []PlacedPart{{
				PartNumber: "P-1", Length: engineering.Length(800), Width: engineering.Length(270), X: 0, Y: 0,
			}},
		}},
		PartCount:   1,
		PartArea:    800 * 270,
		SheetArea:   2500 * 1250,
		WasteArea:   2500*1250 - 800*270,
		Utilization: (800 * 270) / (2500 * 1250),
	}
}

func TestNestingResultValidate(t *testing.T) {
	if err := validNesting().Validate(); err != nil {
		t.Fatalf("valid nesting must pass: %v", err)
	}
	if err := (*NestingResult)(nil).Validate(); err == nil {
		t.Fatal("nil nesting must be rejected")
	}

	n := validNesting()
	n.Sheets = nil
	if err := n.Validate(); err == nil {
		t.Fatal("nesting without sheets must be rejected")
	}

	n = validNesting()
	n.PartCount = 0
	if err := n.Validate(); err == nil {
		t.Fatal("zero part count must be rejected")
	}

	n = validNesting()
	n.PartArea = 2500 * 1250 * 2
	if err := n.Validate(); err == nil {
		t.Fatal("part area exceeding sheet area must be rejected")
	}

	n = validNesting()
	n.WasteArea = -1
	if err := n.Validate(); err == nil {
		t.Fatal("inconsistent waste area must be rejected")
	}

	n = validNesting()
	n.Utilization = 2
	if err := n.Validate(); err == nil {
		t.Fatal("utilization above 1 must be rejected")
	}

	n = validNesting()
	n.Sheets[0].Placed[0].X = 2000
	if err := n.Validate(); err == nil {
		t.Fatal("placed part exceeding sheet bounds must be rejected")
	}

	n = validNesting()
	n.PartCount = 2
	if err := n.Validate(); err == nil {
		t.Fatal("placed parts mismatch must be rejected")
	}
}

func TestManufacturingPackageValidateNesting(t *testing.T) {
	// valid nesting already required by TestManufacturingPackageValidate.
	p := validNesting()
	p.PartCount = 0
	if err := p.Validate(); err == nil {
		t.Fatal("invalid nesting must be rejected")
	}
}
