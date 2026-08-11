package geometry

import "fmt"

// ParamType — тип параметра (ENG-GEO-0102).
type ParamType string

const (
	TypeInteger   ParamType = "integer"
	TypeFloat     ParamType = "float"
	TypeBoolean   ParamType = "boolean"
	TypeString    ParamType = "string"
	TypeEnum      ParamType = "enum"
	TypeLength    ParamType = "length"
	TypeAngle     ParamType = "angle"
	TypeArea      ParamType = "area"
	TypeVolume    ParamType = "volume"
	TypeMass      ParamType = "mass"
	TypeMaterial  ParamType = "material"
	TypeReference ParamType = "reference"
	TypeExpression ParamType = "expression"
	TypeFormula   ParamType = "formula"
)

// ParamState — состояние параметра (ENG-GEO-0102).
type ParamState string

const (
	StateDefined    ParamState = "defined"
	StateCalculated ParamState = "calculated"
	StateInherited  ParamState = "inherited"
	StateLocked     ParamState = "locked"
	StateDerived    ParamState = "derived"
	StateInvalid    ParamState = "invalid"
)

// Parameter — единица параметрической модели (ENG-GEO-0002).
// Каждый параметр имеет владельца (ENG-GEO-0102); значение может быть
// вычислено формулой, зависящей от других параметров модели.
type Parameter struct {
	ID      string
	Owner   string
	Type    ParamType
	State   ParamState
	Value   any
	Formula FormulaFunc
}

// FormulaFunc вычисляет значение параметра по зависимым параметрам.
type FormulaFunc func(m *ParametricModel) (any, error)

// NewParameter создаёт параметр в состоянии Defined.
func NewParameter(id, owner string, typ ParamType, value any) *Parameter {
	return &Parameter{ID: id, Owner: owner, Type: typ, State: StateDefined, Value: value}
}

// NewDerivedParameter создаёт вычисляемый параметр (состояние Derived).
func NewDerivedParameter(id, owner string, typ ParamType, formula FormulaFunc) *Parameter {
	return &Parameter{ID: id, Owner: owner, Type: typ, State: StateDerived, Formula: formula}
}

func (p *Parameter) String() string {
	return fmt.Sprintf("%s(%s,%s)=%v", p.ID, p.Owner, p.Type, p.Value)
}
