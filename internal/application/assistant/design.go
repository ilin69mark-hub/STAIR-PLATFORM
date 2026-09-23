package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

// DesignPriority — приоритет рекомендаций Design Assistant (D1).
type DesignPriority string

// Приоритеты рекомендации.
const (
	PriorityPrice    DesignPriority = "price"    // по цене для клиента (по умолчанию)
	PriorityCost     DesignPriority = "cost"     // по себестоимости
	PriorityMaterial DesignPriority = "material" // по стоимости материала
	PriorityComfort  DesignPriority = "comfort"  // по шагу комфорта 2h+b → 600–640
)

// Valid проверяет известность приоритета.
func (p DesignPriority) Valid() bool {
	switch p {
	case PriorityPrice, PriorityCost, PriorityMaterial, PriorityComfort:
		return true
	}
	return false
}

// target отображает приоритет в целевой метрику оптимизации (EDR-0032).
// Комфорт не является целью оптимизатора (оптимум всегда по цене), поэтому
// для comfort оптимизируем по цене, а ранжирование вариантов ведём по шагу
// комфорта.
func (p DesignPriority) target() stair.OptimizeTarget {
	switch p {
	case PriorityCost:
		return stair.TargetCost
	case PriorityMaterial:
		return stair.TargetMaterial
	case PriorityPrice:
		return stair.TargetPrice
	case PriorityComfort: // comfort
		return stair.TargetPrice
	default:
		return stair.TargetPrice
	}
}

// DesignPreferences — настройки рекомендации Design Assistant.
type DesignPreferences struct {
	// Priority — приоритет; "" ⇒ PriorityPrice.
	Priority DesignPriority
}

// DesignRequest — запрос Design Assistant (D1): конфигурация + предпочтения.
type DesignRequest struct {
	Config      stair.Config
	Preferences DesignPreferences
	// ProjectID — скоуп conversation-memory (S-135, AI-0006); пустая —
	// память для запроса не используется. Проект должен принадлежать tenant
	// (проверяется на уровне хранилища).
	ProjectID string
	// HistoryLimit — число последних сообщений диалога в контекст
	// (0 → DefaultHistoryLimit=10, cap MaxHistoryLimit=50; <0 → без истории).
	HistoryLimit int
}

// designExpert — эксперт D1 (EDR-0036): детерминированный перебор типов
// марша (порядок фиксирован), оптимизация каждого через тул, ранжирование
// по приоритету, рекомендация + альтернативы. Никакой модели: структурный
// ответ строится только из результатов движка.
type designExpert struct{}

// flightOrders — фиксированный порядок перебора типов марша (детерминизм).
var flightOrders = []engineering.FlightType{
	engineering.FlightStraight,
	engineering.FlightLShape,
	engineering.FlightUShape,
	engineering.FlightSpiral,
}

// flightTitle — русское название типа марша.
func flightTitle(f engineering.FlightType) string {
	switch f {
	case engineering.FlightStraight:
		return "прямой марш"
	case engineering.FlightLShape:
		return "L-образный марш (с площадкой)"
	case engineering.FlightUShape:
		return "П-образный марш (с двумя маршами и площадкой)"
	case engineering.FlightSpiral:
		return "спиральный марш (винтовой)"
	default:
		return "прямой марш"
	}
}

// candidate — результат оптимизации конкретного типа марша.
type candidate struct {
	flight  engineering.FlightType
	opt     *stair.OptimizeResult
	price   float64 // цена итоговая, мажорные единицы
	comfort float64 // шаг комфорта S = 2h + b, мм
	score   float64 // метрика ранжирования (меньше = лучше)
}

