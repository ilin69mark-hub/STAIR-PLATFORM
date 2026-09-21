// Package variation — генератор альтернативных конфигураций (вариации
// A/B/C) для неблокирующих нарушений, прежде всего невписываемости
// лестницы в периметр помещения (room_fit). В отличие от советник advisor
// (исправляет blocking-нарушения норм), variation предлагает пользователю
// несколько готовых ВАРИАНТОВ, среди которых он выбирает понравившийся:
//
//	A — сделать лестницу чуть круче, оставаясь в нормах (укороченный марш);
//	B — уменьшить площадку до минимально допустимой;
//	C — предложить другой тип лестницы (прямая / П-образная / L-образная).
//
// Каждый вариант прогоняется через Geometry Engine и проверяется на
// вписываемость; в ответе — только те, что реально влезают в помещение.
package variation

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	enggeo "stairplatform/internal/engine/geometry"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

// fitEps — допуск по сравнению габаритов с размерами помещения (как в
// engine.go при эмиссии room_fit).
const fitEps = 1e-6

// fitTol — «спасительный» допуск для запасных вариантов: если ни один
// вариант не вписывается строго, помечаются и возвращаются ближайшие
// (запас менее 50 мм), чтобы у пользователя не было пустого списка.
const fitTol = 50.0

// ForRoomFit возвращает вариации, устраняющие невписываемость лестницы в
// периметр помещения. Если помещение не задано — пустой срез. Сначала
// ищутся строго вписывающиеся варианты; если их нет — ближайшие к
// габаритам (с допуском fitTol), иначе пустой срез (фронтенд покажет
// только предупреждение).
func ForRoomFit(ctx context.Context, cfg *engineering.StairConfiguration, set *constraint.ConstraintSet) []validation.Variation {
	rw := cfg.RoomWidth.Millimeters()
	rl := cfg.RoomLength.Millimeters()
	if rw <= 0 || rl <= 0 {
		return nil
	}
	var out []validation.Variation
	for _, tol := range []float64{fitEps, fitTol} {
		out = forRoomFitTol(ctx, cfg, set, rw, rl, tol)
		if len(out) > 0 {
			break
		}
	}
	return dedupe(out)
}

// forRoomFitTol собирает вариации A/B/C с заданным допуском вписываемости tol.
func forRoomFitTol(ctx context.Context, cfg *engineering.StairConfiguration, set *constraint.ConstraintSet, rw, rl, tol float64) []validation.Variation {
	comfort := cfg.Length.Millimeters()
	if comfort < solver.ComfortStepMin || comfort > solver.ComfortStepMax {
		comfort = solver.DefaultComfortStep
	}
	height := cfg.Height

	var out []validation.Variation

	// Вариант A — чуть круче, в пределах норм (h ≤ 200 мм).
	if v, ok := steeperVariant(ctx, cfg, height, comfort, set, rw, rl, tol); ok {
		out = append(out, v)
	}
	// Вариант B — площадка поменьше (минимально допустимая).
	if cfg.Flight == engineering.FlightLShape || cfg.Flight == engineering.FlightUShape {
		if v, ok := smallerLandingVariant(ctx, cfg, set, rw, rl, tol); ok {
			out = append(out, v)
		}
	}
	// Вариант C — другие типы лестницы (до двух сходившихся вариантов на тип).
	out = append(out, otherTypeVariants(ctx, cfg, height, comfort, set, rw, rl, tol)...)
	// Вариант D — та же лестница, но компактнее (уже марш, для спирали ещё и
	// меньше радиус). Прямое решение для помещений, где другой тип не влезает:
	// спираль → «спираль компактнее», прямая/L/U → «марш уже».
	out = append(out, buildOtherTypes(ctx, cfg, cfg.Flight, height, comfort, set, rw, rl, tol)...)

	return out
}

