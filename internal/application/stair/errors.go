package stair

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/constraint"
	engmfg "stairplatform/internal/engine/manufacturing"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

// inputIssue превращает ошибку пользовательского ввода (*solver.InputError и
// известные входные ошибки движков) в блокирующий результат валидации с
// русским объяснением (Guide) и указанием, какое поле и как поправить
// (Param/Fix). Возвращает ok=false, если ошибка не является входной — её
// нужно вернуть вызывающему как есть (внутренний сбой).
func inputIssue(err error) (validation.Result, bool) {
	var inp *solver.InputError
	if errors.As(err, &inp) {
		return buildInputResult(inp), true
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no material supports thickness"):
		mm := materialThicknessFromError(msg)
		return buildInputResult(&solver.InputError{
			Code:    constraint.GEO_WIDTH,
			Field:   "Толщина косоура",
			Value:   mm,
			Min:     2,
			Max:     60,
			HasMin:  true,
			HasMax:  true,
			Message: "Толщина косоура не поддерживается каталогом",
			Guide:   fmt.Sprintf("Толщина косоура %.0f мм не поддерживается каталогом материалов (2–60 мм).", mm),
			Fix:     "Задайте толщину косоура в диапазоне 2–60 мм",
		}), true
	case strings.Contains(msg, "lower step count must be in"):
		return buildInputResult(lowerStepInputErrorFromMessage(msg)), true
	}
	return validation.Result{}, false
}

// buildInputResult собирает блокирующий результат валидации из InputError.
func buildInputResult(inp *solver.InputError) validation.Result {
	return validation.Result{
		Valid:    false,
		Blocking: true,
		Issues: []validation.Issue{{
			ID:       "ISSUE-INPUT",
			Code:     inp.Code,
			Severity: constraint.SeverityError,
			Element:  "configuration",
			Message:  inp.Message,
			Param:    inp.Field,
			Guide:    inp.Guide,
			Fix:      inp.Fix,
			Value:    inp.Value,
			Min:      inp.Min,
			Max:      inp.Max,
			HasMin:   inp.HasMin,
			HasMax:   inp.HasMax,
		}},
	}
}

// materialThicknessFromError извлекает толщину из сообщения вида
// "manufacturing: no material supports thickness 70 mm".
func materialThicknessFromError(msg string) float64 {
	i := strings.Index(msg, "thickness ")
	if i < 0 {
		return 0
	}
	rest := msg[i+len("thickness "):]
	j := strings.Index(rest, " mm")
	if j < 0 {
		j = len(rest)
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(rest[:j]), 64)
	if err != nil {
		return 0
	}
	return v
}

