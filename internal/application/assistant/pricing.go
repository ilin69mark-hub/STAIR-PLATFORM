package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"stairplatform/internal/application/stair"
	domprc "stairplatform/internal/domain/pricing"
)

// pricingExpert — эксперт D4 (EDR-0039): ценовая экспертиза конфигурации.
// Даёт разложение цены (материал/машина/труд/накладные/маржа/скидка/налог)
// и экономическую интерпретацию: рентабельность, долю материалов,
// наценку, удельную цену на м² проступи/м³. Ничего не пересчитывает сам —
// читает `PriceBreakdown`/`ManufacturingCostDataset` из конвейера (AI-0000).
// Пороги — эмпирические, зафиксированы для детерминизма.
type pricingExpert struct{}

// пороговые значения ценовой экспертизы.
const (
	prcMaterialShareWarn = 0.6  // доля материала в себестоимости > 60%
	prcMarginWarn        = 0.40 // маржа > 40% — проверить адекватность
	prcMarginInfo        = 0.10 // маржа < 10% — тонкая маржа
	prcOverheadWarn      = 0.30 // накладные > 30% себестоимости
)

func (pricingExpert) analyze(ctx context.Context, tools *Tools, req Request) (*Response, *intent, error) {
	a, ok := req.(AnalysisRequest)
	if !ok {
		return nil, nil, fmt.Errorf("%w: expected AnalysisRequest, got %T", ErrInvalid, req)
	}
	if err := tools.ValidateConfig(a.Config); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	res, err := tools.Calculate(ctx, a.Config, a.Options)
	if err != nil {
		return nil, nil, fmt.Errorf("assistant: pricing: %w", err)
	}
	if res == nil {
		return nil, nil, fmt.Errorf("%w: пустой результат расчёта", ErrNoFeasible)
	}

	// Цена считается только после полного конвейера.
	if res.Validation.Blocking || res.Price == nil {
		resp := &Response{
			Recommendation: "Ценовая экспертиза недоступна: конвейер остановлен, цена не рассчитана.",
			Rating:         0,
			Findings:       collectPipelineFindings(res),
			Suggestions:    fixSuggestions(collectPipelineFindings(res)),
			Notes:          []string{"PriceBreakdown не сформирован."},
		}
		return resp, buildPricingIntent(a, res, resp), nil
	}

	findings := priceFindings(res)
	rating := priceRating(findings)

	resp := &Response{
		Recommendation: priceRecommendation(res, findings),
		Rating:         rating,
		Findings:       findings,
		Suggestions:    priceSuggestions(res),
		Notes:          priceNotes(res),
		Tradeoffs:      []string{"Подбор ставок/цены изменит итог: пересчитайте конфигурацию после правок rates."},
	}
	return resp, buildPricingIntent(a, res, resp), nil
}

// priceFindings — замечания по структуре цены.
func priceFindings(res *stair.Result) []Finding {
	var out []Finding
	b := res.Price
	cur := b.Currency
	pc := b.ProductionCost.Major(cur)
	if pc <= 0 {
		out = append(out, Finding{Severity: "warning", Element: "себестоимость", Message: "нулевая себестоимость"})
		return out
	}
	if share := shareOf(b.Material, b.ProductionCost, cur); share > prcMaterialShareWarn {
		out = append(out, Finding{
			Severity: "warning", Element: "материалы",
			Message: fmt.Sprintf("доля материалов в себестоимости %.0f%% — высокая материалоёмкость", share*100),
		})
	}
	if share := shareOf(b.Overhead, b.ProductionCost, cur); share > prcOverheadWarn {
		out = append(out, Finding{
			Severity: "info", Element: "накладные",
			Message: fmt.Sprintf("накладные составляют %.0f%% себестоимости", share*100),
		})
	}
	margin := shareOf(b.Margin, b.FinalPrice, cur)
	if margin > prcMarginWarn {
		out = append(out, Finding{
			Severity: "info", Element: "маржа",
			Message: fmt.Sprintf("маржа %.0f%% итоговой цены — убедитесь в конкурентоспособности", margin*100),
		})
	}
	if margin > 0 && margin < prcMarginInfo {
		out = append(out, Finding{
			Severity: "warning", Element: "маржа",
			Message: fmt.Sprintf("маржа всего %.0f%% итоговой цены — тонкая прибыль", margin*100),
		})
	}
	return out
}

// shareOf — доля части в целом (major-единицы, 0..1); 0 при некорректном base.
func shareOf(part, base domprc.Money, cur domprc.Currency) float64 {
	bm := base.Major(cur)
	if bm <= 0 {
		return 0
	}
	return part.Major(cur) / bm
}

// priceRating — оценка экономической привлекательности (0..1).
func priceRating(findings []Finding) float64 {
	rating := 1.0
	for _, f := range findings {
		switch f.Severity {
		case "warning":
			rating -= 0.3
		case "info":
			rating -= 0.1
		}
	}
	return clamp01(rating)
}

