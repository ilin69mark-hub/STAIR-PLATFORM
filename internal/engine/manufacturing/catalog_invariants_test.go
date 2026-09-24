package manufacturing

import (
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// TestEveryCatalogMaterialIsManufacturable — инвариант каталога: у КАЖДОГО
// материала из DefaultMaterialRegistry есть листы (MFG-0012) и скорость реза
// (MFG-0009). Этап 1 расширил каталог до 7 кодов (орех/ясень/сосна/кортен),
// а листы и скорости остались от старых трёх: расчёт по новому материалу
// падал либо в blocking «изготовление невозможно», либо в 500 «no cut speed».
func TestEveryCatalogMaterialIsManufacturable(t *testing.T) {
	materials, err := DefaultMaterialRegistry()
	if err != nil {
		t.Fatalf("material registry: %v", err)
	}
	sheets, err := DefaultStockSheetRegistry()
	if err != nil {
		t.Fatalf("stock sheet registry: %v", err)
	}
	rates := DefaultMachineRates()

	for _, m := range materials.Materials() {
		code := m.Code
		if len(sheets.SheetsFor(code)) == 0 {
			t.Errorf("материал %s: нет листов в каталоге MFG-0012", code)
		}
		if _, ok := rates.CutSpeed[code]; !ok {
			t.Errorf("материал %s: нет скорости реза в MFG-0009", code)
		}
	}
}

// TestLargestStockSheetReturnsRealSheet — LargestStockSheet обязан вернуть
// реальный лист из каталога, а не «максимум по каждой оси отдельно»
// (для {8000×4600, 10400×6200} это несуществующий 10400×6200).
func TestLargestStockSheetReturnsRealSheet(t *testing.T) {
	sheets, err := DefaultStockSheetRegistry()
	if err != nil {
		t.Fatalf("stock sheet registry: %v", err)
	}
	for _, code := range []dommfg.MaterialCode{"STEEL-S235", "ALUM-5083", "WOOD-OAK"} {
		l, w, ok := LargestStockSheet(sheets, code)
		if !ok {
			t.Fatalf("материал %s: LargestStockSheet не нашёл лист", code)
		}
		real := false
		for _, s := range sheets.SheetsFor(code) {
			if s.Length.Millimeters() == l && s.Width.Millimeters() == w {
				real = true
				break
			}
		}
		if !real {
			t.Errorf("материал %s: %v×%v нет среди листов каталога", code, l, w)
		}
	}
}
