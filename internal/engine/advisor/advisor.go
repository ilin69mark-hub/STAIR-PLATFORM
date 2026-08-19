// Package advisor — советник по исправлению блокирующих нарушений
// (EDR-0003, BC-003): при blocking-валидации ищет рядом с текущей
// конфигурацией 1–3 готовых варианта, которые проходят все активные
// нормы, и поясняет пользователю, какое поле и как поправить.
// Советник детерминирован, не изменяет модель и переиспользует
// формулы решателя (solver.Solve / SolveLShape) как единый источник.
package advisor

import (
	"fmt"
	"math"
	"strings"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/constraint"
	enggeo "stairplatform/internal/engine/geometry"
	engmfg "stairplatform/internal/engine/manufacturing"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

// MaxSuggestions — сколько готовых вариантов отдаёт советник на нарушение.
const MaxSuggestions = 3

// Input — пользовательская конфигурация, нужная советнику. Берётся
// ДО зануления полей blocking-валидацией в checked-функциях solver.
type Input struct {
	Flight   engineering.FlightType
	HeightMm float64 // H — высота подъёма
	// TargetStepMm — целевая высота ступени h0 (пользовательский ввод).
	TargetStepMm float64
	// ComfortMm — шаг комфорта S (600–640).
	ComfortMm float64
	// LowerStepCount — число ступеней нижнего марша (для L/U), n1.
	LowerStepCount int
	LandingMm      float64 // Wp — ширина площадки (для L/U)
	// WidthMm — ширина марша W (для спирали: зазор от колонны до кромки).
	WidthMm float64
	// OuterRadiusMm — наружный радиус спирали R (для спирали).
	OuterRadiusMm float64
	// Остальные параметры — чтобы проверять кандидата по всем нормам.
	ClearanceMm     float64
	RailingMm       float64
	StringerThickMm float64
	// Material — выбранный конструктором материал (код каталога MFG-0005);
	// пустое значение — автоназначение по толщине (как в Manufacture).
	Material dommfg.MaterialCode
}

// advisorMaterial возвращает материал, по которому оценивается раскрой:
// выбранный конструктором (если задан) или назначенный по толщине косоура
// (та же политика, что в Manufacture).
func advisorMaterial(in Input) (dommfg.MaterialCode, error) {
	if in.Material != "" {
		if _, ok := engmfg.DefaultMaterialRegistry().Find(in.Material); ok {
			return in.Material, nil
		}
	}
	return engmfg.DefaultMaterialForThickness(in.StringerThickMm)
}

// Advise дополняет blocking-результат валидации конкретными
// рекомендациями (Param/Guide/Suggestions) из активного ConstraintSet
// (BC-003): тексты подсказок и признак «перебирать варианты» хранятся
// в правилах, советник лишь подставляет значения и генерирует варианты.
// Для не-blocking результата модель не меняется. Советник лишь обогащает
// Issue, не изменяя Blocking/Valid результата.
func Advise(in Input, set *constraint.ConstraintSet, vr validation.Result) validation.Result {
	if !vr.Blocking || set == nil {
		return vr
	}
	// Варианты геометрии считаются один раз: результат одинаков для всех
	// правил с Suggest=true, а флаг infeasible говорит, что в нормо-диапазоне
	// есть конфигурации, но ни одна не влезает на стандартный лист (MFG-0012).
	suggestions, infeasible, worst := geometry(in, set)
	for i := range vr.Issues {
		it := &vr.Issues[i]
		rule, ok := set.Active(it.Code)
		if !ok || rule.Advice == nil {
			// Правило без подсказки — остаётся только базовый Fix.
			continue
		}
		it.Param = rule.Advice.Param
		it.Guide = renderGuide(rule.Advice.Guide, *it, in)
		if rule.Advice.Suggest {
			it.Suggestions = suggestions
		}
	}
	if infeasible && !hasIssue(vr.Issues, constraint.MFG_SHEET) {
		if in.Flight == engineering.FlightSpiral {
			vr.Issues = append(vr.Issues, spiralInfeasibleIssue(in))
		} else {
			vr.Issues = append(vr.Issues, manufacturingIssue(in, worst))
		}
	}
	return vr
}

// hasIssue проверяет наличие issue с указанным кодом.
func hasIssue(issues []validation.Issue, code constraint.RuleCode) bool {
	for _, it := range issues {
		if it.Code == code {
			return true
		}
	}
	return false
}

// manufacturingIssue — блокирующий issue о невозможности изготовления:
// косоур не помещается на стандартный лист при текущем каталоге (MFG-0012).
func manufacturingIssue(in Input, worst [2]float64) validation.Issue {
	material, err := advisorMaterial(in)
	if err != nil {
		material = dommfg.MaterialCode("STEEL-S235")
	}
	maxLen, maxWid, ok := engmfg.LargestStockSheet(engmfg.DefaultStockSheetRegistry(), material)
	if !ok {
		maxLen, maxWid = 6000, 3000
	}
	return validation.Issue{
		Code:     constraint.MFG_SHEET,
		Severity: constraint.SeverityError,
		Element:  "stringer",
		Message:  "косоур не помещается на стандартный лист",
		Param:    "Изготовление",
		Guide: fmt.Sprintf(
			"Косоур %.0f×%.0f мм (рез %d мм) не помещается на стандартный лист (макс. %.0f×%.0f мм) при высоте подъёма %.0f мм. Изготовление при текущем каталоге листов невозможно.",
			worst[0], worst[1], int(engmfg.DefaultKerf), maxLen, maxWid, in.HeightMm),
	}
}

// spiralInfeasibleIssue — блокирующий issue о невозможности подобрать
// винтовую лестницу, проходящую нормы, ни при какой ширине/радиусе.
func spiralInfeasibleIssue(in Input) validation.Issue {
	return validation.Issue{
		Code:     constraint.GEO_SPIRAL_TREAD,
		Severity: constraint.SeverityError,
		Element:  "configuration",
		Message:  "винтовая лестница не проходит нормы",
		Param:    "Ширина марша",
		Guide: fmt.Sprintf(
			"При высоте подъёма %.0f мм не удаётся подобрать винтовую лестницу, проходящую нормы (проступь, шаг комфорта, угол 30–45°). Уменьшите высоту подъёма или выберите другой тип лестницы.",
			in.HeightMm),
	}
}

// renderGuide подставляет значения в шаблон подсказки правила.
// Плейсхолдеры: {value} — значение нарушенного поля (мм/ед. мм, целое),
// {angle} — угол наклона (градусы, %.1f), {min}/{max} — границы нормы,
// {height} — высота подъёма конфигурации.
func renderGuide(tpl string, it validation.Issue, in Input) string {
	repl := strings.NewReplacer(
		"{value}", fmt.Sprintf("%.0f", it.Value),
		"{angle}", fmt.Sprintf("%.1f", it.Value),
		"{min}", fmt.Sprintf("%.0f", it.Min),
		"{max}", fmt.Sprintf("%.0f", it.Max),
		"{height}", fmt.Sprintf("%.0f", in.HeightMm),
	)
	return repl.Replace(tpl)
}

// geometry перебирает число ступеней в диапазоне, выведенном из нормы
// на высоту ступени (n ≈ H / [maxH..minH]), и возвращает до
// MaxSuggestions вариантов, проходящих все активные нормы и помещающихся
// на стандартный лист (MFG-0012). Для L/U учитывается также число
// ступеней нижнего марша. Возвращает infeasible=true, когда в диапазоне
// были проходящие нормы конфигурации, но ни одна не влезает на лист
// (косоур), и worst — прямоугольник первой такой конфигурации.
func geometry(in Input, set *constraint.ConstraintSet) (out []validation.Suggestion, infeasible bool, worst [2]float64) {
	if in.HeightMm <= 0 || in.ComfortMm <= 0 {
		return nil, false, worst
	}
	rule, ok := set.Active(constraint.GEO_STEP_HEIGHT)
	if !ok || !rule.Range.HasMin || !rule.Range.HasMax {
		return nil, false, worst
	}
	// Окно: n, при котором h=H/n попадает в норму [150..200] мм.
	nStart := mathMax(int(math.Ceil(in.HeightMm/rule.Range.Max))-2, 2)
	nEnd := int(math.Floor(in.HeightMm/rule.Range.Min)) + 2

	if in.Flight == engineering.FlightSpiral {
		return spiralGeometry(in, set, nStart, nEnd)
	}

	anyPass := false
	firstPass := false
	id := func(s validation.Suggestion) string {
		if in.Flight == engineering.FlightStraight || in.Flight == engineering.FlightSpiral {
			return fmt.Sprintf("s%d", s.StepCount)
		}
		return fmt.Sprintf("s%d-%d", s.StepCount, s.LowerStepCount)
	}
	seen := map[string]bool{}
	suggestible := in.Flight != engineering.FlightSpiral

	for n := nStart; n <= nEnd && len(out) < MaxSuggestions; n++ {
		target := engineering.Length(in.HeightMm / float64(n))
		if in.Flight == engineering.FlightLShape || in.Flight == engineering.FlightUShape {
			if in.LandingMm <= 0 {
				break
			}
			for _, n1 := range lowerCandidates(in, n) {
				res, err := solver.SolveLShape(
					engineering.Length(in.HeightMm), target, n1,
					engineering.Length(in.LandingMm), in.ComfortMm)
				if err != nil {
					continue
				}
				if !passes(in, set, res.StepCount, res.StepHeight, res.TreadDepth, res.Angle) {
					continue
				}
				anyPass = true
				rects := [][2]float64{
					{res.LowerRun.Millimeters(), res.LowerHeight.Millimeters() + enggeo.StringerHeel},
					{res.UpperRun.Millimeters(), res.UpperHeight.Millimeters() + enggeo.StringerHeel},
				}
				if !firstPass {
					worst = largestRect(rects)
					firstPass = true
				}
				if !manufactureFeasible(in, rects...) {
					continue
				}
				it := validation.Suggestion{
					StepCount: res.StepCount, LowerStepCount: res.LowerStepCount,
					StepHeightMm: res.StepHeight.Millimeters(),
					TreadDepthMm: res.TreadDepth.Millimeters(),
					AngleDeg:     res.Angle.Degrees(),
				}
				if !seen[id(it)] {
					seen[id(it)] = true
					out = append(out, it)
				}
			}
			continue
		}
		res, err := solver.Solve(engineering.Length(in.HeightMm), target, in.ComfortMm)
		if err != nil {
			continue
		}
		if !passes(in, set, res.StepCount, res.StepHeight, res.TreadDepth, res.Angle) {
			continue
		}
		anyPass = true
		rects := [][2]float64{
			{res.Run.Millimeters(), in.HeightMm + enggeo.StringerHeel},
		}
		if !firstPass {
			worst = largestRect(rects)
			firstPass = true
		}
		if suggestible && !manufactureFeasible(in, rects...) {
			continue
		}
		it := validation.Suggestion{
			StepCount:    res.StepCount,
			StepHeightMm: res.StepHeight.Millimeters(),
			TreadDepthMm: res.TreadDepth.Millimeters(),
			AngleDeg:     res.Angle.Degrees(),
		}
		if !seen[id(it)] {
			seen[id(it)] = true
			out = append(out, it)
		}
	}
	if anyPass && len(out) == 0 {
		infeasible = true
	}
	return out, infeasible, worst
}

// spiralGeometry подбирает готовые варианты винтовой лестницы (EDR-0007),
// проходящие нормы: перебирает число ступеней n из окна высоты ступени и
// для каждого n по замкнутой формуле выводит допустимые ширину W и наружный
// радиус R. Связи (EDR-0007 §4): δ=2π/n, bWalk=δ·(R−W/3), R=W/3+bWalk/δ.
// Кандидата верифицирует solver.SolveSpiral (категоричный источник).
// Возвращает до MaxSuggestions вариантов; infeasible=true — если ни один
// вариант не проходит нормы ни при какой ширине марша.
func spiralGeometry(in Input, set *constraint.ConstraintSet, nStart, nEnd int) (out []validation.Suggestion, infeasible bool, worst [2]float64) {
	if in.WidthMm <= 0 || in.OuterRadiusMm <= 0 {
		return nil, false, worst
	}
	curW := in.WidthMm
	curR := in.OuterRadiusMm
	seen := map[string]bool{}
	anyPass := false
	for n := nStart; n <= nEnd && len(out) < MaxSuggestions; n++ {
		h := in.HeightMm / float64(n)
		delta := 2 * math.Pi / float64(n)
		// Интервал проступи по линии хода по нормам: проступь 260–320,
		// шаг комфорта 600–640, угол наклона 30–45° (bWalk ∈ [h, h·tan60°]).
		blow := mathMax3f(260, 600-2*h, h)
		bhigh := mathMin3f(320, 640-2*h, h*math.Tan(math.Pi/3))
		if blow > bhigh {
			continue
		}
		// Диапазон ширины марша, при котором проступи у колонны/кромки в норме:
		// bIn = bWalk − (2W/3)·δ ≥ 100 → W ≤ 3(bWalk−100)/(2δ);
		// bOut = bWalk + (W/3)·δ ≥ 250 → W ≥ 3(250−bWalk)/δ.
		wMin := mathMax3f(1, 3*(250-blow)/delta, 0)
		wMax := mathMin3f(3*(bhigh-100)/(2*delta), 3*bhigh/(2*delta), 2000)
		if wMin > wMax {
			continue
		}
		for _, w := range spiralWidthCandidates(curW, wMin, wMax) {
			// Нижняя граница bWalk с учётом проступей колонны/кромки и радиуса.
			bLo := mathMax3f(blow, 100+(2.0/3.0)*w*delta, 250-delta*w/3)
			if bLo > bhigh {
				continue
			}
			// Проступь, дающая радиус ближе всего к текущему у пользователя.
			target := (curR - w/3) * delta
			bWalk := clampF(target, bLo, bhigh)
			R := bWalk/delta + w/3
			if R <= w {
				continue
			}
			res, err := solver.SolveSpiral(
				engineering.Length(in.HeightMm),
				engineering.Length(h),
				engineering.Length(w),
				engineering.Length(R),
			)
			if err != nil {
				continue
			}
			if !passes(in, set, res.StepCount, res.StepHeight, res.WalkTread, res.Angle) {
				continue
			}
			anyPass = true
			it := validation.Suggestion{
				StepCount:     res.StepCount,
				StepHeightMm:  res.StepHeight.Millimeters(),
				TreadDepthMm:  res.WalkTread.Millimeters(),
				AngleDeg:      res.Angle.Degrees(),
				OuterRadiusMm: res.OuterRadius.Millimeters(),
				WidthMm:       w,
			}
			id := fmt.Sprintf("s%d-r%.0f-w%.0f", it.StepCount, it.OuterRadiusMm, it.WidthMm)
			if !seen[id] {
				seen[id] = true
				out = append(out, it)
				if len(out) >= MaxSuggestions {
					break
				}
			}
		}
	}
	if anyPass && len(out) == 0 {
		infeasible = true
	}
	return out, infeasible, worst
}

// spiralWidthCandidates — кандидаты ширины марша для спирали в допустимом
// диапазоне [wMin, wMax]: прежде всего текущая ширина пользователя, затем
// середина диапазона и типовые ширины винтовых лестниц (от большей к
// меньшей), округлённые до 50 мм.
func spiralWidthCandidates(cur, wMin, wMax float64) []float64 {
	seen := map[float64]bool{}
	var seq []float64
	add := func(v float64) {
		v = math.Round(v/50) * 50
		if v < wMin || v > wMax || seen[v] {
			return
		}
		seen[v] = true
		seq = append(seq, v)
	}
	add(cur)
	add((wMin + wMax) / 2)
	for _, v := range []float64{1200, 1100, 1000, 900, 800, 700} {
		add(v)
	}
	return seq
}

func mathMax3f(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }
func mathMin3f(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }

// clampF ограничивает значение диапазоном [lo, hi].
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// largestRect возвращает прямоугольник с наибольшей площадью.
func largestRect(rects [][2]float64) [2]float64 {
	best := rects[0]
	for _, r := range rects[1:] {
		if r[0]*r[1] > best[0]*best[1] {
			best = r
		}
	}
	return best
}

// manufactureFeasible проверяет, что все прямоугольники кандидата
// помещаются на стандартный лист материала косоура с учётом kerf.
// Для спирали проверка не выполняется (советник по ней не предлагает
// варианты — фронтенд скрывает кнопки).
func manufactureFeasible(in Input, rects ...[2]float64) bool {
	if in.Flight == engineering.FlightSpiral {
		return true
	}
	material, err := advisorMaterial(in)
	if err != nil {
		return true
	}
	mfrects := make([]engmfg.Rect, 0, len(rects))
	for _, r := range rects {
		mfrects = append(mfrects, engmfg.Rect{Length: r[0], Width: r[1]})
	}
	return engmfg.SheetFeasible(engmfg.DefaultStockSheetRegistry(), material, engmfg.DefaultKerf, mfrects...)
}

// passes проверяет кандидата по всем активным нормам конфигурации.
func passes(in Input, set *constraint.ConstraintSet, stepCount int, sh, b engineering.Length, angle engineering.Angle) bool {
	cand := &engineering.StairConfiguration{
		StepCount:         stepCount,
		StepHeight:        sh,
		TreadDepth:        b,
		Angle:             angle,
		Clearance:         engineering.Length(in.ClearanceMm),
		StringerThickness: engineering.Length(in.StringerThickMm),
		RailingHeight:     engineering.Length(in.RailingMm),
	}
	return !validation.Validate(cand, set).Blocking
}

// lowerCandidates — порядок перебора числа ступеней нижнего марша для
// варианта с n ступенями: сначала исходное значение, затем вокруг.
func lowerCandidates(in Input, n int) []int {
	orig := in.LowerStepCount
	if orig < 1 || orig > n-1 {
		orig = int(math.Round(float64(n) / 2))
		if orig < 1 {
			orig = 1
		}
	}
	seen := map[int]bool{}
	var seq []int
	for _, v := range []int{orig, orig - 1, orig + 1, orig - 2, orig + 2} {
		if v < 1 || v > n-1 || seen[v] {
			continue
		}
		seen[v] = true
		seq = append(seq, v)
	}
	return seq
}

func mathMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
