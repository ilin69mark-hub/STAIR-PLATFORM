package solver

import (
	"fmt"

	"stairplatform/internal/engine/constraint"
)

// InputError — ошибка пользовательского ввода на этапе решения: марш не
// может быть рассчитан, но проблему можно исправить изменением параметров.
// Прикладной слой превращает её в блокирующий результат валидации (HTTP 200)
// с русским объяснением (Guide) и указанием, какое поле и как поправить
// (Param/Fix), вместо технической англоязычной ошибки.
type InputError struct {
	Code    constraint.RuleCode
	Field   string  // Param: человекочитаемое имя поля (русское).
	Value   float64 // нарушенное значение (мм или безразмерное).
	Min     float64 // нижняя граница допустимого значения (если HasMin).
	Max     float64 // верхняя граница допустимого значения (если HasMax).
	HasMin  bool
	HasMax  bool
	Message string // короткий русский текст ошибки.
	Guide   string // развёрнутое русское объяснение с цифрами.
	Fix     string // короткая подсказка, что исправить.
}

// Error реализует error; текст уже пользовательский (русский).
func (e *InputError) Error() string { return e.Message }

// inputError конструирует InputError с обязательными полями.
func inputError(code constraint.RuleCode, field, message, guide, fix string) *InputError {
	return &InputError{Code: code, Field: field, Message: message, Guide: guide, Fix: fix}
}

// rangeError конструирует InputError для значения вне диапазона [min, max].
func rangeError(code constraint.RuleCode, field string, value, min, max float64, message, guide, fix string) *InputError {
	return &InputError{
		Code: code, Field: field, Value: value,
		Min: min, Max: max, HasMin: true, HasMax: true,
		Message: message, Guide: guide, Fix: fix,
	}
}

// minError конструирует InputError для значения ниже нижней границы min.
func minError(code constraint.RuleCode, field string, value, min float64, message, guide, fix string) *InputError {
	return &InputError{
		Code: code, Field: field, Value: value,
		Min: min, HasMin: true,
		Message: message, Guide: guide, Fix: fix,
	}
}

// riseInputError — высота подъёма не положительна.
func riseInputError() *InputError {
	return inputError(
		constraint.GEO_HEIGHT, "Высота",
		"Высота подъёма должна быть положительной",
		"Укажите высоту подъёма больше 0 мм.",
		"Задайте высоту подъёма",
	)
}

// riserInputError — целевая высота ступени не положительна.
func riserInputError() *InputError {
	return inputError(
		constraint.GEO_HEIGHT, "Высота ступени",
		"Высота ступени должна быть положительной",
		"Укажите целевую высоту ступени больше 0 мм.",
		"Задайте высоту ступени",
	)
}

// widthInputError — ширина марша не положительна.
func widthInputError() *InputError {
	return inputError(
		constraint.GEO_WIDTH, "Ширина марша",
		"Ширина марша должна быть положительной",
		"Укажите ширину марша больше 0 мм.",
		"Задайте ширину марша",
	)
}

// noFlightInputError — подъём не даёт ни одной ступени.
func noFlightInputError(hm float64) *InputError {
	return inputError(
		constraint.GEO_HEIGHT, "Высота",
		"Лестница не содержит ступеней",
		fmt.Sprintf("При высоте подъёма %.0f мм получается меньше одной ступени. Увеличьте высоту подъёма или уменьшите высоту ступени.", hm),
		"Увеличьте высоту подъёма или уменьшите высоту ступени",
	)
}

// comfortInputError — шаг комфорта вне диапазона 600–640 мм.
func comfortInputError(step float64) *InputError {
	return rangeError(
		constraint.GEO_COMFORT_STEP, "Шаг комфорта",
		step, ComfortStepMin, ComfortStepMax,
		"Шаг комфорта вне диапазона 600–640 мм",
		fmt.Sprintf("Шаг комфорта %.0f мм вне допустимого диапазона 600–640 мм (норма: 2×высота ступени + проступь). Подберите значение в пределах нормы.", step),
		"Задайте шаг комфорта 600–640 мм",
	)
}

