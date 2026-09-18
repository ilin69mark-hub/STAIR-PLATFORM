package validation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
)

// binding связывает правило EDR-0002 с полем параметрической модели
// и приводит значение поля к единицам правила (ADR-0008: угол rad→deg).
type binding struct {
	code    constraint.RuleCode
	element string
	value   func(cfg *engineering.StairConfiguration) float64
}

// bindings — правила из EDR-0002, применяемые Validation Engine.
func bindings() []binding {
	return []binding{
		{constraint.GEO_STEP_HEIGHT, "step_height", func(c *engineering.StairConfiguration) float64 {
			return c.StepHeight.Millimeters()
		}},
		{constraint.GEO_TREAD_DEPTH, "tread_depth", func(c *engineering.StairConfiguration) float64 {
			return c.TreadDepth.Millimeters()
		}},
		{constraint.GEO_ANGLE, "angle", func(c *engineering.StairConfiguration) float64 {
			return c.Angle.Degrees()
		}},
		{constraint.GEO_CLEARANCE, "clearance", func(c *engineering.StairConfiguration) float64 {
			return c.Clearance.Millimeters()
		}},
		{constraint.GEO_STRINGER_THICKNESS, "stringer_thickness", func(c *engineering.StairConfiguration) float64 {
			return c.StringerThickness.Millimeters()
		}},
		{constraint.SAF_RAILING_HEIGHT, "railing_height", func(c *engineering.StairConfiguration) float64 {
			return c.RailingHeight.Millimeters()
		}},
	}
}

// Validate сверяет параметрическую модель марша с активным ConstraintSet.
// Возвращает детерминированный Result (EDR-0003 §5-6): issues сортируются
// по Code, ID нумеруются после сортировки (ADR-0001). Валидация не изменяет модель.
func Validate(cfg *engineering.StairConfiguration, set *constraint.ConstraintSet) Result {
	var issues []Issue
	for _, b := range bindings() {
		rule, ok := set.Resolve(b.code, b.value(cfg))
		if ok {
			continue
		}
		issue := Issue{
			Code:     rule.Code,
			Severity: rule.Severity,
			Element:  b.element,
			Message:  rule.Message,
			Value:    b.value(cfg),
			Min:      rule.Range.Min,
			Max:      rule.Range.Max,
			HasMin:   rule.Range.HasMin,
			HasMax:   rule.Range.HasMax,
			Fix:      rule.Fix,
		}
		// Подсказка правила (Advice) применяется здесь же, на этапе валидации:
		// Param и Guide (отрендеренный) доступны каждому issue сразу — в т.ч.
		// в сохранённых снапшотах и на любых фронтендах, а не только после
		// прохождения через advisor (ADR-0013).
		if rule.Advice != nil {
			issue.Param = rule.Advice.Param
			issue.Guide = RenderGuide(rule.Advice.Guide, issue, cfg.Height.Millimeters())
		}
		issues = append(issues, issue)
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Code < issues[j].Code
	})
	for i := range issues {
		issues[i].ID = "ISSUE-" + strconv.Itoa(i+1)
	}

	return Result{
		Issues:   issues,
		Valid:    !hasSeverity(issues, constraint.SeverityError),
		Blocking: hasSeverity(issues, constraint.SeverityError),
	}
}

func hasSeverity(issues []Issue, sev constraint.Severity) bool {
	for _, i := range issues {
		if i.Severity == sev {
			return true
		}
	}
	return false
}

// RenderGuide подставляет значения в шаблон подсказки правила (Advice.Guide).
// Плейсхолдеры: {value} — значение нарушенного поля (мм/ед. мм, целое),
// {angle} — угол наклона (градусы, %.1f), {min}/{max} — границы нормы,
// {height} — высота подъёма конфигурации.
func RenderGuide(tpl string, it Issue, heightMm float64) string {
	repl := strings.NewReplacer(
		"{value}", fmt.Sprintf("%.0f", it.Value),
		"{angle}", fmt.Sprintf("%.1f", it.Value),
		"{min}", fmt.Sprintf("%.0f", it.Min),
		"{max}", fmt.Sprintf("%.0f", it.Max),
		"{height}", fmt.Sprintf("%.0f", heightMm),
	)
	return repl.Replace(tpl)
}
