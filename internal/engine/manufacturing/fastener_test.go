package manufacturing

import (
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
	kerngeo "stairplatform/internal/geometry"
)

// Тесты крепежа (MNT-0001, решение владельца 2026-10-08).
//
// Главное, что тут проверяется, — соответствие сметы ГЕОМЕТРИИ: одно
// отверстие в пятке — один комплект крепежа. Отверстия считает движок
// геометрии, и если расчёт берёт их откуда-то ещё, он разойдётся с
// раскроем.

// testCfg — стальной прямой марш 15 ступеней с подступенками: в нём есть
// все пять пяток обоймы выреза.
//
// Railing задан явно (левая сторона). Раньше он не был задан вовсе, и это не
// мешало: спецификация ограждения сторону не читала и всегда считала одну
// нитку. Теперь читает, а геометрия и всегда читала — пустая сторона означает
// «перил нет». Чтобы тестовая база осталась той же, сторона названа.
func testCfg() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width: 900, Height: 2700, Flight: engineering.FlightStraight,
		StepCount: 15, StepHeight: 180, TreadDepth: 270,
		StringerThickness: 8, StepThickness: 5, Riser: true,
		Clearance: 2000, RailingHeight: 900, ApproachSpace: 1000,
		Railing:  engineering.RailingLeft,
		Material: "STEEL-S235", TreadMaterial: "STEEL-S235",
		Comb: engineering.CombFrameDefaults(),
	}
}

// solid с ролью и числом отверстий — тело для счёта крепежа.
func holed(role string, n int) *kerngeo.Solid {
	return kerngeo.NewSolid().WithRole(role).WithHoles(n)
}

// demandOf собирает потребность в ключе «код|назначение».
//
// Ключ с назначением, а не один код: глухарь 6х35 теперь идёт и на пятки
// подступенка, и на анкерные отверстия фланцев ограждения. Суммировать их
// в одну цифру нельзя — тест «по одному глухарю на отверстие пятки» прошёл бы
// и при сломанном фланцевом крепеже, и наоборот.
func demandOf(t *testing.T, cfg *engineering.StairConfiguration, solids ...*kerngeo.Solid) map[string]int {
	t.Helper()
	got, err := FastenerDemandFor(cfg, solids, nil, nil)
	if err != nil {
		t.Fatalf("FastenerDemandFor: %v", err)
	}
	out := map[string]int{}
	for _, d := range got {
		out[d.Code+"|"+d.Purpose] += int(d.Quantity)
	}
	return out
}

// qty — количество по коду и назначению.
func qty(d map[string]int, code, purpose string) int { return d[code+"|"+purpose] }

// TestFastenerPerHoleOnTreadAndRiser — на ступени и подступенки ровно по
// одному глухарю 6х35 на каждое отверстие пяток обоймы выреза.
func TestFastenerPerHoleOnTreadAndRiser(t *testing.T) {
	// 30 пяток под ступенью по 4 отверстия + 28 пяток под подступенком по 3
	// + 2 пластины 1-й ступени по 3 = 120 + 84 + 6 = 210.
	got := demandOf(t, testCfg(),
		holed("heel", 4), holed("web", 3), holed("first_web", 3))
	if n := qty(got, dommfg.ScrewTreadRiserCode, dommfg.PurposeTreadRiser); n != 10 {
		t.Fatalf("глухарей на обойму выреза = %d, want 10 (4+3+3 на деталь)", n)
	}
	if n := qty(got, dommfg.PlugAnchorCode, dommfg.PurposeTreadRiser); n != 0 {
		t.Fatalf("дюбель на пятках подступенка и ступени = %d, want 0", n)
	}
}

// TestFastenerAnchorsConcrete — в бетон и глухарь, и дюбель, по одному на
// каждое отверстие пятки пола и пятки выхода на перекрытие.
func TestFastenerAnchorsConcrete(t *testing.T) {
	got := demandOf(t, testCfg(), holed("anchor", 4), holed("top_anchor", 3))
	anchors := qty(got, dommfg.ScrewAnchorCode, dommfg.PurposeFloor) +
		qty(got, dommfg.ScrewAnchorCode, dommfg.PurposeCeiling)
	if anchors != 7 {
		t.Fatalf("глухарей 10х60 = %d, want 7 (4 в пол + 3 в перекрытие)", anchors)
	}
	plugs := qty(got, dommfg.PlugAnchorCode, dommfg.PurposeFloor) +
		qty(got, dommfg.PlugAnchorCode, dommfg.PurposeCeiling)
	if plugs != 7 {
		t.Fatalf("дюбелей 10х60 = %d, want 7 в бетон", plugs)
	}
}

