// Package scheduler implements a bounded parallel executor with
// deterministic result-slot semantics (Phase B, B3, EDR-0034). Tasks are
// addressed by index: each task writes its result/error into its own slot,
// and the returned error is the first one in index order (or the context
// cancellation). This preserves determinism (ADR-0003) while parallelizing
// independent build/nesting cycles: the order of completion never affects
// the outcome.
package scheduler

import (
	"context"
	"runtime"
	"sync"
)

// Scheduler — ограниченный параллельный исполнитель задач по слотам.
// Zero value непригодно — используйте New.
type Scheduler struct {
	limit int
}

// New создаёт Scheduler с лимитом параллелизма; limit <= 0 → GOMAXPROCS(0).
// limit == 1 даёт строго последовательное выполнение (без goroutine).
func New(limit int) *Scheduler {
	if limit <= 0 {
		limit = runtime.GOMAXPROCS(0)
	}
	if limit < 1 {
		limit = 1
	}
	return &Scheduler{limit: limit}
}

// Execute запускает fn(i) для i ∈ [0, n) с ограниченным параллелизмом.
// Каждая задача сама записывает результат в слот i (result-slot), поэтому
// порядок сборки не влияет на итог. Возвращает:
//   - nil, если все задачи завершились без ошибки;
//   - ошибку первого по возрастанию индекса слота с ошибкой (детерминизм);
//   - ошибку отмены контекста, если контекст был отменён до/во время работы.
func (s *Scheduler) Execute(ctx context.Context, n int, fn func(i int) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if n <= 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.limit == 1 || n == 1 {
		// Последовательный путь: без оверхеда goroutine.
		for i := 0; i < n; i++ {
			if err := fn(i); err != nil {
				return err
			}
		}
		return nil
	}

	errs := make([]error, n)
	sem := make(chan struct{}, s.limit)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Пара задач: acquire/release — только при успешном захвате
			// слота (иначе defer снял бы чужой токен и разбалансировал пул).
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			defer func() { <-sem }()
			if err := ctx.Err(); err != nil {
				errs[i] = err
				return
			}
			errs[i] = fn(i)
		}()
	}
	wg.Wait()

	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
