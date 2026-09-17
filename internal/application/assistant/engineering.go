package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"stairplatform/internal/application/stair"
)

// engineeringExpert — эксперт D2 (EDR-0037): инженерный анализ конкретной
// конфигурации. Проверяет соответствие нормативам STANDARD (EDR-0002):
// высота/проступь ступени, шаг комфорта, угол наклона, просвет, толщина
// косоура, ограждение; собирает замечания конвейера (валидация + геометрия)
// и выводит рекомендацию. «Истина» — в результатах тула Calculate.
type engineeringExpert struct{}

// нормативные диапазоны STANDARD (EDR-0002, constraint/standard.go).
const (
	normStepHeightMin, normStepHeightMax = 150.0, 200.0 // h, мм
	normTreadMin, normTreadMax           = 260.0, 320.0 // b, мм
	normComfortMin, normComfortMax       = 600.0, 640.0 // S = 2h + b, мм
	normAngleMin, normAngleMax           = 30.0, 45.0   // α, град
	normClearanceMin                     = 2000.0       // мм
	normStringerMin                      = 30.0         // мм
	normRailingMin                       = 900.0        // мм
)

func (engineeringExpert) analyze(ctx context.Context, tools *Tools, req Request) (*Response, *intent, error) {
	a, ok := req.(AnalysisRequest)
	if !ok {
		return nil, nil, fmt.Errorf("%w: expected AnalysisRequest, got %T", ErrInvalid, req)
	}
	if err := tools.ValidateConfig(a.Config); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	res, err := tools.Calculate(ctx, a.Config, a.Options)
	if err != nil {
		return nil, nil, fmt.Errorf("assistant: engineering: %w", err)
	}
	if res == nil {
		return nil, nil, fmt.Errorf("%w: пустой результат расчёта", ErrNoFeasible)
	}

	findings := collectPipelineFindings(res)

	if res.Validation.Blocking {
		// Конфигурация не проходит блокирующую валидацию: рекомендация
		// перечисляет критические нарушения.
		resp := &Response{
			Recommendation: "Конфигурация не проходит блокирующую валидацию — расчёт невозможен. Исправьте критические замечания ниже.",
			Rating:         0,
			Findings:       findings,
			Suggestions:    fixSuggestions(findings),
			Notes:          []string{"Инженерный анализ остановлен: нет результатов конвейера."},
		}
		return resp, buildEngineeringIntent(a, res, resp), nil
	}

	// Неблокирующая проверка нормативов по данным конвейера.
	findings = append(findings, checkNorms(a, res)...)

	suggestions := fixSuggestions(findings)
	// Дополнительная рекомендация по комфорту при отклонении.
	if sug := comfortSuggestion(findings); sug.Message != "" {
		suggestions = append(suggestions, sug)
	}

	rating := engineeringRating(findings)
	resp := &Response{
		Recommendation: "Конфигурация удовлетворяет инженерным нормативам (STANDARD).",
		Rating:         rating,
		Findings:       findings,
		Suggestions:    suggestions,
		Notes: []string{
			fmt.Sprintf("Шаг комфорта S = %.1f мм (норматив %g–%g мм).", comfortStepOf(res), normComfortMin, normComfortMax),
		},
	}
	if rating < 1 {
		resp.Recommendation = "Конфигурация требует доработки инженерных параметров: смотрите замечания ниже."
		resp.Tradeoffs = []string{"Исправление критичных нарушений изменит стоимость и геометрию — пересчитайте конфигурацию после правок."}
	}
	return resp, buildEngineeringIntent(a, res, resp), nil
}

// collectPipelineFindings превращает неблокирующие нарушения валидации и
// замечания геометрии в Findings ассистента.
func collectPipelineFindings(res *stair.Result) []Finding {
	out := make([]Finding, 0, len(res.Validation.Issues)+len(res.GeometryIssues))
	for _, iss := range res.Validation.Issues {
		out = append(out, Finding{
			Severity: "warning",
			Element:  iss.Element,
			Message:  iss.Message,
		})
	}
	for _, g := range res.GeometryIssues {
		out = append(out, Finding{
			Severity: "warning",
			Element:  g.Element,
			Message:  g.Message,
		})
	}
	return out
}

// checkNorms проверяет нормативы STANDARD по данным конвейера
// (неблокирующие; каждое нарушение — warning с SDK-подсказкой).
func checkNorms(a AnalysisRequest, res *stair.Result) []Finding {
	var out []Finding
	h := stepHeightOf(res).Millimeters()
	b := treadOf(res).Millimeters()
	cf := comfortStepOf(res)

	checkRange := func(label string, val, min, max float64) {
		sev := "warning"
		msg := fmt.Sprintf("%s = %.1f мм (норматив %g–%g мм)", label, val, min, max)
		if val < min || val > max {
			out = append(out, Finding{Severity: sev, Element: label, Message: msg})
		}
	}
	checkRange("высота ступени h", h, normStepHeightMin, normStepHeightMax)
	checkRange("проступь b", b, normTreadMin, normTreadMax)
	checkRange("шаг комфорта S", cf, normComfortMin, normComfortMax)
	if ang := angleDegOf(res); ang > 0 {
		if ang < normAngleMin || ang > normAngleMax {
			out = append(out, Finding{
				Severity: "warning", Element: "угол наклона",
				Message: fmt.Sprintf("угол наклона = %.1f° (норматив %g–%g°)", ang, normAngleMin, normAngleMax),
			})
		}
	}
	if c := a.Config.Clearance.Millimeters(); c > 0 && c < normClearanceMin {
		out = append(out, Finding{
			Severity: "warning", Element: "просвет",
			Message: fmt.Sprintf("вертикальный просвет = %.0f мм (норматив ≥ %g мм)", c, normClearanceMin),
		})
	}
	if s := a.Config.StringerThickness.Millimeters(); s > 0 && s < normStringerMin {
		out = append(out, Finding{
			Severity: "warning", Element: "толщина косоура",
			Message: fmt.Sprintf("толщина косоура = %.0f мм (норматив ≥ %g мм)", s, normStringerMin),
		})
	}
	if r := a.Config.RailingHeight.Millimeters(); r > 0 && r < normRailingMin {
		out = append(out, Finding{
			Severity: "warning", Element: "ограждение",
			Message: fmt.Sprintf("высота ограждения = %.0f мм (норматив ≥ %g мм)", r, normRailingMin),
		})
	}
	return out
}