// analyze реализует план D1: перебор → оптимизация → ранжирование → ответ.
func (designExpert) analyze(ctx context.Context, tools *Tools, req Request) (*Response, *intent, error) {
	d, ok := req.(DesignRequest)
	if !ok {
		return nil, nil, fmt.Errorf("%w: expected DesignRequest, got %T", ErrInvalid, req)
	}
	prio := d.Preferences.Priority
	if prio == "" {
		prio = PriorityPrice
	}
	if !prio.Valid() {
		return nil, nil, fmt.Errorf("%w: unknown design priority %q", ErrInvalid, prio)
	}
	if err := tools.ValidateConfig(d.Config); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}

	notes := []string{}
	cands := make([]candidate, 0, len(flightOrders))
	for _, f := range flightOrders {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		cfg := d.Config
		cfg.Flight = f
		// Спираль требует R > W; если пользователь не задал радиус —
		// подставляем пробный R = 2·W (детерминированная эвристика).
		if f == engineering.FlightSpiral && cfg.OuterRadius.Millimeters() <= cfg.Width.Millimeters() {
			cfg.OuterRadius = engineering.Length(2 * cfg.Width.Millimeters())
			notes = append(notes, fmt.Sprintf(
				"спираль: наружный радиус R не задан, принят R=2·W=%g мм", cfg.OuterRadius.Millimeters()))
		}

		opt, err := tools.Optimize(ctx, cfg, stair.Options{}, stair.OptimizeRequest{Target: prio.target()})
		if err != nil {
			// Тип марша невыполним для данных параметров — пропускаем.
			continue
		}
		if opt == nil || !opt.Valid || opt.BestResult == nil || opt.BestResult.Price == nil {
			continue
		}
		c := candidate{
			flight:  f,
			opt:     opt,
			price:   opt.BestResult.Price.FinalPrice.Major(opt.BestResult.Price.Currency),
			comfort: comfortStepOf(opt.BestResult),
		}
		c.score = scoreOf(c, prio)
		cands = append(cands, c)
	}
	if len(cands) == 0 {
		return nil, nil, fmt.Errorf("%w: ни один тип марша не выполним для данных параметров", ErrNoFeasible)
	}

	// Ранжирование по метрике приоритета; стабильная сортировка сохраняет
	// порядок перебора при равенстве (детерминизм, ADR-0003).
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score < cands[j].score })

	best := cands[0]
	total := 0.0
	for _, c := range cands {
		total += c.score
	}
	rating := 1.0
	if total > 0 {
		rating = clamp01(1 - best.score/total)
	}

	resp := &Response{
		Recommendation: buildRecommendation(best, prio),
		Rating:         rating,
		Alternatives:   buildAlternatives(cands, best),
		Findings:       buildDesignFindings(best),
		Tradeoffs:      buildTradeoffs(cands, best, prio),
		Notes:          notes,
	}

	it, err := buildDesignIntent(d, prio, cands, best, notes)
	if err != nil {
		return nil, nil, err
	}
	return resp, it, nil
}

// scoreOf — метрика ранжирования: цена/себестоимость/материал — целевое
// значение оптимизатора; комфорт — отклонение от центра норматива 600–640.
func scoreOf(c candidate, prio DesignPriority) float64 {
	if prio == PriorityComfort {
		return math.Abs(630 - c.comfort)
	}
	// Для price/cost/material берём objective оптимизатора (мажорные
	// единицы) — это уже метрика целевого приоритета.
	return c.opt.Objective
}

// comfortStepOf — шаг комфорта S = 2h + b результата (мм).
func comfortStepOf(res *stair.Result) float64 {
	if res == nil {
		return 0
	}
	switch {
	case res.Spiral != nil:
		return res.Spiral.ComfortStep
	case res.LShape != nil:
		return 2*res.LShape.StepHeight.Millimeters() + res.LShape.TreadDepth.Millimeters()
	case res.UShape != nil:
		return 2*res.UShape.StepHeight.Millimeters() + res.UShape.TreadDepth.Millimeters()
	default:
		return 2*res.Flight.StepHeight.Millimeters() + res.Flight.TreadDepth.Millimeters()
	}
}

// buildRecommendation — текст рекомендации с ключевыми параметрами.
func buildRecommendation(c candidate, prio DesignPriority) string {
	res := c.opt.BestResult
	cur := res.Price.Currency
	return fmt.Sprintf(
		"Рекомендуется %s: %d ступеней, высота ступени %g мм, проступь %g мм, "+
			"шаг комфорта ≈ %g мм, цена ≈ %g %s (рекомендация по приоритету «%s»).",
		flightTitle(c.flight), stepCountOf(res),
		stepHeightOf(res).Millimeters(), treadOf(res).Millimeters(),
		c.comfort, res.Price.FinalPrice.Major(cur), cur.Code, prio)
}

// buildAlternatives — альтернативные варианты после лучшего.
func buildAlternatives(cands []candidate, best candidate) []Alternative {
	out := make([]Alternative, 0, len(cands)-1)
	for _, c := range cands {
		if c.flight == best.flight {
			continue
		}
		rate := clamp01(1 - c.score/best.score)
		reason := "сопоставимый результат"
		if c.score > best.score {
			reason = priceGapReason(best, c)
		}
		out = append(out, Alternative{
			Title:  fmt.Sprintf("%s (≈ %g ₽)", flightTitle(c.flight), c.price),
			Rating: rate,
			Reason: reason,
		})
	}
	return out
}

