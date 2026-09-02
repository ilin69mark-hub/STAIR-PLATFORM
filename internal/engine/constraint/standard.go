package constraint

import "fmt"

// Стандартные коды ограничений (EDR-0002).
const (
	GEO_STEP_HEIGHT        RuleCode = "GEO-STEP-HEIGHT"
	GEO_TREAD_DEPTH        RuleCode = "GEO-TREAD-DEPTH"
	GEO_ANGLE              RuleCode = "GEO-ANGLE"
	GEO_CLEARANCE          RuleCode = "GEO-CLEARANCE"
	GEO_STRINGER_THICKNESS RuleCode = "GEO-STRINGER-THICKNESS"
	SAF_RAILING_HEIGHT     RuleCode = "SAF-RAILING-HEIGHT"
	MFG_SHEET              RuleCode = "MFG-SHEET"
)

// Коды входных ошибок решателя (пользовательский ввод не позволяет
// рассчитать марш; прикладной слой превращает их в блокирующий результат
// с русской подсказкой вместо технической ошибки).
const (
	GEO_HEIGHT          RuleCode = "GEO-HEIGHT"          // высота/подъём не дают участка
	GEO_WIDTH           RuleCode = "GEO-WIDTH"           // ширина марша/площадки не положительна
	GEO_COMFORT_STEP    RuleCode = "GEO-COMFORT-STEP"    // шаг комфорта вне 600–640 мм
	GEO_LANDING_WIDTH   RuleCode = "GEO-LANDING-WIDTH"   // площадка уже ширины марша
	GEO_LOWER_STEP      RuleCode = "GEO-LOWER-STEP"      // число ступеней нижнего марша вне диапазона
	GEO_SPIRAL_RADIUS   RuleCode = "GEO-SPIRAL-RADIUS"   // радиус спирали не превышает ширину марша
	GEO_SPIRAL_TREAD    RuleCode = "GEO-SPIRAL-TREAD"    // проступи спирали вне допустимых пределов
	GEO_TREAD_POSITIVE  RuleCode = "GEO-TREAD-POSITIVE"  // проступь неположительна
	GEO_STRINGER_NARROW RuleCode = "GEO-STRINGER-NARROW" // марш уже двух толщин косоура
	MFG_MATERIAL        RuleCode = "MFG-MATERIAL"        // материал не поддерживает толщины деталей
)

