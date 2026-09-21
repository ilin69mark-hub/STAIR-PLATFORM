package jobs

import (
	"context"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/queue"
)

func TestSubmitCalculateQueueNotConfigured(t *testing.T) {
	svc := testService(newFakeRepo(), nil, nil)
	if _, err := svc.SubmitCalculate(context.Background(), "t-1", "u-1", Payload{}); err == nil {
		t.Fatal("expected error for nil queue")
	}
}

func TestRunCalculateCalculatorNotConfigured(t *testing.T) {
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), nil)
	if err := svc.RunCalculate(context.Background(), "t-1", "no-such-job"); err == nil {
		t.Fatal("expected error for nil calculator")
	}
}

func TestRunCalculateJobNotFound(t *testing.T) {
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), func(_ context.Context, _ stair.Config, _ stair.Options) (*stair.Result, error) {
		return &stair.Result{}, nil
	})
	if err := svc.RunCalculate(context.Background(), "t-1", "no-such-job"); err == nil {
		t.Fatal("expected error for missing job")
	}
}

func TestSubmitCalculateNilContext(t *testing.T) {
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), nil)
	//nolint:staticcheck // намеренно nil-ctx: проверяем guard в Service (nil -> Background).
	if j, err := svc.SubmitCalculate(nil, "t-1", "u-1", Payload{}); err != nil {
		t.Fatalf("submit with nil ctx: %v", err)
	} else if j.ID == "" {
		t.Fatal("expected assigned job id")
	}
}

func TestRunCalculateNilContext(t *testing.T) {
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), func(_ context.Context, _ stair.Config, _ stair.Options) (*stair.Result, error) {
		return &stair.Result{}, nil
	})
	//nolint:staticcheck // намеренно nil-ctx: проверяем guard в Service (nil -> Background).
	if err := svc.RunCalculate(nil, "t-1", "no-such-job"); err == nil {
		t.Fatal("expected error for missing job")
	}
}

func TestGetJobNilContext(t *testing.T) {
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), nil)
	j, err := svc.SubmitCalculate(context.Background(), "t-1", "u-1", Payload{})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	//nolint:staticcheck // намеренно nil-ctx: проверяем guard в Service (nil -> Background).
	got, err := svc.GetJob(nil, "t-1", j.ID)
	if err != nil {
		t.Fatalf("get with nil ctx: %v", err)
	}
	if got.ID != j.ID {
		t.Fatal("job id mismatch")
	}
}
