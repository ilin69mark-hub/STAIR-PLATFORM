package validation

import (
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
)

// tolerances_test.go — S-241 п. 3: допуск проверяется по КАЖДОМУ правилу и
// с обеих сторон.
//
// Что было. TestValidateBoundaryWithinTolerance берёт высоту ступени 200.05 и
// требует, чтобы она прошла. Допуск живой и защищён — но только для одного
// правила из семи и только со стороны верхней границы. Остальные шесть
// проходили без защиты: сдвинь допуск у ограждения с 1 до 10, и 899 мм
// (миллиметр ниже брака по ГОСТ 25772) прошёл бы молча.
//
// Допуск здесь — не округление «на всякий случай». Он означает, что значение,
// отличающееся от границы меньше чем на него, считается равным границе:
// метры считаются с конечной точностью, и требовать 899.9999 мм высоты
// вместо 900 — значит наказывать за чужую арифметику.

// ruleCase — правило с его границами, собственным допуском и материалом.
//
// tolerance — ИМЕННО СВОЙ допуск правила, и в этом весь смысл таблицы.
// Допуски разные и осмысленные: у угла 0.01° (градус дробный, округление
// 0.1° означало бы четверть процента уклона), у просвета и ограждения 1 мм
// (целые сантиметры в чертеже), у остальных 0.1 мм. Общий допуск на тест
// проверял бы не то, что написано в коде.
type ruleCase struct {
	name      string
	code      constraint.RuleCode
	min, max  float64
	hasMin    bool
	hasMax    bool
	tolerance float64
	// material — материал заказа: правила производства зависят от него,
	// потому что класс берётся из кода материала и толщины косоура.
	material string
}

func toleranceCases() []ruleCase {
	return []ruleCase{
		{
			name: "высота ступени", code: constraint.GEO_STEP_HEIGHT,
			min: 150, max: 200, hasMin: true, hasMax: true,
			tolerance: 0.1, material: "WOOD-OAK",
		},
		{
			name: "проступь", code: constraint.GEO_TREAD_DEPTH,
			min: 260, max: 320, hasMin: true, hasMax: true,
			tolerance: 0.1, material: "WOOD-OAK",
		},
		{
			name: "угол марша", code: constraint.GEO_ANGLE,
			min: 30, max: 45, hasMin: true, hasMax: true,
			tolerance: 0.01, material: "WOOD-OAK",
		},
		{
			// Просвет: одна граница, снизу. Проверяются и равенство, и шаг
			// за ней, и шаг ВНУТРЬ допуска.
			name: "просвет", code: constraint.GEO_CLEARANCE,
			min: 2000, hasMin: true,
			tolerance: 1, material: "WOOD-OAK",
		},
		{
			name: "толщина косоура из пиломатериала", code: constraint.GEO_STRINGER_THICKNESS,
			min: 30, hasMin: true,
			tolerance: 0.1, material: "WOOD-OAK",
		},
		{
			// Листовой металл: своё правило и свой диапазон (MFG-0005, 3–12).
			name: "толщина листа металлокаркаса", code: constraint.GEO_STRINGER_THICKNESS_SHEET,
			min: 3, max: 12, hasMin: true, hasMax: true,
			tolerance: 0.1, material: "STEEL-C245",
		},
		{
			name: "высота ограждения", code: constraint.SAF_RAILING_HEIGHT,
			min: 900, hasMin: true,
			tolerance: 1, material: "WOOD-OAK",
		},
	}
}

// TestEveryRuleToleranceIsRespectedOnBothSides — каждое правило из семи
// узнаёт значение «на границе ± свой допуск» и честно отвергает «за допуском».
func TestEveryRuleToleranceIsRespectedOnBothSides(t *testing.T) {
	for _, c := range toleranceCases() {
		t.Run(c.name, func(t *testing.T) {
			type probe struct {
				label  string
				value  float64
				issues int
			}
			var probes []probe
			// Шаг ЗА допуском берётся с запасом: 0.001 при допуске 0.01 —
			// это десятая часть допуска, и на границе из-за двоичной
			// точности такое значение ещё может сравниться как равное.
			beyond := c.tolerance / 10
			if beyond == 0 {
				beyond = 0.0001
			}

			if c.hasMin {
				probes = append(probes,
					probe{"ровно граница", c.min, 0},
					// Внутри допуска: требовать 199.9 вместо 200 значит
					// наказывать за чужую арифметику.
					probe{"внутри допуска снизу", c.min - c.tolerance, 0},
					// За допуском — настоящее нарушение, должно быть поймано.
					probe{"за допуском снизу", c.min - c.tolerance - beyond, 1},
				)
			}
			if c.hasMax {
				probes = append(probes,
					probe{"ровно граница", c.max, 0},
					probe{"внутри допуска сверху", c.max + c.tolerance, 0},
					probe{"за допуском сверху", c.max + c.tolerance + beyond, 1},
				)
			}
			if !c.hasMin && !c.hasMax {
				t.Fatalf("у правила %q нет ни одной границы — проверять нечего", c.code)
			}

			for _, p := range probes {
				got := issuesForValue(t, c.code, p.value, c.material)
				if got != p.issues {
					t.Errorf("%s = %v: замечаний %d, ждали %d (%s)",
						c.code, p.value, got, p.issues, p.label)
				}
			}
		})
	}
}