// StandardProfile создаёт нормативный профиль STANDARD (EDR-0002,
// параметры в соответствующих по смыслу единицах; см. EDR по правилам).
func StandardProfile(name string) *ConstraintSet {
	set := NewSet("cs-standard", name)

	mustAdd(set, &Constraint{
		Code:     GEO_STEP_HEIGHT,
		Category: "geometry",
		Severity: SeverityError,
		Range:    Range{Min: 150, Max: 200, HasMin: true, HasMax: true, Tolerance: 0.1},
		Version:  1,
		Active:   true,
		Message:  "высота ступени должна быть в диапазоне 150-200 мм",
		Fix:      "скорректируйте число ступеней или общую высоту подъёма",
		Advice: &ConstraintAdvice{
			Param:   "Высота ступени",
			Guide:   "Высота ступени {value} мм вне нормы ({min}–{max} мм). Измените число ступеней — варианты ниже проходят все нормы и подходят к вашей высоте {height} мм.",
			Suggest: true,
		},
	})
	mustAdd(set, &Constraint{
		Code:     GEO_TREAD_DEPTH,
		Category: "geometry",
		Severity: SeverityError,
		Range:    Range{Min: 260, Max: 320, HasMin: true, HasMax: true, Tolerance: 0.1},
		Version:  1,
		Active:   true,
		Message:  "проступь должна быть в диапазоне 260-320 мм",
		Fix:      "скорректируйте проступь или угол наклона марша",
		Advice: &ConstraintAdvice{
			Param:   "Проступь",
			Guide:   "Проступь {value} мм вне нормы ({min}–{max} мм). Измените число ступеней (или шаг комфорта) — варианты ниже проходят все нормы.",
			Suggest: true,
		},
	})
	mustAdd(set, &Constraint{
		Code:     GEO_ANGLE,
		Category: "geometry",
		Severity: SeverityError,
		Range:    Range{Min: 30, Max: 45, HasMin: true, HasMax: true, Tolerance: 0.01},
		Version:  1,
		Active:   true,
		Message:  "угол наклона марша должен быть в диапазоне 30-45 градусов",
		Fix:      "скорректируйте геометрию марша",
		Advice: &ConstraintAdvice{
			Param:   "Число ступеней",
			Guide:   "Угол наклона {angle}° вне нормы ({min}–{max}°). Измените число ступеней — варианты ниже дают угол в норме при вашей высоте {height} мм.",
			Suggest: true,
		},
	})
	mustAdd(set, &Constraint{
		Code:     GEO_CLEARANCE,
		Category: "geometry",
		Severity: SeverityWarning,
		Range:    Range{Min: 2000, HasMin: true, Tolerance: 1},
		Version:  1,
		Active:   true,
		Message:  "вертикальный просвет должен быть не менее 2000 мм",
		Fix:      "увеличьте высоту помещения или измените разбивку марша",
		Advice: &ConstraintAdvice{
			Param:   "Просвет",
			Guide:   "Просвет {value} мм, нужно не менее {min} мм. Увеличьте поле «Просвет» в конструкторе или измените разбивку марша.",
			Suggest: false,
		},
	})
	mustAdd(set, &Constraint{
		Code:     GEO_STRINGER_THICKNESS,
		Category: "geometry",
		Severity: SeverityWarning,
		Range:    Range{Min: 30, HasMin: true, Tolerance: 0.1},
		Version:  1,
		Active:   true,
		Message:  "толщина косоура должна быть не менее 30 мм",
		Fix:      "увеличьте толщину косоура",
		Advice: &ConstraintAdvice{
			Param:   "Толщина косоура",
			Guide:   "Толщина косоура {value} мм, нужно не менее {min} мм. Увеличьте поле «Толщина косоура».",
			Suggest: false,
		},
	})
	mustAdd(set, &Constraint{
		Code:     SAF_RAILING_HEIGHT,
		Category: "safety",
		Severity: SeverityWarning,
		Range:    Range{Min: 900, HasMin: true, Tolerance: 1},
		Version:  1,
		Active:   true,
		Message:  "высота ограждения должна быть не менее 900 мм",
		Fix:      "увеличьте высоту ограждения",
		Advice: &ConstraintAdvice{
			Param:   "Высота перил",
			Guide:   "Высота ограждения {value} мм, нужно не менее {min} мм. Увеличьте поле «Высота перил».",
			Suggest: false,
		},
	})
	// Спираль (EDR-0007): инварианты проступей/радиуса проверяются в
	// solveSpiral как входные ошибки (GEO_SPIRAL_*), поэтому правила здесь
	// нужны советнику — чтобы по blocking-issue прикрепить готовые варианты.
	mustAdd(set, &Constraint{
		Code:     GEO_SPIRAL_TREAD,
		Category: "geometry",
		Severity: SeverityError,
		Range:    Range{Min: 100, HasMin: true, Tolerance: 1},
		Version:  1,
		Active:   true,
		Message:  "проступи спирали должны быть в допустимых пределах",
		Fix:      "измените радиус спирали или ширину марша",
		Advice: &ConstraintAdvice{
			Param:   "Радиус спирали",
			Guide:   "Проступь {value} мм вне нормы (нужно ≥ {min} мм). Уменьшите радиус спирали или ширину марша — варианты ниже проходят все нормы.",
			Suggest: true,
		},
	})
	mustAdd(set, &Constraint{
		Code:     GEO_SPIRAL_RADIUS,
		Category: "geometry",
		Severity: SeverityError,
		Range:    Range{Min: 1, HasMin: true, Tolerance: 1},
		Version:  1,
		Active:   true,
		Message:  "наружный радиус спирали должен быть больше ширины марша (иначе колонна имеет нулевой или отрицательный радиус)",
		Fix:      "увеличьте радиус спирали или уменьшите ширину марша",
		Advice: &ConstraintAdvice{
			Param:   "Радиус спирали",
			Guide:   "Наружный радиус спирали {value} мм должен быть больше ширины марша {min} мм. Колонна в центре спирали имеет радиус = радиус − ширина марша; при радиусе ≤ ширине марша колонна исчезает или получает отрицательный размер. Увеличьте радиус или уменьшите ширину марша.",
			Suggest: true,
		},
	})
	return set
}

// mustAdd добавляет нормативное правило; ошибка невозможна для
// корректных кодовых констант, поэтому паникуем при инвариантном сбое.
func mustAdd(set *ConstraintSet, c *Constraint) {
	if err := set.Add(c); err != nil {
		panic(fmt.Sprintf("constraint: standard profile: %v", err))
	}
}

// ValidateIntegrity проверяет инварианты BC-003:
// - уникальные коды правил;
// - уникальные версии в рамках кода;
// - одна активная версия правила на код;
// - непересекающиеся диапазоны версий одного правила.
func ValidateIntegrity(set *ConstraintSet) []string {
	var violations []string
	for code, versions := range set.Constraints {
		seenVersions := make(map[int]bool)
		activeCount := 0
		for i := 0; i < len(versions); i++ {
			if seenVersions[versions[i].Version] {
				violations = append(violations,
					fmt.Sprintf("%s: duplicate version %d", code, versions[i].Version))
			}
			seenVersions[versions[i].Version] = true
			if versions[i].Active {
				activeCount++
			}
			for j := i + 1; j < len(versions); j++ {
				if rangesOverlap(versions[i].Range, versions[j].Range) {
					violations = append(violations,
						fmt.Sprintf("%s: overlapping ranges between versions %d and %d",
							code, versions[i].Version, versions[j].Version))
				}
			}
		}
		if activeCount > 1 {
			violations = append(violations, fmt.Sprintf("%s: multiple active versions (%d)", code, activeCount))
		}
	}
	return violations
}

// rangesOverlap проверяет пересечение двух диапазонов.
func rangesOverlap(a, b Range) bool {
	if a.HasMax && b.HasMin && a.Max < b.Min {
		return false
	}
	if b.HasMax && a.HasMin && b.Max < a.Min {
		return false
	}
	return true
}
