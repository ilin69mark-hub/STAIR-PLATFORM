package graph

import (
	"context"
	"testing"

	domevents "stairplatform/internal/domain/events"
)

func TestRevisionEdgeRemoveEdgePredecessors(t *testing.T) {
	g := New()
	addNode(t, g, Node{ID: "a", Type: "param", Payload: 1})
	addNode(t, g, Node{ID: "b", Type: "param", Payload: 2})
	addEdge(t, g, Edge{ID: "a->b", From: "a", To: "b"})

	if g.Revision() == 0 {
		t.Fatal("revision must advance on mutations")
	}
	e, ok := g.Edge("a->b")
	if !ok || e.From != "a" || e.To != "b" {
		t.Fatalf("Edge lookup failed: %+v", e)
	}
	// Ребро без канонического id — ключ from->to.
	addEdge(t, g, Edge{From: "b", To: "a"})
	if e2, ok := g.Edge("b->a"); !ok || e2.To != "a" {
		t.Fatalf("computed edge key failed: %+v", e2)
	}
	if !g.RemoveEdge("b", "a") {
		t.Fatal("RemoveEdge should succeed")
	}
	if g.RemoveEdge("b", "a") {
		t.Fatal("RemoveEdge must be idempotent-false after removal")
	}

	// Predecessors: для узла b входящее ребро от a.
	pred := g.Predecessors("b")
	if len(pred) != 1 || pred[0] != "a" {
		t.Fatalf("Predecessors(b) = %v, want [a]", pred)
	}
	// Узел без входящих рёбер → пусто.
	if out := g.Predecessors("a"); len(out) != 0 {
		t.Fatalf("Predecessors(a) = %v, want empty", out)
	}
}

func TestVersionError(t *testing.T) {
	ve := &VersionError{Message: "snapshot mismatch"}
	if got := ve.Error(); got != "graph: version error: snapshot mismatch" {
		t.Fatalf("VersionError.Error() = %q", got)
	}
}

func TestGraphRemoveNodeFalseAndEmpty(t *testing.T) {
	g := New()
	if g.RemoveNode("ghost") {
		t.Fatal("RemoveNode of missing node should return false")
	}
}

// stubPublisher фиксирует количество PublishAsync-вызовов.
type stubPublisher struct{ asyncCalls int }

func (s *stubPublisher) Publish(context.Context, domevents.Event) error { return nil }
func (s *stubPublisher) PublishAsync(_ context.Context, _ domevents.Event) {
	s.asyncCalls++
}

func TestEventGraphHooks(t *testing.T) {
	ctx := context.Background()
	var added, removed, edgeAdded, dirtied int
	eg := NewEventGraph(nil, GraphEventHooks{
		OnNodeAdded:   func(context.Context, Node) { added++ },
		OnNodeRemoved: func(context.Context, string) { removed++ },
		OnEdgeAdded:   func(context.Context, Edge) { edgeAdded++ },
		OnNodeDirtied: func(context.Context, string) { dirtied++ },
	})

	if err := eg.AddNode(ctx, Node{ID: "a"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := eg.AddNode(ctx, Node{ID: "b"}); err != nil {
		t.Fatalf("AddNode b: %v", err)
	}
	if err := eg.AddEdge(ctx, Edge{From: "a", To: "b"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if !eg.RemoveNode(ctx, "b") {
		t.Fatal("RemoveNode should return true")
	}
	eg.MarkClean(ctx, "a")
	eg.MarkClean(ctx, "ghost") // несуществующий узел — без panic

	if added != 2 || removed != 1 || edgeAdded != 1 || dirtied != 2 {
		t.Fatalf("hooks: added=%d removed=%d edgeAdded=%d dirtied=%d", added, removed, edgeAdded, dirtied)
	}

	// Ошибки транслируются до вызова hooks.
	if err := eg.AddNode(ctx, Node{ID: ""}); err == nil {
		t.Fatal("AddNode empty id must fail")
	}
	if err := eg.AddEdge(ctx, Edge{From: "a", To: "ghost"}); err == nil {
		t.Fatal("AddEdge to missing node must fail")
	}
}

func TestEventGraphNoHooks(t *testing.T) {
	ctx := context.Background()
	eg := NewEventGraph(nil) // без hooks
	if err := eg.AddNode(ctx, Node{ID: "a"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if !eg.HasNode("a") {
		t.Fatal("node missing after AddNode")
	}
}

func TestDefaultEventHooks(t *testing.T) {
	ctx := context.Background()
	// nil-publisher: публикации не происходит, но hook вызывается.
	stub := &stubPublisher{}
	hooks := DefaultEventHooks(stub)
	hooks.OnNodeAdded(ctx, Node{ID: "n1", Type: "straight"})
	if stub.asyncCalls != 1 {
		t.Fatalf("expected 1 async publish, got %d", stub.asyncCalls)
	}
	// nil publisher — ранний выход без паники.
	nilHooks := DefaultEventHooks(nil)
	nilHooks.OnNodeAdded(ctx, Node{ID: "n2"})
	// OnNodeRemoved — no-op.
	nilHooks.OnNodeRemoved(ctx, "n2")
}