// TestEveryStandardRuleIsCoveredByToleranceTest — правил ровно столько же,
// сколько строк в таблице: забытое правило осталось бы без защиты молча.
func TestEveryStandardRuleIsCoveredByToleranceTest(t *testing.T) {
	set := constraint.StandardProfile("standard")
	covered := map[constraint.RuleCode]bool{}
	for _, c := range toleranceCases() {
		covered[c.code] = true
	}
	for code := range set.Constraints {
		if !covered[code] {
			t.Errorf("правило %q есть в STANDARD, но не проверяется допуск: "+
				"правка его Tolerance пройдёт молча", code)
		}
	}
	if len(covered) != len(set.Constraints) {
		t.Errorf("в таблице %d правил, в STANDARD %d", len(covered), len(set.Constraints))
	}
}

// issuesForValue — сколько замечаний даёт правило code на значении value.
//
// Конфигурация берётся заведомо годной (validConfig), и меняется только
// проверяемый параметр: иначе замечаний пришло бы больше, чем проверяем,
// и тест считал бы чужую ошибку.
func issuesForValue(t *testing.T, code constraint.RuleCode, value float64, material string) int {
	t.Helper()
	cfg := validConfig(t)
	cfg.Material = material
	applyRuleValue(t, cfg, code, value)

	res := Validate(cfg, constraint.StandardProfile("standard"))
	n := 0
	for _, i := range res.Issues {
		if i.Code == code {
			n++
		}
	}
	return n
}

// applyRuleValue — подставить значение именно проверяемого параметра.
func applyRuleValue(t *testing.T, cfg *engineering.StairConfiguration, code constraint.RuleCode, v float64) {
	t.Helper()
	//nolint:exhaustive // сюда попадают только правила ПРОФИЛЯ, которых
	// семь; остальные коды RuleCode — это ошибки ввода (GEO_HEIGHT,
	// GEO_WIDTH, MFG_MATERIAL и прочие), они в bindings() не участвуют и
	// физически сюда не доходят. Перечислять их ради линтера значило бы
	// превратить девять нерелевантных строк в шум поверх семи нужных.
	// Пропуск правила ловит TestEveryStandardRuleIsCoveredByToleranceTest.
	switch code {
	case constraint.GEO_STEP_HEIGHT:
		cfg.StepHeight = mustLength(t, v)
	case constraint.GEO_TREAD_DEPTH:
		cfg.TreadDepth = mustLength(t, v)
	case constraint.GEO_ANGLE:
		cfg.Angle = mustAngleDeg(t, v)
	case constraint.GEO_CLEARANCE:
		cfg.Clearance = mustLength(t, v)
	case constraint.GEO_STRINGER_THICKNESS, constraint.GEO_STRINGER_THICKNESS_SHEET:
		cfg.StringerThickness = mustLength(t, v)
	case constraint.SAF_RAILING_HEIGHT:
		cfg.RailingHeight = mustLength(t, v)
	default:
		// Правил много, а проверяем мы семь. Ветка БЕЗ default была бы
		// пропуском: новое правило в STANDARD тихо не попало бы в таблицу
		// и осталось без защиты допуска. С default — нет: сообщение
		// превратилось бы в «добавьте правило в switch» для каждого
		// MFG_/GEO_-кода ошибки ввода, которых здесь не бывает.
		t.Fatalf("в toleranceCases нет правила %q: если оно добавлено в "+
			"STANDARD, его допуск останется незащищённым", code)
	}
}
