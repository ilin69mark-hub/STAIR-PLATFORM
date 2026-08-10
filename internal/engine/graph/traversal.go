package graph

// Traversal предоставляет детерминированные алгоритмы обхода графа
// (ENG-GRAPH-0003). Все результаты сортируются для воспроизводимости.

// Visitor вызывается для каждого посещённого узла; возвращает false для прерывания.
type Visitor func(id string) bool

// DFS выполняет обход в глубину.
func (g *Graph) DFS(start string, visit Visitor) {
	seen := make(map[string]bool)
	g.dfs(start, seen, visit)
}

func (g *Graph) dfs(id string, seen map[string]bool, visit Visitor) {
	if seen[id] {
		return
	}
	seen[id] = true
	if !visit(id) {
		return
	}
	for _, s := range g.Successors(id) {
		g.dfs(s, seen, visit)
	}
}

// BFS выполняет обход в ширину.
func (g *Graph) BFS(start string, visit Visitor) {
	if !g.HasNode(start) {
		return
	}
	seen := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if !visit(id) {
			return
		}
		for _, s := range g.Successors(id) {
			if !seen[s] {
				seen[s] = true
				queue = append(queue, s)
			}
		}
	}
}

// TopologicalSort возвращает узлы в топологическом порядке
// (алгоритм Кана, детерминированный). Ошибка возвращается при цикле.
func (g *Graph) TopologicalSort() ([]string, error) {
	inDegree := make(map[string]int, len(g.nodes))
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, e := range g.edges {
		inDegree[e.To]++
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sortStrings(queue)

	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, s := range g.Successors(id) {
			inDegree[s]--
			if inDegree[s] == 0 {
				queue = append(queue, s)
				for i := len(queue) - 1; i > 0 && queue[i] < queue[i-1]; i-- {
					queue[i], queue[i-1] = queue[i-1], queue[i]
				}
			}
		}
	}

	if len(order) != len(g.nodes) {
		return nil, ErrCycleDetected
	}
	return order, nil
}

// HasCycle возвращает true, если в графе существует цикл.
func (g *Graph) HasCycle() bool {
	_, err := g.TopologicalSort()
	return err != nil
}

// TransitiveDependents возвращает все узлы, зависящие от id
// (прямо или транзитивно), в сортированном порядке.
// Используется для частичного пересчёта (ENG-GRAPH-0013).
func (g *Graph) TransitiveDependents(id string) []string {
	affected := make(map[string]bool)
	g.dfs(id, make(map[string]bool), func(n string) bool {
		affected[n] = true
		return true
	})
	var out []string
	for n := range affected {
		if n != id {
			out = append(out, n)
		}
	}
	sortStrings(out)
	return out
}
