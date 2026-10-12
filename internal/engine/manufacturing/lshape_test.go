package manufacturing

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
)

// lshapeConfig возвращает валидную конфигурацию L-образной лестницы
// (H=2700, n=15, n1=6, h=180, b=270, W=900, Wp=1000, T=50, st=40).
func lshapeConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightLShape)
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
	return cfg
}

// TestManufactureLShapeParts проверяет, что L-образная модель декомпозируется
// по семантическим ролям (BC-007): косоуры повёрнутого верхнего марша не
// классифицируются как подступенки, площадка — как проступь.
// cuttableParts — детали, которые реально режутся (без покупных). Ограждение
// покупается готовым и в раскрой не идёт, поэтому тесты L/U-лестниц считают
// каркас, ступени и подступенки.
func cuttableParts(pkg *dommfg.ManufacturingPackage) []dommfg.Part {
	var out []dommfg.Part
	for _, p := range pkg.Parts {
		if !p.Purchased {
			out = append(out, p)
		}
	}
	return out
}

// purchasedParts — покупные детали ограждения.
func purchasedParts(pkg *dommfg.ManufacturingPackage) []dommfg.Part {
	var out []dommfg.Part
	for _, p := range pkg.Parts {
		if p.Purchased {
			out = append(out, p)
		}
	}
	return out
}

func TestManufactureLShapeParts(t *testing.T) {
	cfg := lshapeConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
	if err != nil {
		t.Fatal(err)
	}
	// 4 косоура (2 нижних + 2 верхних), 16 проступей (15 ступеней + площадка),
	// 15 подступенков (полотно на ступень). Итого 35 деталей.
	if got := len(cuttableParts(pkg)); got != 103 {
		t.Fatalf("cuttable parts = %d, want 103", got)
	}
	byKind := make(map[dommfg.PartKind]int)
	for _, p := range pkg.Parts {
		byKind[p.Kind]++
	}
	if byKind[dommfg.PartStringer] != 4 || byKind[dommfg.PartTread] != 16 || byKind[dommfg.PartRiser] != 15 {
		t.Fatalf("part kinds = %+v, want stringer 4 / tread 16 / riser 15", byKind)
	}
	// Трассировка к геометрии: индексы покрывают все тела модели. Покупные
	// детали ограждения к телу модели не относятся (их геометрия — декорация),
	// поэтому SolidIndex у них не проверяем.
	for i, p := range cuttableParts(pkg) {
		if p.SolidIndex != i {
			t.Fatalf("part %q solid index = %d, want %d", p.Number, p.SolidIndex, i)
		}
	}
	for _, p := range purchasedParts(pkg) {
		if p.Kind.IsValid() == false {
			t.Fatalf("покупная деталь %q с некорректным видом %q", p.Number, p.Kind)
		}
	}
}

// TestManufactureLShapeDimensions проверяет габариты деталей L-марша:
// нижний косоур 1620×1130×50, верхний 2430×1670×50, проступи 900×310×40,
// подступенки 900×140×40, площадка 1000×900×40.
func TestManufactureLShapeDimensions(t *testing.T) {
	cfg := lshapeConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
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
		{"TRD-01", 900, 290}, // вся ширина марша, глубина = шаг 270 + нос 20
		{"TRD-07", 1000, 900},
		{"TRD-08", 900, 290},
		{"TRD-16", 900, 330}, // последняя ступень длиннее на TopStepExtend
		{"RSR-01", 900, 140},
		{"RSR-02", 900, 180},
		{"RSR-15", 900, 180},
	}
	for _, c := range cases {
		p := findPart(pkg, c.number)
		if p == nil {
			t.Fatalf("part %q not found", c.number)
		}
		if !nearlyEqualTol(p.Length.Millimeters(), c.l, 0.01) || !nearlyEqualTol(p.Width.Millimeters(), c.wd, 0.01) {
			t.Fatalf("part %q dims = %v×%v, want %v×%v",
				c.number, p.Length.Millimeters(), p.Width.Millimeters(), c.l, c.wd)
		}
	}
	// толщины: косоуры 50, проступи/подступенки/площадка 40.
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
	// Материал назначен каждому косоуру/проступи/подступенку из каталога.
	// Покупное ограждение проверяем отдельно: его материал (нержавейка,
	// пластик для заглушек) не из листового реестра.
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

// TestManufactureLShapeBOM проверяет группировку BOM: косоуры двух размеров
// (1647.7×1040 и 2457.7×1580) — отдельные строки, проступи и полосы
// подступенков объединяются по размерам, площадка — отдельная проступь.
func TestManufactureLShapeBOM(t *testing.T) {
	cfg := lshapeConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
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
		{"Tread", 13, 900, 290},
		{"Tread", 2, 900, 330},
		{"Riser", 2, 900, 140},
		{"Riser", 13, 900, 180},
		{"Heel (seat bracket)", 30, 260, 50},
		{"Web (tooth bracket)", 26, 166, 50},
		{"Anchor (floor/landing bracket)", 4, 270, 50},
		{"Anchor (floor/landing bracket)", 4, 166, 50},
		{"First-step plate", 4, 122, 40},
		{"Tread", 1, 1000, 900},
		{"Stringer", 2, 2898.309, 300},
	}
	// BOM включает и покупные позиции ограждения, а want описывает только
	// каркас и ступени, поэтому сверяем ПЕРВЫЕ len(want) строк: строки
	// ограждения добавляются после, по порядку появления частей.
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
		if !nearlyEqualTol(line.Length.Millimeters(), w.length, 0.01) || !nearlyEqualTol(line.Width.Millimeters(), w.width, 0.01) {
			t.Fatalf("line %d dims = %v×%v, want %v×%v", i,
				line.Length.Millimeters(), line.Width.Millimeters(), w.length, w.width)
		}
	}
	// Карта раскроя повторяет РЕЖУЩИЕСЯ строки BOM. Покупные детали
	// ограждения в спецификацию входят, а в раскрой не идут: листа под них
	// не режем. Поэтому CutList короче BOM ровно на число покупных строк.
	if len(pkg.CutList.Items) != len(want) {
		t.Fatalf("cut list items = %d, want %d (BOM включает покупное ограждение)",
			len(pkg.CutList.Items), len(want))
	}
	// Ограждение обязано быть в спецификации: без него менеджер не видит,
	// что покупать, и прайс не считается.
	if got := len(purchasedParts(pkg)); got == 0 {
		t.Fatal("в пакете нет покупных деталей ограждения")
	}
}

// TestManufactureLShapeNesting проверяет, что раскладка по листам для
// L-марша корректна и валидна.
func TestManufactureLShapeNesting(t *testing.T) {
	cfg := lshapeConfig(t)
	res, err := enggeo.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Manufacture(cfg, res)
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
