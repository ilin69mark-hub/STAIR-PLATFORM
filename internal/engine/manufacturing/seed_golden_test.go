package manufacturing

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// seedGoldenConfig — 6-ступенчатый сид для золотого гейта S-204:
// те же номиналы, что seedConfig() в пакете geometry
// (900×1080, 6×180×270, косоур 8, проступь 40 с филёнкой R10,
// подступенки, перила 900 слева, STEEL-S235 + WOOD-OAK).
func seedGoldenConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 1080), engineering.FlightStraight)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 6
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 8)
	cfg.StepThickness = mustLength(t, 40)
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "WOOD-OAK"
	cfg.TreadNoseRadiusMM = mustLength(t, 10)
	cfg.Riser = true
	cfg.Railing = engineering.RailingLeft
	cfg.RailingHeight = mustLength(t, 900)
	return cfg
}

// TestSeedPackageGolden — сид 6 ступеней собирается из библиотеки
// побайтово как сегодня (S-204): снимок пакета (детали + BOM) сверяется
// с золотым значением, плюс точный объём/габариты модели.
//
// Зачем золото, если рефакторинг сохраняет поведение «по построению»:
// золото ловит будущий дрейф (а сегодняшнее сохранение доказывают все
// остальные тесты сида: счета, габариты, объёмы, BOM). Значения сняты
// с неизменённого конвейера и сверены с известными числами сида
// (42 тела, объём 108961605, bbox из seed_test) — не выдуманы.
func TestSeedPackageGolden(t *testing.T) {
	cfg := seedGoldenConfig(t)
	gen := genResult(t, cfg)
	pkg, err := Manufacture(cfg, gen, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Объём — допуск 1,0 мм³ (1e-8): тела строятся параллельно (Scheduler),
	// и порядок суммирования объёмов гуляет от прогона к прогону — младшие
	// разряды float64 шумят. Точное == здесь было бы враньём про точность:
	// любое настоящее изменение геометрии сдвигает объём на сотни и тысячи.
	// Константа — с полной точностью из прогона (%.1f в логах прячет
	// даже десятые, сравнивать с округлённым нельзя).
	if got := gen.Measurement.Volume; math.Abs(got-seedGoldenVolume) > 1.0 {
		t.Errorf("seed volume = %.6f, want %.6f", got, seedGoldenVolume)
	}
	if got := len(gen.Mesh.Vertices); got != seedGoldenVerts {
		t.Errorf("seed mesh vertices = %d, want %d", got, seedGoldenVerts)
	}
	if got := len(gen.Mesh.Triangles); got != seedGoldenTris {
		t.Errorf("seed mesh triangles = %d, want %d", got, seedGoldenTris)
	}
	var sb strings.Builder
	for _, p := range pkg.Parts {
		fmt.Fprintf(&sb, "%s|%s|%s|%.1f|%.1fx%.1f|%s|%t\n",
			p.Number, p.Kind, p.Material,
			p.Thickness.Millimeters(), p.Length.Millimeters(), p.Width.Millimeters(),
			p.Where, p.Purchased)
	}
	sb.WriteString("---\n")
	for _, l := range pkg.BOM.Lines {
		fmt.Fprintf(&sb, "%d|%s|%s|%s|%d|%.1fx%.1f\n",
			l.Number, l.PartNumber, l.Description, l.MaterialCode,
			l.Quantity, l.Length.Millimeters(), l.Width.Millimeters())
	}
	if got := sb.String(); got != seedGoldenPackage {
		t.Logf("FULL ACTUAL:\n%s", got)
		t.Errorf("seed package GOLDEN MISMATCH:\n%s", diffGolden(got, seedGoldenPackage))
	}
}

// diffGolden — первые различающиеся строки (чтобы падение читалось).
func diffGolden(got, want string) string {
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	var sb strings.Builder
	n := 0
	for i := 0; i < len(gl) || i < len(wl); i++ {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			fmt.Fprintf(&sb, "строка %d:\n  got:  %s\n  want: %s\n", i+1, g, w)
			if n++; n >= 5 {
				break
			}
		}
	}
	return sb.String()
}

