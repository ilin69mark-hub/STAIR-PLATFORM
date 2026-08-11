package geometry

import (
	"fmt"

	"stairplatform/internal/engine/graph"
)

// ParametricModel — реестр параметров инженерной модели (ENG-GEO-0002).
// Зависимости параметров существуют только через Graph Engine (ENG-GEO-0102):
// изменение параметра помечает его зависимых грязными и пересчитывает
// только их (частичный пересчёт; полный пересчёт запрещён).
type ParametricModel struct {
	g      *graph.Graph
	params map[string]*Parameter
}

// NewParametricModel создаёт пустую параметрическую модель.
func NewParametricModel() *ParametricModel {
	return &ParametricModel{
		g:      graph.New(),
		params: make(map[string]*Parameter),
	}
}

// Add регистрирует параметр. Инварианты: обязательны ID и владелец,
// ID уникален.
func (m *ParametricModel) Add(p *Parameter) error {
	if p.ID == "" {
		return fmt.Errorf("parametric model: parameter id is required")
	}
	if p.Owner == "" {
		return fmt.Errorf("parametric model: parameter %s must have an owner", p.ID)
	}
	if _, exists := m.params[p.ID]; exists {
		return fmt.Errorf("parametric model: duplicate parameter %s", p.ID)
	}
	m.params[p.ID] = p
	m.g.AddNode(graph.Node{ID: p.ID, Type: "parameter"})
	return nil
}

// AddDependency регистрирует зависимость: of зависит от dep (ребро dep→of).
// Зависимости допустимы только через граф; циклы запрещены.
func (m *ParametricModel) AddDependency(dep, of string) error {
	if _, ok := m.params[dep]; !ok {
		return fmt.Errorf("parametric model: unknown parameter %s", dep)
	}
	if _, ok := m.params[of]; !ok {
		return fmt.Errorf("parametric model: unknown parameter %s", of)
	}
	m.g.AddEdge(graph.Edge{From: dep, To: of})
	if m.g.HasCycle() {
		m.g.RemoveEdge(dep, of)
		return fmt.Errorf("parametric model: dependency %s -> %s creates a cycle", dep, of)
	}
	return nil
}

// Set устанавливает значение параметра (State Defined) и помечает его
// грязным для пересчёта зависимых.
func (m *ParametricModel) Set(id string, value any) error {
	p, ok := m.params[id]
	if !ok {
		return fmt.Errorf("parametric model: unknown parameter %s", id)
	}
	if p.State == StateLocked {
		return fmt.Errorf("parametric model: parameter %s is locked", id)
	}
	p.Value = value
	p.State = StateDefined
	m.g.AddNode(graph.Node{ID: id, Type: "parameter"})
	return nil
}

// Parameter возвращает параметр по ID.
func (m *ParametricModel) Parameter(id string) (*Parameter, bool) {
	p, ok := m.params[id]
	return p, ok
}

// Rebuild пересчитывает только зависимых от changed параметров
// в топологическом порядке через Graph Scheduler (ENG-GRAPH-0013).
// Изменённые корни считаются установленными извне и не пересчитываются.
func (m *ParametricModel) Rebuild(changed ...string) error {
	plan, err := graph.NewScheduler(m.g).Plan(changed...)
	if err != nil {
		return err
	}
	for _, id := range plan {
		p := m.params[id]
		if p.Formula == nil {
			continue
		}
		value, err := p.Formula(m)
		if err != nil {
			p.State = StateInvalid
			return fmt.Errorf("parametric model: %s: %w", id, err)
		}
		p.Value = value
		p.State = StateCalculated
	}
	for _, id := range plan {
		m.g.MarkClean(id)
	}
	for _, id := range changed {
		m.g.MarkClean(id)
	}
	return nil
}