// ForAngle возвращает вариации, устраняющие нарушение угла наклона
// (GEO-ANGLE, норма 30–45°). В отличие от советника advisor, который при
// фиксированной проступи перебирает только число ступеней n, здесь
// подбираются И число ступеней n, И шаг комфорта S: проступь b = S − 2h,
// угол = atan(h/b). Это позволяет поднять угол до нормы даже при больших
// проступях (когда ни один n при фиксированной проступи не даёт угол ≥30°).
// Возвращает до трёх вариантов с разнесёнными углами в диапазоне 30–45°.
func ForAngle(ctx context.Context, cfg *engineering.StairConfiguration, set *constraint.ConstraintSet) []validation.Variation {
	height := cfg.Height
	H := height.Millimeters()
	if H <= 0 {
		return nil
	}
	hMin, hMax := 150.0, 200.0
	if r, ok := set.Active(constraint.GEO_STEP_HEIGHT); ok && r.Range.HasMin {
		hMin = r.Range.Min
		if r.Range.HasMax {
			hMax = r.Range.Max
		}
	}
	bMin, bMax := 240.0, 320.0
	if r, ok := set.Active(constraint.GEO_TREAD_DEPTH); ok && r.Range.HasMin {
		bMin = r.Range.Min
		if r.Range.HasMax {
			bMax = r.Range.Max
		}
	}
	// Собираем все допустимые (n, S): h=H/n в норме высоты ступени, S —
	// шаг комфорта из нормы, b=S−2h — проступь в норме, угол в 30–45°.
	type cand struct {
		n   int
		s   float64
		deg float64
	}
	var cands []cand
	for n := 2; n <= 80; n++ {
		h := H / float64(n)
		if h < hMin || h > hMax {
			continue
		}
		for _, s := range []float64{600, 620, 640} {
			b := s - 2*h
			if b < bMin || b > bMax {
				continue
			}
			deg := math.Atan(h/b) * 180 / math.Pi
			if deg < 30 || deg > 45 {
				continue
			}
			cands = append(cands, cand{n, s, deg})
		}
	}
	if len(cands) == 0 {
		return nil
	}
	// Выбираем до трёх вариантов с разнесёнными углами (min/mid/max).
	sort.Slice(cands, func(i, j int) bool { return cands[i].deg < cands[j].deg })
	var chosen []cand
	if len(cands) <= 3 {
		chosen = cands
	} else {
		chosen = []cand{cands[0], cands[len(cands)/2], cands[len(cands)-1]}
	}
	var out []validation.Variation
	for _, ch := range chosen {
		h := H / float64(ch.n)
		label := fmt.Sprintf("Угол %.0f°", ch.deg)
		if v, ok := angleVariant(ctx, cfg, height, h, ch.s, ch.n, set, label); ok {
			out = append(out, v)
		}
	}
	return dedupe(out)
}

// angleVariant строит кандидата под заданные высоту ступени h и шаг комфорта
// s (проступь b = s − 2h выводится solver), прогоняет через Geometry Engine и
// проверяет, что угол действительно вошёл в норму.
func angleVariant(ctx context.Context, cfg *engineering.StairConfiguration, height engineering.Length, h, s float64, n int, set *constraint.ConstraintSet, label string) (validation.Variation, bool) {
	clone := cloneCfg(cfg)
	hs := engineering.Length(h)
	switch cfg.Flight {
	case engineering.FlightStraight:
		r, err := solver.Solve(height, hs, s)
		if err != nil {
			return validation.Variation{}, false
		}
		applyStraight(clone, r)
	case engineering.FlightLShape, engineering.FlightUShape:
		n1 := n / 2
		if n1 < 1 {
			n1 = 1
		}
		lw := cfg.LandingWidth
		if lw.Millimeters() <= 0 {
			lw = cfg.Width
		}
		if cfg.Flight == engineering.FlightLShape {
			r, err := solver.SolveLShape(height, hs, n1, lw, s)
			if err != nil {
				return validation.Variation{}, false
			}
			applyTwoFlight(clone, r.StepCount, r.StepHeight, r.TreadDepth, r.Angle, r.LowerRun)
		} else {
			r, err := solver.SolveUShape(height, hs, n1, lw, s)
			if err != nil {
				return validation.Variation{}, false
			}
			applyTwoFlight(clone, r.StepCount, r.StepHeight, r.TreadDepth, r.Angle, r.LowerRun)
		}
		clone.LowerStepCount = n1
		clone.LandingWidth = lw
	case engineering.FlightSpiral:
		w := cfg.Width.Millimeters()
		if w <= 0 {
			w = 900
		}
		delta := 2 * math.Pi / float64(n)
		bWalk := h / math.Tan((chDegFromS(h, s))*math.Pi/180)
		R := bWalk/delta + w/3
		if R <= w {
			return validation.Variation{}, false
		}
		r, err := solver.SolveSpiral(height, hs, engineering.Length(w), engineering.Length(R))
		if err != nil {
			return validation.Variation{}, false
		}
		clone.Flight = engineering.FlightSpiral
		clone.StepCount = r.StepCount
		clone.StepHeight = r.StepHeight
		clone.Width = engineering.Length(w)
		clone.OuterRadius = engineering.Length(R)
		clone.Angle = r.Angle
		clone.Length = engineering.Length(bWalk)
	default:
		return validation.Variation{}, false
	}
	return tryAngleGenerate(ctx, clone, set, label)
}

