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
	res, err := Nest(cut, mustSheets(t))
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

// Несколько ЛИСТОВ: деталей больше, чем влезает в две заготовки одного листа
// 6000×1500, поэтому листов два.
func TestNestMultipleSheets(t *testing.T) {
	// Вместимость заготовки считает та же формула, что и раскрой, поэтому
	// эталон проверяется против неё: 3 детали в ряду × 5 рядов = 15 на
	// заготовку, 30 на лист. 50 деталей → 4 заготовки → 2 листа.
	perBlank := dommfg.BlankCapacity(8, 800, 270)
	if perBlank != 15 {
		t.Fatalf("вместимость заготовки %d, ожидалось 15 (3 в ряду × 5 рядов)", perBlank)
	}
	qty := 50
	cut := rectCut(t, "STEEL-S235", 8, 800, 270, dommfg.Quantity(qty))
	res, err := Nest(cut, mustSheets(t))
	if err != nil {
		t.Fatal(err)
	}
	want := dommfg.SheetsForBlanks((qty + perBlank - 1) / perBlank)
	if want != 2 {
		t.Fatalf("эталон расчёта разъехался: листов %d, ожидалось 2", want)
	}
	if len(res.Sheets) != want {
		t.Fatalf("листов %d, ожидалось %d", len(res.Sheets), want)
	}
	for _, sh := range res.Sheets {
		if !nearlyEqual(sh.Length.Millimeters(), dommfg.StockSheetLengthMM) ||
			!nearlyEqual(sh.Width.Millimeters(), dommfg.StockSheetWidthMM) {
			t.Fatalf("лист %.0f×%.0f, ожидалось %.0f×%.0f",
				sh.Length.Millimeters(), sh.Width.Millimeters(),
				dommfg.StockSheetLengthMM, dommfg.StockSheetWidthMM)
		}
		assertNoOverlap(t, sh)
	}
}

