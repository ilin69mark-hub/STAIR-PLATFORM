package graph

// Snapshot — неизменяемая копия состояния графа (ENG-GRAPH-0008).
// Идеальна для Undo/Redo, версионирования, аудита и воспроизводимых расчётов.
type Snapshot struct {
	Revision uint64
	Nodes    []Node
	Edges    []Edge
}

// Snapshot создаёт неизменяемый снимок текущего состояния.
func (g *Graph) Snapshot() Snapshot {
	nodes := make([]Node, len(g.nodes))
	edges := make([]Edge, len(g.edges))
	copy(nodes, g.Nodes())

	ids := make([]string, 0, len(g.edges))
	for id := range g.edges {
		ids = append(ids, id)
	}
	sortStrings(ids)
	for i, id := range ids {
		edges[i] = g.edges[id]
	}
	return Snapshot{Revision: g.revision, Nodes: nodes, Edges: edges}
}

// Restore восстанавливает состояние графа из снапшота.
func (g *Graph) Restore(s Snapshot) {
	g.nodes = make(map[string]Node, len(s.Nodes))
	g.edges = make(map[string]Edge, len(s.Edges))
	g.outgoing = make(map[string]map[string]struct{})
	g.incoming = make(map[string]map[string]struct{})
	for _, n := range s.Nodes {
		g.nodes[n.ID] = n
		g.outgoing[n.ID] = make(map[string]struct{})
		g.incoming[n.ID] = make(map[string]struct{})
	}
	for _, e := range s.Edges {
		g.edges[e.ID] = e
		g.outgoing[e.From][e.To] = struct{}{}
		g.incoming[e.To][e.From] = struct{}{}
	}
	g.revision = s.Revision
}