// chDegFromS — угол по высоте ступени h и шагу комфорта s (b = s − 2h).
func chDegFromS(h, s float64) float64 {
	b := s - 2*h
	if b <= 0 {
		return 0
	}
	return math.Atan(h/b) * 180 / math.Pi
}

// tryAngleGenerate прогоняет кандидата через Geometry Engine (санитарная
// проверка, что конфиг собирается) и формирует Variation, только если угол
// действительно вошёл в норму 30–45° И кандидат неблокирующий по всем нормам.
func tryAngleGenerate(ctx context.Context, clone *engineering.StairConfiguration, set *constraint.ConstraintSet, label string) (validation.Variation, bool) {
	if _, err := enggeo.Generate(ctx, clone); err != nil {
		return validation.Variation{}, false
	}
	ang := clone.Angle.Degrees()
	if ang < 29.9 || ang > 45.1 {
		return validation.Variation{}, false
	}
	// Полная проверка норм: высота ступени/проступь не должны блокировать.
	if set != nil && validation.Validate(clone, set).Blocking {
		return validation.Variation{}, false
	}
	desc := fmt.Sprintf("Сделать угол наклона %.0f° (в норме 30–45°): высота ступени %.0f мм, проступь %.0f мм.", ang, clone.StepHeight.Millimeters(), clone.Length.Millimeters())
	return validation.Variation{
		ID:          label,
		Title:       label,
		Description: desc,
		Config:      formFromConfig(clone),
		Fits:        true,
		PassesNorms: true,
		Summary:     fmt.Sprintf("Угол %.1f°, %d ступ., h %.0f мм, b %.0f мм", ang, clone.StepCount, clone.StepHeight.Millimeters(), clone.Length.Millimeters()),
	}, true
}

// FromSuggestions преобразует готовые варианты советника (Suggestions) в
// интерактивные Variations для единообразного UX: там, где советник уже
// подобрал числа ступеней, пользователь получает кнопки «применить».
func FromSuggestions(issue *validation.Issue, cfg *engineering.StairConfiguration) []validation.Variation {
	if len(issue.Suggestions) == 0 {
		return nil
	}
	out := make([]validation.Variation, 0, len(issue.Suggestions))
	for idx, s := range issue.Suggestions {
		clone := cloneCfg(cfg)
		clone.StepHeight = engineering.Length(s.StepHeightMm)
		clone.TreadDepth = engineering.Length(s.TreadDepthMm)
		clone.StepCount = s.StepCount
		if (cfg.Flight == engineering.FlightLShape || cfg.Flight == engineering.FlightUShape) && s.LowerStepCount > 0 {
			clone.LowerStepCount = s.LowerStepCount
		}
		if cfg.Flight == engineering.FlightSpiral && s.WidthMm > 0 && s.OuterRadiusMm > 0 {
			clone.Width = engineering.Length(s.WidthMm)
			clone.OuterRadius = engineering.Length(s.OuterRadiusMm)
		}
		out = append(out, validation.Variation{
			ID:          fmt.Sprintf("%s — вариант %d", issue.Code, idx+1),
			Title:       fmt.Sprintf("n=%d, h≈%.0f, b≈%.0f", s.StepCount, s.StepHeightMm, s.TreadDepthMm),
			Description: fmt.Sprintf("Готовый вариант: %d ступеней, высота %.0f мм, проступь %.0f мм, угол %.1f°.", s.StepCount, s.StepHeightMm, s.TreadDepthMm, s.AngleDeg),
			Config:      formFromConfig(clone),
			Fits:        true,
			PassesNorms: true,
			Summary:     fmt.Sprintf("%d ступ., угол %.1f°", s.StepCount, s.AngleDeg),
		})
	}
	return out
}

