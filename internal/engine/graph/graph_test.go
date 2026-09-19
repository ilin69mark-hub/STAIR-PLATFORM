package graph

import (
	"reflect"
	"testing"
)

func addNode(t *testing.T, g *Graph, n Node) {
	t.Helper()
	if err := g.AddNode(n); err != nil {
		t.Fatalf("AddNode(%q): %v", n.ID, err)
	}
}
func addEdge(t *testing.T, g *Graph, e Edge) {
	t.Helper()
	if err := g.AddEdge(e); err != nil {
		t.Fatalf("AddEdge(%s→%s): %v", e.From, e.To, err)
	}
}

func TestAddNodeAndLookup(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a", Type: "parameter"})

	if g.NodeCount() != 1 {
		t.Fatalf("expected 1 node, got %d", g.NodeCount())
	}
	n, ok := g.Node("a")
	if !ok {
		t.Fatal("node a not found")
	}
	if n.State != NodeStateDirty {
		t.Fatalf("new node should be dirty, got %s", n.State)
	}
	if n.Version != 1 {
		t.Fatalf("expected version 1, got %d", n.Version)
	}
}

func TestAddNodeEmptyID(t *testing.T) {
	g := New()
	err := g.AddNode(Node{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty node ID")
	}
}

func TestAddEdgeRequiresNodes(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "b"})
	addEdge(t, g, Edge{From: "a", To: "b"})

	if !g.HasNode("b") {
		t.Fatal("node b missing")
	}
	if g.EdgeCount() != 1 {
		t.Fatalf("expected 1 edge, got %d", g.EdgeCount())
	}

	// добавление ребра для несуществующего узла возвращает ошибку.
	err := g.AddEdge(Edge{From: "a", To: "missing"})
	if err == nil {
		t.Fatal("expected error on edge with missing node")
	}
}

func TestRemoveNodeRemovesIncidentEdges(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "b"})
	addNode(t, g, Node{ID: "c"})
	addEdge(t, g, Edge{From: "a", To: "b"})
	addEdge(t, g, Edge{From: "b", To: "c"})

	if !g.RemoveNode("b") {
		t.Fatal("failed to remove b")
	}
	if g.EdgeCount() != 0 {
		t.Fatalf("incident edges should be removed, got %d", g.EdgeCount())
	}
}

func TestTopologicalSort(t *testing.T) {
	g := New()
	for _, id := range []string{"a", "b", "c", "d"} {
		addNode(t, g, Node{ID: id})
	}
	addEdge(t, g, Edge{From: "a", To: "b"})
	addEdge(t, g, Edge{From: "a", To: "c"})
	addEdge(t, g, Edge{From: "c", To: "d"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected cycle error: %v", err)
	}
	expected := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(order, expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
}

func TestCycleDetection(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "b"})
	addEdge(t, g, Edge{From: "a", To: "b"})
	addEdge(t, g, Edge{From: "b", To: "a"})

	if !g.HasCycle() {
		t.Fatal("expected cycle to be detected")
	}
	if _, err := g.TopologicalSort(); err != ErrCycleDetected {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

func TestTransitiveDependents(t *testing.T) {
	g := New()
	for _, id := range []string{"a", "b", "c", "d"} {
		addNode(t, g, Node{ID: id})
	}
	addEdge(t, g, Edge{From: "a", To: "b"})
	addEdge(t, g, Edge{From: "b", To: "c"})
	addEdge(t, g, Edge{From: "c", To: "d"})

	deps := g.TransitiveDependents("a")
	expected := []string{"b", "c", "d"}
	if !reflect.DeepEqual(deps, expected) {
		t.Fatalf("expected %v, got %v", expected, deps)
	}
}

func TestTransitiveDependentsSkipsSelf(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "b"})
	addEdge(t, g, Edge{From: "a", To: "b"})

	deps := g.TransitiveDependents("b")
	if len(deps) != 0 {
		t.Fatalf("expected no dependents for leaf, got %v", deps)
	}
}

func TestSchedulerPartialRebuild(t *testing.T) {
	g := New()
	for _, id := range []string{"p1", "p2", "g", "m"} {
		addNode(t, g, Node{ID: id, Type: "parameter"})
	}
	addEdge(t, g, Edge{From: "p1", To: "g", Type: "depends_on"})
	addEdge(t, g, Edge{From: "p2", To: "g", Type: "depends_on"})
	addEdge(t, g, Edge{From: "g", To: "m", Type: "depends_on"})

	rec := NewScheduler(g)
	plan, err := rec.Plan("p1")
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	expected := []string{"g", "m"}
	if !reflect.DeepEqual(plan, expected) {
		t.Fatalf("partial rebuild plan = %v, want %v", plan, expected)
	}
}

func TestSchedulerExecutesOncePerNode(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "b"})
	addNode(t, g, Node{ID: "c"})
	addEdge(t, g, Edge{From: "a", To: "c"})
	addEdge(t, g, Edge{From: "b", To: "c"})

	rec := NewScheduler(g)
	calls := make(map[string]int)
	err := rec.Execute([]string{"a", "b"}, func(id string) error {
		calls[id]++
		return nil
	})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	// корни изменения (a, b) установлены извне и не пересчитываются.
	if calls["c"] != 1 {
		t.Fatalf("node c must run exactly once, got %d", calls["c"])
	}
	if len(calls) != 1 {
		t.Fatalf("only dependents must be recomputed, got %v", calls)
	}
}

func TestSnapshotRestore(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "b"})
	addEdge(t, g, Edge{From: "a", To: "b"})
	snap := g.Snapshot()

	addNode(t, g, Node{ID: "c"})
	if g.NodeCount() != 3 {
		t.Fatalf("expected 3 nodes, got %d", g.NodeCount())
	}

	g.Restore(snap)
	if g.NodeCount() != 2 {
		t.Fatalf("restore failed: expected 2 nodes, got %d", g.NodeCount())
	}
}

func TestDeterministicOrder(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "z"})
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "m"})
	addNode(t, g, Node{ID: "b"})
	addEdge(t, g, Edge{From: "a", To: "b"})
	addEdge(t, g, Edge{From: "z", To: "m"})
	addEdge(t, g, Edge{From: "m", To: "b"})

	o1, _ := g.TopologicalSort()
	o2, _ := g.TopologicalSort()
	if !reflect.DeepEqual(o1, o2) {
		t.Fatalf("topological sort must be deterministic: %v vs %v", o1, o2)
	}

	var visited []string
	g.BFS("a", func(id string) bool { visited = append(visited, id); return true })
	reb := make([]string, len(visited))
	copy(reb, visited)
	visited = visited[:0]
	g.BFS("a", func(id string) bool { visited = append(visited, id); return true })
	if !reflect.DeepEqual(reb, visited) {
		t.Fatalf("BFS must be deterministic: %v vs %v", reb, visited)
	}
}

func TestDirtyDetection(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a"})
	addNode(t, g, Node{ID: "b"})
	g.MarkClean("a")
	g.MarkClean("b")

	if len(g.DirtyNodes()) != 0 {
		t.Fatalf("expected no dirty nodes, got %v", g.DirtyNodes())
	}
	addNode(t, g, Node{ID: "a"}) // повторное добавление помечает dirty
	dirty := g.DirtyNodes()
	if len(dirty) != 1 || dirty[0] != "a" {
		t.Fatalf("expected [a] dirty, got %v", dirty)
	}
}