const (
	seedGoldenVolume = 108961604.880078
	seedGoldenVerts  = 36836
	seedGoldenTris   = 14152
)

const seedGoldenPackage = `STR-01|stringer|STEEL-S235|8.0|1924.8x300.0|косоур А|false
STR-02|stringer|STEEL-S235|8.0|1924.8x300.0|косоур Б|false
TRD-01|tread|WOOD-OAK|40.0|900.0x290.0|ступень 1|false
TRD-02|tread|WOOD-OAK|40.0|900.0x290.0|ступень 2|false
TRD-03|tread|WOOD-OAK|40.0|900.0x290.0|ступень 3|false
TRD-04|tread|WOOD-OAK|40.0|900.0x290.0|ступень 4|false
TRD-05|tread|WOOD-OAK|40.0|900.0x290.0|ступень 5|false
TRD-06|tread|WOOD-OAK|40.0|900.0x330.0|ступень 6|false
RSR-01|riser|WOOD-OAK|40.0|900.0x140.0|ступень 1|false
RSR-02|riser|WOOD-OAK|40.0|900.0x180.0|ступень 2|false
RSR-03|riser|WOOD-OAK|40.0|900.0x180.0|ступень 3|false
RSR-04|riser|WOOD-OAK|40.0|900.0x180.0|ступень 4|false
RSR-05|riser|WOOD-OAK|40.0|900.0x180.0|ступень 5|false
RSR-06|riser|WOOD-OAK|40.0|900.0x180.0|ступень 6|false
HLT-01|heel|STEEL-S235|4.0|260.0x50.0|ступень 1, сторона А|false
HLT-02|heel|STEEL-S235|4.0|260.0x50.0|ступень 2, сторона А|false
HLT-03|heel|STEEL-S235|4.0|260.0x50.0|ступень 3, сторона А|false
HLT-04|heel|STEEL-S235|4.0|260.0x50.0|ступень 4, сторона А|false
HLT-05|heel|STEEL-S235|4.0|260.0x50.0|ступень 5, сторона А|false
HLT-06|heel|STEEL-S235|4.0|260.0x50.0|ступень 6, сторона А|false
WEB-01|web|STEEL-S235|4.0|166.0x50.0|ступень 2|false
WEB-02|web|STEEL-S235|4.0|166.0x50.0|ступень 3|false
WEB-03|web|STEEL-S235|4.0|166.0x50.0|ступень 4|false
WEB-04|web|STEEL-S235|4.0|166.0x50.0|ступень 5|false
WEB-05|web|STEEL-S235|4.0|166.0x50.0|ступень 6|false
ANC-01|anchor|STEEL-S235|4.0|270.0x50.0|пол, сторона А|false
ANC-02|anchor|STEEL-S235|4.0|166.0x50.0|перекрытие, сторона А|false
FPW-01|first_web|STEEL-S235|4.0|122.0x40.0|ступень 1, сторона А|false
HLT-07|heel|STEEL-S235|4.0|260.0x50.0|ступень 1, сторона Б|false
HLT-08|heel|STEEL-S235|4.0|260.0x50.0|ступень 2, сторона Б|false
HLT-09|heel|STEEL-S235|4.0|260.0x50.0|ступень 3, сторона Б|false
HLT-10|heel|STEEL-S235|4.0|260.0x50.0|ступень 4, сторона Б|false
HLT-11|heel|STEEL-S235|4.0|260.0x50.0|ступень 5, сторона Б|false
HLT-12|heel|STEEL-S235|4.0|260.0x50.0|ступень 6, сторона Б|false
WEB-06|web|STEEL-S235|4.0|166.0x50.0|ступень 2|false
WEB-07|web|STEEL-S235|4.0|166.0x50.0|ступень 3|false
WEB-08|web|STEEL-S235|4.0|166.0x50.0|ступень 4|false
WEB-09|web|STEEL-S235|4.0|166.0x50.0|ступень 5|false
WEB-10|web|STEEL-S235|4.0|166.0x50.0|ступень 6|false
ANC-03|anchor|STEEL-S235|4.0|270.0x50.0|пол, сторона Б|false
ANC-04|anchor|STEEL-S235|4.0|166.0x50.0|перекрытие, сторона Б|false
FPW-02|first_web|STEEL-S235|4.0|122.0x40.0|ступень 1, сторона Б|false
RAL-43|rail_top|STAINLESS-304|1.5|1947.0x42.4|поручень, сторона А|true
PST-44|post|STAINLESS-304|1.5|805.7x42.4|стойка 1, сторона А|true
PST-45|post|STAINLESS-304|1.5|805.7x42.4|стойка 2, сторона А|true
RGL-46|rigel|STAINLESS-304|1.0|1622.5x12.0|ригель, ряд 1, сторона А|true
RGL-47|rigel|STAINLESS-304|1.0|1622.5x12.0|ригель, ряд 2, сторона А|true
TIP-48|tip|STAINLESS-304|1.0|70.0x50.8|наконечник, стойка 1, сторона А|true
FLG-49|flange|STAINLESS-304|8.0|86.0x86.0|фланец, стойка 1, сторона А|true
SLP-50|slip|STAINLESS-304|805.7|43.0x43.0|муфта, стойка 1, сторона А|true
DEC-51|deco|STAINLESS-304|17.0|88.0x88.0|накладка, стойка 1, сторона А|true
TIP-52|tip|STAINLESS-304|1.0|70.0x50.8|наконечник, стойка 2, сторона А|true
FLG-53|flange|STAINLESS-304|8.0|86.0x86.0|фланец, стойка 2, сторона А|true
SLP-54|slip|STAINLESS-304|805.7|43.0x43.0|муфта, стойка 2, сторона А|true
DEC-55|deco|STAINLESS-304|17.0|88.0x88.0|накладка, стойка 2, сторона А|true
ENC-56|end_cap|PLASTIC|1.0|39.2x39.2|торец поручня (начало), сторона А|true
ENC-57|end_cap|PLASTIC|1.0|39.2x39.2|торец поручня (конец), сторона А|true
---
1|STR-01|Stringer|STEEL-S235|2|1924.8x300.0
2|TRD-01|Tread|WOOD-OAK|5|900.0x290.0
3|TRD-06|Tread|WOOD-OAK|1|900.0x330.0
4|RSR-01|Riser|WOOD-OAK|1|900.0x140.0
5|RSR-02|Riser|WOOD-OAK|5|900.0x180.0
6|HLT-01|Heel (seat bracket)|STEEL-S235|12|260.0x50.0
7|WEB-01|Web (tooth bracket)|STEEL-S235|10|166.0x50.0
8|ANC-01|Anchor (floor/landing bracket)|STEEL-S235|2|270.0x50.0
9|ANC-02|Anchor (floor/landing bracket)|STEEL-S235|2|166.0x50.0
10|FPW-01|First-step plate|STEEL-S235|2|122.0x40.0
11|RAL-43|Перила (поручень)|STAINLESS-304|1|1947.0x42.4
12|PST-44|Стойка опоры|STAINLESS-304|2|805.7x42.4
13|RGL-46|Ригель|STAINLESS-304|2|1622.5x12.0
14|TIP-48|Наконечник регулируемый|STAINLESS-304|2|70.0x50.8
15|FLG-49|Фланец опоры|STAINLESS-304|2|86.0x86.0
16|SLP-50|Накидная часть|STAINLESS-304|2|43.0x43.0
17|DEC-51|Декоративная накладка|STAINLESS-304|2|88.0x88.0
18|ENC-56|Торцевая заглушка|PLASTIC|2|39.2x39.2
`
