// Package graph реализует STAIR-KERNEL Graph Engine (ENG-GRAPH-0001).
// Домен-независимый граф зависимостей: Node/Edge registry, обход,
// разрешение зависимостей, инкрементальное обновление, снапшоты.
package graph

import "fmt"

const (
	// NodeStateDirty — узел изменён и требует пересчёта зависимых.
	NodeStateDirty = "dirty"
	// NodeStateClean — узел актуален.
	NodeStateClean = "clean"
)

// Node — вершина графа зависимостей.
type Node struct {
	ID       string
	Type     string
	Version  uint64
	State    string
	Payload  any
	Metadata map[string]string
}

// Edge — направленное ребро зависимостей from → to.
type Edge struct {
	ID       string
	From     string
	To       string
	Type     string
	Weight   float64
	Metadata map[string]string
}

// Graph — in-memory граф с O(1) доступом к узлу и O(E) обходом.
type Graph struct {
	nodes    map[string]Node
	edges    map[string]Edge
	outgoing map[string]map[string]struct{}
	incoming map[string]map[string]struct{}
	revision uint64
}

// New создаёт пустой граф.
func New() *Graph {
	return &Graph{
		nodes:    make(map[string]Node),
		edges:    make(map[string]Edge),
		outgoing: make(map[string]map[string]struct{}),
		incoming: make(map[string]map[string]struct{}),
	}
}

// Revision возвращает неизменяемый номер ревизии графа.
func (g *Graph) Revision() uint64 { return g.revision }

// NodeCount возвращает количество узлов.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// EdgeCount возвращает количество рёбер.
func (g *Graph) EdgeCount() int { return len(g.edges) }

// AddNode добавляет или обновляет узел и отмечает его dirty.
func (g *Graph) AddNode(n Node) error {
	if n.ID == "" {
		return fmt.Errorf("graph: node id must not be empty")
	}
	n.Version++
	n.State = NodeStateDirty
	g.nodes[n.ID] = n
	if _, ok := g.outgoing[n.ID]; !ok {
		g.outgoing[n.ID] = make(map[string]struct{})
		g.incoming[n.ID] = make(map[string]struct{})
	}
	g.revision++
	return nil
}

// Node возвращает узел и признак его существования (O(1)).
func (g *Graph) Node(id string) (Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

// HasNode проверяет существование узла.
func (g *Graph) HasNode(id string) bool {
	_, ok := g.nodes[id]
	return ok
}

// RemoveNode удаляет узел и все инцидентные рёбра.
func (g *Graph) RemoveNode(id string) bool {
	if _, ok := g.nodes[id]; !ok {
		return false
	}
	for from := range g.incoming[id] {
		g.removeEdgeFrom(from, id)
	}
	for to := range g.outgoing[id] {
		g.removeEdgeFrom(id, to)
	}
	delete(g.nodes, id)
	delete(g.outgoing, id)
	delete(g.incoming, id)
	g.revision++
	return true
}

// AddEdge добавляет ребро from → to.
func (g *Graph) AddEdge(e Edge) error {
	if !g.HasNode(e.From) || !g.HasNode(e.To) {
		return fmt.Errorf("graph: edge requires existing nodes (%s -> %s)", e.From, e.To)
	}
	if e.ID == "" {
		e.ID = e.From + "->" + e.To
	}
	g.edges[e.ID] = e
	g.outgoing[e.From][e.To] = struct{}{}
	g.incoming[e.To][e.From] = struct{}{}
	g.revision++
	return nil
}

// Edge возвращает ребро по ID.
func (g *Graph) Edge(id string) (Edge, bool) {
	e, ok := g.edges[id]
	return e, ok
}

// RemoveEdge удаляет ребро from → to.
func (g *Graph) RemoveEdge(from, to string) bool {
	return g.removeEdgeFrom(from, to)
}

func (g *Graph) removeEdgeFrom(from, to string) bool {
	key := from + "->" + to
	if _, ok := g.edges[key]; !ok {
		// поиск по каноническому ключу; при отсутствии пробуем сканирование
		key = ""
	}
	if key == "" {
		for id, e := range g.edges {
			if e.From == from && e.To == to {
				key = id
				break
			}
		}
		if key == "" {
			return false
		}
	}
	delete(g.edges, key)
	delete(g.outgoing[from], to)
	delete(g.incoming[to], from)
	g.revision++
	return true
}

// Successors возвращает исходящие узлы (детерминированно).
func (g *Graph) Successors(id string) []string {
	succ := make([]string, 0, len(g.outgoing[id]))
	for s := range g.outgoing[id] {
		succ = append(succ, s)
	}
	sortStrings(succ)
	return succ
}

// Predecessors возвращает входящие узлы (детерминированно).
func (g *Graph) Predecessors(id string) []string {
	pred := make([]string, 0, len(g.incoming[id]))
	for p := range g.incoming[id] {
		pred = append(pred, p)
	}
	sortStrings(pred)
	return pred
}

// Nodes возвращает все узлы в сортированном порядке (детерминированно).
func (g *Graph) Nodes() []Node {
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sortStrings(ids)
	out := make([]Node, 0, len(ids))
	for _, id := range ids {
		out = append(out, g.nodes[id])
	}
	return out
}

// MarkClean снимает dirty-статус с узла.
func (g *Graph) MarkClean(id string) {
	n, ok := g.nodes[id]
	if !ok {
		return
	}
	n.State = NodeStateClean
	g.nodes[id] = n
}

// DirtyNodes возвращает узлы в состоянии dirty.
func (g *Graph) DirtyNodes() []string {
	var out []string
	for id, n := range g.nodes {
		if n.State == NodeStateDirty {
			out = append(out, id)
		}
	}
	sortStrings(out)
	return out
}

func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}