// configInputError переводит ошибки конфигурации (энелоп каталога и
// StairConfiguration.Validate) в понятную пользователю входную ошибку.
func configInputError(err error) *solver.InputError {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "supported maximum 6000"):
		return &solver.InputError{
			Code:    constraint.GEO_HEIGHT,
			Field:   "Высота",
			Message: "Высота подъёма превышает поддерживаемый максимум",
			Guide:   "Максимальная высота подъёма — 6000 мм (ограничение стандартных листов). Уменьшите высоту до 6000 мм или менее.",
			Fix:     "Уменьшите высоту подъёма до 6000 мм или менее",
		}
	case strings.Contains(msg, "supported maximum 5000"):
		return &solver.InputError{
			Code:    constraint.GEO_SPIRAL_RADIUS,
			Field:   "Радиус спирали",
			Message: "Радиус спирали превышает поддерживаемый максимум",
			Guide:   "Максимальный наружный радиус спирали — 5000 мм (ограничение стандартных листов). Уменьшите радиус до 5000 мм или менее.",
			Fix:     "Уменьшите радиус спирали до 5000 мм или менее",
		}
	case strings.Contains(msg, "width must be positive"):
		return &solver.InputError{
			Code: constraint.GEO_WIDTH, Field: "Ширина марша",
			Message: "Ширина марша должна быть положительной",
			Guide:   "Укажите ширину марша больше 0 мм.",
			Fix:     "Задайте ширину марша",
		}
	case strings.Contains(msg, "height must be positive"):
		return &solver.InputError{
			Code: constraint.GEO_HEIGHT, Field: "Высота",
			Message: "Высота подъёма должна быть положительной",
			Guide:   "Укажите высоту подъёма больше 0 мм.",
			Fix:     "Задайте высоту подъёма",
		}
	case strings.Contains(msg, "flight type is required"):
		return &solver.InputError{
			Code: constraint.GEO_HEIGHT, Field: "Тип лестницы",
			Message: "Не выбран тип лестницы",
			Guide:   "Выберите тип марша (прямой, L-образный, П-образный или спиральный).",
			Fix:     "Выберите тип марша",
		}
	case strings.Contains(msg, "stringer thickness must not be negative"):
		return &solver.InputError{
			Code: constraint.GEO_STRINGER_THICKNESS, Field: "Толщина косоура",
			Message: "Толщина косоура не может быть отрицательной",
			Guide:   "Толщина косоура должна быть неотрицательной.",
			Fix:     "Задайте толщину косоура",
		}
	case strings.Contains(msg, "step thickness must not be negative"):
		return &solver.InputError{
			Code: constraint.GEO_WIDTH, Field: "Толщина ступени",
			Message: "Толщина ступени не может быть отрицательной",
			Guide:   "Толщина ступени должна быть неотрицательной.",
			Fix:     "Задайте толщину ступени",
		}
	case strings.Contains(msg, "clearance must not be negative"):
		return &solver.InputError{
			Code: constraint.GEO_CLEARANCE, Field: "Просвет",
			Message: "Просвет не может быть отрицательным",
			Guide:   "Вертикальный просвет должен быть неотрицательным.",
			Fix:     "Задайте просвет",
		}
	case strings.Contains(msg, "railing height must not be negative"):
		return &solver.InputError{
			Code: constraint.SAF_RAILING_HEIGHT, Field: "Высота перил",
			Message: "Высота перил не может быть отрицательной",
			Guide:   "Высота перил должна быть неотрицательной.",
			Fix:     "Задайте высоту перил",
		}
	case strings.Contains(msg, "landing width must not be negative"):
		return &solver.InputError{
			Code: constraint.GEO_WIDTH, Field: "Ширина площадки",
			Message: "Ширина площадки не может быть отрицательной",
			Guide:   "Ширина площадки должна быть неотрицательной.",
			Fix:     "Задайте ширину площадки",
		}
	case strings.Contains(msg, "lower step count must not be negative"):
		return &solver.InputError{
			Code: constraint.GEO_LOWER_STEP, Field: "Нижних ступеней",
			Message: "Число ступеней нижнего марша не может быть отрицательным",
			Guide:   "Число ступеней нижнего марша должно быть неотрицательным.",
			Fix:     "Задайте число ступеней нижнего марша",
		}
	case strings.Contains(msg, "outer radius must exceed the stair width"):
		return &solver.InputError{
			Code: constraint.GEO_SPIRAL_RADIUS, Field: "Радиус спирали",
			Message: "Радиус спирали не превышает ширину марша",
			Guide:   "Наружный радиус спирали должен быть больше ширины марша (радиус колонны должен оставаться положительным).",
			Fix:     "Увеличьте радиус спирали",
		}
	case strings.Contains(msg, "landing width must be at least the flight width"):
		return &solver.InputError{
			Code: constraint.GEO_LANDING_WIDTH, Field: "Ширина площадки",
			Message: "Ширина площадки меньше ширины марша",
			Guide:   "Ширина площадки должна быть не меньше ширины марша.",
			Fix:     "Увеличьте ширину площадки",
		}
	case strings.Contains(msg, "lower step count must be in"):
		return lowerStepInputErrorFromMessage(msg)
	case strings.Contains(msg, "width must exceed two stringer thicknesses"):
		return &solver.InputError{
			Code: constraint.GEO_STRINGER_NARROW, Field: "Ширина марша",
			Message: "Ширина марша меньше двух толщин косоура",
			Guide:   "Ширина марша должна превышать две толщины косоура.",
			Fix:     "Увеличьте ширину марша",
		}
	case strings.Contains(msg, "not found in catalog"):
		mat := materialCodeFromError(msg)
		return &solver.InputError{
			Code: constraint.MFG_MATERIAL, Field: "Материал",
			Message: "Материал не найден в каталоге",
			Guide:   fmt.Sprintf("Материал “%s” отсутствует в каталоге материалов (сталь, алюминий, дуб). Выберите материал из списка.", mat),
			Fix:     "Выберите материал из списка",
		}
	case strings.Contains(msg, "does not support thickness"):
		mat := materialCodeFromError(msg)
		mm := materialThicknessFromError(msg)
		part := materialPartFromError(msg)
		minT, maxT := 20.0, 60.0
		if reg, err := engmfg.DefaultMaterialRegistry(); err == nil {
			if m, ok := reg.Find(dommfg.MaterialCode(mat)); ok {
				minT, maxT = m.MinThickness, m.MaxThickness
			}
		}
		return &solver.InputError{
			Code: constraint.MFG_MATERIAL, Field: "Материал",
			Value:   mm,
			Min:     minT,
			Max:     maxT,
			HasMin:  true,
			HasMax:  true,
			Message: "Материал не поддерживает заданную толщину",
			Guide:   fmt.Sprintf("Материал “%s” выпускается в толщинах %.0f–%.0f мм, а %s задан толщиной %.0f мм. Уменьшите толщину или выберите другой материал.", mat, minT, maxT, part, mm),
			Fix:     fmt.Sprintf("Задайте толщину %s в диапазоне %.0f–%.0f мм или смените материал", part, minT, maxT),
		}
	case strings.Contains(msg, "width ") && strings.Contains(msg, "exceeds maximum") && strings.Contains(msg, "for material"):
		mat := materialCodeFromError(msg)
		maxW := 3000.0
		if reg, err := engmfg.DefaultMaterialRegistry(); err == nil {
			if m, ok := reg.Find(dommfg.MaterialCode(mat)); ok {
				maxW = m.MaxWidthMm
			}
		}
		return &solver.InputError{
			Code: constraint.MFG_MATERIAL, Field: "Ширина марша",
			Value: maxW, Max: maxW, HasMax: true,
			Message: "Ширина марша превышает максимум для материала",
			Guide:   fmt.Sprintf("Для материала “%s” максимальная ширина марша — %.0f мм (ограничение стандартных листов MFG-0012). Уменьшите ширину или выберите другой материал.", mat, maxW),
			Fix:     fmt.Sprintf("Уменьшите ширину марша до %.0f мм или менее", maxW),
		}
	case strings.Contains(msg, "rise height") && strings.Contains(msg, "exceeds maximum") && strings.Contains(msg, "for material"):
		mat := materialCodeFromError(msg)
		maxH := 4550.0
		if reg, err := engmfg.DefaultMaterialRegistry(); err == nil {
			if m, ok := reg.Find(dommfg.MaterialCode(mat)); ok {
				maxH = m.MaxHeightMm
			}
		}
		return &solver.InputError{
			Code: constraint.MFG_MATERIAL, Field: "Высота",
			Value: maxH, Max: maxH, HasMax: true,
			Message: "Высота подъёма превышает максимум для материала",
			Guide:   fmt.Sprintf("Для материала “%s” максимальная высота подъёма — %.0f мм (ограничение стандартных листов MFG-0012). Уменьшите высоту или выберите другой материал.", mat, maxH),
			Fix:     fmt.Sprintf("Уменьшите высоту подъёма до %.0f мм или менее", maxH),
		}
	case strings.Contains(msg, "invalid railing sides"):
		return &solver.InputError{
			Code: constraint.SAF_RAILING_HEIGHT, Field: "Перила",
			Message: "Неверное значение стороны перил",
			Guide:   "Стороны перил: без, слева, справа или с двух сторон (см. от первой ступени по ходу подъёма: слева — левые, справа — правые).",
			Fix:     "Выберите сторону перил из списка",
		}
	case strings.Contains(msg, "invalid turn direction"):
		return &solver.InputError{
			Code: constraint.GEO_LANDING_WIDTH, Field: "Направление поворота",
			Message: "Неверное направление поворота площадки",
			Guide:   "Поворот площадки задаётся влево или вправо относительно хода подъёма.",
			Fix:     "Выберите направление поворота",
		}
	case strings.Contains(msg, "invalid spiral direction"):
		return &solver.InputError{
			Code: constraint.GEO_SPIRAL_RADIUS, Field: "Направление спирали",
			Message: "Неверное направление закрутки спирали",
			Guide:   "Направление спирали: по часовой (cw) или против часовой (ccw) стрелки при виде сверху.",
			Fix:     "Выберите направление спирали",
		}
	default:
		// API-001 (forensic 2026-09-24): любая ошибка engineering.Validate(),
		// не попавшая в таблицу выше, раньше доезжала до транспорта «прочей»
		// и отдавалась 500 internal_error. Теперь это обычная входная ошибка
		// с блокирующим результатом валидации (200 + validation.valid=false) —
		// тем же контрактом, что у ошибок solver'а и который уже ожидает
		// фронт (projects.ts: validateStair читает r.validation).
		return genericConfigInputError(msg)
	}
}

