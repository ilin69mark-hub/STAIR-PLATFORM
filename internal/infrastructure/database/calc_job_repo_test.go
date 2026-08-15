package database

import (
	"context"
	"errors"
	"os"
	"testing"

	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

func newCalcJobRepo(t *testing.T) (*CalcJobRepository, *ProjectRepository) {
	t.Helper()
	url := os.Getenv("STAIR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STAIR_TEST_DATABASE_URL not set; skipping database integration test")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return NewCalcJobRepository(pool), NewProjectRepository(pool)
}

// sampleJobPayload — валидный вход расчёта для calc_jobs (прямой марш).
func sampleJobPayload() jobs.Payload {
	return jobs.Payload{Config: stair.Config{
		Width:             900,
		Height:            2700,
		Flight:            engineering.FlightStraight,
		StepHeight:        180,
		StringerThickness: 50,
		StepThickness:     40,
		Clearance:         80,
		RailingHeight:     900,
	}}
}

// TestCalcJobLifecycle: create(pending) → get → mark running → succeeded с
// результатом; проверяется консистентность статусов и данных.
func TestCalcJobLifecycle(t *testing.T) {
	repo, proj := newCalcJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, proj)
	owner := testOwnerID(t, proj, tenant)

	j := &jobs.Job{
		ID:       itoaUD(),
		TenantID: tenant,
		UserID:   owner,
		Type:     "calc.calculate",
		Status:   jobs.StatusPending,
		Payload:  sampleJobPayload(),
	}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if j.ID == "" || j.CreatedAt.IsZero() {
		t.Fatalf("expected assigned id/created_at, got %+v", j)
	}

	got, err := repo.GetByID(ctx, tenant, j.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != jobs.StatusPending || got.UserID != owner || got.Type != "calc.calculate" {
		t.Fatalf("unexpected job: %+v", got)
	}
	if got.Payload.Config.Width != 900 || got.Payload.Config.Height != 2700 {
		t.Fatalf("payload not roundtripped: %+v", got.Payload.Config)
	}

	if err := repo.MarkRunning(ctx, tenant, j.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	res := &stair.Result{}
	if err := repo.MarkSucceeded(ctx, tenant, j.ID, res); err != nil {
		t.Fatalf("MarkSucceeded: %v", err)
	}
	got, err = repo.GetByID(ctx, tenant, j.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != jobs.StatusSucceeded || got.Result == nil || got.Error != "" {
		t.Fatalf("unexpected after success: %+v", got)
	}
	if got.StartedAt.IsZero() || got.FinishedAt.IsZero() {
		t.Fatalf("timestamps not set: %+v", got)
	}
}

// TestCalcJobFailure: mark failed persists текст ошибки (инвариант 2).
func TestCalcJobFailure(t *testing.T) {
	repo, proj := newCalcJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, proj)

	j := &jobs.Job{ID: itoaUD(), TenantID: tenant, Type: "calc.calculate",
		Status: jobs.StatusPending, Payload: sampleJobPayload()}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.MarkFailed(ctx, tenant, j.ID, "boom: geometry failed"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got, err := repo.GetByID(ctx, tenant, j.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != jobs.StatusFailed || got.Error != "boom: geometry failed" || got.Result != nil {
		t.Fatalf("unexpected after failure: %+v", got)
	}
}

// TestCalcJobTenantScope: чужая tenant'ом запись не видна (инвариант 3).
func TestCalcJobTenantScope(t *testing.T) {
	repo, proj := newCalcJobRepo(t)
	ctx := context.Background()
	tenantA := testTenantID(t, proj)

	j := &jobs.Job{ID: itoaUD(), TenantID: tenantA, Type: "calc.calculate",
		Status: jobs.StatusPending, Payload: sampleJobPayload()}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000002", j.ID); !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for other tenant, got %v", err)
	}
	if err := repo.MarkRunning(ctx, "00000000-0000-0000-0000-000000000002", j.ID); !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on update for other tenant, got %v", err)
	}
}