// priceRecommendation — главный вывод по цене.
func priceRecommendation(res *stair.Result, findings []Finding) string {
	b := res.Price
	warns := 0
	for _, f := range findings {
		if f.Severity == "warning" {
			warns++
		}
	}
	if warns == 0 {
		return fmt.Sprintf("Цена корректна и сбалансирована: итог %g %s.", b.FinalPrice.Major(b.Currency), b.Currency.Code)
	}
	return fmt.Sprintf("Структура цены требует внимания (%d замечаний): смотрите детали.", warns)
}

// priceSuggestions — экономические рекомендации.
func priceSuggestions(res *stair.Result) []Suggestion {
	var out []Suggestion
	b := res.Price
	cur := b.Currency
	if share := shareOf(b.Material, b.ProductionCost, cur); share > prcMaterialShareWarn {
		out = append(out, Suggestion{
			Message:   "Снизьте материалоёмкость: другой материал/раскрой (см. мануфактурный ассистент) уменьшит цену.",
			Rationale: "доминирование материала в себестоимости",
		})
	}
	if share := shareOf(b.Margin, b.FinalPrice, cur); share > 0 && share < prcMarginInfo {
		out = append(out, Suggestion{
			Message:   "Маржа ниже 10%: пересмотрите цену или сократите постоянные издержки.",
			Rationale: "тонкая маржа ограничивает запас прочности",
		})
	}
	return out
}

// priceNotes — ключевые финансовые метрики.
func priceNotes(res *stair.Result) []string {
	b := res.Price
	cur := b.Currency
	out := []string{
		fmt.Sprintf("Итог: %g %s; себестоимость %g; маржа %g (%.0f%%).",
			b.FinalPrice.Major(cur), cur.Code,
			b.ProductionCost.Major(cur), b.Margin.Major(cur),
			marginPct(b)),
	}
	if res.Cost != nil {
		if res.Cost.PartArea > 0 {
			out = append(out, fmt.Sprintf("Удельная цена: %g %s/м² проступи.",
				b.FinalPrice.Major(cur)/res.Cost.PartArea*1e6, cur.Code))
		}
		if res.Cost.Mass > 0 {
			out = append(out, fmt.Sprintf("Цена на массу: %g %s/кг.",
				b.FinalPrice.Major(cur)/res.Cost.Mass, cur.Code))
		}
	}
	return out
}

// marginPct — доля маржи в итоговой цене, %.
func marginPct(b *domprc.PriceBreakdown) float64 {
	final := b.FinalPrice.Major(b.Currency)
	if final <= 0 {
		return 0
	}
	return b.Margin.Major(b.Currency) / final * 100
}

// buildPricingIntent — материал комментария. nil-безопасен при остановке
// конвейера (Price отсутствует).
func buildPricingIntent(a AnalysisRequest, res *stair.Result, resp *Response) *intent {
	b := res.Price
	if b == nil {
		return &intent{
			Kind:         string(KindPricing),
			Instructions: "Ты — экономист по калькуляции лестниц. Объясни, почему цена недоступна. По-русски, 2–4 предложения.",
			UserTask:     "Проанализируй структуру цены конфигурации лестницы.",
			Context:      fmt.Sprintf("Конфигурация: %s. Цена не рассчитана (конвейер остановлен).", flightTitle(a.Config.Flight)),
		}
	}
	data, err := json.Marshal(map[string]interface{}{
		"flight":          string(a.Config.Flight),
		"currency":        b.Currency.Code,
		"final_price":     b.FinalPrice.Major(b.Currency),
		"production_cost": b.ProductionCost.Major(b.Currency),
		"material":        b.Material.Major(b.Currency),
		"machine":         b.Machine.Major(b.Currency),
		"labor":           b.Labor.Major(b.Currency),
		"margin":          b.Margin.Major(b.Currency),
		"margin_pct":      marginPct(b),
		"findings":        resp.Findings,
	})
	if err != nil {
		data = nil
	}
	return &intent{
		Kind:         string(KindPricing),
		Instructions: "Ты — экономист по калькуляции лестниц. Объясни пользователю структуру цены и что изменить для экономии или адекватной маржи. По-русски, 3–7 предложений, только по фактам контекста.",
		UserTask:     "Проанализируй структуру цены конфигурации лестницы.",
		Context:      fmt.Sprintf("Конфигурация: %s. Итог %g %s, себестоимость %g, материалы %g, машина %g, труд %g, маржа %g (%.0f%%).", flightTitle(a.Config.Flight), b.FinalPrice.Major(b.Currency), b.Currency.Code, b.ProductionCost.Major(b.Currency), b.Material.Major(b.Currency), b.Machine.Major(b.Currency), b.Labor.Major(b.Currency), b.Margin.Major(b.Currency), marginPct(b)),
		Data:         data,
	}
}
