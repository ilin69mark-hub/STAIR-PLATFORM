package stair

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// Минимальная толщина ступени — 40 мм у дерева (решение владельца
// 2026-10-05). Это ОТДЕЛЬНОЕ правило от общего MinThickness материала: ступень
// под скругление R10 требует полки, а косоур из той же породы остаётся
// допустимым от 20 мм. Если правило сольют с MinThickness, деревянный косоур
// 20 мм станет невозможен — и сохранённые конфигурации начнут падать с 422.
//
// Тесты ниже фиксируют именно эту разницу: ступень 20 мм отвергается, косоур
// 20 мм принимается.

func woodStairWithThickness(t *testing.T, stringer, step float64) Config {
	t.Helper()
	cfg := referenceConfig()
	cfg.Material = "STEEL-C245" // каркас стальной, чтобы косоур проверялся отдельно
	cfg.TreadMaterial = "WOOD-OAK"
	cfg.StringerThickness = engineering.Length(stringer)
	cfg.StepThickness = engineering.Length(step)
	return cfg
}

func TestWoodTreadMinThicknessIs40(t *testing.T) {
	s := NewService()
	// Каталог отвергает конфигурацию НЕ ошибкой транспорта, а blocking-issue
	// с конкретными числами (errors.go превращает строки material-ошибок в
	// MFG_MATERIAL). Проверяем именно это состояние.
	for _, thick := range []float64{20, 25, 30, 39} {
		res, err := s.Calculate(context.Background(), woodStairWithThickness(t, 8, thick), Options{})
		if err != nil {
			t.Fatalf("oak tread %v mm: %v", thick, err)
		}
		if !res.Validation.Blocking {
			t.Fatalf("oak tread %v mm must be blocked (минимум ступени 40)", thick)
		}
		if res.Package != nil {
			t.Errorf("tread %v mm: производственный пакет не должен собираться", thick)
		}
	}
	// Ровно 40 — минимально допустимо.
	res, err := s.Calculate(context.Background(), woodStairWithThickness(t, 8, 40), Options{})
	if err != nil {
		t.Fatalf("oak tread 40 mm must be accepted: %v", err)
	}
	if res.Validation.Blocking {
		t.Fatalf("oak tread 40 mm must not block: %+v", res.Validation.Issues)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("oak tread 40 mm must produce a price")
	}
}

// TestWoodStringerStaysThinWhileTreadIs40 — правило 40 мм относится только к
// ступени. Косоур из той же породы 20 мм остаётся допустимым: это разные
// детали, и ужесточение косоура сломало бы сохранённые конфигурации.
func TestWoodStringerStaysThinWhileTreadIs40(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = "WOOD-OAK"
	cfg.TreadMaterial = "WOOD-OAK"
	cfg.StringerThickness = engineering.Length(20)
	cfg.StepThickness = engineering.Length(40)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("wood stringer 20 mm must be accepted: %v", err)
	}
	if res.Validation.Blocking {
		t.Fatalf("wood stringer 20 mm must not block: %+v", res.Validation.Issues)
	}
	// Ступень 20 мм при тонком косоуре — по-прежнему блокируется.
	cfg.StepThickness = engineering.Length(20)
	res, err = s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if !res.Validation.Blocking {
		t.Fatal("oak tread 20 mm must be blocked even with a 20 mm stringer")
	}
}

// TestSteelTreadThicknessUnaffectedByWoodRule — правило 40 мм не должно
// распространяться на металл: стальная ступень 6 мм остаётся допустимой.
func TestSteelTreadThicknessUnaffectedByWoodRule(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = "STEEL-C245"
	cfg.TreadMaterial = "STEEL-C245"
	cfg.StringerThickness = engineering.Length(8)
	cfg.StepThickness = engineering.Length(6)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("steel tread 6 mm must be accepted: %v", err)
	}
	if res.Validation.Blocking {
		t.Fatalf("steel tread 6 mm must not block: %+v", res.Validation.Issues)
	}
}

