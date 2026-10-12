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

// nearlyEqualTol — сравнение с допуском: размеры заготовки пластины
// иррациональны (длина по склону через корень), и сверять их с точностью
// 1e-6 бессмысленно.
func nearlyEqualTol(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
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
	// Материал задаём ЯВНО. Раньше он выводился из толщины: 50 мм подходили
	// стали (диапазон 2–60), и тесты молча проверяли стальной марш на
	// конфигурации без материала. С выходом стали на выпуск 3–8 мм такой
	// вывод стал давать дуб, и тесты проверяли бы не то.
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "STEEL-S235"
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
	// Металлокаркас: 4 куска косоур��в-гребёнки (2 косоура × 2 сварных куска,
	// длиннее поля лазерного стола 3000 мм), 15 ступеней, 15 подступенков и
	// 64 пятки выреза (30 + 28 + 2 + 2 + 2).
	if got := len(cuttableParts(pkg)); got != 98 {
		t.Fatalf("cuttable parts = %d, want 98", got)
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
	// 4 куска косоура вместо 2: каждый из двух косоур��в длиннее поля
	// лазерного стола 3000 мм и режется пополам со сварным стыком.
	if byKind[dommfg.PartStringer] != 4 || byKind[dommfg.PartTread] != 15 || byKind[dommfg.PartRiser] != 15 {
		t.Fatalf("part kinds = %+v, want stringer 4 (2 косоура × 2 куска) / tread 15 / riser 15", byKind)
	}
	// трассировка к геометрии: индексы неубывают и покрывают все тела.
	// Строгого равенства индексу детали больше нет: из одного тела косоура
	// выходит несколько деталей, и у них индекс тела общий.
	prev := -1
	for _, p := range cuttableParts(pkg) {
		if p.SolidIndex < prev {
			t.Fatalf("part %q solid index = %d меньше предыдущего %d: трассировка нарушена",
				p.Number, p.SolidIndex, prev)
		}
		prev = p.SolidIndex
	}
}

func TestManufacturePartDimensions(t *testing.T) {
	pkg, err := Manufacture(testConfig(t), genResult(t, testConfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	// Косоур-гребёнка — заготовка это ПОЛОСА: длина марша × ШАГ СТУПЕНИ,
	// а не габаритный блок «длина × высота марша». Раньше здесь стояло
	// 4050×2660, и это была ровно та ошибка, из-за которой стальной каркас
	// стоил 8,5 тонны на марш: зубья пилы уходят в обрез, а не в заготовку.
	// Толщина 8 — это лист лазерного раскроя: материал детали выводится из
	// толщины, а деталь без заданного материала считается сталью. 50 мм
	// (ширина секции косоура) в расход материала не идёт.
	// Длина пластины — ПО СКЛОНУ (4620.65 мм), а ширина — глубина
	// пластины по нормали (300 мм). Раньше здесь стоял горизонтальный
	// Косоур длиннее поля лазерного стола 3000 мм, поэтому режется на 2
	// равных куска: (4845,31 + 12 мм припуска на шип) / 2 = 2428,65. Куски
	// равные — короткий хвост съел бы целый ряд заготовки.
	if p := findPart(pkg, "STR-01"); p != nil {
		if !nearlyEqualTol(p.Length.Millimeters(), 2428.653107013688, 0.01) || !nearlyEqual(p.Width.Millimeters(), 300) ||
			!nearlyEqual(p.Thickness.Millimeters(), 12) {
			t.Fatalf("stringer dims = %v×%v×%v, want 2428.65×300×12 (2 куска, гребёнка со спиной 300)",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
		if p.JointCount != 2 || p.JointIndex != 0 || !p.HasMortise || p.HasTongue {
			t.Fatalf("STR-01 joint = %d/%d tongue=%v mortise=%v, want 0/2 mortise без шипа",
				p.JointIndex, p.JointCount, p.HasTongue, p.HasMortise)
		}
	} else {
		t.Fatal("STR-01 not found")
	}
	// Второй кусок несёт и шип, и паз: он стыкуется с обоими соседями.
	if p := findPart(pkg, "STR-02"); p != nil {
		if p.JointIndex != 1 || !p.HasMortise || !p.HasTongue {
			t.Fatalf("STR-02 joint = %d/%d tongue=%v mortise=%v, want 1/2 оба признака",
				p.JointIndex, p.JointCount, p.HasTongue, p.HasMortise)
		}
	}
	// Проступь: во всю ширину марша 900, глубина = шаг + нос (270 + 20 = 290).
	// Толщина 12 — лист лазерного раскроя, как и у косоура.
	if p := findPart(pkg, "TRD-01"); p != nil {
		if !nearlyEqual(p.Length.Millimeters(), 900) || !nearlyEqual(p.Width.Millimeters(), 290) ||
			!nearlyEqual(p.Thickness.Millimeters(), 12) {
			t.Fatalf("tread dims = %v×%v×%v, want 900×290×12",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
	} else {
		t.Fatal("TRD-01 not found")
	}
	// Подступенок: полотно во всю ширину марша (тонок по X). У ПЕРВОЙ
	// ступени он стоит на полу, поэтому ниже посадки на подъём: h − st = 140.
	if p := findPart(pkg, "RSR-01"); p != nil {
		if !nearlyEqual(p.Length.Millimeters(), 900) || !nearlyEqual(p.Width.Millimeters(), 140) ||
			!nearlyEqual(p.Thickness.Millimeters(), 12) {
			t.Fatalf("riser dims = %v×%v×%v, want 900×140×12",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
	} else {
		t.Fatal("RSR-01 not found")
	}
	// Второй подступенок и дальше — на ВЕСЬ подъём ступени (h = 180).
	if p := findPart(pkg, "RSR-02"); p != nil {
		if !nearlyEqual(p.Length.Millimeters(), 900) || !nearlyEqual(p.Width.Millimeters(), 180) ||
			!nearlyEqual(p.Thickness.Millimeters(), 12) {
			t.Fatalf("riser 2 dims = %v×%v×%v, want 900×180×12",
				p.Length.Millimeters(), p.Width.Millimeters(), p.Thickness.Millimeters())
		}
	} else {
		t.Fatal("RSR-02 not found")
	}
	// материал назначен каждому косоуру/проступи/подступенку из каталога.
	for _, p := range pkg.Parts {
		if _, ok := mustMaterials(t).Find(p.Material); !ok {
			if p.Purchased {
				continue // покупное ограждение: свой материал, не из листа
			}
			t.Fatalf("part %q material %q not in registry", p.Number, p.Material)
		}
	}
}

func TestManufactureBOM(t *testing.T) {
	pkg, err := Manufacture(testConfig(t), genResult(t, testConfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	// BOM включает 10 строк каркаса/ступеней плюс 8 покупных позиций
	// ограждения. Режется только каркас — строки ограждения в раскрой не идут.
	if len(pkg.BOM.Lines) != 18 {
		t.Fatalf("BOM lines = %d, want 18 (10 каркаса + 8 ограждения)",
			len(pkg.BOM.Lines))
	}
	if len(pkg.CutList.Items) != 10 {
		t.Fatalf("cut list items = %d, want 10 (покупное ограждение не раскраивается)",
			len(pkg.CutList.Items))
	}
	// порядок строк детерминирован: косоуры, проступи, подступенки
	// (единое полотно 900×140).
	want := []struct {
		desc     string
		quantity int
		length   float64
		width    float64
	}{
		{"Stringer", 4, 2428.653107013688, 300}, // 2 косоура × 2 куска: поле лазерного стола 3000 мм
		{"Tread", 14, 900, 290},
		{"Tread", 1, 900, 330},  // последняя ступень длиннее на TopStepExtend
		{"Riser", 1, 900, 140},  // первая: от пола до посадки
		{"Riser", 14, 900, 180}, // остальные: на весь подъём
		{"Heel (seat bracket)", 30, 260, 50},
		{"Web (tooth bracket)", 28, 166, 50},
		{"Anchor (floor/landing bracket)", 2, 270, 50},
		{"Anchor (floor/landing bracket)", 2, 166, 50},
		{"First-step plate", 2, 122, 40},
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
	if len(pkg.CutList.Items) != 10 {
		t.Fatalf("cut list items = %d, want 10 — карта раскроя повторяет только "+
			"режущиеся строки BOM; покупное ограждение в неё не входит",
			len(pkg.CutList.Items))
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
	// Без ступеней остаются косоуры и покупное ограждение. Ограждение не
	// зависит от числа ступеней (только от длины марша), поэтому при нулевом
	// StepCount оно тоже пустое.
	if got := len(cuttableParts(pkg)); got != 2 {
		t.Fatalf("cuttable parts = %d, want 2 (stringers only)", got)
	}
	// Первая строка BOM — косоур,数量 2. Дальше идут покупные позиции
	// ограждения: они в спецификацию входят, но в раскрой не идут.
	if len(pkg.BOM.Lines) < 1 || pkg.BOM.Lines[0].Description != "Stringer" ||
		int(pkg.BOM.Lines[0].Quantity) != 2 {
		t.Fatalf("BOM[0] = %+v, want Stringer x2", pkg.BOM.Lines[0])
	}
	for _, line := range pkg.BOM.Lines[1:] {
		if line.MaterialCode != "STAINLESS-304" && line.MaterialCode != "PLASTIC" {
			t.Fatalf("строка BOM %q с неожиданным материалом %q — каркаса быть не должно",
				line.Description, line.MaterialCode)
		}
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
	if int(pkg.Nesting.PartCount) != len(cuttableParts(pkg)) {
		t.Fatalf("nesting parts = %d, want %d", pkg.Nesting.PartCount, len(pkg.Parts))
	}
	// n=15, всё из стали: заготовки — куски косоура 2428,65×300, проступи
	// 804×310 и подступенки 804×140, лист 8 мм.
	//
	// Раскрой идёт по полю лазерного стола 3000×1500, детали ложатся в
	// заготовки с отступом 10 и зазором 10 (шпатка технолога 2026-10-05), а
	// заготовки — по две на купленный лист 6000×1500. Трёх листов хватает на
	// 8 мм (косоуры, проступи, подступенки), 5 мм и 4 мм.
	//
	// До починки косоур брался габаритным блоком 4050×2660 и детали были по
	// 50–40 мм: пять листов, 10,4 тонны и цена в десять раз выше реальной.
	if len(pkg.Nesting.Sheets) != 3 {
		t.Fatalf("sheets = %d, want 3", len(pkg.Nesting.Sheets))
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

// Длинная боковая пластина не влезает в поле лазерного стола 3000×1500,
// поэтому марш режется на несколько кусков со сварным стыком «шип-паз».
// Проверяем, что кусков ровно столько, сколько нужно, и что каждый в пределе.
//
// Предел берётся из поля стола минус зазор листа (8 мм → зазор 10 мм), а не из
// MaxHeightMm материала: MaxHeightMm = 6000 ограничивает ИЗДЕЛИЕ по высоте
// подъёма, а не деталь раскроя.
func TestPlateStringerSplitsLongFlights(t *testing.T) {
	cfg := testConfig(t)
	cfg.StepCount = 34
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 280)
	cfg.StringerThickness = mustLength(t, 8)
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "STEEL-S235"
	limit := dommfg.MaxPartLengthMM(8)
	if limit <= 0 {
		t.Fatal("предел длины детали не найден для поля стола")
	}
	pkg, err := Manufacture(cfg, genResult(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	var plates []dommfg.Part
	for _, p := range pkg.Parts {
		if p.Kind == dommfg.PartStringer {
			plates = append(plates, p)
		}
	}
	if len(plates) < 4 {
		t.Fatalf("косоуров %d, марш высотой %v должен резаться на несколько деталей", len(plates), cfg.Height)
	}
	total := 0.0
	for _, p := range plates {
		l := p.Length.Millimeters()
		if l > limit {
			t.Fatalf("деталь %q длиной %v превышает поле стола %v", p.Number, l, limit)
		}
		total += l
	}
	// Сумма кусков равна длине пластины: стык ничего не теряет.
	if total <= limit {
		t.Fatalf("сумма кусков %v не покрывает длину пластины", total)
	}
}
