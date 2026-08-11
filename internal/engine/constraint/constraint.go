// Package constraint реализует Constraint Engine (BC-003).
// Единый источник инженерных ограничений для всех движков платформы.
// Контекст не изменяет модель — только хранит и предоставляет правила.
package constraint

import "fmt"

// Severity — уровень нарушения ограничения (EDR-0003).
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// RuleCode — уникальный код ограничения (BC-003 invariant).
type RuleCode string

// Range — числовой диапазон ограничения с допуском (BC-003).
type Range struct {
	Min       float64
	Max       float64
	HasMin    bool
	HasMax    bool
	Tolerance float64
}

// Contains проверяет, попадает ли значение в диапазон (с допуском).
func (r Range) Contains(value float64) bool {
	if r.HasMin && value+r.Tolerance < r.Min {
		return false
	}
	if r.HasMax && value-r.Tolerance > r.Max {
		return false
	}
	return true
}

// Constraint — ограничение с кодом, категорией, диапазоном и версией.
type Constraint struct {
	Code     RuleCode
	Category string
	Severity Severity
	Range    Range
	Version  int
	Active   bool
	Message  string
	Fix      string
}

// ConstraintSet — агрегат (BC-003): полный набор ограничений проекта.
// Одно правило (RuleCode) может иметь несколько версий (RuleVersion);
// активна ровно одна версия (инвариант BC-003).
type ConstraintSet struct {
	ID          string
	Name        string
	Constraints map[RuleCode][]*Constraint
}

// NewSet создаёт пустой набор ограничений.
func NewSet(id, name string) *ConstraintSet {
	return &ConstraintSet{ID: id, Name: name, Constraints: make(map[RuleCode][]*Constraint)}
}

// Add добавляет версию правила.
// Инвариант BC-003: правило имеет уникальный код, а версия — уникальный номер.
func (s *ConstraintSet) Add(c *Constraint) error {
	if c.Code == "" {
		return fmt.Errorf("constraint: rule code is required")
	}
	if c.Version < 1 {
		return fmt.Errorf("constraint: %s version must be >= 1, got %d", c.Code, c.Version)
	}
	for _, existing := range s.Constraints[c.Code] {
		if existing.Version == c.Version {
			return fmt.Errorf("constraint: duplicate version %d of rule %s", c.Version, c.Code)
		}
	}
	s.Constraints[c.Code] = append(s.Constraints[c.Code], c)
	return nil
}

// Activate делает активной ровно одну версию правила (инвариант BC-003).
// Остальные правила набора не затрагиваются.
func (s *ConstraintSet) Activate(code RuleCode, version int) error {
	versions := s.Constraints[code]
	if len(versions) == 0 {
		return fmt.Errorf("constraint: unknown code %s", code)
	}
	var target *Constraint
	for _, c := range versions {
		if c.Version == version {
			target = c
			break
		}
	}
	if target == nil {
		return fmt.Errorf("constraint: %s has no version %d", code, version)
	}
	for _, c := range versions {
		c.Active = c == target
	}
	return nil
}

// Active возвращает активную версию правила или false.
func (s *ConstraintSet) Active(code RuleCode) (*Constraint, bool) {
	for _, c := range s.Constraints[code] {
		if c.Active {
			return c, true
		}
	}
	return nil, false
}

// Resolve находит активную версию правила и проверяет попадание значения в диапазон.
// Возвращает нарушение, если значение вне диапазона.
func (s *ConstraintSet) Resolve(code RuleCode, value float64) (*Constraint, bool) {
	c, ok := s.Active(code)
	if !ok {
		return c, true
	}
	return c, c.Range.Contains(value)
}
