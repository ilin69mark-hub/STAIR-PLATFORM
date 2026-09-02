// Package validation реализует Validation Engine (EDR-0003).
// Сверяет параметрическую модель марша с активным ConstraintSet (BC-003)
// и возвращает детерминированный список ValidationIssue.
// Валидация не изменяет модель (BC-003).
package validation

import "stairplatform/internal/engine/constraint"

// Suggestion — конкретный вариант конфигурации, который проходит
// все активные нормы (заполняется советником advisor). Набор полей —
// через что можно применить исправление (по ним фронтенд собирает
// параметры и повторяет расчёт).
type Suggestion struct {
	StepCount      int     // n — общее число ступеней
	LowerStepCount int     // n1 — число ступеней нижнего марша (0 для прямого/спирали)
	WinderCount    int     // nw — число поворотных ступеней (0 для площадки/прямого/спирали)
	StepHeightMm   float64 // h — высота ступени
	TreadDepthMm   float64 // b — проступь
	AngleDeg       float64 // α — угол наклона, градусы
	// OuterRadiusMm и WidthMm — для спирали (EDR-0007): наружный радиус R и
	// ширина марша W; у прямых/L/U маршей равны 0 (не применяются).
	OuterRadiusMm float64 // R — наружный радиус спирали
	WidthMm       float64 // W — ширина марша
}

// Variation — альтернативная конфигурация (вариант A/B/C), устраняющая
// неблокирующее нарушение (напр. room_fit). Config — переопределение полей
// формы: ключи совпадают с полями ConfigForm фронтенда, значения — строки
// (как в форме). Фронтенд сливает их в текущий конфиг и пересчитывает
// (превью), не заменяя исходный, пока пользователь не выберет вариант.
// Summary — краткое описание варианта (габариты, ступени, угол).
type Variation struct {
	ID          string
	Title       string
	Description string
	Config      map[string]string
	Fits        bool
	// PassesNorms — кандидат проходит ВСЕ активные нормы (неблокирующий).
	// Выставляется генератором вариаций (variation) после проверки через
	// validation.Validate; фронтенду/аудиту сигнализирует, что применение
	// варианта не приведёт к blocking-ошибке (см. bugfix: «круче» → проступь).
	PassesNorms bool
	Summary     string
}

// Issue — одно нарушение правила (EDR-0003 §4).
type Issue struct {
	ID       string
	Code     constraint.RuleCode
	Severity constraint.Severity
	Element  string
	Message  string
	Value    float64
	Min      float64
	Max      float64
	HasMin   bool
	HasMax   bool
	Fix      string
	// Param — поле конструктора, которое надо поправить (например,
	// «Высота ступени»). Guide — понятное описание блокировки и решения.
	// Suggestions — готовые проходящие нормы варианты конфигурации.
	// Заполняются советником advisor при blocking-валидации (additive).
	// Variations — альтернативные конфигурации для неблокирующих
	// нарушений (room_fit и т.п.), предлагаемые как варианты выбора.
	Param       string
	Guide       string
	Suggestions []Suggestion
	Variations  []Variation
}

// Result — итог валидации (EDR-0003 §5).
type Result struct {
	Issues   []Issue
	Valid    bool
	Blocking bool
}
