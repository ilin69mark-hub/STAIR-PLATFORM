package solver

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// LShapeResult — результат расчёта L-образной лестницы (EDR-0005 §3,
// выходные значения): два прямых марша (нижний/верхний) и площадка.
// Верхний марш повёрнут на 90° относительно нижнего; параметры ступени
// (h, b, α) общие для обоих маршей.
type LShapeResult struct {
	StepCount      int                // n — общее число ступеней
	LowerStepCount int                // n1 — ступени нижнего марша
	UpperStepCount int                // n2 — ступени верхнего марша
	StepHeight     engineering.Length // h — уточнённая высота ступени (общая)
	TreadDepth     engineering.Length // b — уточнённая проступь (общая)
	Angle          engineering.Angle  // α — угол наклона (общий)
	LowerHeight    engineering.Length // H1 — высота окончания нижнего марша (= уровень площадки)
	UpperHeight    engineering.Length // H2 — высота окончания верхнего марша (= H)
	LowerRun       engineering.Length // L1 — длина нижнего марша по горизонтали
	UpperRun       engineering.Length // L2 — длина верхнего марша по горизонтали
	LowerStringer  engineering.Length // R1 — длина косоура нижнего марша
	UpperStringer  engineering.Length // R2 — длина косоура верхнего марша
	LandingWidth   engineering.Length // Wp — ширина площадки
}

// Apply записывает результат расчёта L-образной лестницы в
// параметрическую конфигурацию. Length/StringerLength отражают нижний
// марш; LandingWidth и LowerStepCount — специфичные для L-марша поля.
func (r LShapeResult) Apply(cfg *engineering.StairConfiguration) {
	cfg.StepCount = r.StepCount
	cfg.StepHeight = r.StepHeight
	cfg.TreadDepth = r.TreadDepth
	cfg.Length = r.LowerRun
	cfg.StringerLength = r.LowerStringer
	cfg.Angle = r.Angle
	cfg.LandingWidth = r.LandingWidth
	cfg.LowerStepCount = r.LowerStepCount
}

// SolveLShape рассчитывает L-образную лестницу по EDR-0005 §4.
// H — общая высота подъёма, h0 — целевая высота ступени, n1 — число
// ступеней нижнего марша, wp — ширина площадки, s — шаг комфорта
// (по умолчанию DefaultComfortStep).
func SolveLShape(H, h0 engineering.Length, n1 int, wp engineering.Length, s ...float64) (LShapeResult, error) {
	var step float64
	if len(s) > 0 {
		step = s[0]
	} else {
		step = DefaultComfortStep
	}

	hm := H.Millimeters()
	if hm <= 0 {
		return LShapeResult{}, fmt.Errorf("solver: rise height must be positive")
	}
	h0m := h0.Millimeters()
	if h0m <= 0 {
		return LShapeResult{}, fmt.Errorf("solver: target riser must be positive")
	}
	wpm := wp.Millimeters()
	if wpm <= 0 {
		return LShapeResult{}, fmt.Errorf("solver: landing width must be positive")
	}
	if step < ComfortStepMin || step > ComfortStepMax {
		return LShapeResult{}, fmt.Errorf("solver: comfort step %v out of range %v-%v", step, ComfortStepMin, ComfortStepMax)
	}

	// §4.1 число ступеней; §7 edge case: высота без участка.
	n := int(math.Round(hm / h0m))
	if n < 1 {
		return LShapeResult{}, fmt.Errorf("solver: no flight (n < 1) for rise %v", hm)
	}

	// §4.5 разбивка по маршам: 1 ≤ n1 ≤ n−1, n2 ≥ 1.
	if n1 < 1 || n1 > n-1 {
		return LShapeResult{}, fmt.Errorf("solver: lower step count %d out of range [1, %d]", n1, n-1)
	}

	// §4.2 уточнённая высота ступени (общая).
	h := hm / float64(n)
	// §4.3 проступь (общая).
	b := step - 2*h
	if b <= 0 {
		return LShapeResult{}, fmt.Errorf("solver: tread depth must be positive, got %v", b)
	}
	// §4.4 угол наклона (общий).
	alpha := math.Atan(h / b)

	// §4.5 высоты секций.
	n2 := n - n1
	h1 := float64(n1) * h
	h2 := float64(n2) * h
	// §4.6 длины маршей по горизонтали.
	l1 := float64(n1) * b
	l2 := float64(n2) * b
	// §4.7 длины косоуров.
	r1 := math.Sqrt(l1*l1 + h1*h1)
	r2 := math.Sqrt(l2*l2 + h2*h2)

	return LShapeResult{
		StepCount:      n,
		LowerStepCount: n1,
		UpperStepCount: n2,
		StepHeight:     engineering.Length(h),
		TreadDepth:     engineering.Length(b),
		Angle:          engineering.Angle(alpha),
		LowerHeight:    engineering.Length(h1),
		UpperHeight:    engineering.Length(h2),
		LowerRun:       engineering.Length(l1),
		UpperRun:       engineering.Length(l2),
		LowerStringer:  engineering.Length(r1),
		UpperStringer:  engineering.Length(r2),
		LandingWidth:   engineering.Length(wpm),
	}, nil
}

// SolveCheckedLShape решает L-образный марш, применяет результат к
// конфигурации и валидирует его (EDR-0003 §3, DOM-0014). При blocking-
// ошибке результат в конфигурацию НЕ записывается; возвращается
// validation.Result для диагностики. Инвариант EDR-0005 §4.8 (Wp ≥ W)
// проверяется как ошибка (EDR-0005 §7).
func SolveCheckedLShape(cfg *engineering.StairConfiguration, set *constraint.ConstraintSet, s ...float64) (LShapeResult, validation.Result, error) {
	res, err := SolveLShape(cfg.Height, cfg.StepHeight, cfg.LowerStepCount, cfg.LandingWidth, s...)
	if err != nil {
		return LShapeResult{}, validation.Result{}, err
	}
	// §4.8 инвариант площадки: Wp ≥ W.
	if res.LandingWidth.Millimeters() < cfg.Width.Millimeters() {
		return LShapeResult{}, validation.Result{}, fmt.Errorf(
			"solver: landing width %v must be at least stair width %v", res.LandingWidth.Millimeters(), cfg.Width.Millimeters())
	}
	res.Apply(cfg)
	vr := validation.Validate(cfg, set)
	if vr.Blocking {
		cfg.StepCount = 0
		cfg.StepHeight = 0
		cfg.TreadDepth = 0
		cfg.Length = 0
		cfg.StringerLength = 0
		cfg.Angle = 0
		cfg.LandingWidth = 0
		cfg.LowerStepCount = 0
	}
	return res, vr, nil
}
