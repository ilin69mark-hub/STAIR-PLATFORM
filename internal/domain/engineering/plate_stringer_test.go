package engineering

import "testing"

// Норматив на листовой (сварной) косоур (решение владельца 2026-10-05).
// Ключ таблицы — ТОЛЩИНА ЛИСТА, а длина марша, ширина, глубина тела и высота
// заготовки — её пределы. Обратная структура к нормативу на дерево.
func TestPlateStringerBandForThickness(t *testing.T) {
	cases := []struct {
		thick    float64
		found    bool
		runMax   float64
		runLabel string
		widthMax float64
		minBody  float64
		blankMin float64
	}{
		{thick: 8, found: true, runMax: 3000, runLabel: "до 2.5 – 3.0 м", widthMax: 1000, minBody: 120, blankMin: 250},
		{thick: 10, found: true, runMax: 4000, runLabel: "3.0 – 4.0 м", widthMax: 1200, minBody: 140, blankMin: 300},
		{thick: 12, found: true, runMax: 0, runLabel: "более 4.0 м", widthMax: 1400, minBody: 160, blankMin: 350},
		// Толщин вне таблицы норматив не описывает.
		{thick: 6, found: false},
		{thick: 20, found: false},
	}
	for _, c := range cases {
		b, ok := PlateStringerBandForThickness(c.thick)
		if ok != c.found {
			t.Fatalf("%.0f мм: found %v, ожидалось %v", c.thick, ok, c.found)
		}
		if !ok {
			continue
		}
		if b.RunMaxMM != c.runMax || b.WidthMaxMM != c.widthMax ||
			b.MinBodyMM != c.minBody || b.BlankMinMM != c.blankMin || b.RunLabel != c.runLabel {
			t.Errorf("%.0f мм: строка %+v не совпадает с ожиданием", c.thick, b)
		}
	}
}

// Минимальная толщина листа по длине марша: полосы должны идти без дыр.
// 8 мм покрывает до 3.0 м (верхний конец диапазона «2.5 – 3.0 м»), 10 мм —
// 3.0–4.0 м, 12 мм — всё, что длиннее.
func TestMinPlateStringerThicknessByRun(t *testing.T) {
	cases := []struct{ run, want float64 }{
		{run: 1000, want: 8},
		{run: 3000, want: 8},
		{run: 3001, want: 10},
		{run: 4000, want: 10},
		{run: 4001, want: 12},
		{run: 9000, want: 12},
	}
	for _, c := range cases {
		if got := MinPlateStringerThicknessMM(c.run, 900); got != c.want {
			t.Errorf("марш %.0f мм: толщина %.0f, ожидалось %.0f", c.run, got, c.want)
		}
	}
}

// Ширина может только ПОДНЯТЬ толщину: широкий марш на тонком листе
// нормативом не разрешён.
func TestMinPlateStringerThicknessByWidth(t *testing.T) {
	cases := []struct {
		run, width, want float64
	}{
		{run: 2000, width: 900, want: 8},
		{run: 2000, width: 1000, want: 8},
		{run: 2000, width: 1001, want: 10},
		{run: 2000, width: 1200, want: 10},
		{run: 2000, width: 1201, want: 12},
		{run: 2000, width: 1400, want: 12},
		// За пределами самой широкой строки — тоже 12 мм.
		{run: 2000, width: 1401, want: 12},
		// Длина и ширина складываются: марш 3500 мм при ширине 1300 требует 12.
		{run: 3500, width: 1300, want: 12},
	}
	for _, c := range cases {
		if got := MinPlateStringerThicknessMM(c.run, c.width); got != c.want {
			t.Errorf("марш %.0f, ширина %.0f: толщина %.0f, ожидалось %.0f",
				c.run, c.width, got, c.want)
		}
	}
}