// steeperVariant — увеличивает целевую высоту ступени, сокращая число
// ступеней и длину марша (чуть круче). Целевая высота ограничена так, чтобы
// проступь b = comfort − 2·h оставалась в норме (bugfix: ранее жёсткие 195 мм
// давали проступь вне 260–320 мм и блокировку при применении).
func steeperVariant(ctx context.Context, cfg *engineering.StairConfiguration, height engineering.Length, comfort float64, set *constraint.ConstraintSet, rw, rl, tol float64) (validation.Variation, bool) {
	// Минимально допустимая проступь из нормы (по умолчанию 260 мм).
	bMin := 260.0
	if set != nil {
		if r, ok := set.Active(constraint.GEO_TREAD_DEPTH); ok && r.Range.HasMin {
			bMin = r.Range.Min
		}
	}
	// h ≤ (comfort − bMin) / 2, чтобы b = comfort − 2h ≥ bMin; и не выше
	// верхней границы нормы высоты ступени (берём 195 мм как «чуть круче»).
	target := math.Min(195.0, (comfort-bMin)/2)
	if target < 150.0 {
		// При данном шаге комфорта валидный «круче» вариант невозможен.
		target = 195.0
	}
	clone := cloneCfg(cfg)
	switch cfg.Flight {
	case engineering.FlightStraight:
		r, err := solver.Solve(height, engineering.Length(target), comfort)
		if err != nil {
			return validation.Variation{}, false
		}
		applyStraight(clone, r)
	case engineering.FlightLShape:
		r, err := solver.SolveLShape(height, engineering.Length(target), cfg.LowerStepCount, cfg.LandingWidth, comfort)
		if err != nil {
			return validation.Variation{}, false
		}
		applyTwoFlight(clone, r.StepCount, r.StepHeight, r.TreadDepth, r.Angle, r.LowerRun)
	case engineering.FlightUShape:
		r, err := solver.SolveUShape(height, engineering.Length(target), cfg.LowerStepCount, cfg.LandingWidth, comfort)
		if err != nil {
			return validation.Variation{}, false
		}
		applyTwoFlight(clone, r.StepCount, r.StepHeight, r.TreadDepth, r.Angle, r.LowerRun)
	case engineering.FlightSpiral:
		return validation.Variation{}, false
	default:
		return validation.Variation{}, false
	}
	// НЕ переопределяем clone.StepHeight = target после applyStraight/applyTwoFlight:
	// солвер сам подбирает ближайшую допустимую высоту ступени (целое число
	// ступеней), и принудительная установка target рвет тождество
	// comfort = 2·StepHeight + TreadDepth, из-за чего formFromConfig отдавал
	// comfortStepMM > 640 и фронт блокировал применение варианта (баг
	// «сделать круче» не пересчитывает). Оставляем солверное значение.
	return tryGenerate(ctx, clone, set, rw, rl, tol, "A: сделать круче (в допусках)", "Увеличить высоту ступени до верхней границы нормы — марш короче, занимает меньше места.")
}

// smallerLandingVariant — уменьшает площадку до минимально допустимой
// (глубина и ширина = ширине марша), сохраняя параметры ступеней.
func smallerLandingVariant(ctx context.Context, cfg *engineering.StairConfiguration, set *constraint.ConstraintSet, rw, rl, tol float64) (validation.Variation, bool) {
	clone := cloneCfg(cfg)
	w := cfg.Width.Millimeters()
	if w <= 0 {
		return validation.Variation{}, false
	}
	clone.LandingDepth = engineering.Length(w)
	clone.LandingWidth = engineering.Length(w)
	return tryGenerate(ctx, clone, set, rw, rl, tol, "B: уменьшить площадку", "Сделать площадку минимально допустимой — габариты лестницы сокращаются.")
}

// otherTypeVariants — предлагает другие типы лестницы, которые могут
// вписаться в то же помещение (прямая / П-образная / L-образная / спираль).
// Для каждого кандидата выполняется адаптивный подбор ширины марша и
// крутизны (см. buildOtherType): система сама подбирает параметры, чтобы
// лестница влезла в заданный периметр.
func otherTypeVariants(ctx context.Context, cfg *engineering.StairConfiguration, height engineering.Length, comfort float64, set *constraint.ConstraintSet, rw, rl, tol float64) []validation.Variation {
	var candidates []engineering.FlightType
	switch cfg.Flight {
	case engineering.FlightLShape:
		candidates = []engineering.FlightType{engineering.FlightStraight, engineering.FlightUShape, engineering.FlightSpiral}
	case engineering.FlightUShape:
		candidates = []engineering.FlightType{engineering.FlightStraight, engineering.FlightLShape, engineering.FlightSpiral}
	case engineering.FlightStraight:
		candidates = []engineering.FlightType{engineering.FlightLShape, engineering.FlightUShape, engineering.FlightSpiral}
	case engineering.FlightSpiral:
		candidates = []engineering.FlightType{engineering.FlightStraight, engineering.FlightLShape, engineering.FlightUShape}
	default:
		candidates = []engineering.FlightType{engineering.FlightStraight, engineering.FlightLShape, engineering.FlightUShape}
	}
	out := make([]validation.Variation, 0, len(candidates))
	for _, ft := range candidates {
		out = append(out, buildOtherTypes(ctx, cfg, ft, height, comfort, set, rw, rl, tol)...)
	}
	return out
}

