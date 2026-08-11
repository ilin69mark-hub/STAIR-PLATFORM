// Package validation реализует Validation Engine (EDR-0003).
// Сверяет параметрическую модель марша с активным ConstraintSet (BC-003)
// и возвращает детерминированный список ValidationIssue.
// Валидация не изменяет модель (BC-003).
package validation

import "stairplatform/internal/engine/constraint"

// Issue — одно нарушение правила (EDR-0003 §4).
type Issue struct {
	ID       string
	Code     constraint.RuleCode
	Severity constraint.Severity
	Element  string
	Message  string
	Value    float64
	Min      float64
	Max      float64
	HasMin   bool
	HasMax   bool
	Fix      string
}

// Result — итог валидации (EDR-0003 §5).
type Result struct {
	Issues   []Issue
	Valid    bool
	Blocking bool
}
