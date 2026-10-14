package cad

import (
	"bytes"
	"math"
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	kerngeo "stairplatform/internal/geometry"
)

// ДОКУМЕНТ РЕЗА: гейты содержимого.
//
// Проверяется не «файл записался», а ЧТО в нём. Прежний DXF писал 3DFACE
// треугольники меша — и это проходило любой тест на непустой вывод. Документ
// реза без проверки содержимого — это файл, который выглядит правильно и
// режет не то.

// cuttingFixture — деталь: прямоугольная плита с круглым отверстием внутри
// и овалом снаружи (за пределами детали — технологическое поле, в рез не идёт).
func cuttingFixture() []cuttingPart {
	hole := circlePoints2D(5, 24, 30, 20)
	field := circlePoints2D(5, 24, 200, 60)
	return []cuttingPart{{
		Number:   "HLT-01",
		Material: "STEEL-C245",
		Outer: []kerngeo.Point2D{
			{X: 0, Y: 0}, {X: 260, Y: 0}, {X: 260, Y: 50}, {X: 0, Y: 50},
		},
		Holes:  [][]kerngeo.Point2D{hole, field},
		DXF:    kerngeo.Point2D{X: 10, Y: 20},
		Length: 260,
		Width:  50,
	}}
}

// circlePoints2D — окружность заданным числом точек.
func circlePoints2D(radius float64, segs int, cx, cy float64) []kerngeo.Point2D {
	out := make([]kerngeo.Point2D, segs)
	for i := range segs {
		a := 2 * math.Pi * float64(i) / float64(segs)
		out[i] = kerngeo.Point2D{X: cx + radius*math.Cos(a), Y: cy + radius*math.Sin(a)}
	}
	return out
}

// TestCuttingDXFEmitsRealCircles — круглое отверстие выходит сущностью CIRCLE,
// а не многоугольником. Это то, ради чего владелец потребовал «честную дугу».
func TestCuttingDXFEmitsRealCircles(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCuttingDXF(&buf, cuttingFixture()); err != nil {
		t.Fatalf("writeCuttingDXF: %v", err)
	}
	out := buf.String()
	if strings.Count(out, "\nCIRCLE\n") != 1 {
		t.Fatalf("ожидалась одна сущность CIRCLE, найдено %d:\n%s",
			strings.Count(out, "\nCIRCLE\n"), out)
	}
	// Радиус в файле ровно тот, по которому считали диаметр детали.
	if !strings.Contains(out, "40\n5.000000\n") {
		t.Errorf("в CIRCLE нет радиуса 5.000000:\n%s", out)
	}
	// Внешний контур — ломаная, не 3DFACE.
	if strings.Contains(out, "3DFACE") {
		t.Error("в документе реза остались 3DFACE — это дамп меша, резать им нельзя")
	}
	if !strings.Contains(out, "LWPOLYLINE") {
		t.Error("в документе реза нет внешнего контура детали")
	}
}

// TestCuttingDXFSkipsHolesOutsidePart — отверстие в поле заготовки в рез не
// попадает.
func TestCuttingDXFSkipsHolesOutsidePart(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCuttingDXF(&buf, cuttingFixture()); err != nil {
		t.Fatalf("writeCuttingDXF: %v", err)
	}
	if n := strings.Count(buf.String(), "\nCIRCLE\n"); n != 1 {
		t.Errorf("отверстий в рез вышло %d, want 1: отверстие в поле детали лазер не ведёт", n)
	}
}