// Строка по ДЛИНЕ марша нужна для текста подсказки и не совпадает со строкой
// по толщине: марш 4050 мм у листа 8 мм попадает в строку «более 4.0 м».
func TestPlateStringerBandForRun(t *testing.T) {
	cases := []struct {
		run       float64
		wantLabel string
	}{
		{run: 1000, wantLabel: "до 2.5 – 3.0 м"},
		{run: 3000, wantLabel: "до 2.5 – 3.0 м"},
		{run: 3001, wantLabel: "3.0 – 4.0 м"},
		{run: 4000, wantLabel: "3.0 – 4.0 м"},
		{run: 4001, wantLabel: "более 4.0 м"},
	}
	for _, c := range cases {
		if got := PlateStringerBandForRun(c.run).RunLabel; got != c.wantLabel {
			t.Errorf("марш %.0f мм: полоса %q, ожидалось %q", c.run, got, c.wantLabel)
		}
	}
}

func TestPlateStringerSpec(t *testing.T) {
	// Лист 8 мм на марше 4050 мм — норматив не выполнен, нужен 12.
	sp := PlateStringerSpecFor(8, 4050, 900)
	if sp.BandFound != true || sp.Band.ThicknessMM != 8 {
		t.Errorf("строка по толщине 8 мм не найдена: %+v", sp.Band)
	}
	if !sp.ThicknessTooThin {
		t.Error("лист 8 мм на марше 4050 мм должен быть признан слишком тонким")
	}
	if sp.RequiredMM != 12 {
		t.Errorf("требуемая толщина %.0f, ожидалось 12", sp.RequiredMM)
	}
	if sp.WidthBeyondNarrow {
		t.Error("ширина 900 мм за пределами норматива? Нет")
	}
	// Лист 12 мм на марше 4050 мм — толщины хватает.
	if sp := PlateStringerSpecFor(12, 4050, 900); sp.ThicknessTooThin {
		t.Error("лист 12 мм на марше 4050 мм должен проходить по толщине")
	}
	// Ширина 1500 мм — за пределами самой широкой строки.
	if sp := PlateStringerSpecFor(12, 2000, 1500); !sp.WidthBeyondNarrow {
		t.Error("ширина 1500 мм должна быть помечена как за пределами норматива")
	}
	// Толщина вне таблицы.
	if sp := PlateStringerSpecFor(6, 1000, 900); sp.BandFound {
		t.Error("6 мм не значится в нормативе и не должна давать строку")
	}
}

