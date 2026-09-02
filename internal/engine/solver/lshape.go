package solver

import (
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
	LandingDepth   engineering.Length // Ld — глубина площадки (вдоль нижнего марша, X)
	RoomWidth      engineering.Length // габарит помещения по X (мм), для fit-check
	RoomLength     engineering.Length // габарит помещения по Y (мм), для fit-check
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
	cfg.LandingDepth = r.LandingDepth
	cfg.LowerStepCount = r.LowerStepCount
}

// twoFlightValues — результат решателя марша с площадкой (EDR-0005 §3
// EDR-0006 §3, общие для L- и П-образного маршей).
type twoFlightValues struct {
	n, n1, n2, nw int
	h, b, alpha   float64
	h1, h2        float64
	l1, l2        float64
	r1, r2        float64
	wp            float64
}

// solveTwoFlight вычисляет общий для L- и П-образного маршей блок формул
// (§4): n, h, b, α, разбивку n1/n2, высоты/длины секций, косоуры, Wp.
// Валидация входов и edge cases §7 выполняются здесь; результат —
// единый источник для SolveLShape и SolveUShape (ADR-0003).
func solveTwoFlight(H, h0 engineering.Length, n1 int, wp engineering.Length, s ...float64) (twoFlightValues, error) {
	var step float64
	if len(s) > 0 {
		step = s[0]
	} else {
		step = DefaultComfortStep
	}

	hm := H.Millimeters()
	if hm <= 0 {
		return twoFlightValues{}, riseInputError()
	}
	h0m := h0.Millimeters()
	if h0m <= 0 {
		return twoFlightValues{}, riserInputError()
	}
	wpm := wp.Millimeters()
	if wpm <= 0 {
		return twoFlightValues{}, landingPositiveError()
	}
	if step < ComfortStepMin || step > ComfortStepMax {
		return twoFlightValues{}, comfortInputError(step)
	}

	// §4.1 число ступеней; §7 edge case: высота без участка.
	n := int(math.Round(hm / h0m))
	if n < 1 {
		return twoFlightValues{}, noFlightInputError(hm)
	}

	// §4.5 разбивка по маршам: 1 ≤ n1 ≤ n−1, n2 ≥ 1.
	if n1 < 1 || n1 > n-1 {
		return twoFlightValues{}, lowerStepInputError(n1, n)
	}

	// §4.2 уточнённая высота ступени (общая).
	h := hm / float64(n)
	// §4.3 проступь (общая).
	b := step - 2*h
	if b <= 0 {
		return twoFlightValues{}, treadPositiveError(b)
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

	return twoFlightValues{
		n: n, n1: n1, n2: n2, nw: 0,
		h: h, b: b, alpha: alpha,
		h1: h1, h2: h2,
		l1: l1, l2: l2,
		r1: r1, r2: r2,
		wp: wpm,
	}, nil
}

// solveUShapeWinder вычисляет блок формул для П-образного марша с
// поворотными ступенями (EDR-0006 §4, поворот на 180°): общие n1/nw/n2,
// h, b, α, высоты/длины секций, косоуры, Wp (ширина просвета). Число
// поворотных ступеней nw входит в общее число ступеней n = n1 + nw + n2.
// Инвариант §7: 1 ≤ n1, nw ≥ 3, n2 ≥ 1.
func solveUShapeWinder(H, h0 engineering.Length, n1, nw int, wp engineering.Length, s ...float64) (twoFlightValues, error) {
	var step float64
	if len(s) > 0 {
		step = s[0]
	} else {
		step = DefaultComfortStep
	}

	hm := H.Millimeters()
	if hm <= 0 {
		return twoFlightValues{}, riseInputError()
	}
	h0m := h0.Millimeters()
	if h0m <= 0 {
		return twoFlightValues{}, riserInputError()
	}
	wpm := wp.Millimeters()
	if wpm <= 0 {
		return twoFlightValues{}, landingPositiveError()
	}
	if step < ComfortStepMin || step > ComfortStepMax {
		return twoFlightValues{}, comfortInputError(step)
	}

	// §4.1 число ступеней; поворотные ступени входят в общее число.
	n := int(math.Round(hm / h0m))
	if n < 1 {
		return twoFlightValues{}, noFlightInputError(hm)
	}

	// §4.5 разбивка по маршам и повороту: 1 ≤ n1, nw ≥ 3, n2 ≥ 1.
	if n1 < 1 || n1 > n-1 {
		return twoFlightValues{}, lowerStepInputError(n1, n)
	}
	if nw < 3 {
		return twoFlightValues{}, winderCountInputError(nw)
	}
	n2 := n - n1 - nw
	if n2 < 1 {
		return twoFlightValues{}, upperStepInputError(n2, n, nw)
	}

	// §4.2 уточнённая высота ступени (общая для прямых и поворотных).
	h := hm / float64(n)
	// §4.3 проступь (общая).
	b := step - 2*h
	if b <= 0 {
		return twoFlightValues{}, treadPositiveError(b)
	}
	// §4.4 угол наклона (общий).
	alpha := math.Atan(h / b)

	// §4.5 высоты секций: нижний марш n1·h, поворот nw·h, верхний n2·h.
	h1 := float64(n1) * h
	h2 := float64(n2) * h
	// §4.6 длины маршей по горизонтали (поворотные ступени не добавляют
	// прямого пробега — они занимают поворот).
	l1 := float64(n1) * b
	l2 := float64(n2) * b
	// §4.7 длины косоуров.
	r1 := math.Sqrt(l1*l1 + h1*h1)
	r2 := math.Sqrt(l2*l2 + h2*h2)

	return twoFlightValues{
		n: n, n1: n1, n2: n2, nw: nw,
		h: h, b: b, alpha: alpha,
		h1: h1, h2: h2,
		l1: l1, l2: l2,
		r1: r1, r2: r2,
		wp: wpm,
	}, nil
}

// SolveLShape рассчитывает L-образную лестницу по EDR-0005 §4.
// H — общая высота подъёма, h0 — целевая высота ступени, n1 — число
// ступеней нижнего марша, wp — ширина площадки, s — шаг комфорта
// (по умолчанию DefaultComfortStep).
func SolveLShape(H, h0 engineering.Length, n1 int, wp engineering.Length, s ...float64) (LShapeResult, error) {
	v, err := solveTwoFlight(H, h0, n1, wp, s...)
	if err != nil {
		return LShapeResult{}, err
	}
	return LShapeResult{
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
		return LShapeResult{}, validation.Result{}, landingNarrowError(
			res.LandingWidth.Millimeters(), cfg.Width.Millimeters())
	}
	res.Apply(cfg)
	// Эффективная глубина площадки (при 0 — равна ширине марша).
	ld := cfg.LandingDepth
	if ld.Millimeters() <= 0 {
		ld = cfg.Width
	}
	res.LandingDepth = ld
	res.RoomWidth = cfg.RoomWidth
	res.RoomLength = cfg.RoomLength
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
