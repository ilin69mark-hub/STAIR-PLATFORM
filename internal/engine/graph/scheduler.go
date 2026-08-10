package graph

// Scheduler реализует частичный пересчёт графа зависимостей
// (ENG-GRAPH-0006, ENG-GRAPH-0013).
// Dirty Nodes → Dependency Resolution → Execution Queue (топологический порядок).
// Полный пересчёт запрещён, когда возможен частичный.

type Scheduler struct {
	graph *Graph
}

func NewScheduler(g *Graph) *Scheduler {
	return &Scheduler{graph: g}
}

// Plan возвращает узлы, требующие пересчёта после изменения заданных корней,
// в топологическом порядке. Узел не выполняется дважды за один проход.
// Источники изменения (changed) сами не пересчитываются — они установлены
// извне; пересчитываются только их зависимые.
func (s *Scheduler) Plan(changed ...string) ([]string, error) {
	dirty := make(map[string]bool)
	for _, id := range changed {
		if s.graph.HasNode(id) {
			dirty[id] = true
		}
	}

	// расширяем через транзитивные зависимые.
	expanded := make(map[string]bool)
	for id := range dirty {
		for _, d := range s.graph.TransitiveDependents(id) {
			expanded[d] = true
		}
	}
	for id := range expanded {
		dirty[id] = true
	}

	// пересечение dirty-узлов с топологическим порядком;
	// корни изменения исключаются (они уже установлены).
	order, err := s.graph.TopologicalSort()
	if err != nil {
		return nil, err
	}
	var plan []string
	for _, id := range order {
		if dirty[id] && !contains(changed, id) {
			plan = append(plan, id)
		}
	}
	return plan, nil
}

// Execute выполняет функцию recompute для каждого узла плана
// в топологическом порядке и отмечает узлы как clean.
func (s *Scheduler) Execute(changed []string, recompute func(id string) error) error {
	plan, err := s.Plan(changed...)
	if err != nil {
		return err
	}
	for _, id := range plan {
		if err := recompute(id); err != nil {
			return err
		}
		s.graph.MarkClean(id)
	}
	for _, id := range changed {
		s.graph.MarkClean(id)
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
