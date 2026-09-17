package graphql

import (
	"context"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/events"
)

func TestSubscriptionPipelineStatusChanged(t *testing.T) {
	bus := events.NewBus()
	svc := stair.NewService()
	repo := &MockProjectRepository{}
	r := NewResolver(svc, bus, repo)
	sub := r.Subscription()
	ch, err := sub.PipelineStatusChanged(context.Background(), "cfg-1")
	if err != nil || ch == nil {
		t.Fatalf("want ok, got %v %v", ch, err)
	}
}

func TestSubscriptionAnalysisProgress(t *testing.T) {
	bus := events.NewBus()
	svc := stair.NewService()
	repo := &MockProjectRepository{}
	r := NewResolver(svc, bus, repo)
	ch, err := r.Subscription().AnalysisProgress(context.Background(), "cfg-2")
	if err != nil || ch == nil {
		t.Fatalf("want ok, got %v", err)
	}
}

func TestSubscriptionNotifications(t *testing.T) {
	bus := events.NewBus()
	r := NewResolver(stair.NewService(), bus, &MockProjectRepository{})
	ch, err := r.Subscription().Notifications(context.Background(), "u-1")
	if err != nil || ch == nil {
		t.Fatalf("want ok, got %v", err)
	}
}

func TestNewResolver(t *testing.T) {
	r := NewResolver(nil, nil, nil)
	if r.configs == nil || r.results == nil {
		t.Fatal("want initialized maps")
	}
}
