package solver

import (
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// UShapeResult — результат расчёта П-образной лестницы (EDR-0006 §3,
// выходные значения): два прямых марша (нижний/верхний) и площадка.
// Верхний марш параллелен нижнему и развёрнут на 180°; параметры ступени
// (h, b, α) общие для обоих маршей.
type UShapeResult struct {
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

// Apply записывает результат расчёта П-образной лестницы в
// параметрическую конфигурацию. Length/StringerLength отражают нижний
// марш; LandingWidth и LowerStepCount — специфичные для маршей с площадкой
// поля (EDR-0006).
func (r UShapeResult) Apply(cfg *engineering.StairConfiguration) {
	cfg.StepCount = r.StepCount
	cfg.StepHeight = r.StepHeight
	cfg.TreadDepth = r.TreadDepth
	cfg.Length = r.LowerRun
	cfg.StringerLength = r.LowerStringer
	cfg.Angle = r.Angle
	cfg.LandingWidth = r.LandingWidth
	cfg.LowerStepCount = r.LowerStepCount
}

// SolveUShape рассчитывает П-образную лестницу по EDR-0006 §4.
// H — общая высота подъёма, h0 — целевая высота ступени, n1 — число
// ступеней нижнего марша, wp — ширина площадки, s — шаг комфорта
// (по умолчанию DefaultComfortStep). Формулы §4 идентичны EDR-0005
// (разбивка n1/n2, H1/H2, L1/L2, R1/R2); отличие — поворот верхнего
// марша на 180° (геометрия, не Solver).
func SolveUShape(H, h0 engineering.Length, n1 int, wp engineering.Length, s ...float64) (UShapeResult, error) {
	v, err := solveTwoFlight(H, h0, n1, wp, s...)
	if err != nil {
		return UShapeResult{}, err
	}
	return UShapeResult{
		StepCount:      v.n,
		LowerStepCount: v.n1,
		UpperStepCount: v.n2,
		StepHeight:     engineering.Length(v.h),
		TreadDepth:     engineering.Length(v.b),
		Angle:          engineering.Angle(v.alpha),
		LowerHeight:    engineering.Length(v.h1),
		UpperHeight:    engineering.Length(v.h2),
		LowerRun:       engineering.Length(v.l1),
		UpperRun:       engineering.Length(v.l2),
		LowerStringer:  engineering.Length(v.r1),
		UpperStringer:  engineering.Length(v.r2),
		LandingWidth:   engineering.Length(v.wp),
	}, nil
}

// SolveCheckedUShape решает П-образный марш, применяет результат к
// конфигурации и валидирует его (EDR-0003 §3, DOM-0014). При blocking-
// ошибке результат в конфигурацию НЕ записывается; возвращается
// validation.Result для диагностики. Инвариант EDR-0006 §4.8 (Wp ≥ W)
// проверяется как ошибка (EDR-0006 §7).
func SolveCheckedUShape(cfg *engineering.StairConfiguration, set *constraint.ConstraintSet, s ...float64) (UShapeResult, validation.Result, error) {
	res, err := SolveUShape(cfg.Height, cfg.StepHeight, cfg.LowerStepCount, cfg.LandingWidth, s...)
	if err != nil {
		return UShapeResult{}, validation.Result{}, err
	}
	// §4.8 инвариант площадки: Wp ≥ W.
	if res.LandingWidth.Millimeters() < cfg.Width.Millimeters() {
		return UShapeResult{}, validation.Result{}, landingNarrowError(
			res.LandingWidth.Millimeters(), cfg.Width.Millimeters())
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
