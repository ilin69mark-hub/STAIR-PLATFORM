package validation

import (
	"sort"
	"strconv"

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
