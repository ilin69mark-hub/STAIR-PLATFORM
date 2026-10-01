package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/jobs"
)

// SEC-01 + DB-3 (forensic 2026-09-27) — владение заданием и его захват.
//
// SEC-01: фикс SEC-004 ввёл `AND (user_id = $3 OR user_id IS NULL)`, что
// оставило обход owner-скоупа. NULL возникает двумя путями, оба доказаны:
//   (а) Create: `var userID any; if j.UserID != ""` — Bearer-даёт "",
//       задание сохраняется с NULL;
//   (б) FK ON DELETE SET NULL при удалении пользователя.
//
// DB-3: MarkRunning шёл без условия по статусу, поэтому при at-least-once
// доставке два воркера брали одно задание (воспроизведено 4 из 4).
//
// Требует STAIR_TEST_DATABASE_URL; без него тесты скипаются.

func c1xJobRepo(t *testing.T) (*CalcJobRepository, *ProjectRepository, *AuthRepository) {
	t.Helper()
	ar, pr := integrationAuthRepo(t)
	return NewCalcJobRepository(pr.pool), pr, ar
}

func c1xUser(t *testing.T, ar *AuthRepository, tenant, tag string) *auth.User {
	t.Helper()
	u := &auth.User{
		TenantID: tenant, Email: fmt.Sprintf("c1x-%s-%d@test.dev", tag, uniqueSuffix()),
		Name: "C1X", Role: auth.RoleUser, Status: auth.StatusActive,
	}
	if err := ar.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u
}

var c1xCounter struct {
	sync.Mutex
	n int64
}

func uniqueSuffix() int64 {
	c1xCounter.Lock()
	defer c1xCounter.Unlock()
	c1xCounter.n++
	return c1xCounter.n
}

// TestSEC001_OrphanJobIsNotReadableByOtherUser — ядро SEC-01: задание без
// владельца не должно читаться посторонним пользователем того же tenant'а.
func TestSEC001_OrphanJobIsNotReadableByOtherUser(t *testing.T) {
	repo, pr, ar := c1xJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	other := c1xUser(t, ar, tenant, "other")

	// Путь (а): userID == "" — ровно то, что даёт Bearer-аутентификация.
	orphan := &jobs.Job{
		ID:       fmt.Sprintf("c1x-orphan-%d", uniqueSuffix()),
		TenantID: tenant, UserID: "", Type: "calc.calculate", Status: jobs.StatusPending,
	}
	if err := repo.Create(ctx, orphan); err != nil {
		t.Fatalf("Create orphan job: %v", err)
	}
	var isNull bool
	if err := pr.pool.QueryRow(ctx,
		`SELECT user_id IS NULL FROM calc_jobs WHERE id = $1`, orphan.ID).Scan(&isNull); err != nil {
		t.Fatalf("verify null owner: %v", err)
	}
	if !isNull {
		t.Fatal("precondition: job must be stored with NULL user_id")
	}

	if _, err := repo.GetByIDForUser(ctx, tenant, other.ID, orphan.ID); !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("SEC-01: чужой пользователь прочитал осиротевшее задание (err=%v)", err)
	}
}

// TestSEC001_EmptyUserIDCannotReadAnything — без предъявляемого владельца
// чтение невозможно в принципе. Иначе «userID == ”» совпал бы с NULL-владельцем.
func TestSEC001_EmptyUserIDCannotReadAnything(t *testing.T) {
	repo, pr, ar := c1xJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := c1xUser(t, ar, tenant, "owner")

	owned := &jobs.Job{
		ID:       fmt.Sprintf("c1x-owned-%d", uniqueSuffix()),
		TenantID: tenant, UserID: owner.ID, Type: "calc.calculate", Status: jobs.StatusPending,
	}
	if err := repo.Create(ctx, owned); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.GetByIDForUser(ctx, tenant, "", owned.ID); !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("пустой userID не должен давать доступ (err=%v)", err)
	}
}

// TestSEC001_OwnerStillReadsOwnJob — скоуп не должен сломать легитимный путь.
func TestSEC001_OwnerStillReadsOwnJob(t *testing.T) {
	repo, pr, ar := c1xJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := c1xUser(t, ar, tenant, "own")

	j := &jobs.Job{
		ID:       fmt.Sprintf("c1x-own-%d", uniqueSuffix()),
		TenantID: tenant, UserID: owner.ID, Type: "calc.calculate", Status: jobs.StatusPending,
	}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByIDForUser(ctx, tenant, owner.ID, j.ID)
	if err != nil {
		t.Fatalf("владелец обязан читать своё задание: %v", err)
	}
	if got.ID != j.ID {
		t.Errorf("got %s, want %s", got.ID, j.ID)
	}
}