// otherCand — собранный кандидат варианта с метриками для выбора «ближайшего
// к исходным параметрам» (минимум изменений) и «самого компактного».
type otherCand struct {
	v  validation.Variation
	w  float64    // итоговая ширина марша
	h  float64    // итоговая высота ступени
	r  float64    // наружный радиус (для спирали, иначе 0)
	bb [2]float64 // габариты по результату Geometry Engine (X, Y)
}

// buildOtherTypes — адаптивный подбор параметров для альтернативного типа
// лестницы ft, чтобы она вписалась в помещение. Вместо фиксированной ширины
// марша перебирает допустимые (тип × крутизна × ширина [× радиус спирали])
// и возвращает до двух вариантов: ближайший к исходным параметрам и самый
// компактный (минимальная площадь). Если ни один не влезает — пусто.
func buildOtherTypes(ctx context.Context, cfg *engineering.StairConfiguration, ft engineering.FlightType, height engineering.Length, comfort float64, set *constraint.ConstraintSet, rw, rl, tol float64) []validation.Variation {
	cands := collectOtherType(ctx, cfg, ft, height, comfort, set, rw, rl, tol)
	if len(cands) == 0 {
		return nil
	}
	origW := cfg.Width.Millimeters()
	if origW <= 0 {
		origW = 900
	}
	origH := cfg.StepHeight.Millimeters()
	if origH <= 0 {
		origH = 180
	}
	// Индекс ближайшего к исходным параметрам. Для спирали учитывается и
	// наружный радиус (близость радиуса важнее для пользователя, чем выигрыш
	// в пару миллиметров высоты ступени).
	origR := cfg.OuterRadius.Millimeters()
	score := func(c otherCand) float64 {
		s := math.Abs(c.w-origW) + 3*math.Abs(c.h-origH)
		if origR > 0 {
			s += math.Abs(c.r - origR)
		}
		return s
	}
	best := 0
	bestScore := score(cands[0])
	for i, c := range cands[1:] {
		s := score(c)
		if s < bestScore {
			best, bestScore = i+1, s
		}
	}
	out := []validation.Variation{cands[best].v}
	// Самый компактный (минимальная площадь footprint) — если это не тот же
	// кандидат и конфиг другой (dedupe на верхнем уровне уберёт повторы).
	compact := 0
	minArea := cands[0].bb[0] * cands[0].bb[1]
	for i, c := range cands[1:] {
		a := c.bb[0] * c.bb[1]
		if a < minArea {
			compact, minArea = i+1, a
		}
	}
	if compact != best && cands[compact].v.Config != nil &&
		fmt.Sprintf("%v", cands[compact].v.Config) != fmt.Sprintf("%v", cands[best].v.Config) {
		out = append(out, cands[compact].v)
	}
	return out
}

// collectOtherType перебирает допустимые высоты ступени (вверх до верхней
// границы нормы — чем круче, тем короче марш) и ширины марша (вниз до
// абсолютного минимума нормы 300 мм), а для спирали — ещё и наружный радиус.
// Каждый кандидат прогоняется через Geometry Engine и оставляется, только
// если вписывается в помещение (с допуском tol).
func collectOtherType(ctx context.Context, cfg *engineering.StairConfiguration, ft engineering.FlightType, height engineering.Length, comfort float64, set *constraint.ConstraintSet, rw, rl, tol float64) []otherCand {
	origW := cfg.Width.Millimeters()
	if origW <= 0 {
		origW = 900
	}
	origH := cfg.StepHeight.Millimeters()
	if origH <= 0 {
		origH = 180
	}
	const hMin, hMax = 150.0, 200.0
	if origH < hMin {
		origH = hMin
	}
	if origH > hMax {
		origH = hMax
	}
	// Ниже абсолютного минимума нормы не опускаемся (300 мм), чтобы не
	// предлагать заведомо недопустимо узкие марши.
	minW := 300.0
	var out []otherCand
	for h := origH; h <= hMax+1e-6; h += 5 {
		for w := origW; w >= minW-1e-6; w -= 50 {
			if ft == engineering.FlightSpiral {
				for R := w + 500; R >= w+50; R -= 50 {
					if v, bb, ok := tryOtherAt(ctx, cfg, ft, height, comfort, set, rw, rl, tol, h, w, R); ok {
						out = append(out, otherCand{v: v, w: w, h: h, r: R, bb: bb})
					}
				}
			} else if v, bb, ok := tryOtherAt(ctx, cfg, ft, height, comfort, set, rw, rl, tol, h, w, 0); ok {
				out = append(out, otherCand{v: v, w: w, h: h, r: 0, bb: bb})
			}
		}
	}
	return out
}

