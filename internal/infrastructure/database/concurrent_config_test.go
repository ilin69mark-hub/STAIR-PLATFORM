package database

import (
	"context"
	"sync"
	"testing"

	"stairplatform/internal/application/project"
	"stairplatform/internal/engine/validation"
)

// TestConcurrentConfigurationRevision (DB-001, forensic 2026-09-24) —
// конкурентные сохранения конфигурации одного проекта не должны падать на
// UNIQUE(project_id, revision) и не должны терять записи: каждая из N
// параллельных операций получает свою ревизию.
func TestConcurrentConfigurationRevision(t *testing.T) {
	repo := integrationAI(t)
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)
	p := &project.Project{Name: "Race", Description: "t", Status: project.StatusDraft}
	if err := pr.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	const workers = 8
	ids := make([]string, workers)
	errs := make([]error, workers)
	revs := make([]int, workers)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // максимально одновременный старт
			cfg := &project.StairConfiguration{
				ProjectID: p.ID, WidthMM: 900, HeightMM: 2700, Flight: "straight",
				StepHeightMM: 180, ComfortStepMM: 630,
			}
			_, errs[i] = pr.SaveCalculationWithConfig(ctx, tenant, cfg,
				project.Snapshot{Validation: validation.Result{Valid: true}})
			ids[i] = cfg.ID
			revs[i] = cfg.Revision
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d failed (DB-001 регрессия): %v", i, err)
		}
	}
	seenID := make(map[string]int, workers)
	seenRev := make(map[int]int, workers)
	for i := range ids {
		if ids[i] == "" {
			t.Fatalf("worker %d: пустой id конфигурации", i)
		}
		if prev, dup := seenID[ids[i]]; dup {
			t.Fatalf("worker %d: id совпал с worker %d (%s)", i, prev, ids[i])
		}
		seenID[ids[i]] = i
		if revs[i] < 1 || revs[i] > workers {
			t.Fatalf("worker %d: ревизия %d вне [1,%d]", i, revs[i], workers)
		}
		if prev, dup := seenRev[revs[i]]; dup {
			t.Fatalf("ревизия %d продублирована (workers %d и %d) — гонка ревизий", revs[i], prev, i)
		}
		seenRev[revs[i]] = i
	}
	if len(seenRev) != workers {
		t.Fatalf("ожидалось %d уникальных ревизий, получено %d", workers, len(seenRev))
	}

	// Текущая ревизия проекта соответствует одной из сохранённых.
	cur, err := pr.GetLatestConfiguration(ctx, tenant, p.ID)
	if err != nil {
		t.Fatalf("GetLatestConfiguration: %v", err)
	}
	if _, ok := seenID[cur.ID]; !ok {
		t.Fatalf("текущая конфигурация %s не из набора сохранённых", cur.ID)
	}
}