// TestFastenerAnchorsWood — в дерево дюбель не нужен: глухарь вворачивается
// прямо. Это и есть причина, по которой в админке два отдельных выбора.
func TestFastenerAnchorsWood(t *testing.T) {
	cfg := testCfg()
	cfg.FloorFixing = engineering.BaseWood
	cfg.CeilingFixing = engineering.BaseWood
	got := demandOf(t, cfg, holed("anchor", 4), holed("top_anchor", 3))
	anchors := qty(got, dommfg.ScrewAnchorCode, dommfg.PurposeFloor) +
		qty(got, dommfg.ScrewAnchorCode, dommfg.PurposeCeiling)
	if anchors != 7 {
		t.Fatalf("глухарей 10х60 = %d, want 7", anchors)
	}
	plugs := qty(got, dommfg.PlugAnchorCode, dommfg.PurposeFloor) +
		qty(got, dommfg.PlugAnchorCode, dommfg.PurposeCeiling)
	if plugs != 0 {
		t.Fatalf("дюбелей в дереве = %d, want 0", plugs)
	}
}

// TestFastenerAnchorsMixed — пол бетонный, перекрытие деревянное: дюбель
// только на полу. Проверка именно смешанного случая, потому что оба
// основания берутся из разных полей конфигурации.
func TestFastenerAnchorsMixed(t *testing.T) {
	cfg := testCfg()
	cfg.FloorFixing = engineering.BaseConcrete
	cfg.CeilingFixing = engineering.BaseWood
	got, err := FastenerDemandFor(cfg, []*kerngeo.Solid{holed("anchor", 4), holed("top_anchor", 3)}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var plugFloor, plugCeiling int
	for _, d := range got {
		if d.Code != dommfg.PlugAnchorCode {
			continue
		}
		switch d.Purpose {
		case dommfg.PurposeFloor:
			plugFloor = int(d.Quantity)
		case dommfg.PurposeCeiling:
			plugCeiling = int(d.Quantity)
		}
	}
	if plugFloor != 4 || plugCeiling != 0 {
		t.Fatalf("дюбели: пол=%d (want 4), перекрытие=%d (want 0)", plugFloor, plugCeiling)
	}
}

// TestFastenerExtraByObject — доп. крепёж по объекту добавляется СВЕРХ
// автоматического, а его количество парсится из «КОД×N».
func TestFastenerExtraByObject(t *testing.T) {
	cfg := testCfg()
	cfg.ExtraFasteners = []string{"PLUG-8x80", "SCREW-8x80@12"}
	got := demandOf(t, cfg, holed("anchor", 4))
	if n := qty(got, "PLUG-8x80", dommfg.PurposeObject); n != 1 {
		t.Fatalf("дюбель 8х80 = %d шт, want 1 (без множителя)", n)
	}
	if n := qty(got, "SCREW-8x80", dommfg.PurposeObject); n != 12 {
		t.Fatalf("глухарей 8х80 = %d шт, want 12 (из «SCREW-8x80@12»)", n)
	}
	if n := qty(got, dommfg.PlugAnchorCode, dommfg.PurposeFloor); n != 4 {
		t.Fatalf("автоматические дюбели пола = %d, want 4 (доп. не должны их заменять)", n)
	}
}

// TestFastenerUnknownExtraIgnored — код вне каталога молча пропускается и
// сообщается обработчику: справочник редактируется в админке, и старый
// проект не должен переставать считаться.
func TestFastenerUnknownExtraIgnored(t *testing.T) {
	cfg := testCfg()
	cfg.ExtraFasteners = []string{"PLUG-НЕТ-ТАКОГО"}
	var unknown []string
	got, err := FastenerDemandFor(cfg, nil, nil, func(code string) { unknown = append(unknown, code) })
	if err != nil {
		t.Fatalf("неизвестный код не должен ронять расчёт: %v", err)
	}
	for _, d := range got {
		if d.Purpose == dommfg.PurposeObject {
			t.Fatalf("неизвестный код попал в смету: %+v", d)
		}
	}
	if len(unknown) != 1 || unknown[0] != "PLUG-НЕТ-ТАКОГО" {
		t.Fatalf("обработчик получил %v, want [PLUG-НЕТ-ТАКОГО]", unknown)
	}
}

// TestFastenerNoRailingNoRailFasteners — лестница без перил не получает
// крепежа ограждения.
//
// Сторона ограждения обязана быть названа явно (так же, как в геометрии):
// пустое значение и RailingNone означают, что перил нет. Спецификация до
// S-189 читала всегда «одна нитка» и выставляла 29 покупных деталей ограждения
// лестнице, у которой перил нет вовсе.
func TestFastenerNoRailingNoRailFasteners(t *testing.T) {
	for _, side := range []engineering.RailingSide{engineering.RailingNone, ""} {
		cfg := testCfg()
		cfg.Railing = side
		got := demandOf(t, cfg)
		if n := qty(got, dommfg.ScrewTreadRiserCode, dommfg.PurposeRailFlange); n != 0 {
			t.Errorf("сторона %q: глухарей на фланцы = %d, want 0", side, n)
		}
		if n := qty(got, dommfg.ScrewRailPostCode, dommfg.PurposeRailPost); n != 0 {
			t.Errorf("сторона %q: саморезов на стойки = %d, want 0", side, n)
		}
		if parts := purchasedRailingParts(cfg.StepCount, cfg.TreadDepth.Millimeters(),
			railLengthMM(cfg), cfg.RailingHeight.Millimeters(),
			enggeo.RailingSideCount(cfg), 1); len(parts) != 0 {
			t.Errorf("сторона %q: в спецификации %d покупных деталей ограждения, want 0",
				side, len(parts))
		}
	}
}

// TestRailingBothSidesDoubleFasteners — на двух сторонах крепежа вдвое
// больше, и спецификация тоже вдвое больше. Считать одну нитку при двух
// сторонах было ошибкой: геометрия строит две независимые линии перил.
func TestRailingBothSidesDoubleFasteners(t *testing.T) {
	one := testCfg()
	both := testCfg()
	both.Railing = engineering.RailingBoth

	g1, g2 := demandOf(t, one), demandOf(t, both)
	for _, c := range []struct{ code, purpose string }{
		{dommfg.ScrewTreadRiserCode, dommfg.PurposeRailFlange},
		{dommfg.ScrewRailPostCode, dommfg.PurposeRailPost},
	} {
		a, b := qty(g1, c.code, c.purpose), qty(g2, c.code, c.purpose)
		if a == 0 {
			t.Fatalf("%s: на одной стороне ничего не посчитано", c.purpose)
		}
		if b != 2*a {
			t.Errorf("%s: на двух сторонах %d, на одной %d, want ровно вдвое", c.purpose, b, a)
		}
	}
}

// TestFastenerNoHolesNoDemand — деталь без отверстий (косоур, ступень) не
// тянет за собой крепёж.
//
// Проверяются назначения, которые считаются ПО ТЕЛАМ МОДЕЛИ. Крепёж
// ограждения от тел не зависит (тела ограждения в модель не входят) и
// проверяется отдельно — см. TestFastenerRailingFlange и RailPost.
func TestFastenerNoHolesNoDemand(t *testing.T) {
	got := demandOf(t, testCfg(), holed("stringer", 0), holed("tread", 0), holed("riser", 0))
	for key, n := range got {
		if strings.HasSuffix(key, "|"+dommfg.PurposeRailFlange) ||
			strings.HasSuffix(key, "|"+dommfg.PurposeRailPost) {
			continue
		}
		t.Fatalf("потребность по телам модели = %s %d, want пусто", key, n)
	}
}

// TestFastenerRailingFlange — на фланец опоры столько глухарей 6х35,
// сколько анкерных отверстий в цанге. На марше 15×270 четыре стойки, в
// цанге по три отверстия: 4 × 3 = 12 (решение владельца 2026-10-13).
func TestFastenerRailingFlange(t *testing.T) {
	got := demandOf(t, testCfg())
	posts := railingPostCount(15, 270)
	want := posts * enggeo.RailFlangeHoles()
	if want != 12 {
		t.Fatalf("тестовая база поехала: стойки=%d, отверстий=%d, want 4 и 3", posts, enggeo.RailFlangeHoles())
	}
	if n := qty(got, dommfg.ScrewTreadRiserCode, dommfg.PurposeRailFlange); n != want {
		t.Fatalf("глухарей на фланцы = %d, want %d (4 стойки × 3 отверстия)", n, want)
	}
	// В ступень фланец крепится к пятке/проступи деревянной лестницы или
	// стальной — дюбель на нём не появляется ни при каком основании.
	if n := qty(got, dommfg.PlugAnchorCode, dommfg.PurposeRailFlange); n != 0 {
		t.Fatalf("дюбелей на фланцах = %d, want 0", n)
	}
}

// TestFastenerRailPost — на стойку 2 самореза 5х32 (решение владельца
// 2026-10-13). Количество стойек то же, что и в спецификации ограждения:
// один счётчик, одна правда.
func TestFastenerRailPost(t *testing.T) {
	got := demandOf(t, testCfg())
	posts := railingPostCount(15, 270)
	want := posts * dommfg.ScrewRailPostQtyPerPost
	if n := qty(got, dommfg.ScrewRailPostCode, dommfg.PurposeRailPost); n != want {
		t.Fatalf("саморезов на стойки = %d, want %d (%d стойки × %d)",
			n, want, posts, dommfg.ScrewRailPostQtyPerPost)
	}
}

// TestRailingFastenerCountsMatchSpec — крепёж ограждения не может разойтись
// со спецификацией: число фланцев и стоек там и здесь считается одним
// счётчиком railingPostCount. Тест ловит возврат собственной формулы в любой
// из двух сторон.
func TestRailingFastenerCountsMatchSpec(t *testing.T) {
	cfg := testCfg()
	parts := purchasedRailingParts(cfg.StepCount, cfg.TreadDepth.Millimeters(),
		railLengthMM(cfg), cfg.RailingHeight.Millimeters(), enggeo.RailingSideCount(cfg), 1)
	flanges, posts := 0, 0
	for _, p := range parts {
		// Остальные покупные детали ограждения крепёж не несут: поручень и
		// ригели идут по нитке, наконечник держится на шпильке.
		if p.Kind == dommfg.PartFlange {
			flanges++
		}
		if p.Kind == dommfg.PartPost {
			posts++
		}
	}
	got := demandOf(t, cfg)
	if n := qty(got, dommfg.ScrewTreadRiserCode, dommfg.PurposeRailFlange); n != flanges*enggeo.RailFlangeHoles() {
		t.Fatalf("глухарей на фланцы = %d, а фланцев в спецификации %d × %d отверстий = %d",
			n, flanges, enggeo.RailFlangeHoles(), flanges*enggeo.RailFlangeHoles())
	}
	if n := qty(got, dommfg.ScrewRailPostCode, dommfg.PurposeRailPost); n != posts*dommfg.ScrewRailPostQtyPerPost {
		t.Fatalf("саморезов на стойки = %d, а стоек в спецификации %d × %d = %d",
			n, posts, dommfg.ScrewRailPostQtyPerPost, posts*dommfg.ScrewRailPostQtyPerPost)
	}
}

// TestRailingFlangeHolesReportedByGeometry — цанга объявляет число своих
// анкерных отверстий на теле. Производство этим не пользуется (тела
// ограждения в модель не входят), но объявление обязано совпадать с
// геометрией: иначе отверстия есть, а «знает» о них только чертёж.
//
// Роль clamp носят ДВА тела — диск с отверстиями и глухая муфта, — поэтому
// отверстия суммируются по всем телам роли: у диска 3, у муфты 0.
func TestRailingFlangeHolesReportedByGeometry(t *testing.T) {
	solids, err := enggeo.BuildRailingDecor(testCfg())
	if err != nil {
		t.Fatalf("BuildRailingDecor: %v", err)
	}
	clamps, holes := 0, 0
	for _, s := range solids {
		if s != nil && s.Role() == "clamp" {
			clamps++
			holes += s.Holes()
		}
	}
	if clamps == 0 {
		t.Fatal("в ограждении нет ни одной цанги — тест бессмысленен")
	}
	wantPosts := railingPostCount(15, 270) * enggeo.RailingSideCount(testCfg())
	if holes != wantPosts*enggeo.RailFlangeHoles() {
		t.Fatalf("цанги объявляют %d отверстий, а стоек %d × %d отверстий = %d",
			holes, wantPosts, enggeo.RailFlangeHoles(), wantPosts*enggeo.RailFlangeHoles())
	}
}
