package engineering

import "testing"

// Норматив на деревянный косоур-балку (решение владельца 2026-10-05):
// полосы 40 / 50 / 60 / 80 мм по длине марша по проекции и 100 мм при
// ширине марша более 1200 мм. Таблица — источник истины, зеркала в калькуляторе
// и в тестах обязаны ссылаться на неё, а не вписывать числа строкой.
func TestTimberStringerBandForRun(t *testing.T) {
	cases := []struct {
		run          float64
		label        string
		minThick     float64
		blankMin     float64
		blankMax     float64
		minBody      float64
		needsSupport bool
	}{
		{run: 1000, label: "до 2.0 м", minThick: 40, blankMin: 250, blankMax: 0, minBody: 120},
		{run: 2000, label: "до 2.0 м", minThick: 40, blankMin: 250, blankMax: 0, minBody: 120},
		// Граница полосы включается в нижнюю: 2001 уже «2.0 – 3.0 м».
		{run: 2001, label: "2.0 – 3.0 м", minThick: 50, blankMin: 250, blankMax: 300, minBody: 130},
		{run: 3000, label: "2.0 – 3.0 м", minThick: 50, blankMin: 250, blankMax: 300, minBody: 130},
		{run: 3001, label: "3.0 – 4.0 м", minThick: 60, blankMin: 300, blankMax: 0, minBody: 150},
		{run: 4000, label: "3.0 – 4.0 м", minThick: 60, blankMin: 300, blankMax: 0, minBody: 150},
		// Последняя полоса: уточнение владельца — 80 мм, а не «70 и более».
		{run: 4001, label: "более 4.0 м", minThick: 80, blankMin: 350, blankMax: 0, minBody: 0, needsSupport: true},
		{run: 9000, label: "более 4.0 м", minThick: 80, blankMin: 350, blankMax: 0, minBody: 0, needsSupport: true},
	}
	for _, c := range cases {
		b := TimberStringerBandFor(c.run)
		if b.RunLabel != c.label {
			t.Errorf("run %v: полоса %q, ожидалось %q", c.run, b.RunLabel, c.label)
		}
		if b.MinThicknessMM != c.minThick {
			t.Errorf("run %v (%s): толщина %.0f, ожидалось %.0f", c.run, b.RunLabel, b.MinThicknessMM, c.minThick)
		}
		if b.BlankMinMM != c.blankMin || b.BlankMaxMM != c.blankMax {
			t.Errorf("run %v (%s): заготовка %v–%v, ожидалось %.0f–%.0f",
				c.run, b.RunLabel, b.BlankMinMM, b.BlankMaxMM, c.blankMin, c.blankMax)
		}
		if b.MinBodyMM != c.minBody {
			t.Errorf("run %v (%s): глубина тела %.0f, ожидалось %.0f", c.run, b.RunLabel, b.MinBodyMM, c.minBody)
		}
		if b.NeedsSupport != c.needsSupport {
			t.Errorf("run %v (%s): NeedsSupport %v, ожидалось %v", c.run, b.RunLabel, b.NeedsSupport, c.needsSupport)
		}
	}
}

// Ширина марша важнее длины: при ширине БОЛЕЕ 1200 мм толщина 100 мм в любой
// полосе (уточнение владельца 2026-10-05). Ровно 1200 мм — ещё не «более»,
// там действует полоса по длине.
func TestTimberWideStringerThicknessOverridesRun(t *testing.T) {
	byRun := []float64{1000, 2500, 3500, 5000}
	for _, run := range byRun {
		if got := MinTimberStringerThicknessMM(run, 1201); got != 100 {
			t.Errorf("run %v, ширина 1201: толщина %.0f, ожидалось 100", run, got)
		}
		if got := MinTimberStringerThicknessMM(run, 1500); got != 100 {
			t.Errorf("run %v, ширина 1500: толщина %.0f, ожидалось 100", run, got)
		}
		// Ровно 1200 — полоса по длине, а не 100 мм.
		if got, want := MinTimberStringerThicknessMM(run, 1200),
			TimberStringerBandFor(run).MinThicknessMM; got != want {
			t.Errorf("run %v, ширина 1200: толщина %.0f, ожидалось %.0f (полоса по длине)", run, got, want)
		}
	}
}

// Полосы обязаны идти по возрастанию и без дыр: иначе марш попал бы не в ту
// строку норматива.
func TestTimberStringerBandsAreMonotonic(t *testing.T) {
	prev := 0.0
	for i, b := range TimberStringerBands {
		if b.MinThicknessMM <= 0 {
			t.Fatalf("полоса %d (%s): толщина должна быть положительной", i, b.RunLabel)
		}
		if b.RunMaxMM <= 0 {
			if i != len(TimberStringerBands)-1 {
				t.Errorf("полоса %d (%s) без верхней границы стоит не последней", i, b.RunLabel)
			}
			continue
		}
		if b.RunMaxMM <= prev {
			t.Errorf("полоса %d (%s): верхняя граница %.0f не больше предыдущей %.0f",
				i, b.RunLabel, b.RunMaxMM, prev)
		}
		prev = b.RunMaxMM
	}
	// Требуемая нормативом толщина обязана влезать в выпуск материала,
	// иначе каталог отвергнет ровно то, что предписывает норматив.
	for _, thick := range []float64{40, 50, 60, 80, 100} {
		if thick > float64(TimberMaxThickMM) {
			t.Fatalf("норматив требует %.0f мм, а TimberMaxThickMM = %d", thick, TimberMaxThickMM)
		}
	}
}

func TestTimberStringerLabels(t *testing.T) {
	if got := TimberStringerBandFor(1000).BlankLabel(); got != "250" {
		t.Errorf("заготовка = %q, ожидалось \"250\"", got)
	}
	if got := TimberStringerBandFor(2500).BlankLabel(); got != "250–300" {
		t.Errorf("заготовка = %q, ожидалось \"250–300\"", got)
	}
	if got := TimberStringerBandFor(1000).BodyLabel(); got != "не менее 120 мм" {
		t.Errorf("тело = %q", got)
	}
	// В последней полосе вместо числа — требование опоры: вносится позже,
	// поэтому это информация, а не блокировка.
	if got := TimberStringerBandFor(5000).BodyLabel(); got != "требуется промежуточная опора (столб)" {
		t.Errorf("тело = %q", got)
	}
}
