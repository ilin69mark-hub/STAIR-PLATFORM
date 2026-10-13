package manufacturing

import (
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// TestPurchasedRailingInSpecNotInNesting — ограждение обязано быть в
// спецификации и НЕ в карте раскроя.
//
// Ограждение покупается готовым (решение владельца): стойки, поручень,
// ригели и фитинги не раскраиваются из листа. Раньше ограждения в пакете не
// было вообще — decompose() разбирает только тело модели, а геометрия
// ограждения строится как декорация. Этот тест закрывает обе стороны:
// спецификация содержит ограждение, раскрой — нет.
func TestPurchasedRailingInSpecNotInNesting(t *testing.T) {
	cfg := testConfig(t)
	cfg.RailingHeight = mustLength(t, 900)
	pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
	if err != nil {
		t.Fatalf("manufacture: %v", err)
	}

	// 1. Все восемь покупных позиций ограждения есть в пакете.
	wantKinds := []dommfg.PartKind{
		dommfg.PartRailTop, dommfg.PartPost, dommfg.PartRigel, dommfg.PartTip,
		dommfg.PartFlange, dommfg.PartSlip, dommfg.PartDeco, dommfg.PartEndCap,
	}
	counts := map[dommfg.PartKind]int{}
	for _, p := range pkg.Parts {
		if p.Purchased {
			counts[p.Kind]++
		}
	}
	for _, k := range wantKinds {
		if counts[k] == 0 {
			t.Errorf("в пакете нет покупных деталей %q", k)
		}
	}

	// 2. Количество стоек совпадает с геометрией: правило выбора опор одно
	// (balusterSteps в геометрии / railingPostIndices здесь). На марше
	// 15×270 это 4 стойки, 6 ригелей, 4 наконечника.
	if got := counts[dommfg.PartPost]; got != 4 {
		t.Errorf("стоек %d, want 4 (правило выбора опор разошлось с геометрией)", got)
	}
	if got := counts[dommfg.PartRigel]; got != 6 {
		t.Errorf("ригелей %d, want 6 (2 ряда × 3 пролёта)", got)
	}
	if got := counts[dommfg.PartTip]; got != 4 {
		t.Errorf("наконечников %d, want 4 (по одному на стойку)", got)
	}
	if got := counts[dommfg.PartEndCap]; got != 2 {
		t.Errorf("торцевых заглушек %d, want 2", got)
	}

	// 3. В карте раскроя покупных деталей НЕТ.
	for _, item := range pkg.CutList.Items {
		if p := findPart(pkg, item.PartNumber); p != nil && p.Purchased {
			t.Errorf("покупная деталь %q попала в карту раскроя — листа под неё не режем",
				item.PartNumber)
		}
	}

	// 4. В спецификации ограждение ЕСТЬ — иначе менеджер не знает, что купить.
	descriptions := map[string]bool{}
	for _, l := range pkg.BOM.Lines {
		descriptions[l.Description] = true
	}
	for _, d := range []string{
		"Перила (поручень)", "Стойка опоры", "Ригель",
		"Наконечник регулируемый", "Фланец опоры", "Накидная часть",
		"Декоративная накладка", "Торцевая заглушка",
	} {
		if !descriptions[d] {
			t.Errorf("в спецификации нет строки %q", d)
		}
	}

	// 5. Покупные детали не попадают в техмаршрут: резать нечего.
	plan, err := PlanOperations(pkg, DefaultMachineRates())
	if err != nil {
		t.Fatalf("plan operations: %v", err)
	}
	for _, op := range plan.Parts {
		if p := findPart(pkg, op.PartNumber); p != nil && p.Purchased {
			t.Errorf("покупная деталь %q попала в техмаршрут", op.PartNumber)
		}
	}
}