// TestThicknessErrorGuideNamesTreadMinimum — подсказка в blocking-issue
// обязана называть ПРАВИЛЬНЫЙ предел: 40 мм для ступени и 20 мм для косоура
// из того же дуба. Иначе пользователь «чинит» конфигурацию до неверного числа.
func TestThicknessErrorGuideNamesTreadMinimum(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), woodStairWithThickness(t, 8, 20), Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if !res.Validation.Blocking {
		t.Fatal("oak tread 20 mm must be blocked")
	}
	var issue *validation.Issue
	for i := range res.Validation.Issues {
		if res.Validation.Issues[i].Code == constraint.MFG_MATERIAL {
			issue = &res.Validation.Issues[i]
			break
		}
	}
	if issue == nil {
		t.Fatalf("want MFG_MATERIAL issue, got %+v", res.Validation.Issues)
	}
	if issue.Min != 40 {
		t.Fatalf("issue.Min = %v, want 40 (минимум ступени)", issue.Min)
	}
	if !strings.Contains(issue.Guide, "40") {
		t.Fatalf("guide must name the 40 mm tread minimum: %q", issue.Guide)
	}
	// Тот же материал на КОСОУРЕ даёт общий минимум 20 мм: значит правило
	// действительно различает детали, а не подменяет MinThickness целиком.
	cfg := referenceConfig()
	cfg.Material = "WOOD-OAK"
	cfg.TreadMaterial = "WOOD-OAK"
	cfg.StringerThickness = engineering.Length(20)
	cfg.StepThickness = engineering.Length(40)
	res, err = s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if res.Validation.Blocking {
		t.Fatalf("oak stringer 20 mm must not block: %+v", res.Validation.Issues)
	}
}

