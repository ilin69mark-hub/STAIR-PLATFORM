package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/infrastructure/queue"
)

// fakeRepo — in-memory реализация порта Repository для юнит-тестов.
type fakeRepo struct {
	jobs map[string]*Job
}

func newFakeRepo() *fakeRepo { return &fakeRepo{jobs: map[string]*Job{}} }

func (f *fakeRepo) Create(_ context.Context, j *Job) error {
	f.jobs[j.ID] = j
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, tenantID, id string) (*Job, error) {
	j, ok := f.jobs[id]
	if !ok || j.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return j, nil
}

func (f *fakeRepo) MarkRunning(_ context.Context, tenantID, id string) error {
	j, err := f.GetByID(context.Background(), tenantID, id)
	if err != nil {
		return err
	}
	j.Status = StatusRunning
	return nil
}

func (f *fakeRepo) MarkSucceeded(_ context.Context, tenantID, id string, res *stair.Result) error {
	j, err := f.GetByID(context.Background(), tenantID, id)
	if err != nil {
		return err
	}
	j.Status = StatusSucceeded
	j.Result = res
	return nil
}

func (f *fakeRepo) MarkFailed(_ context.Context, tenantID, id, errMsg string) error {
	j, err := f.GetByID(context.Background(), tenantID, id)
	if err != nil {
		return err
	}
	j.Status = StatusFailed
	j.Error = errMsg
	return nil
}

// fakeFailQueue — очередь, чей Enqueue падает (сбой постановки задания).
type fakeFailQueue struct{ queue.JobQueue }

func (fakeFailQueue) Enqueue(context.Context, queue.Job) error { return errors.New("redis down") }

func testService(repo Repository, q queue.JobQueue, calc CalculateFunc) *Service {
	return NewService(repo, q, calc)
}

func TestSubmitCalculateEnqueues(t *testing.T) {
	ctx := context.Background()
	q := queue.NewMemoryQueue()
	svc := testService(newFakeRepo(), q, nil)

	cfg := stair.Config{
		Width: engineering.Length(900), Height: engineering.Length(2700),
		Flight: engineering.FlightStraight, StepHeight: engineering.Length(180),
	}
	j, err := svc.SubmitCalculate(ctx, "t-1", "u-1", Payload{Config: cfg, Options: stair.Options{}})
	if err != nil {
		t.Fatalf("SubmitCalculate: %v", err)
	}
	if j.ID == "" || j.Status != StatusPending || j.Type != queue.JobCalcCalculate {
		t.Fatalf("unexpected job: %+v", j)
	}
	if j.TenantID != "t-1" || j.UserID != "u-1" {
		t.Fatalf("scope: %+v", j)
	}

	// В очередь попало задание calc.calculate с лёгким payload {job_id,tenant_id}.
	dq, ok, err := q.Dequeue(ctx)
	if err != nil || !ok {
		t.Fatalf("dequeue: ok=%v err=%v", ok, err)
	}
	if dq.Type != queue.JobCalcCalculate {
		t.Fatalf("type = %q", dq.Type)
	}
	var p struct {
		JobID    string `json:"job_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(dq.Payload, &p); err != nil {
		t.Fatalf("unmarshal queue payload: %v", err)
	}
	if p.JobID != j.ID || p.TenantID != "t-1" {
		t.Fatalf("queue payload = %+v, want job_id %s tenant t-1", p, j.ID)
	}
}

func TestSubmitCalculateEnqueueFailureMarksFailed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := testService(repo, fakeFailQueue{}, nil)

	_, err := svc.SubmitCalculate(ctx, "t-1", "u-1", Payload{})
	if err == nil {
		t.Fatal("expected enqueue error")
	}
	// Инвариант 1: запись не сирота — помечена failed.
	id := ""
	for k := range repo.jobs {
		id = k
	}
	j := repo.jobs[id]
	if j.Status != StatusFailed || j.Error == "" {
		t.Fatalf("expected failed with error, got %+v", j)
	}
}

func TestSubmitCalculateInvalidTenant(t *testing.T) {
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), nil)
	if _, err := svc.SubmitCalculate(context.Background(), "", "u", Payload{}); err == nil {
		t.Fatal("expected error for empty tenant")
	}
}

func TestRunCalculateSuccess(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	calc := func(_ context.Context, _ stair.Config, _ stair.Options) (*stair.Result, error) {
		return &stair.Result{}, nil
	}
	svc := testService(repo, queue.NewMemoryQueue(), calc)

	j, err := svc.SubmitCalculate(ctx, "t-1", "u-1", Payload{})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := svc.RunCalculate(ctx, "t-1", j.ID); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := svc.GetJob(ctx, "t-1", j.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusSucceeded || got.Result == nil {
		t.Fatalf("unexpected after run: %+v", got)
	}
}

func TestRunCalculateFailure(t *testing.T) {
	ctx := context.Background()
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), func(_ context.Context, _ stair.Config, _ stair.Options) (*stair.Result, error) {
		return nil, errors.New("boom: geometry failed")
	})
	j, err := svc.SubmitCalculate(ctx, "t-1", "u-1", Payload{})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := svc.RunCalculate(ctx, "t-1", j.ID); err == nil {
		t.Fatalf("expected run error")
	}
	got, err := svc.GetJob(ctx, "t-1", j.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusFailed || got.Error == "" {
		t.Fatalf("unexpected after failure: %+v", got)
	}
}

func TestGetJobScopedByTenant(t *testing.T) {
	ctx := context.Background()
	svc := testService(newFakeRepo(), queue.NewMemoryQueue(), nil)
	j, err := svc.SubmitCalculate(ctx, "t-1", "u-1", Payload{})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.GetJob(ctx, "t-2", j.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for other tenant, got %v", err)
	}
}

// TestResultJSONRoundtrip: stair.Result (реальный расчёт) безопасен для
// хранения в calc_jobs.result JSONB — полный конвейер сериализуется и
// восстанавливается без потерь (инвариант 2, EDR-0035).
func TestResultJSONRoundtrip(t *testing.T) {
	cfg := stair.Config{
		Width:             900,
		Height:            2700,
		Flight:            engineering.FlightStraight,
		StepHeight:        180,
		StringerThickness: 50,
		StepThickness:     40,
		Clearance:         80,
		RailingHeight:     900,
	}
	res, err := stair.NewService().Calculate(context.Background(), cfg, stair.Options{})
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}

	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var got stair.Result
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	fn, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(fn) != string(b) {
		t.Fatal("result didn't survive JSON roundtrip")
	}
}
