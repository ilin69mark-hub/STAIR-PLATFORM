package stair

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/domain/manufacturing"
)

// hintFor собирает конфигурацию и возвращает подсказку норматива по полю
// «Толщина косоура».
func hintFor(t *testing.T, material string, w, h float64) string {
	t.Helper()
	cfg := Config{
		Width:             engineering.Length(w),
		Height:            engineering.Length(h),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		Riser:             true,
		Clearance:         engineering.Length(2500),
		RailingHeight:     engineering.Length(1000),
		ApproachSpace:     engineering.Length(1000),
		Material:          manufacturing.MaterialCode(material),
		TreadMaterial:     manufacturing.MaterialCode(material),
	}
	s := NewService()
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	return res.Hints["stringerThicknessMM"]
}

// TestStringerNormHintTimberBands — подсказка по деревянной балке повторяет
// норматив владельца (2026-10-05). Раньше эти числа жили копией в TypeScript,
// и вместе с ней разошлись бы с сервером.
func TestStringerNormHintTimberBands(t *testing.T) {
	// Ожидания посчитаны по серверному решалке (solve.go), а НЕ по старой
	// оценке из TypeScript. Расхождение между ними было реальным:
	// при H = 1200 фронт считал проекцию как H/h·b = 1800 мм и советовал
	// полосу «до 2.0 м» → 40 мм, а сервер решает n = round(H/h0) = 7,
	// b = 630 − 2·(1200/7) = 287,1 мм, проекция 2010 мм → «2.0 – 3.0 м»
	// → 50 мм. Разница 10 мм толщины косоура — то есть разница в деньгах.
	cases := []struct {
		name   string
		height float64
		want   []string
	}{
		{"короткий марш", 1200, []string{"2.0 – 3.0 м", "от 50 мм", "заготовка 250–300", "не менее 130 мм"}},
		{"длинный марш", 3600, []string{"более 4.0 м", "от 80 мм", "заготовка 350", "опора"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hint := hintFor(t, "WOOD-OAK", 900, tc.height)
			if hint == "" {
				t.Fatalf("подсказка пуста для высоты %.0f", tc.height)
			}
			for _, w := range tc.want {
				if !strings.Contains(hint, w) {
					t.Errorf("в подсказке %q нет %q", hint, w)
				}
			}
		})
	}
}

// TestStringerNormHintWideFlight — марш шире 1200 мм: толщина берётся по
// ширине, и подсказка обязана это объяснять.
func TestStringerNormHintWideFlight(t *testing.T) {
	hint := hintFor(t, "WOOD-OAK", 1300, 1200)
	if !strings.Contains(hint, "100 мм") {
		t.Errorf("в подсказке %q нет толщины 100 мм по ширине", hint)
	}
	if !strings.Contains(hint, "ширине") {
		t.Errorf("в подсказке %q не сказано, что толщина взята по ширине", hint)
	}
}

// TestStringerNormHintPlateFrame — листовой каркас: подсказка собирается из
// таблицы листа (2026-10-06), а не из деревянной.
func TestStringerNormHintPlateFrame(t *testing.T) {
	hint := hintFor(t, "STEEL-C245", 900, 2700)
	if hint == "" {
		t.Fatal("подсказка пуста для листового каркаса")
	}
	if !strings.Contains(hint, "лист") {
		t.Errorf("в подсказке %q нет слова «лист»", hint)
	}
	if strings.Contains(hint, "заготовка 250") {
		t.Errorf("в подсказке листового каркаса %q просочилась деревянная строка", hint)
	}
}

// TestStringerNormHintAlwaysPresent — подсказка приходит ВСЕГДА, а не только
// при нарушении норматива: человек видит её при вводе, а не после ошибки.
func TestStringerNormHintAlwaysPresent(t *testing.T) {
	for _, h := range []float64{1200, 2700, 3300} {
		if hint := hintFor(t, "WOOD-OAK", 900, h); hint == "" {
			t.Errorf("высота %.0f: подсказка пуста, хотя норматив не нарушен", h)
		}
	}
}

// TestStringerNormHintWorstSegment — при Г-образном марше длины маршей
// разные, и подсказка обязана показывать ХУДШИЙ, а не первый попавшийся.
func TestStringerNormHintWorstSegment(t *testing.T) {
	cfg := Config{
		Width:             engineering.Length(900),
		Height:            engineering.Length(3300),
		Flight:            engineering.FlightLShape,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		LowerStepCount:    12,
		LandingWidth:      engineering.Length(900),
		LandingDepth:      engineering.Length(900),
		Riser:             true,
		Clearance:         engineering.Length(2500),
		RailingHeight:     engineering.Length(1000),
		ApproachSpace:     engineering.Length(1000),
		Material:          manufacturing.MaterialCode("WOOD-OAK"),
		TreadMaterial:     manufacturing.MaterialCode("WOOD-OAK"),
	}
	res, err := NewService().Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	hint := res.Hints["stringerThicknessMM"]
	if hint == "" {
		t.Fatal("подсказка пуста для Г-образного марша")
	}
	// Подсказка обязана называть ХУДШУЮ из полос, а не первую попавшуюся.
	// При H = 3300 и LShape сервер выбирает «3.0 – 4.0 м» (60 мм) как более
	// требовательную из двух маршей.
	if !strings.Contains(hint, "3.0 – 4.0 м") {
		t.Errorf("в подсказке %q нет полосы худшего марша «3.0 – 4.0 м»", hint)
	}
	if strings.Contains(hint, "2.0 – 3.0 м") {
		t.Errorf("в подсказке %q названа не худшая полоса", hint)
	}
}
