package graph

import (
	"context"

	domevents "stairplatform/internal/domain/events"
)

// EventPublisher — интерфейс для публикации событий из Graph Engine.
// Определяется в graph package для избежания циклических зависимостей.
type EventPublisher interface {
	Publish(ctx context.Context, event domevents.Event) error
	PublishAsync(ctx context.Context, event domevents.Event)
}

// GraphEventHooks — обработчики событий графа (вызываются при мутациях).
type GraphEventHooks struct {
	OnNodeAdded    func(ctx context.Context, node Node)
	OnNodeRemoved  func(ctx context.Context, nodeID string)
	OnNodeUpdated  func(ctx context.Context, node Node)
	OnNodeDirtied  func(ctx context.Context, nodeID string)
	OnEdgeAdded    func(ctx context.Context, edge Edge)
	OnEdgeRemoved  func(ctx context.Context, from, to string)
	OnGraphChanged func(ctx context.Context, revision uint64)
}

// EventGraph — Graph Engine с event hooks для интеграции с Event Bus.
type EventGraph struct {
	*Graph
	hooks     GraphEventHooks
	publisher EventPublisher
}

// NewEventGraph создаёт Graph с event hooks.
func NewEventGraph(publisher EventPublisher, hooks ...GraphEventHooks) *EventGraph {
	var h GraphEventHooks
	if len(hooks) > 0 {
		h = hooks[0]
	}
	return &EventGraph{
		Graph:     New(),
		hooks:     h,
		publisher: publisher,
	}
}

// AddNode добавляет узел и вызывает hooks.
func (eg *EventGraph) AddNode(ctx context.Context, n Node) error {
	if err := eg.Graph.AddNode(n); err != nil {
		return err
	}
	if eg.hooks.OnNodeAdded != nil {
		eg.hooks.OnNodeAdded(ctx, n)
	}
	return nil
}

// RemoveNode удаляет узел и вызывает hooks.
func (eg *EventGraph) RemoveNode(ctx context.Context, id string) bool {
	n, existed := eg.Node(id)
	ok := eg.Graph.RemoveNode(id)
	if ok && eg.hooks.OnNodeRemoved != nil {
		eg.hooks.OnNodeRemoved(ctx, id)
	}
	_ = existed
	_ = n
	return ok
}

// AddEdge добавляет ребро и вызывает hooks.
func (eg *EventGraph) AddEdge(ctx context.Context, e Edge) error {
	if err := eg.Graph.AddEdge(e); err != nil {
		return err
	}
	if eg.hooks.OnEdgeAdded != nil {
		eg.hooks.OnEdgeAdded(ctx, e)
	}
	return nil
}

// MarkClean снимает dirty-статус и вызывает hooks.
func (eg *EventGraph) MarkClean(ctx context.Context, id string) {
	eg.Graph.MarkClean(id)
	if eg.hooks.OnNodeDirtied != nil {
		// MarkClean — это "обновлённый узел", оповещаем.
		eg.hooks.OnNodeDirtied(ctx, id)
	}
}

// WithContext создаёт GraphEventHooks, который публикует события через publisher.
func DefaultEventHooks(publisher EventPublisher) GraphEventHooks {
	return GraphEventHooks{
		OnNodeAdded: func(ctx context.Context, node Node) {
			if publisher == nil {
				return
			}
			event := domevents.GeometryUpdated{
				BaseEvent: domevents.BaseEvent{
					Meta: domevents.NewEventMetadata(
						domevents.EventGeometryUpdated,
						"graph",
						"system",
						"",
						domevents.NewCorrelationID(),
						"",
					),
				},
				GeometryID: node.ID,
				FlightType: node.Type,
			}
			publisher.PublishAsync(ctx, event)
		},
		OnNodeRemoved: func(ctx context.Context, nodeID string) {
			// Можно добавить событие удаления при необходимости.
		},
	}
}
