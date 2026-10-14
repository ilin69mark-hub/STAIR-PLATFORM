package stair

import (
	"context"
	"math"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/validation"
)

// Норматив на листовой (сварный) косоур (решение владельца 2026-10-05):
// ключ таблицы — толщина листа, а длина марша, ширина, глубина тела и высота
// заготовки — её пределы.
//
// Уровень — ПРЕДУПРЕЖДЕНИЕ, как у норматива на дерево: подсказывает, но не
// блокирует расчёт.
func plateIssue(t *testing.T, res *Result) *validation.Issue {
	t.Helper()
	for i := range res.Validation.Issues {
		if res.Validation.Issues[i].ID == "plate_stringer_norm" {
			return &res.Validation.Issues[i]
		}
	}
	return nil
}

// steelTestWidthMM — ширина, которую задавал каждый вызов. Параметр убран:
// по решению владельца от 2026-10-06 ширина заготовки, как и толщина, берётся
// из норматива автоматически, и значение из теста всё равно не доезжает.
const steelTestWidthMM = 900.0

func steelPlateConfig(height, thick float64) Config {
	cfg := referenceConfig()
	cfg.Material = "STEEL-C245"
	cfg.TreadMaterial = "STEEL-C245"
	cfg.Height = engineering.Length(height)
	cfg.Width = engineering.Length(steelTestWidthMM)
	cfg.StringerThickness = engineering.Length(thick)
	cfg.StepThickness = engineering.Length(6)
	return cfg
}

// Толщина листа и ширина заготовки берутся из норматива АВТОМАТИЧЕМИ по длине
// и ширине марша (решение владельца 2026-10-06). Клиент присылает что угодно —
// включая конструктивные 50 мм от деревянной модели, — и это не должно доезжать
// ни до геометрии, ни до цены.
func TestPlateStringerNormThicknessIsAutomatic(t *testing.T) {
	s := NewService()
	// H=2700 → проекция 4050 мм → строка «более 4.0 м» → лист 12 мм.
	for _, sent := range []float64{8, 50} {
		res, err := s.Calculate(context.Background(), steelPlateConfig(2700, sent), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if got := res.StringerThickness.Millimeters(); got != 12 {
			t.Errorf("прислали %.0f мм → в расчёте %.0f, ожидалось 12 по нормативу", sent, got)
		}
		if iss := plateIssue(t, res); iss != nil && strings.Contains(iss.Message, "требует лист") {
			t.Errorf("толщина приведена к норме, предупреждение лишнее: %q", iss.Message)
		}
		if res.Validation.Blocking {
			t.Error("расчёт по нормативу не должен блокироваться")
		}
	}
}

// Нормативная толщина обязана дойти до ЗАГОТОВКИ детали, а не только до эха в
// ответе: раскрой и цена считаются по ней.
func TestPlateStringerNormReachesPartBlank(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), steelPlateConfig(2700, 50), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Package == nil {
		t.Fatal("производственный пакет не собран")
	}
	for _, part := range res.Package.Parts {
		if part.Kind != dommfg.PartStringer {
			continue
		}
		if got := part.Thickness.Millimeters(); math.Abs(got-12) > 0.01 {
			t.Fatalf("%s: заготовка %.0f мм, ожидалось 12 по нормативу", part.Number, got)
		}
		// Заготовка гребёнки 350 мм, а не утверждённые 300: ширина детали
		// берётся из геометрии, а геометрия построена по поднятой спинке.
		if got := part.Width.Millimeters(); math.Abs(got-350) > 0.01 {
			t.Fatalf("%s: ширина заготовки %v, ожидалось 350", part.Number, got)
		}
		return
	}
	t.Fatal("в пакете нет косоура")
}

// Короткий марш попадает в строку 8 мм, длинный — в 12: толщина растёт вместе
// с маршем, а не берётся «по потолку» для любого металла.
func TestPlateStringerNormFollowsFlightLength(t *testing.T) {
	s := NewService()
	cases := []struct {
		height float64
		want   float64
	}{
		{1800, 8},  // проекция ≈ 2600 мм — строка «до 2.5 – 3.0 м»
		{2700, 12}, // проекция 4050 мм — строка «более 4.0 м»
	}
	for _, c := range cases {
		res, err := s.Calculate(context.Background(), steelPlateConfig(c.height, 8), Options{})
		if err != nil {
			t.Fatalf("H=%.0f: %v", c.height, err)
		}
		if got := res.StringerThickness.Millimeters(); got != c.want {
			t.Errorf("H=%.0f: толщина %.0f, ожидалось %.0f", c.height, got, c.want)
		}
	}
}

// Ширина заготовки гребёнки тоже нормативная: 12 мм требует от 350 мм, а
// утверждённые 300 не годятся. Подставляется сама, без предупреждения.
func TestPlateStringerNormBlankIsAutomatic(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), steelPlateConfig(2700, 12), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.StringerSpine.Millimeters(); got != 350 {
		t.Fatalf("ширина заготовки гребёнки %.0f, ожидалось 350 по нормативу 12 мм", got)
	}
	if iss := plateIssue(t, res); iss != nil && strings.Contains(iss.Message, "заготовки гребёнки") {
		t.Errorf("заготовка приведена к норме, предупреждение лишнее: %q", iss.Message)
	}
	// Короткий марш остаётся на утверждённых 300 мм: норматив 8 мм требует
	// от 250, и увеличивать заготовку незачем.
	res, err = s.Calculate(context.Background(), steelPlateConfig(1800, 8), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.StringerSpine.Millimeters(); got != 300 {
		t.Errorf("короткий марш: ширина заготовки %.0f, ожидалось 300 (утверждённый чертёж)", got)
	}
	if iss := plateIssue(t, res); iss != nil {
		t.Errorf("короткий марш нормативу соответствует: %q", iss.Message)
	}
}

// Толщина листа 10 и 12 мм должны быть вообще допустимы: без этого норматив
// неисполним — каталог отвергал бы предписанную толщину.
func TestPlateSteelThicknessTenAndTwelveAreAllowed(t *testing.T) {
	s := NewService()
	for _, th := range []float64{8, 10, 12} {
		res, err := s.Calculate(context.Background(), steelPlateConfig(1800, th), Options{})
		if err != nil {
			t.Fatalf("лист %.0f мм: %v", th, err)
		}
		if res.Validation.Blocking {
			t.Errorf("лист %.0f мм не должен блокировать расчёт: %+v",
				th, res.Validation.Issues)
		}
		if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
			t.Errorf("лист %.0f мм: расчёта цены нет", th)
		}
	}
}

// К деревянному каркасу норматив для листа не применяется — там своя таблица.
func TestPlateStringerNormIgnoresWoodFrame(t *testing.T) {
	s := NewService()
	cfg := steelPlateConfig(2700, 8)
	cfg.Material = "WOOD-OAK"
	cfg.TreadMaterial = "WOOD-OAK"
	cfg.StepThickness = engineering.Length(40)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if iss := plateIssue(t, res); iss != nil {
		t.Fatalf("деревянный каркас не должен попадать под норматив на лист: %q", iss.Message)
	}
}
