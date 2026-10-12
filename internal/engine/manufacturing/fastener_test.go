package manufacturing

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
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
func testCfg() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width: 900, Height: 2700, Flight: engineering.FlightStraight,
		StepCount: 15, StepHeight: 180, TreadDepth: 270,
		StringerThickness: 8, StepThickness: 5, Riser: true,
		Clearance: 2000, RailingHeight: 900, ApproachSpace: 1000,
		Material: "STEEL-S235", TreadMaterial: "STEEL-S235",
		Comb: engineering.CombFrameDefaults(),
	}
}

// solid с ролью и числом отверстий — тело для счёта крепежа.
func holed(role string, n int) *kerngeo.Solid {
	return kerngeo.NewSolid().WithRole(role).WithHoles(n)
}

func demandOf(t *testing.T, cfg *engineering.StairConfiguration, solids ...*kerngeo.Solid) map[string]int {
	t.Helper()
	got, err := FastenerDemandFor(cfg, solids, nil, nil)
	if err != nil {
		t.Fatalf("FastenerDemandFor: %v", err)
	}
	out := map[string]int{}
	for _, d := range got {
		out[d.Code] += int(d.Quantity)
	}
	return out
}

// TestFastenerPerHoleOnTreadAndRiser — на ступени и подступенки ровно по
// одному глухарю 6х35 на каждое отверстие пяток обоймы выреза.
func TestFastenerPerHoleOnTreadAndRiser(t *testing.T) {
	// 30 пяток под ступенью по 4 отверстия + 28 пяток под подступенком по 3
	// + 2 пластины 1-й ступени по 3 = 120 + 84 + 6 = 210.
	got := demandOf(t, testCfg(),
		holed("heel", 4), holed("web", 3), holed("first_web", 3))
	if got[dommfg.ScrewTreadRiserCode] != 10 {
		t.Fatalf("глухарей на обойму выреза = %d, want 10 (4+3+3 на деталь)", got[dommfg.ScrewTreadRiserCode])
	}
	if _, ok := got[dommfg.PlugAnchorCode]; ok {
		t.Fatal("дюбель не должен появляться на пятках подступенка и ступени")
	}
}

// TestFastenerAnchorsConcrete — в бетон и глухарь, и дюбель, по одному на
// каждое отверстие пятки пола и пятки выхода на перекрытие.
func TestFastenerAnchorsConcrete(t *testing.T) {
	got := demandOf(t, testCfg(), holed("anchor", 4), holed("top_anchor", 3))
	if got[dommfg.ScrewAnchorCode] != 7 {
		t.Fatalf("глухарей 10х60 = %d, want 7 (4 в пол + 3 в перекрытие)", got[dommfg.ScrewAnchorCode])
	}
	if got[dommfg.PlugAnchorCode] != 7 {
		t.Fatalf("дюбелей 10х60 = %d, want 7 в бетон", got[dommfg.PlugAnchorCode])
	}
}

// TestFastenerAnchorsWood — в дерево дюбель не нужен: глухарь вворачивается
// прямо. Это и есть причина, по которой в админке два отдельных выбора.
func TestFastenerAnchorsWood(t *testing.T) {
	cfg := testCfg()
	cfg.FloorFixing = engineering.BaseWood
	cfg.CeilingFixing = engineering.BaseWood
	got := demandOf(t, cfg, holed("anchor", 4), holed("top_anchor", 3))
	if got[dommfg.ScrewAnchorCode] != 7 {
		t.Fatalf("глухарей 10х60 = %d, want 7", got[dommfg.ScrewAnchorCode])
	}
	if got[dommfg.PlugAnchorCode] != 0 {
		t.Fatalf("дюбелей в дереве = %d, want 0", got[dommfg.PlugAnchorCode])
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
	if got["PLUG-8x80"] != 1 {
		t.Fatalf("дюбель 8х80 = %d шт, want 1 (без множителя)", got["PLUG-8x80"])
	}
	if got["SCREW-8x80"] != 12 {
		t.Fatalf("глухарей 8х80 = %d шт, want 12 (из «SCREW-8x80@12»)", got["SCREW-8x80"])
	}
	if got[dommfg.PlugAnchorCode] != 4 {
		t.Fatalf("автоматические дюбели пола = %d, want 4 (доп. не должны их заменять)", got[dommfg.PlugAnchorCode])
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
	if len(got) != 0 {
		t.Fatalf("потребность = %+v, want пусто", got)
	}
	if len(unknown) != 1 || unknown[0] != "PLUG-НЕТ-ТАКОГО" {
		t.Fatalf("обработчик получил %v, want [PLUG-НЕТ-ТАКОГО]", unknown)
	}
}

// TestFastenerNoHolesNoDemand — деталь без отверстий (косоур, ступень) не
// тянет за собой крепёж.
func TestFastenerNoHolesNoDemand(t *testing.T) {
	got := demandOf(t, testCfg(), holed("stringer", 0), holed("tread", 0), holed("riser", 0))
	if len(got) != 0 {
		t.Fatalf("потребность = %v, want пусто", got)
	}
}