// Особенности конструкции — это текст для калькулятора, а не правило: строки
// обязаны его нести, иначе колонка «особенности» из таблицы владельца пропала бы.
func TestPlateStringerBandsCarryNotes(t *testing.T) {
	// Решение владельца 2026-10-06: требование обвязки уголком снято, уголок
	// пока не нужен. Жёсткости добираемся шириной заготовки гребёнки, которую
	// норматив и так требует (см. NormalizePlateStringer). Когда уголок
	// понадобится, данные и элементы чертежа внесутся отдельно.
	want := map[float64]string{
		8:  "домашних лестниц",
		10: "без промежуточных опор",
		12: "монокосоура",
	}
	for _, b := range PlateStringerBands {
		sub, ok := want[b.ThicknessMM]
		if !ok {
			t.Fatalf("нет ожидания для %.0f мм", b.ThicknessMM)
		}
		if !contains(b.Notes, sub) {
			t.Errorf("%.0f мм: особенности %q не содержат %q", b.ThicknessMM, b.Notes, sub)
		}
		if b.BlankLabel() == "" || b.BodyLabel == "" {
			t.Errorf("%.0f мм: пустая подпись заготовки или тела", b.ThicknessMM)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Толщина листа выводится из нормативной строки по длине и ширине марша, а не
// берётся из конфигурации: у стали это лист 8 / 10 / 12 мм, а не ширина
// сечения бруса.
func TestPlateStringerThicknessFor(t *testing.T) {
	cases := []struct {
		name  string
		runs  []float64
		width float64
		want  float64
	}{
		{"короткий марш", []float64{2400}, 900, 8},
		{"на границе 3 м", []float64{3000}, 900, 8},
		{"средний марш", []float64{3600}, 900, 10},
		{"длинный марш", []float64{4545}, 900, 12},
		{"широкий марш поднимает толщину", []float64{2400}, 1300, 12},
		// У П-лестницы маршей два: короткий нижний не должен утягивать
		// толщину вниз, длинный верхний её требует.
		{"два марша, берётся максимум", []float64{2100, 4200}, 900, 12},
	}
	for _, c := range cases {
		if got := PlateStringerThicknessFor(c.runs, c.width); got != c.want {
			t.Errorf("%s: толщина %.0f, ожидалось %.0f", c.name, got, c.want)
		}
	}
}

// Стальной косоур приходит из клиента с конструктивными 50 мм (ширина сечения
// бруса от деревянной модели) — в конвейере их быть не должно.
func TestNormalizePlateStringerReplacesBeamThickness(t *testing.T) {
	c := &StairConfiguration{
		Material:          "STEEL-S235",
		Width:             Length(900),
		StringerThickness: Length(50),
	}
	thickness, spine, changed := NormalizePlateStringer(c, []float64{4545})
	if !changed || thickness != 12 {
		t.Fatalf("толщина %.0f (changed=%v), ожидалось 12 по нормативу", thickness, changed)
	}
	if c.StringerThickness.Millimeters() != 12 {
		t.Fatalf("в конфигурации осталось %.0f мм", c.StringerThickness.Millimeters())
	}
	// Заготовка гребёнки поднимается до высоты исходной заготовки своей
	// строки: 12 мм → от 350, утверждённые 300 не годятся.
	if spine != 350 || c.CombOrDefault().StringerSpine != 350 {
		t.Fatalf("ширина гребёнки %.0f, ожидалось 350", spine)
	}
}

// Материал не задан — 50 мм это доска, а не ошибочная цифра: по толщине детали
// автоназначение выбирает между сталью и пиломатериалом, и подмена сдвинула бы
// изделие в другую геометрию.
func TestNormalizePlateStringerKeepsWoodAndUnsetMaterial(t *testing.T) {
	for _, material := range []string{"", "WOOD-OAK", "WOOD-WALNUT"} {
		c := &StairConfiguration{
			Material:          material,
			Width:             Length(900),
			StringerThickness: Length(50),
		}
		if _, _, changed := NormalizePlateStringer(c, []float64{4545}); changed {
			t.Errorf("материал %q: конфигурация изменена, а не должна", material)
		}
		if c.StringerThickness.Millimeters() != 50 {
			t.Errorf("материал %q: толщина %.0f, ожидалось 50",
				material, c.StringerThickness.Millimeters())
		}
	}
}

// Норматив требует МИНИМУМ. Если заготовка задана больше (осознанное решение
// чертёжника на кручение), резать её вниз нельзя.
func TestNormalizePlateStringerNeverShrinksComb(t *testing.T) {
	c := &StairConfiguration{
		Material:          "STEEL-S235",
		Width:             Length(900),
		StringerThickness: Length(50),
		Comb:              &CombFrame{StringerSpine: 420},
	}
	_, spine, changed := NormalizePlateStringer(c, []float64{2400})
	if !changed {
		t.Fatal("толщина должна быть приведена к нормативу")
	}
	if spine != 420 {
		t.Fatalf("ширина гребёнки уменьшена до %.0f: осознанное решение резать нельзя", spine)
	}
}

// Без маршей норматив неприменим: длину марша узнать неоткуда, и подставлять
// толщину по догадке нельзя.
func TestNormalizePlateStringerNeedsRun(t *testing.T) {
	c := &StairConfiguration{Material: "STEEL-S235", Width: Length(900), StringerThickness: Length(50)}
	if _, _, changed := NormalizePlateStringer(c, nil); changed {
		t.Fatal("без длины марша норматив не применяется")
	}
	if c.StringerThickness.Millimeters() != 50 {
		t.Fatalf("толщина изменена на %.0f без данных о марше", c.StringerThickness.Millimeters())
	}
}
