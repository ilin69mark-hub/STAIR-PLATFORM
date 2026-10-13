package cad

import (
	"bytes"
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// СПЕЦИФИКАЦИЯ: гейты содержимого.
//
// Спецификация — документ, по которому считают и проверяют комплект. Если в
// ней нет колонки стикеров, связать её с наклейками не по чему, и монтажник
// сверяет «15 проступей» с охапкой деталей на глаз.

// specPackage — пакет с двумя строками: две проступи и косоур.
func specPackage() *dommfg.ManufacturingPackage {
	return &dommfg.ManufacturingPackage{
		BOM: dommfg.BOM{Lines: []dommfg.BOMLine{{
			Number: 1, PartNumber: "TRD-01", Description: "Проступь",
			MaterialCode: "WOOD-OAK", Thickness: 40, Quantity: 2,
			Length: 900, Width: 290,
			Numbers: []dommfg.PartNumber{"TRD-01", "TRD-02"},
		}, {
			Number: 2, PartNumber: "STR-01", Description: "Косоур",
			MaterialCode: "STEEL-S235", Thickness: 8, Quantity: 1,
			Length: 2428, Width: 300,
			Numbers: []dommfg.PartNumber{"STR-01"},
		}}},
	}
}

// TestWriteSpecListsStickers — колонка стикеров содержит номера позиции.
func TestWriteSpecListsStickers(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSpec(&buf, specPackage()); err != nil {
		t.Fatalf("WriteSpec: %v", err)
	}
	out := buf.String()
	// Стикеры обеих проступей — в одной ячейке строки позиции.
	if !strings.Contains(out, "TRD-01, TRD-02") && !strings.Contains(out, "TRD-01,TRD-02") {
		t.Errorf("в строке проступей нет обоих стикеров:\n%s", out)
	}
	if !strings.Contains(out, "STR-01") {
		t.Errorf("в строке косоура нет стикера:\n%s", out)
	}
}

// TestWriteSpecHeaderStable — заголовок зафиксирован.
func TestWriteSpecHeaderStable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSpec(&buf, specPackage()); err != nil {
		t.Fatalf("WriteSpec: %v", err)
	}
	first := strings.SplitN(buf.String(), "\n", 2)[0]
	want := "поз;деталь;материал;толщина_мм;длина_мм;ширина_мм;колво;стикеры"
	if first != want {
		t.Errorf("заголовок изменился: %q, want %q", first, want)
	}
}

// TestWriteSpecRequiresPackage — без пакета спецификации нет.
func TestWriteSpecRequiresPackage(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSpec(&buf, nil); err == nil {
		t.Fatal("спецификация без пакета не отменена")
	}
}

// TestWriteSpecRowsMatchQuantity — количество в строке совпадает с числом
// стикеров. Расхождение означало бы, что позиция считает одно, а наклеек
// напечатано другое.
func TestWriteSpecRowsMatchQuantity(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSpec(&buf, specPackage()); err != nil {
		t.Fatalf("WriteSpec: %v", err)
	}
	lines := splitCSV(buf.String())
	if len(lines) != 3 {
		t.Fatalf("строк %d (заголовок + 2 позиции), want 3", len(lines))
	}
	// Строка проступей: колво 2 и два стикера.
	cols := splitRow(lines[1])
	if cols[6] != "2" {
		t.Errorf("количество в строке проступей = %q, want 2", cols[6])
	}
	if cols[7] != "TRD-01, TRD-02" && cols[7] != "TRD-01,TRD-02" {
		t.Errorf("стикеры в строке проступей = %q", cols[7])
	}
}
