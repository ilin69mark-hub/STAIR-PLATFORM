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

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	"stairplatform/internal/engine/constraint"
	enggeo "stairplatform/internal/engine/geometry"
	engmfg "stairplatform/internal/engine/manufacturing"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

// MaxSuggestions — сколько готовых вариантов отдаёт советник на нарушение.
const MaxSuggestions = 4

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
	// TurnKind — тип поворота (площадка/поворотные ступени, только U).
	TurnKind engineering.TurnKind
	// WinderCount — число поворотных ступеней (для U в режиме поворота).
	WinderCount int
	LandingMm   float64 // Wp — ширина площадки/просвета (для L/U)
	// WidthMm — ширина марша W.
	WidthMm float64
	// Остальные параметры — чтобы проверять кандидата по всем нормам.
	ClearanceMm     float64
	RailingMm       float64
	StringerThickMm float64
	// StepThicknessMm — толщина проступи st: определяет вертикальный
	// габарит косоура (H − st) для оценки раскроя.
	StepThicknessMm float64
	// Material — выбранный конструктором материал (код каталога MFG-0005);
	// пустое значение — автоназначение по толщине (как в Manufacture).
	Material dommfg.MaterialCode
}

// advisorMaterial возвращает материал, по которому оценивается раскрой:
// выбранный конструктором (если задан) или назначенный по толщине косоура
// (та же политика, что в Manufacture).
func advisorMaterial(in Input) (dommfg.MaterialCode, error) {
	if in.Material != "" {
		reg, err := engmfg.DefaultMaterialRegistry()
		if err != nil {
			return "", fmt.Errorf("advisor: default registry: %w", err)
		}
		if _, ok := reg.Find(in.Material); ok {
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
		it.Guide = validation.RenderGuide(rule.Advice.Guide, *it, in.HeightMm)
		if rule.Advice.Suggest {
			it.Suggestions = suggestions
		}
	}
	if infeasible && !hasIssue(vr.Issues, constraint.MFG_SHEET) {
		vr.Issues = append(vr.Issues, manufacturingIssue(in, worst))
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

// manufacturingIssue — блокирующий issue о невозможности изготовления.
//
// Подсказка зависит от материала, потому что ограничения разные:
//
// Сталь режется лазером по рабочему полю стола 3000×1500. По длине деталь
// режется кусками со сварным стыком «шип-паз», а вот по ширине не режется:
// поперечный рез отделил бы зубья гребёнки от спинки.
//
// Дерево режется на другом станке и сварным стыком не собирается, поэтому его
// ограничение — каталог листов, и подсказка ссылается на самый крупный лист
// материала.
func manufacturingIssue(in Input, worst [2]float64) validation.Issue {
	guide := fmt.Sprintf(
		"Косоур %.0f×%.0f мм не помещается в рабочее поле лазерного стола %.0f×%.0f мм при высоте подъёма %.0f мм. По длине длинный косоур режется кусками со сварным стыком «шип-паз», но ширина превышает поле вместе с отступом от края.",
		worst[0], worst[1], dommfg.BlankLengthMM, dommfg.BlankWidthMM, in.HeightMm)
	if length, width, ok := advisorLargestSheet(in); ok {
		guide = fmt.Sprintf(
			"Косоур %.0f×%.0f мм не влезает ни в один лист: самый крупный лист материала %.0f×%.0f мм при высоте подъёма %.0f мм. Дерево режется на станке и сварным стыком не собирается — укоротите марш.",
			worst[0], worst[1], length, width, in.HeightMm)
	}
	return validation.Issue{
		Code:     constraint.MFG_SHEET,
		Severity: constraint.SeverityError,
		Element:  "stringer",
		Message:  "косоур не помещается в поле лазерного стола",
		Param:    "Изготовление",
		Guide:    guide,
	}
}

// advisorLargestSheet — самый крупный лист древесного материала, если
// материал вовсе не металл. ok=false для стали: у неё ограничение не каталог,
// а поле лазерного стола.
func advisorLargestSheet(in Input) (length, width float64, ok bool) {
	material, err := advisorMaterial(in)
	if err != nil {
		return 0, 0, false
	}
	if engmfg.IsLaserMaterial(material) {
		return 0, 0, false
	}
	registry, err := engmfg.DefaultStockSheetRegistry()
	if err != nil {
		return 0, 0, false
	}
	sheets := registry.SheetsFor(material)
	if len(sheets) == 0 {
		return 0, 0, false
	}
	for _, sh := range sheets {
		if l, w := sh.Length.Millimeters(), sh.Width.Millimeters(); l > length {
			length, width = l, w
		}
	}
	return length, width, length > 0
}

// renderGuide перенесён в validation.RenderGuide (SRC: владеет шаблонами
// подсказок в constraint; валидация применяет Advice на своём этапе).

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

	anyPass := false
	firstPass := false
	id := func(s validation.Suggestion) string {
		if in.Flight == engineering.FlightStraight {
			return fmt.Sprintf("s%d", s.StepCount)
		}
		return fmt.Sprintf("s%d-%d", s.StepCount, s.LowerStepCount)
	}
	seen := map[string]bool{}
	suggestible := true

	// Кандидаты числа ступеней — разнесены по всему допустимому окну
	// (минимальное/максимальное/промежуточные), а не подряд идущие:
	// так пользователь получает ДЕЙСТВИТЕЛЬНО разные варианты (самый
	// крутой, самый пологий, средние), а не почти одинаковые n±1.
	for _, n := range spreadInts(nStart, nEnd, MaxSuggestions) {
		if len(out) >= MaxSuggestions {
			break
		}
		target := engineering.Length(in.HeightMm / float64(n))
		if in.Flight == engineering.FlightUShape && in.TurnKind == engineering.TurnWinder {
			// П-образный с поворотными ступенями: перебираем n1 и число
			// поворотных ступеней nw (≥3), решатель учитывает их в общем n.
			if in.LandingMm <= 0 {
				break
			}
			for _, n1 := range lowerCandidates(in, n) {
				for _, nw := range winderCandidates(in, n, n1) {
					res, err := solver.SolveUShapeWinder(
						engineering.Length(in.HeightMm), target, n1, nw,
						engineering.Length(in.LandingMm), in.ComfortMm)
					if err != nil {
						continue
					}
					if !passes(in, set, res.StepCount, res.StepHeight, res.TreadDepth, res.Angle) {
						continue
					}
					anyPass = true
					rects := [][2]float64{
						// Заготовка косоура-гребёнки имеет постоянную глубину листа 300 мм
						// (engine/geometry.StringerPlateDepthMM) и длину по склону. Раньше
						// здесь стояло StringerExtent(высота подъёма, толщина ступени) —
						// это высота МАРША, а не размер детали: для подъёма 2700 мм
						// получался «косоур» 4050×2700, который не влезал в поле
						// лазерного стола по ширине и отсекал все варианты.
						{res.LowerRun.Millimeters(), enggeo.StringerPlateDepthMM},
						{res.UpperRun.Millimeters(), enggeo.StringerPlateDepthMM},
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
						WinderCount:  res.WinderCount,
						StepHeightMm: res.StepHeight.Millimeters(),
						TreadDepthMm: res.TreadDepth.Millimeters(),
						AngleDeg:     res.Angle.Degrees(),
					}
					iid := fmt.Sprintf("s%d-%d-w%d", res.StepCount, res.LowerStepCount, res.WinderCount)
					if !seen[iid] {
						seen[iid] = true
						out = append(out, it)
					}
				}
			}
			continue
		}
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
					{res.LowerRun.Millimeters(), enggeo.StringerPlateDepthMM},
					{res.UpperRun.Millimeters(), enggeo.StringerPlateDepthMM},
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
			{res.Run.Millimeters(), enggeo.StringerPlateDepthMM},
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

// manufactureFeasible проверяет, что прямоугольники кандидата изготовляемы.
//
// Для СТАЛИ ограничение — РАБОЧЕЕ ПОЛЕ лазерного стола 3000×1500 с учётом
// отступа и зазора по толщине листа. Раньше проверка шла против каталога
// листов (до 10400×6200): деталь 5200 мм считалась вместимой, хотя стол
// 3000×1500 и её можно только разрезать на куски со сварным стыком. Теперь
// кандидат, который нельзя разрезать, отсеивается сразу.
//
// Для ДЕРЕВА ограничение — каталог листов: дерево режется на другом станке, и
// поле лазерного стола к нему отношения не имеет.
func manufactureFeasible(in Input, rects ...[2]float64) bool {
	// Толщина листа для шпатки зазоров: косоур режется из листа своей
	// толщины, поэтому берём толщину косоура.
	thickness := in.StringerThickMm
	if thickness <= 0 {
		thickness = in.StepThicknessMm
	}
	if thickness <= 0 {
		return true // данных мало — не блокируем совет
	}
	// Материал определяет, чем режут: сталь — лазером по полю стола и длинный
	// косоур режется кусками, дерево — станком по каталогу листов целиком.
	material, err := advisorMaterial(in)
	if err != nil {
		return true // каталог не собрался — не блокируем совет
	}
	for _, r := range rects {
		if !engmfg.SplittableFeasibleFor(material, thickness, r[0], r[1]) {
			return false
		}
	}
	return true
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

// winderCandidates — порядок перебора числа поворотных ступеней nw для
// варианта с n ступенями и нижним маршем n1: сначала исходное (≥3), затем
// вокруг; отбрасываем значения, оставляющие верхнему маршу <1 ступени
// (n2 = n − n1 − nw ≥ 1).
func winderCandidates(in Input, n, n1 int) []int {
	orig := in.WinderCount
	if orig < 3 {
		orig = 3
	}
	seen := map[int]bool{}
	var seq []int
	for _, v := range []int{orig, orig + 1, orig - 1, orig + 2, orig - 2} {
		if v < 3 || n-n1-v < 1 || seen[v] {
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

// spreadInts возвращает до k равномерно разнесённых целых чисел из
// диапазона [start, end] включительно (в том числе концы), чтобы набор
// кандидатов охватывал весь допустимый диапазон, а не был подряд идущим.
func spreadInts(start, end, k int) []int {
	if end < start {
		return nil
	}
	n := end - start + 1
	if n <= k {
		out := make([]int, 0, n)
		for i := start; i <= end; i++ {
			out = append(out, i)
		}
		return out
	}
	out := make([]int, 0, k)
	step := float64(n-1) / float64(k-1)
	for i := 0; i < k; i++ {
		out = append(out, start+int(math.Round(float64(i)*step)))
	}
	return out
}