// treadPositiveError — проступь получается неположительной.
func treadPositiveError(b float64) *InputError {
	return inputError(
		constraint.GEO_TREAD_POSITIVE, "Шаг комфорта",
		"Проступь получается неположительной",
		fmt.Sprintf("При текущих параметрах проступь (шаг комфорта − 2×высота ступени) равна %.1f мм. Уменьшите шаг комфорта или высоту ступени.", b),
		"Уменьшите шаг комфорта или высоту ступени",
	)
}

// landingPositiveError — ширина площадки не положительна.
func landingPositiveError() *InputError {
	return inputError(
		constraint.GEO_WIDTH, "Ширина площадки",
		"Ширина площадки должна быть положительной",
		"Укажите ширину площадки больше 0 мм.",
		"Задайте ширину площадки",
	)
}

// lowerStepInputError — число ступеней нижнего марша вне диапазона.
func lowerStepInputError(n1, n int) *InputError {
	max := float64(n - 1)
	return rangeError(
		constraint.GEO_LOWER_STEP, "Нижних ступеней",
		float64(n1), 1, max,
		"Число ступеней нижнего марша вне диапазона",
		fmt.Sprintf("Число ступеней нижнего марша должно быть от 1 до %d (всего ступеней %d).", n-1, n),
		"Задайте число ступеней нижнего марша",
	)
}

// landingNarrowError — площадка уже ширины марша (инвариант Wp ≥ W).
func landingNarrowError(wp, width float64) *InputError {
	return minError(
		constraint.GEO_LANDING_WIDTH, "Ширина площадки",
		wp, width,
		"Ширина площадки меньше ширины марша",
		fmt.Sprintf("Ширина площадки %.0f мм меньше ширины марша %.0f мм. Площадка должна быть не уже марша.", wp, width),
		fmt.Sprintf("Увеличьте ширину площадки минимум до %.0f мм", width),
	)
}

// radiusNarrowError — наружный радиус спирали не превышает ширину марша.
func radiusNarrowError(rm, wm float64) *InputError {
	return minError(
		constraint.GEO_SPIRAL_RADIUS, "Радиус спирали",
		rm, wm,
		"Радиус спирали не превышает ширину марша",
		fmt.Sprintf("Наружный радиус спирали %.0f мм должен быть больше ширины марша %.0f мм (радиус колонны должен оставаться положительным).", rm, wm),
		fmt.Sprintf("Увеличьте радиус спирали минимум до %.0f мм", wm+1),
	)
}

// spiralTreadInputError — проступи спирали вне допустимых пределов.
func spiralTreadInputError(kind string, value, min, max float64) *InputError {
	var message, guide, fix string
	switch kind {
	case "inner":
		message = "Проступь у колонны меньше 100 мм"
		guide = fmt.Sprintf("Проступь у колонны %.0f мм, нужно не менее %.0f мм. Уменьшите радиус спирали или ширину марша.", value, min)
		fix = "Уменьшите радиус спирали или ширину марша"
	case "outer":
		message = "Проступь у наружной кромки меньше 250 мм"
		guide = fmt.Sprintf("Проступь у наружной кромки %.0f мм, нужно не менее %.0f мм. Увеличьте радиус спирали или число ступеней.", value, min)
		fix = "Увеличьте радиус спирали или число ступеней"
	default:
		message = "Проступь по линии хода вне диапазона 260–320 мм"
		guide = fmt.Sprintf("Проступь по линии хода %.0f мм вне допустимого диапазона 260–320 мм. Измените радиус спирали или число ступеней.", value)
		fix = "Измените радиус спирали или число ступеней"
	}
	if kind == "walk" {
		return rangeError(
			constraint.GEO_SPIRAL_TREAD, "Радиус спирали",
			value, min, max,
			message, guide, fix,
		)
	}
	return minError(
		constraint.GEO_SPIRAL_TREAD, "Радиус спирали",
		value, min,
		message, guide, fix,
	)
}