// У МЕТАЛЛА типоразмер листа один (6000×1500, сводная шпатка технолога
// 2026-10-05), поэтому выбирать не из чего: расход задаётся заготовками внутри
// листа, а не подбором размера. Раньше листов было четыре, и для мелкой детали
// выбирался самый маленький 2500×1250.
func TestNestSteelSheetSizeIsFixed(t *testing.T) {
	cut := rectCut(t, "STEEL-S235", 8, 800, 270, 1)
	res, err := Nest(cut, mustSheets(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sheets) != 1 {
		t.Fatal("одна деталь должна занять один лист")
	}
	if !nearlyEqual(res.Sheets[0].Length.Millimeters(), 6000) ||
		!nearlyEqual(res.Sheets[0].Width.Millimeters(), 1500) {
		t.Fatalf("лист %.0f×%.0f, ожидалось 6000×1500",
			res.Sheets[0].Length.Millimeters(), res.Sheets[0].Width.Millimeters())
	}
}

func TestNestDeterminism(t *testing.T) {
	cut := dommfg.CutList{Items: []dommfg.CutItem{
		{PartNumber: "TRD", MaterialCode: "STEEL-S235", Thickness: mustLength(t, 8), Length: mustLength(t, 800), Width: mustLength(t, 270), Quantity: 15},
		{PartNumber: "RSR", MaterialCode: "STEEL-S235", Thickness: mustLength(t, 40), Length: mustLength(t, 800), Width: mustLength(t, 180), Quantity: 15},
		{PartNumber: "STR", MaterialCode: "STEEL-S235", Thickness: mustLength(t, 8), Length: mustLength(t, 2644), Width: mustLength(t, 300), Quantity: 2},
	}}
	a, err := Nest(cut, mustSheets(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Nest(cut, mustSheets(t))
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

// Признаки сварного стыка «шип-паз» доезжают до размещения на листе: без них
// куски косоура на карте раскроя неразличимы, и монтажник не видит, какой к
// какому примыкает.
func TestNestCarriesJointMarks(t *testing.T) {
	cut := dommfg.CutList{Items: []dommfg.CutItem{{
		PartNumber: "STR", MaterialCode: "STEEL-S235", Thickness: mustLength(t, 8),
		Length: mustLength(t, 2427), Width: mustLength(t, 300), Quantity: 2,
		HasTongue: true, HasMortise: true, JointCount: 2,
	}}}
	res, err := Nest(cut, mustSheets(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sheets) == 0 {
		t.Fatal("must place parts")
	}
	for _, sh := range res.Sheets {
		for _, p := range sh.Placed {
			if !p.HasTongue || !p.HasMortise {
				t.Fatalf("%s: tongue=%v mortise=%v, оба признака должны доехать до размещения",
					p.PartNumber, p.HasTongue, p.HasMortise)
			}
			if p.JointCount != 2 {
				t.Fatalf("%s: joint count = %d, want 2", p.PartNumber, p.JointCount)
			}
		}
	}
}

// Столбец в BOM склеивает 4 куска косоура в одну строку: шип есть не у каждого,
// но карта раскроя обязана показать, что стык в позиции есть.
func TestBuildBOMAggregatesJointMarks(t *testing.T) {
	parts := []dommfg.Part{
		{Number: "STR-01", Kind: dommfg.PartStringer, Material: "STEEL-S235",
			Thickness: mustLength(t, 8), Length: mustLength(t, 2427), Width: mustLength(t, 300),
			JointIndex: 0, JointCount: 2, HasMortise: true},
		{Number: "STR-02", Kind: dommfg.PartStringer, Material: "STEEL-S235",
			Thickness: mustLength(t, 8), Length: mustLength(t, 2427), Width: mustLength(t, 300),
			JointIndex: 1, JointCount: 2, HasTongue: true, HasMortise: true},
	}
	_, cut := buildBOM(parts)
	if len(cut.Items) != 1 {
		t.Fatalf("cut items = %d, want 1 (4 куска косоура в одну строку)", len(cut.Items))
	}
	it := cut.Items[0]
	if it.Quantity != 2 || !it.HasTongue || !it.HasMortise || it.JointCount != 2 {
		t.Fatalf("joint marks lost in BOM: %+v", it)
	}
}

func TestNestErrors(t *testing.T) {
	if _, err := Nest(dommfg.CutList{}, mustSheets(t)); err == nil {
		t.Fatal("empty cut list must be rejected")
	}
	if _, err := Nest(rectCut(t, "STEEL-S235", 40, 800, 270, 1), nil); err == nil {
		t.Fatal("nil registry must be rejected")
	}
	// материал без листов в каталоге.
	if _, err := Nest(rectCut(t, "TITAN-X", 40, 800, 270, 1), mustSheets(t)); err == nil {
		t.Fatal("material without sheets must be rejected")
	}
	// Деталь крупнее рабочего ПОЛЯ СТОЛА: даже после разбиения на куски её
	// нечем резать. Проверяется именно поле 3000×1500, а не купленный лист
	// 6000×1500 — по листу деталь 3200 мм «влезала» бы.
	if _, err := Nest(rectCut(t, "STEEL-S235", 40, 3200, 2000, 1), mustSheets(t)); err == nil {
		t.Fatal("part longer than the laser field must be rejected")
	}
	// Деталь в поле стола принимается.
	if _, err := Nest(rectCut(t, "STEEL-S235", 40, 2644, 300, 1), mustSheets(t)); err != nil {
		t.Fatalf("part inside the laser field must be accepted: %v", err)
	}
}

// Деталь шире поля лазерного стола не влезает даже после разбиения на куски:
// по длине стык «шип-паз» возможен, по ширине — нет.
func TestNestFeasibilityError(t *testing.T) {
	cut := rectCut(t, "STEEL-S235", 8, 12000, 9000, 1)
	_, err := Nest(cut, mustSheets(t))
	var feas *FeasibilityError
	if !errors.As(err, &feas) {
		t.Fatalf("want *FeasibilityError, got %T: %v", err, err)
	}
	if feas.PartLen != 12000 || feas.PartWid != 9000 {
		t.Fatalf("feasibility fields = %+v, want len=12000 wid=9000", feas)
	}
	if feas.FieldLength != 3000 || feas.FieldWidth != 1500 {
		t.Errorf("поле стола = %.0f×%.0f, ожидалось 3000×1500", feas.FieldLength, feas.FieldWidth)
	}
	if feas.Thickness != 8 {
		t.Errorf("толщина листа = %v, ожидалось 8 (из неё берётся зазор шпатки)", feas.Thickness)
	}
}

// Предел — рабочее поле лазерного стола 3000×1500 с отступом и зазором
// шпатки по толщине листа. Раньше пределом был купленный лист (до
// 10400×6200), и деталь 5200 мм «влезала», хотя её можно только разрезать.
func TestSheetFeasible(t *testing.T) {
	cases := []struct {
		name   string
		rects  []Rect
		expect bool
	}{
		{"проступи 900×300 влезают", []Rect{{900, 300}}, true},
		{"поворот граней не мешает", []Rect{{300, 900}}, true},
		{"пятки 270×50 влезают", []Rect{{270, 50}}, true},
		{"косоур-кусок 2644×300 влезает", []Rect{{2644, 300}}, true},
		{"косоур 2980×300 влезает (поле 3000 − отступ 10)", []Rect{{2980, 300}}, true},
		{"косоур 2990×300 не влезает: 2990 + 2×10 > 3000", []Rect{{2990, 300}}, false},
		{"деталь по ширине 1480 влезает: 1480 + 2×10 = 1500", []Rect{{1480, 1480}}, true},
		{"деталь по ширине 1500 не влезает: 1500 + 2×10 > 1500", []Rect{{1500, 1500}}, false},
		{"деталь шире поля 1600×1600", []Rect{{1600, 1600}}, false},
	}
	for _, tc := range cases {
		if got := SheetFeasible(8, tc.rects...); got != tc.expect {
			t.Errorf("%s: SheetFeasible = %v, want %v", tc.name, got, tc.expect)
		}
	}
}

func TestLargestStockSheet(t *testing.T) {
	// Сталь: единственный типоразмер 6000×1500 (сводная шпатка технолога
	// 2026-10-05). Раньше в каталоге были листы до 10400×6200, и «максимальный
	// лист» показывался пользователю в сообщениях.
	l, w, ok := LargestStockSheet(mustSheets(t), "STEEL-S235")
	if !ok || l != 6000 || w != 1500 {
		t.Fatalf("largest steel sheet = %v×%v ok=%v, want 6000×1500", l, w, ok)
	}
	// Дерево не тронуто: его листы остались до 9000×4600.
	if l, w, ok := LargestStockSheet(mustSheets(t), "WOOD-OAK"); !ok || l != 9000 || w != 4600 {
		t.Fatalf("largest wood sheet = %v×%v ok=%v, want 9000×4600", l, w, ok)
	}
	if _, _, ok := LargestStockSheet(mustSheets(t), "TITAN-X"); ok {
		t.Fatal("unknown material must report ok=false")
	}
}

// assertNoOverlap проверяет, что на листе нет наложений деталей. Смысл
// раскроя в этом и состоит: площадь деталей, сложенная дважды, дала бы
// неверную утилизацию и неверную цену материала.
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

// Металл и дерево раскраиваются по РАЗНЫМ правилам (решение владельца
// 2026-10-05): металл — по рабочему полю лазерного стола 3000×1500 с
// раскладкой на заготовки и листы 6000×1500; дерево — по листам каталога,
// его раскрой отложен на потом.
func TestSheetFeasiblePerMaterial(t *testing.T) {
	cases := []struct {
		label     string
		thickness float64
		length    float64
		width     float64
		expect    bool
	}{
		{"сталь: проступь 900×300", 8, 900, 300, true},
		{"сталь: кусок косоура 2644×300", 8, 2644, 300, true},
		{"сталь: ровно поле 2980×1480", 8, 2980, 1480, true},
		{"сталь: на 1 мм шире поля", 8, 2981, 300, false},
		{"сталь: деталь длиннее поля", 8, 3200, 300, false},
		{"сталь: деталь шире поля", 8, 1500, 1500, false},
	}
	for _, c := range cases {
		got := SheetFeasible(c.thickness, Rect{Length: c.length, Width: c.width})
		if got != c.expect {
			t.Errorf("%s: SheetFeasible = %v, want %v", c.label, got, c.expect)
		}
	}
}

// Дерево раскраивается по листам каталога (2500×600 … 9000×4600) — этот путь
// не тронут полем лазерного стола, потому что дерево режется на другом станке.
func TestNestWoodUsesStockSheets(t *testing.T) {
	cases := []struct {
		label   string
		length  float64
		width   float64
		expectL float64
		expectW float64
	}{
		{"деталь 2400×590 → лист 2500×600", 2400, 590, 2500, 600},
		{"деталь 2480×1240 → лист 2500×1250", 2480, 1240, 2500, 1250},
		{"деталь 5900×2900 → плита 6000×3000", 5900, 2900, 6000, 3000},
		{"деталь 8900×4500 → плита 9000×4600", 8900, 4500, 9000, 4600},
	}
	for _, c := range cases {
		res, err := Nest(rectCut(t, "WOOD-OAK", 40, c.length, c.width, 1), mustSheets(t))
		if err != nil {
			t.Fatalf("%s: %v", c.label, err)
		}
		if len(res.Sheets) != 1 {
			t.Fatalf("%s: листов %d, ожидался 1", c.label, len(res.Sheets))
		}
		s := res.Sheets[0]
		if s.Length.Millimeters() != c.expectL || s.Width.Millimeters() != c.expectW {
			t.Errorf("%s: лист %.0f×%.0f, ожидалось %.0f×%.0f",
				c.label, s.Length.Millimeters(), s.Width.Millimeters(), c.expectL, c.expectW)
		}
		assertNoOverlap(t, s)
	}
}

// Металл: детали раскладываются в заготовки 3000×1500, а заготовки — по две на
// лист 6000×1500. Число листов = ⌈заготовок / 2⌉.
func TestNestSteelUsesBlanksThenSheets(t *testing.T) {
	gap := dommfg.CutGapMM(8)
	off := dommfg.CutEdgeOffsetMM(8)
	usableL := dommfg.BlankLengthMM - 2*off
	// Косоур-кусок 2644 мм: в ряд влезает один, рядов по ширине 300 мм столько,
	// сколько помещается в заготовку.
	res, err := Nest(rectCut(t, "STEEL-S235", 8, 2644, 300, 4), mustSheets(t))
	if err != nil {
		t.Fatal(err)
	}
	perRow := int((usableL + gap) / (2644 + gap))
	if perRow != 1 {
		t.Fatalf("в ряд влезает %d кусков, ожидался 1", perRow)
	}
	rows := int((dommfg.BlankWidthMM - 2*off + gap) / (300 + gap))
	perBlank := rows * perRow
	if perBlank < 4 {
		t.Fatalf("в заготовку влезает %d кусков, ожидалось не меньше 4", perBlank)
	}
	blanks := (4 + perBlank - 1) / perBlank
	wantSheets := dommfg.SheetsForBlanks(blanks)
	if len(res.Sheets) != wantSheets {
		t.Fatalf("листов %d, ожидалось %d (заготовок %d)", len(res.Sheets), wantSheets, blanks)
	}
	for _, s := range res.Sheets {
		if s.Length.Millimeters() != dommfg.StockSheetLengthMM ||
			s.Width.Millimeters() != dommfg.StockSheetWidthMM {
			t.Errorf("лист %.0f×%.0f, ожидалось %.0f×%.0f",
				s.Length.Millimeters(), s.Width.Millimeters(),
				dommfg.StockSheetLengthMM, dommfg.StockSheetWidthMM)
		}
		assertNoOverlap(t, s)
		// Деталь не должна вылезать за лист.
		for _, p := range s.Placed {
			if p.X+p.Length.Millimeters() > s.Length.Millimeters()+1e-6 ||
				p.Y+p.Width.Millimeters() > s.Width.Millimeters()+1e-6 {
				t.Errorf("деталь %q выходит за лист: %.0f×%.0f в (%.0f, %.0f)",
					p.PartNumber, p.Length.Millimeters(), p.Width.Millimeters(), p.X, p.Y)
			}
		}
	}
}

func TestNestAreaMetrics(t *testing.T) {
	cut := rectCut(t, "STEEL-S235", 40, 800, 270, 12)
	res, err := Nest(cut, mustSheets(t))
	if err != nil {
		t.Fatal(err)
	}
	// Лист металла 6000×1500 (сводная шпатка технолога 2026-10-05): 12 деталей
	// 800×270 помещаются в две заготовки одного листа без остатка.
	wantPart := float64(12 * 800 * 270)
	wantSheet := dommfg.StockSheetLengthMM * dommfg.StockSheetWidthMM
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
