package engineering

import "fmt"

// ParameterState — жизненный цикл параметра (DOM-0023).
type ParameterState string

const (
	ParameterCreated      ParameterState = "created"
	ParameterValidated    ParameterState = "validated"
	ParameterCalculated   ParameterState = "calculated"
	ParameterApplied      ParameterState = "applied"
	ParameterReferenced   ParameterState = "referenced"
	ParameterModified     ParameterState = "modified"
	ParameterRecalculated ParameterState = "recalculated"
	ParameterArchived     ParameterState = "archived"
)

// ParameterSource — источник значения параметра (DOM-0023).
type ParameterSource string

const (
	SourceUser          ParameterSource = "user"
	SourceFormula       ParameterSource = "formula"
	SourceImport        ParameterSource = "import"
	SourceAI            ParameterSource = "ai"
	SourceManufacturing ParameterSource = "manufacturing"
	SourceExternal      ParameterSource = "external_system"
)

// Parameter — единица параметрической модели.
// Параметры являются единственным источником истины геометрии.
type Parameter struct {
	ID        string
	Name      string
	Unit      string
	Value     any
	Source    ParameterSource
	State     ParameterState
	Formula   string
	DependsOn []string
	Revision  *Revision
}

// NewParameter создаёт параметр в состоянии Created.
func NewParameter(id, name, unit string, value any, source ParameterSource) *Parameter {
	return &Parameter{
		ID:        id,
		Name:      name,
		Unit:      unit,
		Value:     value,
		Source:    source,
		State:     ParameterCreated,
		DependsOn: []string{},
	}
}

// Validate переводит параметр в состояние Validated.
// Проверяются: тип, единицы, диапазон (DOM-0023).
func (p *Parameter) Validate() error {
	if p.ID == "" || p.Name == "" {
		return fmt.Errorf("parameter: id and name are required")
	}
	if p.Unit == "" {
		return fmt.Errorf("parameter %s: unit is required", p.ID)
	}
	if p.Value == nil {
		return fmt.Errorf("parameter %s: value is required", p.ID)
	}
	p.State = ParameterValidated
	return nil
}

// Calculate присваивает вычисленное значение (Source == Formula)
// и переводит параметр в состояние Calculated.
func (p *Parameter) Calculate(value any) error {
	if p.Source != SourceFormula {
		return fmt.Errorf("parameter %s: only formula parameters can be calculated", p.ID)
	}
	p.Value = value
	p.State = ParameterCalculated
	return nil
}

// Apply переводит валидный параметр в состояние Applied.
func (p *Parameter) Apply() error {
	if p.State == ParameterCreated {
		if err := p.Validate(); err != nil {
			return err
		}
	}
	p.State = ParameterApplied
	return nil
}

// Modify отмечает ручное изменение параметра (DOM-0023) —
// триггер пересчёта зависимых узлов через Graph Engine.
func (p *Parameter) Modify(value any) error {
	if p.State == ParameterArchived {
		return fmt.Errorf("parameter %s: archived parameter cannot be modified", p.ID)
	}
	p.Value = value
	p.State = ParameterModified
	return nil
}

// Recalculate переводит модифицированный параметр в состояние Recalculated.
func (p *Parameter) Recalculate(value any) {
	p.Value = value
	p.State = ParameterRecalculated
}

// Archive переводит параметр в состояние Archived.
func (p *Parameter) Archive() {
	p.State = ParameterArchived
}