// TestCuttingDXFClosedPolylines — ломаные замкнуты (код 70 = 1).
//
// Незамкнутый контур в документе реза означает, что лазер не дорежет контур:
// деталь остаётся соединённой с обрезком по одному краю.
func TestCuttingDXFClosedPolylines(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCuttingDXF(&buf, cuttingFixture()); err != nil {
		t.Fatalf("writeCuttingDXF: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "70\n1\n") {
		t.Errorf("нет ни одной замкнутой ломаной (70 = 1):\n%s", out)
	}
	if strings.Contains(out, "70\n0\n") {
		t.Error("найдена незамкнутая ломаная — контур детали не будет дорезан")
	}
}

// TestCuttingDXFNumbersOnOwnLayer — номер детали пишется на отдельном слое.
//
// Слой отдельный, чтобы при подготовке файла к резке номера отключались
// одной кнопкой: лазер не режет текст, а на экране станка номер видеть надо.
func TestCuttingDXFNumbersOnOwnLayer(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCuttingDXF(&buf, cuttingFixture()); err != nil {
		t.Fatalf("writeCuttingDXF: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "\nTEXT\n") {
		t.Fatal("номер детали не попал в файл — оператор не знает, что вырезает")
	}
	if !strings.Contains(out, "1\nHLT-01\n") {
		t.Errorf("в файле нет текста HLT-01:\n%s", out)
	}
	if !strings.Contains(out, "8\nНOМЕРА\n") {
		t.Error("номер детали не на своём слое — его нельзя отключить перед резкой")
	}
}

// TestCuttingDXFDeterministic — два прогона дают байтово равный файл.
//
// Файл реза попадает в архив проекта и сверяется при повторном расчёте.
// Расхождение байтов означало бы «деталь изменилась» там, где она не менялась.
func TestCuttingDXFDeterministic(t *testing.T) {
	var a, b bytes.Buffer
	if err := writeCuttingDXF(&a, cuttingFixture()); err != nil {
		t.Fatal(err)
	}
	if err := writeCuttingDXF(&b, cuttingFixture()); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Error("два прогона дали разный файл")
	}
}

// TestWriteCuttingRequiresPackage — без производственного пакета документ
// реза не делается, и ошибка называет причину.
func TestWriteCuttingRequiresPackage(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCutting(&buf, nil); err == nil {
		t.Fatal("документ реза без пакета не отменён")
	}
	if err := WriteCutting(&buf, &dommfg.ManufacturingPackage{}); err == nil {
		t.Fatal("документ реза с пустым раскроем не отменён")
	}
}

// TestWriteCuttingPlacesPartsOnSheet — детали попадают в файл на свои места
// листа, а не в нули.
func TestWriteCuttingPlacesPartsOnSheet(t *testing.T) {
	pkg := cuttingPackage(t)
	var buf bytes.Buffer
	if err := WriteCutting(&buf, pkg); err != nil {
		t.Fatalf("WriteCutting: %v", err)
	}
	out := buf.String()
	// Первая вершина контура детали сдвинута на место на листе (X = 100).
	if !strings.Contains(out, "10\n100.0000\n20\n200.0000\n") {
		t.Errorf("контур детали не сдвинут на место листа:\n%s", out)
	}
	// Номер детали тоже на месте листа.
	if !strings.Contains(out, "10\n100.0000\n20\n200.0000\n40\n5.00\n1\nHLT-01\n") {
		t.Errorf("номер детали не на месте листа:\n%s", out)
	}
}

// cuttingPackage — минимальный пакет: одна позиция раскроя, размещённая на
// листе в (100, 200).
func cuttingPackage(t *testing.T) *dommfg.ManufacturingPackage {
	t.Helper()
	ring := circlePoints2D(5, 24, 30, 20)
	domHoles := make([]dommfg.Point2D, 0, len(ring))
	for _, p := range ring {
		domHoles = append(domHoles, dommfg.Point2D{X: p.X, Y: p.Y})
	}
	item := dommfg.CutItem{
		PartNumber:   "HLT-01",
		MaterialCode: "STEEL-C245",
		Length:       260,
		Width:        50,
		Quantity:     1,
		Profile: []dommfg.Point2D{
			{X: 0, Y: 0}, {X: 260, Y: 0}, {X: 260, Y: 50}, {X: 0, Y: 50},
		},
		Holes: [][]dommfg.Point2D{domHoles},
	}
	return &dommfg.ManufacturingPackage{
		CutList: dommfg.CutList{Items: []dommfg.CutItem{item}},
		Nesting: &dommfg.NestingResult{Sheets: []dommfg.SheetLayout{{
			MaterialCode: "STEEL-C245",
			Length:       3000,
			Width:        1500,
			Placed: []dommfg.PlacedPart{{
				PartNumber: "HLT-01", Length: 260, Width: 50, X: 100, Y: 200,
			}},
		}}},
	}
}
