package solver

import (
	"math"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	"stairplatform/internal/engine/validation"
)

// FullTurnRadians — полный угол поворота спиральной лестницы
// (EDR-0007 §4.3): 360° = 2π радиан. Константа.
const FullTurnRadians = 2 * math.Pi

// SpiralResult — результат расчёта спиральной лестницы с центральной
// колонной (EDR-0007 §3, выходные значения).
type SpiralResult struct {
	StepCount    int                // n — число ступеней
	StepHeight   engineering.Length // h — уточнённая высота ступени
	OuterRadius  engineering.Length // R — наружный радиус марша
	ColumnRadius engineering.Length // r — радиус центральной колонны
	WalkRadius   engineering.Length // r_walk — радиус линии хода
	InnerTread   engineering.Length // b_in — проступь у колонны
	WalkTread    engineering.Length // b_walk — проступь по линии хода
	OuterTread   engineering.Length // b_out — проступь у наружной кромки
	Angle        engineering.Angle  // α — угол подъёма винтовой линии
	AngularStep  float64            // δ — угловой шаг одной ступени, рад
	ArcLength    engineering.Length // L — длина дуги марша по наружной кромке
	ComfortStep  float64            // S = 2h + b_walk, мм
	AngularTotal float64            // α_total — полный угол поворота, рад (= 2π)
}

// Apply записывает результат расчёта спиральной лестницы в параметрическую
// конфигурацию (EDR-0007 §3). Проступь (TreadDepth) отражает линию хода;
// Length/StringerLength не применимы к спирали и остаются как есть.
func (r SpiralResult) Apply(cfg *engineering.StairConfiguration) {
	cfg.StepCount = r.StepCount
	cfg.StepHeight = r.StepHeight
	cfg.TreadDepth = r.WalkTread
	cfg.Angle = r.Angle
}

// spiralValues — результат блока формул спирального марша (EDR-0007 §4),
// общий для SolveSpiral (единый источник, ADR-0003).
type spiralValues struct {
	n      int
	h      float64
	rm     float64
	r      float64
	rWalk  float64
	delta  float64
	bIn    float64
	bWalk  float64
	bOut   float64
	alpha  float64
	arcLen float64
}

// solveSpiral вычисляет блок формул EDR-0007 §4 для спиральной лестницы
// с центральной колонной. H — высота подъёма, h0 — целевая высота ступени,
// W — ширина марша, R — наружный радиус. Валидация входов и edge cases
// выполняются здесь (EDR-0007 §7).
func solveSpiral(H, h0, W, R engineering.Length) (spiralValues, error) {
	hm := H.Millimeters()
	if hm <= 0 {
		return spiralValues{}, riseInputError()
	}
	h0m := h0.Millimeters()
	if h0m <= 0 {
		return spiralValues{}, riserInputError()
	}
	wm := W.Millimeters()
	if wm <= 0 {
		return spiralValues{}, widthInputError()
	}
	rm := R.Millimeters()
	if rm <= wm {
		return spiralValues{}, radiusNarrowError(rm, wm)
	}

	// §4.1 число ступеней; §7 edge case: высота без участка.
	n := int(math.Round(hm / h0m))
	if n < 1 {
		return spiralValues{}, noFlightInputError(hm)
	}

	// §4.2 уточнённая высота ступени.
	h := hm / float64(n)

	// §4.4 радиусы.
	r := rm - wm
	rWalk := r + (2.0/3.0)*wm

	// §4.3 полный угол и угловой шаг.
	delta := FullTurnRadians / float64(n)

	// §4.5 проступи.
	bIn := delta * r
	bWalk := delta * rWalk
	bOut := delta * rm

	// §7 edge cases: проступи не могут быть бессмысленно малы/велики.
	if bIn < 100 {
		return spiralValues{}, spiralTreadInputError("inner", bIn, 100, 0)
	}
	if bOut < 250 {
		return spiralValues{}, spiralTreadInputError("outer", bOut, 250, 0)
	}
	if bWalk < 260 || bWalk > 320 {
		return spiralValues{}, spiralTreadInputError("walk", bWalk, 260, 320)
	}

	// §4.6 шаг комфорта по линии хода S = 2h + b_walk (600–640, EDR-0001).
	s := 2*h + bWalk
	if s < ComfortStepMin || s > ComfortStepMax {
		return spiralValues{}, comfortInputError(s)
	}

	// §4.6 угол подъёма винтовой линии (по линии хода).
	alpha := math.Atan(h / bWalk)

	// §4.7 длина дуги по наружной кромке.
	arcLen := delta * rm * float64(n)

	return spiralValues{
		n: n, h: h, rm: rm, r: r, rWalk: rWalk,
		delta: delta,
		bIn:   bIn, bWalk: bWalk, bOut: bOut,
		alpha: alpha, arcLen: arcLen,
	}, nil
}

// SolveSpiral рассчитывает спиральную лестницу по EDR-0007 §4.
// H — высота подъёма, h0 — целевая высота ступени, W — ширина марша
// (колонна → наружная кромка), R — наружный радиус.
func SolveSpiral(H, h0, W, R engineering.Length) (SpiralResult, error) {
	v, err := solveSpiral(H, h0, W, R)
	if err != nil {
		return SpiralResult{}, err
	}
	return SpiralResult{
		StepCount:    v.n,
		StepHeight:   engineering.Length(v.h),
		OuterRadius:  engineering.Length(v.rm),
		ColumnRadius: engineering.Length(v.r),
		WalkRadius:   engineering.Length(v.rWalk),
		InnerTread:   engineering.Length(v.bIn),
		WalkTread:    engineering.Length(v.bWalk),
		OuterTread:   engineering.Length(v.bOut),
		Angle:        engineering.Angle(v.alpha),
		AngularStep:  v.delta,
		ArcLength:    engineering.Length(v.arcLen),
		ComfortStep:  2*v.h + v.bWalk,
		AngularTotal: FullTurnRadians,
	}, nil
}

// SolveCheckedSpiral решает спиральный марш, применяет результат к
// конфигурации и валидирует его (EDR-0003 §3, DOM-0014). При blocking-
// ошибке результат в конфигурацию НЕ записывается; возвращается
// validation.Result для диагностики. Инварианты EDR-0007 §4.5-4.6
// проверяются как ошибки (EDR-0007 §7).
func SolveCheckedSpiral(cfg *engineering.StairConfiguration, set *constraint.ConstraintSet) (SpiralResult, validation.Result, error) {
	res, err := SolveSpiral(cfg.Height, cfg.StepHeight, cfg.Width, cfg.OuterRadius)
	if err != nil {
		return SpiralResult{}, validation.Result{}, err
	}
	res.Apply(cfg)
	vr := validation.Validate(cfg, set)
	if vr.Blocking {
		cfg.StepCount = 0
		cfg.StepHeight = 0
		cfg.TreadDepth = 0
		cfg.Angle = 0
	}
	return res, vr, nil
}
