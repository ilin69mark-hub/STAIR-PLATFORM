package manufacturing

import (
	"math"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
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
	// (enggeo.RailingPostSteps — единый источник, S-206). На марше
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

// TestPurchasedRailingAxisLengths — поручень и ригели в смете идут
// длиной по оси, а не горизонтальной проекцией (S-206).
//
// Ожидания для поручня — независимая формула n·hypot(b, h) (та же линия,
// что строит геометрия: от x=0 до x=n·b по уклону h/b). Для ригелей —
// контракт с геометрией: длина каждого пролёта обязана совпасть с осью
// между его опорами (RailingPostSteps — единый источник расстановки).
// Кейс b=280 обязателен: на b=270 вылет 2×135 случайно равен глубине
// ступени, и проекция совпадала с горизонталью модели — баг там невидим.
func TestPurchasedRailingAxisLengths(t *testing.T) {
	cases := []struct {
		name string
		n    int
		b, h float64
	}{
		{"seed 6x270x180", 6, 270, 180},
		{"не270 6x280x175", 6, 280, 175},
		{"эталон 15x270x180", 15, 270, 180},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.StepCount = c.n
			cfg.StepHeight = mustLength(t, c.h)
			cfg.TreadDepth = mustLength(t, c.b)
			pkg, err := Manufacture(cfg, genResult(t, cfg), nil)
			if err != nil {
				t.Fatalf("manufacture: %v", err)
			}
			pitch := math.Hypot(c.b, c.h)

			// Поручень: один на сторону, длина n·hypot(b, h).
			wantRail := float64(c.n) * pitch
			rails := 0
			for _, p := range pkg.Parts {
				if !p.Purchased || p.Kind != dommfg.PartRailTop {
					continue
				}
				rails++
				if got := p.Length.Millimeters(); math.Abs(got-wantRail) > 1e-6 {
					t.Errorf("поручень %.4f, want по оси %.4f", got, wantRail)
				}
			}
			if rails != 1 {
				t.Errorf("поручней %d, want 1 (одна сторона)", rails)
			}

			// Ригели: каждый пролёт — своей длиной по оси (пролёты бывают
			// неравными, усреднять запрещено). Мультимножество: каждый
			// пролёт встречается rows=2 раза (одна сторона).
			steps := enggeo.RailingPostSteps(c.n, c.b)
			want := map[float64]int{}
			for i := 0; i+1 < len(steps); i++ {
				l := float64(steps[i+1]-steps[i]) * pitch
				want[l] += 2
			}
			got := map[float64]int{}
			for _, p := range pkg.Parts {
				if !p.Purchased || p.Kind != dommfg.PartRigel {
					continue
				}
				got[p.Length.Millimeters()]++
			}
			if len(got) != len(want) {
				t.Fatalf("ригели: длин %d %v, want %d %v", len(got), got, len(want), want)
			}
			for l, n := range want {
				if got[l] != n {
					t.Errorf("ригелей длиной %.4f: %d, want %d", l, got[l], n)
				}
			}
		})
	}
}