// tryOtherAt строит конфигурацию типа ft с заданными высотой ступени h,
// шириной марша w (и наружным радиусом R для спирали), проверяет
// вписываемость через Geometry Engine и возвращает готовую Variation
// вместе с фактическими габаритами.
func tryOtherAt(ctx context.Context, cfg *engineering.StairConfiguration, ft engineering.FlightType, height engineering.Length, comfort float64, set *constraint.ConstraintSet, rw, rl, tol, h, w, R float64) (validation.Variation, [2]float64, bool) {
	clone := cloneCfg(cfg)
	clone.Flight = ft
	clone.Width = engineering.Length(w)
	// Тот же тип, что и у исходной лестницы — это «компактнее» вариант D,
	// а не замена типа; подбираем честные заголовки/описания.
	sameType := ft == cfg.Flight
	switch ft {
	case engineering.FlightStraight:
		title, desc := "C: прямая лестница (без площадки)", "Отказаться от поворотной площадки — один прямой марш."
		if sameType {
			title, desc = "D: сделать марш уже", "Уменьшить ширину марша той же прямой лестницы — она займёт меньше места в помещении."
		}
		r, err := solver.Solve(height, engineering.Length(h), comfort)
		if err != nil {
			return validation.Variation{}, [2]float64{}, false
		}
		applyStraight(clone, r)
		// StepHeight НЕ переопределяем вручную: applyStraight/applyTwoFlight
		// уже записали согласованное со солвером значение (целое число
		// ступеней). Принудительная установка h рвала тождество
		// comfort = 2·StepHeight + TreadDepth и делала конфиг невалидным для
		// geometry.Generate (варианты L/U/прямой-C не предлагались).
		return tryGenerateMeta(ctx, clone, set, rw, rl, tol, title, desc)
	case engineering.FlightLShape:
		title, desc := "C: L-образная лестница (с поворотом)", "Заменить на L-образную с площадкой — поворот экономит длину."
		if sameType {
			title, desc = "D: сделать марш уже (L)", "Уменьшить ширину марша и площадки той же L-образной лестницы."
		}
		n := int(0.5 + height.Millimeters()/h)
		if n < 2 {
			n = 2
		}
		n1 := n / 2
		if n1 < 1 {
			n1 = 1
		}
		wp := cfg.LandingWidth.Millimeters()
		minWp := math.Max(w*1.1, 600)
		if wp < minWp {
			wp = minWp
		}
		r, err := solver.SolveLShape(height, engineering.Length(h), n1, engineering.Length(wp), comfort)
		if err != nil {
			return validation.Variation{}, [2]float64{}, false
		}
		applyTwoFlight(clone, r.StepCount, r.StepHeight, r.TreadDepth, r.Angle, r.LowerRun)
		clone.LandingWidth = engineering.Length(wp)
		clone.LandingDepth = engineering.Length(wp)
		clone.LowerStepCount = n1
		// StepHeight НЕ переопределяем вручную (см. выше).
		return tryGenerateMeta(ctx, clone, set, rw, rl, tol, title, desc)
	case engineering.FlightUShape:
		title, desc := "C: П-образная лестница", "Заменить на П-образную — два параллельных марша с площадкой между ними."
		if sameType {
			title, desc = "D: сделать марш уже (П)", "Уменьшить ширину марша и площадки той же П-образной лестницы."
		}
		n := int(0.5 + height.Millimeters()/h)
		if n < 2 {
			n = 2
		}
		n1 := n / 2
		if n1 < 1 {
			n1 = 1
		}
		wp := cfg.LandingWidth.Millimeters()
		minWp := math.Max(w*1.1, 600)
		if wp < minWp {
			wp = minWp
		}
		r, err := solver.SolveUShape(height, engineering.Length(h), n1, engineering.Length(wp), comfort)
		if err != nil {
			return validation.Variation{}, [2]float64{}, false
		}
		applyTwoFlight(clone, r.StepCount, r.StepHeight, r.TreadDepth, r.Angle, r.LowerRun)
		clone.LandingWidth = engineering.Length(wp)
		clone.LandingDepth = engineering.Length(wp)
		clone.LowerStepCount = n1
		// StepHeight НЕ переопределяем вручную (см. выше).
		return tryGenerateMeta(ctx, clone, set, rw, rl, tol, title, desc)
	case engineering.FlightSpiral:
		title, desc := "C: спиральная лестница", "Заменить на спиральную — самая компактная, вписывается в узкое помещение."
		if sameType {
			title, desc = "D: спираль компактнее", "Уменьшить радиус и ширину спирали, чтобы вписаться в помещение."
		}
		if R <= w {
			return validation.Variation{}, [2]float64{}, false
		}
		r, err := solver.SolveSpiral(height, engineering.Length(h), engineering.Length(w), engineering.Length(R))
		if err != nil {
			return validation.Variation{}, [2]float64{}, false
		}
		clone.Flight = engineering.FlightSpiral
		// Apply записывает согласованные параметры марша (StepCount,
		// StepHeight, TreadDepth=WalkTread, Angle) — без единого источника
		// проступь в клоне остаётся 0 для конфигов без TreadDepth, и
		// кандидат ошибочно блокируется по норме проступи (GEO-TREAD-DEPTH).
		r.Apply(clone)
		clone.Width = engineering.Length(w)
		clone.OuterRadius = engineering.Length(R)
		clone.Length = engineering.Length(r.ArcLength)
		return tryGenerateMeta(ctx, clone, set, rw, rl, tol, title, desc)
	}
	return validation.Variation{}, [2]float64{}, false
}