// TestSEC001_JobOfDeletedOwnerIsNotLeaked — путь (б): FK ON DELETE SET NULL
// осиротевшее задание не должно достаться другому пользователю.
func TestSEC001_JobOfDeletedOwnerIsNotLeaked(t *testing.T) {
	repo, pr, ar := c1xJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	victim := c1xUser(t, ar, tenant, "victim")
	survivor := c1xUser(t, ar, tenant, "survivor")

	j := &jobs.Job{
		ID:       fmt.Sprintf("c1x-del-%d", uniqueSuffix()),
		TenantID: tenant, UserID: victim.ID, Type: "calc.calculate", Status: jobs.StatusPending,
	}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.GetByIDForUser(ctx, tenant, victim.ID, j.ID); err != nil {
		t.Fatalf("precondition: owner must read own job: %v", err)
	}
	if _, err := pr.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, victim.ID); err != nil {
		t.Fatalf("delete owner: %v", err)
	}

	if _, err := repo.GetByIDForUser(ctx, tenant, survivor.ID, j.ID); !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("SEC-01: после удаления владельца задание утекло (err=%v)", err)
	}
}

// TestDB3_ConcurrentClaimOnlyOneWins — ядро DB-3. До фикса все конкурентные
// вызовы возвращали nil (воспроизведено 4 из 4).
func TestDB3_ConcurrentClaimOnlyOneWins(t *testing.T) {
	repo, pr, ar := c1xJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := c1xUser(t, ar, tenant, "claim")

	j := &jobs.Job{
		ID:       fmt.Sprintf("c1x-race-%d", uniqueSuffix()),
		TenantID: tenant, UserID: owner.ID, Type: "calc.calculate", Status: jobs.StatusPending,
	}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = repo.MarkRunning(ctx, tenant, j.ID)
		}(i)
	}
	close(start)
	wg.Wait()

	won, already, other := 0, 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			won++
		case errors.Is(e, jobs.ErrAlreadyClaimed):
			already++
		default:
			other++
		}
	}
	if won != 1 {
		t.Fatalf("DB-3: захвативших %d (ожидаем ровно 1); already=%d other=%d", won, already, other)
	}
	if other != 0 {
		t.Errorf("неожиданные ошибки: %d (%v)", other, errs)
	}
}

// TestDB3_ClaimDoesNotWipeFinishedResult — инвариант «running ⇒ result IS
// NULL». Повторный захват уже завершённого задания не должен обнулять
// записанный результат.
func TestDB3_ClaimDoesNotWipeFinishedResult(t *testing.T) {
	repo, pr, ar := c1xJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, pr)
	owner := c1xUser(t, ar, tenant, "done")

	j := &jobs.Job{
		ID:       fmt.Sprintf("c1x-done-%d", uniqueSuffix()),
		TenantID: tenant, UserID: owner.ID, Type: "calc.calculate", Status: jobs.StatusPending,
	}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.MarkRunning(ctx, tenant, j.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if err := repo.MarkSucceeded(ctx, tenant, j.ID, nil); err != nil {
		t.Fatalf("MarkSucceeded: %v", err)
	}

	// Повторный захват обязан быть отвергнут.
	if err := repo.MarkRunning(ctx, tenant, j.ID); !errors.Is(err, jobs.ErrAlreadyClaimed) {
		t.Fatalf("повторный захват успешного задания: err=%v (ожидался ErrAlreadyClaimed)", err)
	}
	var hasResult bool
	var status string
	if err := pr.pool.QueryRow(ctx,
		`SELECT result IS NOT NULL, status FROM calc_jobs WHERE id = $1`, j.ID).
		Scan(&hasResult, &status); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !hasResult {
		t.Error("результат затёрт повторным захватом")
	}
	if status != string(jobs.StatusSucceeded) {
		t.Errorf("status = %q, want succeeded", status)
	}
}

// TestDB3_ClaimMissingJobIsNotFound — «нет такого задания» и «уже взято»
// обязаны оставаться разными ошибками: от них зависят действия воркера.
func TestDB3_ClaimMissingJobIsNotFound(t *testing.T) {
	repo, _, _ := c1xJobRepo(t)
	ctx := context.Background()
	tenant := testTenantID(t, NewProjectRepository(repo.pool))
	err := repo.MarkRunning(ctx, tenant, "00000000-0000-0000-0000-0000c1x0001")
	if !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("want ErrNotFound for a missing job, got %v", err)
	}
	if errors.Is(err, jobs.ErrAlreadyClaimed) {
		t.Error("отсутствующее задание не должно выглядеть как «уже взято»")
	}
}
