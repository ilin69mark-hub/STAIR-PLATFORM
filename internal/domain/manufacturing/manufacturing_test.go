package manufacturing

import (
	"reflect"
	"testing"

	"stairplatform/internal/domain/engineering"
)

func testMaterial(t *testing.T, code MaterialCode) *Material {
	t.Helper()
	return &Material{
		Code: code, Name: "Test " + string(code), Category: "Steel",
		Density: 7850, MinThickness: 2, MaxThickness: 60,
	}
}

func TestMaterialValidate(t *testing.T) {
	if err := testMaterial(t, "M-1").Validate(); err != nil {
		t.Fatalf("valid material must pass: %v", err)
	}
	if err := (&Material{}).Validate(); err == nil {
		t.Fatal("empty material must be rejected")
	}
	m := testMaterial(t, "M-1")
	m.Density = 0
	if err := m.Validate(); err == nil {
		t.Fatal("non-positive density must be rejected")
	}
	m = testMaterial(t, "M-1")
	m.MaxThickness = 1
	if err := m.Validate(); err == nil {
		t.Fatal("inverted thickness range must be rejected")
	}
}

func TestMaterialSupportsThickness(t *testing.T) {
	m := testMaterial(t, "M-1")
	if !m.SupportsThickness(2) || !m.SupportsThickness(60) || !m.SupportsThickness(40) {
		t.Fatal("thickness inside range must be supported")
	}
	if m.SupportsThickness(1.9) || m.SupportsThickness(61) {
		t.Fatal("thickness outside range must not be supported")
	}
}

func TestMaterialRegistry(t *testing.T) {
	reg, err := NewMaterialRegistry(testMaterial(t, "A"), testMaterial(t, "B"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewMaterialRegistry(testMaterial(t, "A"), testMaterial(t, "A")); err == nil {
		t.Fatal("duplicate code must be rejected")
	}
	if _, err := NewMaterialRegistry(&Material{}); err == nil {
		t.Fatal("invalid material must be rejected")
	}
	if m, ok := reg.Find("A"); !ok || m.Code != "A" {
		t.Fatal("find by code")
	}
	if _, ok := reg.Find("missing"); ok {
		t.Fatal("missing code must not be found")
	}
	// детерминированный порядок.
	mats := reg.Materials()
	if len(mats) != 2 || mats[0].Code != "A" || mats[1].Code != "B" {
		t.Fatalf("material order must be deterministic: %+v", mats)
	}
}

func TestPartKindValidation(t *testing.T) {
	for _, k := range []PartKind{PartStringer, PartTread, PartRiser} {
		if !k.IsValid() {
			t.Fatalf("kind %q must be valid", k)
		}
	}
	if PartKind("bogus").IsValid() {
		t.Fatal("unknown kind must be invalid")
	}
}

func TestManufacturingPackageValidate(t *testing.T) {
	mk := func() *ManufacturingPackage {
		return &ManufacturingPackage{
			Parts: []Part{{
				Number: "P-1", Kind: PartTread, Material: "M",
				Thickness: engineering.Length(40), Length: engineering.Length(800), Width: engineering.Length(270),
				SolidIndex: 0,
			}},
			BOM: BOM{Lines: []BOMLine{{
				Number: 1, PartNumber: "P-1", Description: "tread", MaterialCode: "M",
				Thickness: engineering.Length(40), Quantity: 1,
				Length: engineering.Length(800), Width: engineering.Length(270),
			}}},
			CutList: CutList{},
			Nesting: &NestingResult{
				Sheets: []SheetLayout{{
					MaterialCode: "M", Thickness: engineering.Length(40),
					Length: engineering.Length(800), Width: engineering.Length(270),
					Placed: []PlacedPart{{
						PartNumber: "P-1", Length: engineering.Length(800), Width: engineering.Length(270),
						X: 0, Y: 0,
					}},
				}},
				PartCount: 1, PartArea: 800 * 270, SheetArea: 800 * 270,
				WasteArea: 0, Utilization: 1,
			},
		}
	}

	if err := mk().Validate(); err != nil {
		t.Fatalf("valid package must pass: %v", err)
	}

	p := mk()
	p.Parts[0].Number = ""
	if err := p.Validate(); err == nil {
		t.Fatal("empty part number must be rejected")
	}

	p = mk()
	p.Parts = append(p.Parts, p.Parts[0])
	if err := p.Validate(); err == nil {
		t.Fatal("duplicate part number must be rejected")
	}

	p = mk()
	p.Parts[0].Material = ""
	if err := p.Validate(); err == nil {
		t.Fatal("missing material must be rejected")
	}

	p = mk()
	p.Parts[0].Length = engineering.Length(100)
	if err := p.Validate(); err == nil {
		t.Fatal("length below width must be rejected")
	}

	p = mk()
	p.BOM.Lines[0].PartNumber = "UNKNOWN"
	if err := p.Validate(); err == nil {
		t.Fatal("BOM referencing unknown part must be rejected")
	}

	p = mk()
	p.BOM.Lines[0].Quantity = 0
	if err := p.Validate(); err == nil {
		t.Fatal("non-positive quantity must be rejected")
	}

	p = mk()
	p.BOM.Lines[0].Number = 2
	if err := p.Validate(); err == nil {
		t.Fatal("non-sequential BOM line must be rejected")
	}
}

func TestManufacturingPackageValidateEmpty(t *testing.T) {
	if err := (&ManufacturingPackage{}).Validate(); err == nil {
		t.Fatal("package without parts must be rejected")
	}
	if err := (*ManufacturingPackage)(nil).Validate(); err == nil {
		t.Fatal("nil package must be rejected")
	}
}

func TestRegistryDeterminism(t *testing.T) {
	a, _ := NewMaterialRegistry(testMaterial(t, "A"), testMaterial(t, "B"))
	b, _ := NewMaterialRegistry(testMaterial(t, "A"), testMaterial(t, "B"))
	if !reflect.DeepEqual(a.Materials(), b.Materials()) {
		t.Fatal("registry construction must be deterministic")
	}
}