// tryGenerate прогоняет кандидата через Geometry Engine, проверяет
// вписываемость в помещение И прохождение всех активных норм, и формирует
// Variation с конфигом формы. Вариант отдаётся, только если он и влезает в
// комнату (с допуском tol), и неблокирующий по нормам (bugfix: «сделать
// круче» ранее могло давать проступь вне 260–320 мм и блокировать расчёт
// при применении).
func tryGenerate(ctx context.Context, clone *engineering.StairConfiguration, set *constraint.ConstraintSet, rw, rl, tol float64, title, desc string) (validation.Variation, bool) {
	v, _, ok := tryGenerateMeta(ctx, clone, set, rw, rl, tol, title, desc)
	return v, ok
}

// tryGenerateMeta — как tryGenerate, но дополнительно возвращает фактические
// габариты (X, Y) из Geometry Engine для ранжирования «самый компактный».
func tryGenerateMeta(ctx context.Context, clone *engineering.StairConfiguration, set *constraint.ConstraintSet, rw, rl, tol float64, title, desc string) (validation.Variation, [2]float64, bool) {
	res, err := enggeo.Generate(ctx, clone)
	if err != nil {
		return validation.Variation{}, [2]float64{}, false
	}
	// Проверка норм: кандидат не должен быть блокирующим (проступь,
	// высота ступени, угол и пр.). Иначе вариант не предлагаем.
	// При set == nil (напр. тесты) проверка норм пропускается.
	if set != nil && validation.Validate(clone, set).Blocking {
		return validation.Variation{}, [2]float64{}, false
	}
	bb := res.Measurement.BoundingBox
	bx, by := bb.Max.X, bb.Max.Y
	if bx > rw+tol || by > rl+tol {
		return validation.Variation{}, [2]float64{}, false
	}
	summary := fmt.Sprintf("Габариты %.0f×%.0f мм, %d ступ., угол %.1f°", bx, by, clone.StepCount, clone.Angle.Degrees())
	if tol > fitEps {
		// «Спасительный» вариант — вписывается впритык (запас < 50 мм).
		// Честно помечаем, чтобы пользователь видел, что это почти впритык.
		summary += " · впритык к помещению (запас < 50 мм)"
	}
	return validation.Variation{
		ID:          title,
		Title:       title,
		Description: desc,
		Config:      formFromConfig(clone),
		Fits:        true,
		PassesNorms: true,
		Summary:     summary,
	}, [2]float64{bx, by}, true
}

// cloneCfg — глубокая копия конфигурации (поля-значения, перила/направления
// — строковые, тоже копируются по значению).
func cloneCfg(cfg *engineering.StairConfiguration) *engineering.StairConfiguration {
	c := *cfg
	return &c
}

// applyStraight записывает результат прямого марша, сбрасывая поля площадки.
func applyStraight(cfg *engineering.StairConfiguration, r solver.FlightResult) {
	cfg.Flight = engineering.FlightStraight
	cfg.StepCount = r.StepCount
	cfg.StepHeight = r.StepHeight
	cfg.TreadDepth = r.TreadDepth
	cfg.Length = r.Run
	cfg.StringerLength = r.Stringer
	cfg.Angle = r.Angle
	cfg.LandingWidth = engineering.Length(0)
	cfg.LandingDepth = engineering.Length(0)
	cfg.LowerStepCount = 0
	cfg.TurnKind = ""
	cfg.WinderCount = 0
}

// applyTwoFlight записывает общие для L/П-марша поля (ступени/угол/пролёт),
// сохраняя площадку и разбивку нижнего марша из исходной конфигурации.
func applyTwoFlight(cfg *engineering.StairConfiguration, n int, h, b engineering.Length, a engineering.Angle, run engineering.Length) {
	cfg.StepCount = n
	cfg.StepHeight = h
	cfg.TreadDepth = b
	cfg.Length = run
	cfg.Angle = a
}