// buildDesignFindings — замечания из конвейера лучшего кандидата.
func buildDesignFindings(c candidate) []Finding {
	var out []Finding
	res := c.opt.BestResult
	if res.Validation.Blocking {
		out = append(out, Finding{Severity: "error", Element: "validation", Message: "конфигурация не проходит блокирующую валидацию"})
	}
	for _, iss := range res.Validation.Issues {
		s := "warning"
		out = append(out, Finding{Severity: s, Element: iss.Element, Message: iss.Message})
	}
	for _, g := range res.GeometryIssues {
		out = append(out, Finding{Severity: "info", Element: g.Element, Message: g.Message})
	}
	return out
}

// buildTradeoffs — компромиссы между лучшим и альтернативами.
func buildTradeoffs(cands []candidate, best candidate, prio DesignPriority) []string {
	var out []string
	for _, c := range cands {
		if c.flight == best.flight {
			continue
		}
		switch prio {
		case PriorityComfort:
			out = append(out, fmt.Sprintf("%s: шаг комфорта %g мм (норматив 600–640 мм)",
				flightTitle(c.flight), c.comfort))
		case PriorityPrice, PriorityCost, PriorityMaterial:
			out = append(out, fmt.Sprintf("%s: %g %s",
				flightTitle(c.flight), c.price, moneyCodeOf(c)))
		default:
			out = append(out, fmt.Sprintf("%s: %g %s",
				flightTitle(c.flight), c.price, moneyCodeOf(c)))
		}
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func stepCountOf(res *stair.Result) int {
	switch {
	case res.Spiral != nil:
		return res.Spiral.StepCount
	case res.LShape != nil:
		return res.LShape.StepCount
	case res.UShape != nil:
		return res.UShape.StepCount
	default:
		return res.Flight.StepCount
	}
}

func stepHeightOf(res *stair.Result) engineering.Length {
	switch {
	case res.Spiral != nil:
		return res.Spiral.StepHeight
	case res.LShape != nil:
		return res.LShape.StepHeight
	case res.UShape != nil:
		return res.UShape.StepHeight
	default:
		return res.Flight.StepHeight
	}
}

func treadOf(res *stair.Result) engineering.Length {
	switch {
	case res.Spiral != nil:
		return res.Spiral.WalkTread
	case res.LShape != nil:
		return res.LShape.TreadDepth
	case res.UShape != nil:
		return res.UShape.TreadDepth
	default:
		return res.Flight.TreadDepth
	}
}

// moneyCodeOf возвращает код валюты результата кандидата.
func moneyCodeOf(c candidate) string {
	if c.opt == nil || c.opt.BestResult == nil || c.opt.BestResult.Price == nil {
		return "RUB"
	}
	return c.opt.BestResult.Price.Currency.Code
}

// priceGapReason — описание разницы цены между вариантами.
func priceGapReason(best, alt candidate) string {
	cur := moneyCodeOf(alt)
	return fmt.Sprintf("цена выше рекомендуемого на %g %s", alt.price-best.price, cur)
}

// buildDesignIntent — материал для модели: нарратив и строгий JSON контекста.
func buildDesignIntent(d DesignRequest, prio DesignPriority, cands []candidate, best candidate, notes []string) (*intent, error) {
	rows := make([]map[string]interface{}, 0, len(cands))
	for _, c := range cands {
		rows = append(rows, map[string]interface{}{
			"flight":  string(c.flight),
			"price":   c.price,
			"comfort": c.comfort,
			"score":   c.score,
			"best":    c.flight == best.flight,
		})
	}
	data, err := json.Marshal(map[string]interface{}{
		"width_mm":   d.Config.Width.Millimeters(),
		"height_mm":  d.Config.Height.Millimeters(),
		"priority":   string(prio),
		"candidates": rows,
		"notes":      notes,
	})
	if err != nil {
		return nil, fmt.Errorf("assistant: design intent: %w", err)
	}

	var b strings.Builder
	b.WriteString("Перебраны типы марша; оптимум по приоритету ")
	b.WriteString(string(prio))
	b.WriteString(":\n")
	for _, c := range cands {
		mark := " "
		if c.flight == best.flight {
			mark = "*"
		}
		fmt.Fprintf(&b, " %s %s: цена=%g, комфорт=%g\n",
			mark, flightTitle(c.flight), c.price, c.comfort)
	}
	if len(notes) > 0 {
		b.WriteString("Примечания: " + strings.Join(notes, "; ") + "\n")
	}

	return &intent{
		Kind:         string(KindDesign),
		Instructions: "Ты — эксперт по лестницам. Опиши пользователю рекомендацию ДОСТУПНО, по-русски, в 3–7 предложениях. Не выдумывай цифры: все они из контекста.",
		UserTask:     "Помоги выбрать тип лестничного марша.",
		Context:      b.String(),
		Data:         data,
	}, nil
}