// genericConfigInputError — fallback для ошибок доменной валидации без
// явного кейса: сохраняет исходный текст (он уже на русском и без
// внутренних деталей) и оборачивает в InputError, чтобы вызывающий получил
// блокирующий результат вместо 500.
func genericConfigInputError(msg string) *solver.InputError {
	text := strings.TrimSpace(strings.TrimPrefix(msg, "stair:"))
	if text == "" {
		text = "Некорректные параметры конфигурации"
	}
	return &solver.InputError{
		Code:    constraint.GEO_WIDTH, // нейтральный код: элемент конфигурации
		Field:   "Параметры лестницы",
		Message: capitalizeFirst(text),
		Guide:   "Проверьте параметры конфигурации: " + text,
		Fix:     "Исправьте параметры конфигурации",
	}
}

// capitalizeFirst поднимает первую букву сообщения (для пользовательского текста).
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 'a' + 'A'
	}
	return string(r)
}

// materialCodeFromError извлекает код материала из ошибки вида
// "stair: material \"WOOD-OAK\" does not support thickness 70 mm of косоура".
func materialCodeFromError(msg string) string {
	i := strings.Index(msg, "material \"")
	if i < 0 {
		return ""
	}
	rest := msg[i+len("material \""):]
	j := strings.Index(rest, "\"")
	if j < 0 {
		return rest
	}
	return strings.TrimSpace(rest[:j])
}