// fixSuggestions — предложения по устранению замечаний (Suggestion).
func fixSuggestions(findings []Finding) []Suggestion {
	var out []Suggestion
	has := func(element string) bool {
		for _, f := range findings {
			if f.Element == element {
				return true
			}
		}
		return false
	}
	if has("высота ступени h") {
		out = append(out, Suggestion{
			Message:   "Скорректируйте число ступеней: высота h войдёт в норматив 150–200 мм.",
			Rationale: "правило STANDARD GEO-STEP-HEIGHT",
		})
	}
	if has("шаг комфорта S") {
		out = append(out, Suggestion{
			Message:   "Добейтесь шага комфорта 2h+b в диапазоне 600–640 мм подбором проступи.",
			Rationale: "правило STEP-COMFORT 2h+b",
		})
	}
	if has("угол наклона") {
		out = append(out, Suggestion{
			Message:   "Угол наклона вне норматива: измените высоту/проступь или длину марша.",
			Rationale: "правило STANDARD GEO-ANGLE",
		})
	}
	for _, f := range findings {
		if f.Element == "просвет" || f.Element == "толщина косоура" || f.Element == "ограждение" {
			out = append(out, Suggestion{
				Message:   f.Message,
				Rationale: "правило STANDARD (соответствующий код)",
			})
		}
	}
	return out
}

// comfortSuggestion — отдельная подсказка по комфорту, если замечание о
// шаге комфорта ещё не покрыто.
func comfortSuggestion(findings []Finding) Suggestion {
	for _, f := range findings {
		if f.Element == "шаг комфорта S" {
			return Suggestion{
				Message:   "Используйте ассистента design: он подберёт оптимум с нужным шагом комфорта.",
				Rationale: "комфортный шаг 2h+b: 600–640 мм",
			}
		}
	}
	return Suggestion{}
}

// engineeringRating — оценка применимости конфигурации (0..1):
// доля «нормативных» замечаний в пользу конфигурации.
func engineeringRating(findings []Finding) float64 {
	if len(findings) == 0 {
		return 1
	}
	warns := 0
	for _, f := range findings {
		if f.Severity == "warning" {
			warns++
		}
	}
	if warns == 0 {
		return 1
	}
	r := 1 - float64(warns)/float64(len(findings)+warns)*0.9
	return clamp01(r)
}

// buildEngineeringIntent — материал комментария (нарратив + JSON).
func buildEngineeringIntent(a AnalysisRequest, res *stair.Result, resp *Response) *intent {
	data, err := json.Marshal(map[string]interface{}{
		"flight":      string(a.Config.Flight),
		"width_mm":    a.Config.Width.Millimeters(),
		"height_mm":   a.Config.Height.Millimeters(),
		"step_height": stepHeightOf(res).Millimeters(),
		"tread":       treadOf(res).Millimeters(),
		"comfort":     comfortStepOf(res),
		"angle_deg":   angleDegOf(res),
		"findings":    resp.Findings,
	})
	if err != nil {
		data = nil
	}
	return &intent{
		Kind:         string(KindEngineering),
		Instructions: "Ты — инженер по лестничным конструкциям. Дай пользователю объяснение недостатков и что исправить. По-русски, 3–7 предложений, только по фактам контекста.",
		UserTask:     "Проверь инженерно-конструкторскую корректность конфигурации лестницы.",
		Context:      fmt.Sprintf("Конфигурация: %s, W=%g мм, H=%g мм; результаты: h=%g мм, b=%g мм, S=%g мм (комфорт), α=%g°. Замечаний: %d.", flightTitle(a.Config.Flight), a.Config.Width.Millimeters(), a.Config.Height.Millimeters(), stepHeightOf(res).Millimeters(), treadOf(res).Millimeters(), comfortStepOf(res), angleDegOf(res), len(resp.Findings)),
		Data:         data,
	}
}

// angleDegOf возвращает угол наклона результата в градусах (0 — не найден).
func angleDegOf(res *stair.Result) float64 {
	switch {
	case res.Spiral != nil:
		return res.Spiral.Angle.Degrees()
	case res.LShape != nil:
		return res.LShape.Angle.Degrees()
	case res.UShape != nil:
		return res.UShape.Angle.Degrees()
	default:
		return res.Flight.Angle.Degrees()
	}
}