// formFromConfig сериализует конфигурацию в поля формы конструктора
// (ключи совпадают с ConfigForm фронтенда, значения — строки), чтобы
// фронтенд мог слить вариант в текущий конфиг и пересчитать.
//
// ВАЖНО: значения нормируются в допустимые диапазоны фронтенда, иначе
// применение варианта (applyVariation) падает на validateForm и расчёт
// «не пересчитывается». Особенно критично comfortStepMM (шаг комфорта):
// оно вычисляется как 2·StepHeight + TreadDepth и для «круче» варианта
// (где StepHeight фиксируется отдельно от решения солвера) может выйти за
// предел 600–640 мм, хотя сама геометрия влезает. Клампим, чтобы не
// блокировать применение (бэкенд при расчёте всё равно пересоберёт проступь
// из stepHeightMM + comfortStepMM).
func formFromConfig(cfg *engineering.StairConfiguration) map[string]string {
	ff := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	stepH := clamp(cfg.StepHeight.Millimeters(), 150, 200)
	comfort := clamp(2*cfg.StepHeight.Millimeters()+cfg.TreadDepth.Millimeters(), 600, 640)
	// Для L/П-марша фронтенд требует landing ≥ 600 мм И ≥ ширины марша
	// (validateForm, min 600). Генератор вариаций мог получить площадку
	// уже 600, но на узких маршах (w*1.1 < 600) значение оказывалось ниже
	// порога → применение (applyVariation) падало на validateForm и расчёт
	// «не пересчитывался». Клампим к max(600, width), чтобы вариант
	// гарантированно применялся (бэкенд пересоберёт геометрию из ширины
	// площадки; для П-марша глубина площадки солвером не используется).
	lw := cfg.LandingWidth.Millimeters()
	ld := cfg.LandingDepth.Millimeters()
	if cfg.Flight == engineering.FlightLShape || cfg.Flight == engineering.FlightUShape {
		minLanding := math.Max(600, cfg.Width.Millimeters())
		lw = clamp(lw, minLanding, 5000)
		ld = clamp(ld, minLanding, 5000)
	}
	m := map[string]string{
		"flight":              string(cfg.Flight),
		"heightMm":            ff(cfg.Height.Millimeters()),
		"stepHeightMM":        ff(stepH),
		"comfortStepMM":       ff(comfort),
		"widthMM":             ff(cfg.Width.Millimeters()),
		"landingWidthMM":      ff(lw),
		"landingDepthMM":      ff(ld),
		"roomWidthMM":         ff(cfg.RoomWidth.Millimeters()),
		"roomLengthMM":        ff(cfg.RoomLength.Millimeters()),
		"lowerStepCountMM":    strconv.Itoa(cfg.LowerStepCount),
		"clearanceMm":         ff(cfg.Clearance.Millimeters()),
		"railingMm":           ff(cfg.RailingHeight.Millimeters()),
		"stringerThicknessMm": ff(cfg.StringerThickness.Millimeters()),
		"stepThicknessMm":     ff(cfg.StepThickness.Millimeters()),
		"turnKind":            string(cfg.TurnKind),
		"winderCount":         strconv.Itoa(cfg.WinderCount),
		"outerRadiusMM":       ff(cfg.OuterRadius.Millimeters()),
		"direction":           string(cfg.Direction),
		"railingLower":        string(cfg.RailingLower),
		"railingLanding":      string(cfg.RailingLanding),
		"railingUpper":        string(cfg.RailingUpper),
		"railing":             string(cfg.Railing),
		"spiralDirection":     string(cfg.SpiralDirection),
	}
	// Вариация несёт только те поля, что реально задаёт: пустые значения
	// (напр. railing/direction/сегменты перил для варианта, где они не
	// применимы) удаляем, чтобы не затирать выбор пользователя при слиянии
	// конфига на клиенте (clobbering → невалидная форма → пересчёт падает).
	for k, v := range m {
		if v == "" {
			delete(m, k)
		}
	}
	return m
}

// clamp ограничивает v диапазоном [lo, hi]. Используется при сериализации
// вариантов, чтобы значения гарантированно проходили фронтенд-валидацию.
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// dedupe — убирает дубликаты вариантов по сериализованному конфигу.
func dedupe(in []validation.Variation) []validation.Variation {
	seen := map[string]bool{}
	out := make([]validation.Variation, 0, len(in))
	for _, v := range in {
		key := fmt.Sprintf("%v", v.Config)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}
