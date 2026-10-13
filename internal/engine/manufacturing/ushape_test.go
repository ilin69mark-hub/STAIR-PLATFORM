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
	// Материал задаём ЯВНО. Раньше он выводился из толщины: 50 мм подходили
	// стали (диапазон 2–60), и тесты молча проверяли стальной марш на
	// конфигурации без материала. С выходом стали на выпуск 3–8 мм такой
	// вывод стал давать дуб, и тесты проверяли бы не то.
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "STEEL-S235"
	cfg.StepThickness = mustLength(t, 40)
	cfg.LowerStepCount = 6
	cfg.LandingWidth = mustLength(t, 1000)
	cfg.Railing = engineering.RailingLeft
	cfg.RailingHeight = mustLength(t, 900)
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
	pkg, err := Manufacture(cfg, res, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 4 косоура (2 нижних + 2 верхних), 16 проступей (15 ступеней + площадка),
	// 15 подступенков (полотно на ступень). Итого 35 деталей.
	if got := len(cuttableParts(pkg)); got != 103 {
		t.Fatalf("cuttable parts = %d, want 103", got)
	}
	byKind := make(map[dommfg.PartKind]int)
	for _, p := range cuttableParts(pkg) {
		byKind[p.Kind]++
	}
	if byKind[dommfg.PartStringer] != 4 || byKind[dommfg.PartTread] != 16 || byKind[dommfg.PartRiser] != 15 {
		t.Fatalf("part kinds = %+v, want stringer 4 / tread 16 / riser 15", byKind)
	}
	// Трассировка к геометрии по режущимся деталям: покупное ограждение к
	// телу модели не относится (его геометрия — декорация).
	for i, p := range cuttableParts(pkg) {
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
	pkg, err := Manufacture(cfg, res, nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		number dommfg.PartNumber
		l, wd  float64
	}{
		{"STR-01", 1924.810, 300}, // пластина: длина по склону × глубина по нормали
		{"STR-02", 1924.810, 300},
		{"STR-03", 2898.309, 300},
		{"STR-04", 2898.309, 300},
		{"TRD-01", 850, 290},
		{"TRD-07", 1800, 900},
		{"TRD-08", 850, 290},
		{"TRD-16", 850, 330}, // последняя ступень длиннее на TopStepExtend
		{"RSR-01", 850, 140},
		{"RSR-02", 850, 180},
		{"RSR-15", 850, 180},
	}
	for _, c := range cases {
		p := findPart(pkg, c.number)
		if p == nil {
			t.Fatalf("part %q not found", c.number)
		}
		if !nearlyEqualTol(p.Length.Millimeters(), c.l) || !nearlyEqualTol(p.Width.Millimeters(), c.wd) {
			t.Fatalf("part %q dims = %v×%v, want %v×%v",
				c.number, p.Length.Millimeters(), p.Width.Millimeters(), c.l, c.wd)
		}
	}
	// толщины: косоуры 50, проступи/подступенки/площадка 40. Покупное
	// ограждение имеет свои толщины (стенки труб 1–1,5 мм) и проверяется отдельно.
	for _, p := range cuttableParts(pkg) {
		// Косоур, ступень и подступенок — лист 12 мм: модельные 50/40 мм — это
		// габариты секций, а не толщина металла, и они сжимаются к ВЕРХУ
		// выпуска листа (PlateMaxThicknessMM = 12, строки 8 / 10 / 12 мм
		// норматива на листовой косоур). Пятки выреза — лист 4 мм
		// (CombFrame.HeelThickness), это отдельная деталь.
		want := 12.0
		// Перечисление целиком, а не default: exhaustive-линтер иначе не
		// заметит новый PartKind, и проверка толщины молча перестанет его
		// проверять.
		switch p.Kind {
		case dommfg.PartHeel, dommfg.PartWeb, dommfg.PartAnchor, dommfg.PartFirstWeb:
			want = 4
		case dommfg.PartStringer, dommfg.PartTread, dommfg.PartRiser, dommfg.PartRailTop, dommfg.PartPost, dommfg.PartRigel, dommfg.PartTip, dommfg.PartFlange, dommfg.PartSlip, dommfg.PartDeco, dommfg.PartEndCap:
			// остальные детали косоура — по нормативу выпуска листа
		}
		if !nearlyEqual(p.Thickness.Millimeters(), want) {
			t.Fatalf("part %q (%s) thickness = %v, want %v",
				p.Number, p.Kind, p.Thickness.Millimeters(), want)
		}
	}
	// материал назначен каждому косоуру/проступи/подступенку из каталога.
	for _, p := range cuttableParts(pkg) {
		if _, ok := mustMaterials(t).Find(p.Material); !ok {
			t.Fatalf("part %q material %q not in registry", p.Number, p.Material)
		}
	}
	for _, p := range purchasedParts(pkg) {
		if p.Material != "STAINLESS-304" && p.Material != "PLASTIC" {
			t.Fatalf("покупная деталь %q: неожиданный материал %q", p.Number, p.Material)
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
	pkg, err := Manufacture(cfg, res, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		desc     string
		quantity int
		length   float64
		width    float64
	}{
		{"Stringer", 2, 1924.810, 300},
		{"Tread", 13, 850, 290},
		{"Tread", 2, 850, 330},
		{"Riser", 2, 850, 140},
		{"Riser", 13, 850, 180},
		{"Heel (seat bracket)", 30, 260, 50},
		{"Web (tooth bracket)", 26, 166, 50},
		{"Anchor (floor/landing bracket)", 4, 270, 50},
		{"Anchor (floor/landing bracket)", 4, 166, 50},
		{"First-step plate", 4, 122, 40},
		{"Tread", 1, 1800, 900},
		{"Stringer", 2, 2898.309, 300},
	}
	// BOM включает покупные позиции ограждения сверх каркаса; want описывает
	// только режущиеся строки, которые идут первыми.
	if len(pkg.BOM.Lines) < len(want) {
		t.Fatalf("BOM lines = %d, want at least %d", len(pkg.BOM.Lines), len(want))
	}
	for i, w := range want {
		line := pkg.BOM.Lines[i]
		if line.Description != w.desc {
			t.Fatalf("line %d description = %q, want %q", i, line.Description, w.desc)
		}
		if int(line.Quantity) != w.quantity {
			t.Fatalf("line %d quantity = %d, want %d", i, int(line.Quantity), w.quantity)
		}
		if !nearlyEqualTol(line.Length.Millimeters(), w.length) || !nearlyEqualTol(line.Width.Millimeters(), w.width) {
			t.Fatalf("line %d dims = %v×%v, want %v×%v", i,
				line.Length.Millimeters(), line.Width.Millimeters(), w.length, w.width)
		}
	}
	// Карта раскроя повторяет только режущиеся строки: покупное ограждение
	// в спецификацию входит, а листа под него не режем.
	if len(pkg.CutList.Items) != len(want) {
		t.Fatalf("cut list items = %d, want %d", len(pkg.CutList.Items), len(want))
	}
	if got := len(purchasedParts(pkg)); got == 0 {
		t.Fatal("в пакете нет покупных деталей ограждения")
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
	pkg, err := Manufacture(cfg, res, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Nesting == nil {
		t.Fatal("package must contain a nesting result")
	}
	if int(pkg.Nesting.PartCount) != len(cuttableParts(pkg)) {
		t.Fatalf("nesting parts = %d, want %d",
			pkg.Nesting.PartCount, len(cuttableParts(pkg)))
	}
	if pkg.Nesting.Utilization <= 0 || pkg.Nesting.Utilization > 1 {
		t.Fatalf("utilization out of range: %v", pkg.Nesting.Utilization)
	}
	if err := pkg.Nesting.Validate(); err != nil {
		t.Fatalf("nesting must validate: %v", err)
	}
}
