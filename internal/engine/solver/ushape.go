package solver

import (
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// UShapeResult — результат расчёта П-образной лестницы (EDR-0006 §3,
// выходные значения): два прямых марша (нижний/верхний) и площадка либо
// поворотные ступени. Верхний марш параллелен нижнему и развёрнут на 180°;
// параметры ступени (h, b, α) общие для прямых и поворотных ступеней.
type UShapeResult struct {
	StepCount      int                // n — общее число ступеней
	LowerStepCount int                // n1 — ступени нижнего марша
	UpperStepCount int                // n2 — ступени верхнего марша
	WinderCount    int                // nw — поворотные ступени (0 для площадки)
	StepHeight     engineering.Length // h — уточнённая высота ступени (общая)
	TreadDepth     engineering.Length // b — уточнённая проступь (общая)
	Angle          engineering.Angle  // α — угол наклона (общий)
	LowerHeight    engineering.Length // H1 — высота окончания нижнего марша (= уровень поворота)
	UpperHeight    engineering.Length // H2 — высота окончания верхнего марша (= H)
	LowerRun       engineering.Length // L1 — длина нижнего марша по горизонтали
	UpperRun       engineering.Length // L2 — длина верхнего марша по горизонтали
	LowerStringer  engineering.Length // R1 — длина косоура нижнего марша
	UpperStringer  engineering.Length // R2 — длина косоура верхнего марша
	LandingWidth   engineering.Length // Wp — ширина площадки (платформа) либо просвета (поворот)
}

// Apply записывает результат расчёта П-образной лестницы в
// параметрическую конфигурацию. Length/StringerLength отражают нижний
// марш; LandingWidth, LowerStepCount и WinderCount — специфичные для маршей
// с площадкой/поворотом поля (EDR-0006).
func (r UShapeResult) Apply(cfg *engineering.StairConfiguration) {
	cfg.StepCount = r.StepCount
	cfg.StepHeight = r.StepHeight
	cfg.TreadDepth = r.TreadDepth
	cfg.Length = r.LowerRun
	cfg.StringerLength = r.LowerStringer
	cfg.Angle = r.Angle
	cfg.LandingWidth = r.LandingWidth
	cfg.LowerStepCount = r.LowerStepCount
	cfg.WinderCount = r.WinderCount
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
		WinderCount:    v.nw,
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

// SolveUShapeWinder рассчитывает П-образную лестницу с поворотными
// ступенями (EDR-0006 §4, поворот на 180°): нижний прямой марш (n1),
// набор из nw поворотных ступеней (веер на 180° в просвете шириной wp)
// и верхний прямой марш (n2 = n − n1 − nw). Параметры ступени (h, b, α)
// общие для прямых и поворотных ступеней; поворотные ступени входят в
// общее число ступеней n.
func SolveUShapeWinder(H, h0 engineering.Length, n1, nw int, wp engineering.Length, s ...float64) (UShapeResult, error) {
	v, err := solveUShapeWinder(H, h0, n1, nw, wp, s...)
	if err != nil {
		return UShapeResult{}, err
	}
	return UShapeResult{
		StepCount:      v.n,
		LowerStepCount: v.n1,
		UpperStepCount: v.n2,
		WinderCount:    v.nw,
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
// validation.Result для диагностики. Тип поворота (площадка или поворотные
// ступени) выбирается полем cfg.TurnKind.
func SolveCheckedUShape(cfg *engineering.StairConfiguration, set *constraint.ConstraintSet, s ...float64) (UShapeResult, validation.Result, error) {
	var res UShapeResult
	var err error
	if cfg.TurnKind == engineering.TurnWinder {
		res, err = SolveUShapeWinder(cfg.Height, cfg.StepHeight, cfg.LowerStepCount, cfg.WinderCount, cfg.LandingWidth, s...)
	} else {
		res, err = SolveUShape(cfg.Height, cfg.StepHeight, cfg.LowerStepCount, cfg.LandingWidth, s...)
	}
	if err != nil {
		return UShapeResult{}, validation.Result{}, err
	}
	// §4.8 инвариант площадки (только режим площадки): Wp ≥ W.
	if cfg.TurnKind != engineering.TurnWinder && res.LandingWidth.Millimeters() < cfg.Width.Millimeters() {
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
		cfg.WinderCount = 0
	}
	return res, vr, nil
}