// materialPartFromError извлекает имя детали из ошибки
// "stair: material ... does not support thickness 70 mm of косоура".
func materialPartFromError(msg string) string {
	i := strings.Index(msg, " mm of ")
	if i < 0 {
		return "деталь"
	}
	return strings.TrimSpace(msg[i+len(" mm of "):])
}

// lowerStepMaxFromError извлекает верхнюю границу диапазона нижнего марша из
// сообщения вида "stair: lower step count must be in [1, N] for L".
func lowerStepMaxFromError(msg string) int {
	i := strings.Index(msg, "[1, ")
	if i < 0 {
		return 0
	}
	rest := msg[i+len("[1, "):]
	j := strings.Index(rest, "]")
	if j < 0 {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(rest[:j]))
	if err != nil {
		return 0
	}
	return v
}

// lowerStepInputErrorFromMessage строит входную ошибку GEO-LOWER-STEP с
// конкретным допустимым диапазоном [1..StepCount-1], извлечённым из
// сообщения доменной валидации. Без распознанного диапазона — общий текст.
func lowerStepInputErrorFromMessage(msg string) *solver.InputError {
	max := lowerStepMaxFromError(msg)
	guide := "Число ступеней нижнего марша должно быть в допустимом диапазоне."
	if max > 0 {
		guide = fmt.Sprintf("Число ступеней нижнего марша должно быть от 1 до %d (всего ступеней %d).", max, max+1)
	}
	return &solver.InputError{
		Code: constraint.GEO_LOWER_STEP, Field: "Нижних ступеней",
		Message: "Число ступеней нижнего марша вне диапазона",
		Guide:   guide,
		Fix:     "Задайте число ступеней нижнего марша",
	}
}