// Норматив на деревянный косоур-балку (решение владельца 2026-10-05):
// минимальная толщина 40 / 50 / 60 / 80 мм по длине марша по проекции и
// 100 мм при ширине более 1200 мм.
//
// Проверка живёт ПОСЛЕ решалки — до неё длины марша ещё нет, — и по каждому
// маршу отдельно: у L/П-лестницы два марша, и короткий нижний при длинном
// верхнем не должен вытягивать толщину обоих.
//
// Уровень — ПРЕДУПРЕЖДЕНИЕ, тем же, что и действующее GEO-STRINGER-THICKNESS:
// норматив подсказывает толщину и размеры заготовки, но не отменяет уже
// посчитанные изделия. Если тест начнёт требовать блокировку, это отдельное
// решение владельца, а не побочный эффект правки.
func TestTimberStringerNormWarning(t *testing.T) {
	s := NewService()
	cfgFor := func(height, width, thick float64) Config {
		cfg := referenceConfig()
		cfg.Material = "WOOD-OAK"
		cfg.TreadMaterial = "WOOD-OAK"
		cfg.Height = engineering.Length(height)
		cfg.Width = engineering.Length(width)
		cfg.StringerThickness = engineering.Length(thick)
		cfg.StepThickness = engineering.Length(40)
		return cfg
	}
	findIssue := func(res *Result) *validation.Issue {
		for i := range res.Validation.Issues {
			if res.Validation.Issues[i].Code == constraint.GEO_STRINGER_THICKNESS &&
				res.Validation.Issues[i].ID == "timber_stringer_thickness" {
				return &res.Validation.Issues[i]
			}
		}
		return nil
	}

	// H=2700 → проекция 4050 мм → полоса «более 4.0 м» → нужно 80 мм.
	res, err := s.Calculate(context.Background(), cfgFor(2700, 900, 50), Options{})
	if err != nil {
		t.Fatal(err)
	}
	iss := findIssue(res)
	if iss == nil {
		t.Fatal("косоур 50 мм при проекции 4050 мм должен давать предупреждение норматива")
	}
	if iss.Severity != constraint.SeverityWarning {
		t.Errorf("severity = %v, ожидался %v (норматив не блокирует расчёт)",
			iss.Severity, constraint.SeverityWarning)
	}
	if res.Validation.Blocking {
		t.Error("предупреждение норматива не должно блокировать расчёт")
	}
	if !strings.Contains(iss.Message, "80") {
		t.Errorf("сообщение должно называть требуемые 80 мм: %q", iss.Message)
	}
	// Подсказка несёт заготовку и глубину тела — это и есть «информация в
	// калькуляторе», ради которой правило вносилось.
	if !strings.Contains(iss.Guide, "350") {
		t.Errorf("подсказка должна называть заготовку 350 мм: %q", iss.Guide)
	}
	if !strings.Contains(iss.Guide, "опора") {
		t.Errorf("подсказка должна упоминать промежуточную опору для марша > 4 м: %q", iss.Guide)
	}

	// Ширина более 1200 мм — 100 мм в любой полосе.
	res, err = s.Calculate(context.Background(), cfgFor(1800, 1300, 50), Options{})
	if err != nil {
		t.Fatal(err)
	}
	iss = findIssue(res)
	if iss == nil || !strings.Contains(iss.Message, "100") {
		t.Fatalf("ширина 1300 мм должна требовать 100 мм, получили %+v", iss)
	}
	if !strings.Contains(iss.Guide, "Ширина марша") {
		t.Errorf("подсказка должна объяснить, что толщина взята по ширине: %q", iss.Guide)
	}

	// Достаточная толщина — предупреждения нет.
	res, err = s.Calculate(context.Background(), cfgFor(2700, 900, 80), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if iss := findIssue(res); iss != nil {
		t.Errorf("косоур 80 мм при проекции 4050 мм соответствует нормативу, предупреждение лишнее: %q", iss.Message)
	}
	// Короткий марш: проекция 2700 мм → достаточно 50 мм.
	res, err = s.Calculate(context.Background(), cfgFor(1800, 900, 50), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if iss := findIssue(res); iss != nil {
		t.Errorf("проекция 2700 мм и косоур 50 мм соответствуют норме: %q", iss.Message)
	}
}

// Правило про БАЛКУ, поэтому листовой металлокаркас его не касается: стальной
// косоур 8 мм на марше 5 м — законная конструкция, а не нарушение.
func TestTimberStringerNormIgnoresMetalFrame(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = "STEEL-C245"
	cfg.TreadMaterial = "WOOD-OAK"
	cfg.Height = engineering.Length(4500)
	cfg.Width = engineering.Length(900)
	cfg.StringerThickness = engineering.Length(8)
	cfg.StepThickness = engineering.Length(40)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range res.Validation.Issues {
		if is.ID == "timber_stringer_thickness" {
			t.Fatalf("стальной каркас не должен попадать под норматив на балку: %q", is.Message)
		}
	}
}

// Толщина металлической ступени НЕ выбирается: всегда 5 мм (решение владельца
// 2026-10-05). У стали в каталоге одна ступень, поэтому клиент может прислать
// что угодно — и это «что угодно» обязано схлопнуться в 5 мм ДО проверки
// толщин, геометрии и раскроя.
func TestMetalTreadThicknessIsAlwaysFive(t *testing.T) {
	s := NewService()
	metal := func(step float64) Config {
		cfg := referenceConfig()
		cfg.Material = "STEEL-C245"
		cfg.TreadMaterial = "STEEL-C245"
		cfg.StringerThickness = engineering.Length(8)
		cfg.StepThickness = engineering.Length(step)
		return cfg
	}
	// Прежняя форма предлагала 3–8 мм, старые проекты хранят 6 мм, а ручной
	// импорт может принести что угодно — включает заведомо недопустимое.
	for _, in := range []float64{0, 3, 4, 5, 6, 8, 40, 120} {
		res, err := s.Calculate(context.Background(), metal(in), Options{})
		if err != nil {
			t.Fatalf("ступень %.0f мм: %v", in, err)
		}
		if res.Validation.Blocking {
			t.Errorf("ступень %.0f мм не должна блокировать: %+v",
				in, res.Validation.Issues)
		}
		if got := res.StepThickness.Millimeters(); got != 5 {
			t.Errorf("ступень %.0f мм приведена к %.0f мм, ожидалось 5", in, got)
		}
	}
}

// Подступенок из того же листа идёт толщиной ступени: у стальной лестницы
// не должно быть двух разных толщин листа из одного проката.
func TestMetalRiserFollowsFixedTreadThickness(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = "STEEL-C245"
	cfg.TreadMaterial = "STEEL-C245"
	cfg.StringerThickness = engineering.Length(8)
	cfg.StepThickness = engineering.Length(8) // было в форме до решения
	cfg.RiserThickness = engineering.Length(8)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Validation.Blocking {
		t.Fatalf("расчёт заблокирован: %+v", res.Validation.Issues)
	}
	// Подступенок по умолчанию наследует толщину ступени → тоже 5 мм.
	for _, p := range res.Package.Parts {
		if p.Kind == dommfg.PartRiser && p.Thickness.Millimeters() != 5 {
			t.Fatalf("подступенок %.1f мм, ожидалось 5 (толщина ступени)",
				p.Thickness.Millimeters())
		}
	}
}

// У ДЕРЕВА толщина ступени по-прежнему выбирается: правило про металл и не
// должно затронуть пиломатериал.
func TestWoodTreadThicknessStillUserChosen(t *testing.T) {
	s := NewService()
	for _, in := range []float64{40, 60, 80} {
		cfg := referenceConfig()
		cfg.Material = "STEEL-C245"
		cfg.TreadMaterial = "WOOD-OAK"
		cfg.StringerThickness = engineering.Length(8)
		cfg.StepThickness = engineering.Length(in)
		res, err := s.Calculate(context.Background(), cfg, Options{})
		if err != nil {
			t.Fatalf("дуб %.0f мм: %v", in, err)
		}
		if res.Validation.Blocking {
			t.Errorf("дуб %.0f мм не должен блокировать: %+v", in, res.Validation.Issues)
		}
		if got := res.StepThickness.Millimeters(); got != in {
			t.Errorf("дуб: толщина ступени %.0f мм, ожидалось %.0f (выбор сохраняется)", got, in)
		}
	}
}
