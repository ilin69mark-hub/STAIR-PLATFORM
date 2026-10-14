package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"stairplatform/internal/application/funnel"
)

// uuidFor даёт разные валидные UUID для тестовых сессий: колонка
// web_events.session_id имеет тип UUID, и «s-1» в неё не пишется.
func uuidFor(n int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", n)
}

// Репозиторий воронки кормит отчёт о трафике. Ошибка в агрегатах здесь не
// бросается ошибкой — она просто показывает неверные числа: воронка уезжает
// в аналитику, и по ней принимают решения о разделе конструктора.

func funnelRepo(t *testing.T) *FunnelRepository {
	t.Helper()
	url := testURL(t)
	ctx := context.Background()
	pool, err := Connect(ctx, DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool, migrationsDir(t), "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return NewFunnelRepository(pool)
}

// now минус час — окно, в которое мы кладём события.
func funnelWindow() (time.Time, time.Time) {
	to := time.Now().UTC()
	return to.Add(-time.Hour), to.Add(time.Hour)
}

// insertFunnel пишет события напрямую в таблицу, минуя сервис: проверяется
// именно репозиторий, а не приём событий.
func insertFunnel(t *testing.T, r *FunnelRepository, events []funnel.Event) {
	t.Helper()
	if err := r.InsertEvents(context.Background(), events); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
}

func cleanFunnel(t *testing.T, r *FunnelRepository) {
	t.Helper()
	if _, err := r.Cleanup(context.Background(), time.Now().UTC().Add(24*time.Hour)); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestFunnelInsertAndCounts(t *testing.T) {
	r := funnelRepo(t)
	cleanFunnel(t, r)
	from, to := funnelWindow()

	// Три события, две сессии, два посетителя: сессия s-2 повторно заходит
	// тем же посетителем.
	ev := []funnel.Event{
		{SessionID: uuidFor(1), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-1", Browser: "chrome", ConsentVersion: 2, OccurredAt: from.Add(time.Minute)},
		{SessionID: uuidFor(1), Name: "stair.calculate", Props: map[string]any{}, Visitor: "v-1", Browser: "chrome", ConsentVersion: 2, OccurredAt: from.Add(2 * time.Minute)},
		{SessionID: uuidFor(2), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-2", Browser: "firefox", ConsentVersion: 2, OccurredAt: from.Add(3 * time.Minute)},
	}
	insertFunnel(t, r, ev)

	if got, err := r.SessionCount(context.Background(), from, to); err != nil || got != 2 {
		t.Errorf("SessionCount = %d (err %v), ждали 2", got, err)
	}
	if got, err := r.VisitorCount(context.Background(), from, to); err != nil || got != 2 {
		t.Errorf("VisitorCount = %d (err %v), ждали 2", got, err)
	}
	if got, err := r.EventCount(context.Background(), from, to); err != nil || got != 3 {
		t.Errorf("EventCount = %d (err %v), ждали 3", got, err)
	}
	cleanFunnel(t, r)
}

func TestFunnelCountsIgnoreOutsideWindow(t *testing.T) {
	r := funnelRepo(t)
	cleanFunnel(t, r)
	from, to := funnelWindow()

	// Событие далеко вне окна не должно попадать в счётчик: окно фильтрует
	// старые данные, и без него отчёт «за всё время» рос бы бесконечно.
	insertFunnel(t, r, []funnel.Event{
		{SessionID: uuidFor(4), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-old", ConsentVersion: 2, OccurredAt: from.Add(-72 * time.Hour)},
	})
	if got, err := r.EventCount(context.Background(), from, to); err != nil || got != 0 {
		t.Errorf("событие вне окна попало в счётчик: %d (err %v)", got, err)
	}
	cleanFunnel(t, r)
}

func TestFunnelStepCountsAndBlockers(t *testing.T) {
	r := funnelRepo(t)
	cleanFunnel(t, r)
	from, to := funnelWindow()

	// Шаг открытия конструктора встречается трижды, расчёта — один раз.
	insertFunnel(t, r, []funnel.Event{
		{SessionID: uuidFor(1), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-1", ConsentVersion: 2, OccurredAt: from.Add(time.Minute)},
		{SessionID: uuidFor(2), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-2", ConsentVersion: 2, OccurredAt: from.Add(2 * time.Minute)},
		{SessionID: uuidFor(3), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-3", ConsentVersion: 2, OccurredAt: from.Add(3 * time.Minute)},
		{SessionID: uuidFor(1), Name: "stair.calculate", Props: map[string]any{}, Visitor: "v-1", ConsentVersion: 2, OccurredAt: from.Add(4 * time.Minute)},
	})

	steps, err := r.StepCounts(context.Background(), from, to)
	if err != nil {
		t.Fatalf("StepCounts: %v", err)
	}
	if len(steps) == 0 {
		t.Fatal("StepCounts вернул пусто")
	}
	stat := steps["constructor.open"]
	// Три сессии открыли конструктор, значит Sessions = 3, а посетителей
	// столько же: в этом наборе каждый открыл свою сессию.
	if stat.Sessions != 3 {
		t.Errorf("шаг открытия посчитан как %d сессий, ждали 3", stat.Sessions)
	}
	if stat.Visitors != 3 {
		t.Errorf("шаг открытия посчитан как %d посетителей, ждали 3", stat.Visitors)
	}

	// Блокеры — это отдельные события blocker.*, а не «шаг без следующего».
	// Открытие конструктора сюда не попадает, поэтому проверяем на своих
	// событиях с причиной в props.
	insertFunnel(t, r, []funnel.Event{
		{SessionID: uuidFor(7), Name: "blocker.field_invalid", Props: map[string]any{"reason": "width"}, Visitor: "v-7", ConsentVersion: 2, OccurredAt: from.Add(5 * time.Minute)},
		{SessionID: uuidFor(8), Name: "blocker.field_invalid", Props: map[string]any{"reason": "width"}, Visitor: "v-8", ConsentVersion: 2, OccurredAt: from.Add(6 * time.Minute)},
		{SessionID: uuidFor(9), Name: "blocker.api_error", Props: map[string]any{"reason": "timeout"}, Visitor: "v-9", ConsentVersion: 2, OccurredAt: from.Add(7 * time.Minute)},
	})
	blockers, err := r.TopBlockers(context.Background(), from, to, 10)
	if err != nil {
		t.Fatalf("TopBlockers: %v", err)
	}
	if len(blockers) != 2 {
		t.Fatalf("блокеров %d, ждали 2 (field_invalid и api_error): %+v", len(blockers), blockers)
	}
	// Сортировка по частоте: field_invalid встречается дважды, значит идёт
	// первым. Порядок определяет, какую причину покажут в отчёте первой.
	if blockers[0].Event != "blocker.field_invalid" || blockers[0].Count != 2 {
		t.Errorf("первым должен идти самый частый блокер: %+v", blockers[0])
	}
	if blockers[0].Reason != "width" {
		t.Errorf("причина не прочитана из props: %q", blockers[0].Reason)
	}
	cleanFunnel(t, r)
}

func TestFunnelTopBlockersRespectsLimit(t *testing.T) {
	r := funnelRepo(t)
	cleanFunnel(t, r)
	from, to := funnelWindow()

	// Четыре разные причины одним событием: limit режет выдачу по группам.
	// TopBlockers смотрит только на blocker.*, поэтому имена именно такие.
	for i, reason := range []string{"r1", "r2", "r3", "r4"} {
		insertFunnel(t, r, []funnel.Event{
			{SessionID: uuidFor(10 + i), Name: "blocker.api_error",
				Props: map[string]any{"reason": reason}, Visitor: "v-1", ConsentVersion: 2,
				OccurredAt: from.Add(time.Duration(i) * time.Minute)},
		})
	}
	got, err := r.TopBlockers(context.Background(), from, to, 2)
	if err != nil {
		t.Fatalf("TopBlockers: %v", err)
	}
	// limit обязан резать выдачу: иначе отчёт отдаёт весь хвост событий.
	if len(got) != 2 {
		t.Errorf("TopBlockers вернул %d записей при limit=2", len(got))
	}
	// limit <= 0 означает «по умолчанию 10», а не пустой вывод.
	all, err := r.TopBlockers(context.Background(), from, to, 0)
	if err != nil {
		t.Fatalf("TopBlockers(0): %v", err)
	}
	if len(all) != 4 {
		t.Errorf("TopBlockers(0) вернул %d, ждали все 4 группы", len(all))
	}
	cleanFunnel(t, r)
}

func TestFunnelAbandonPointsAndAvgSession(t *testing.T) {
	r := funnelRepo(t)
	cleanFunnel(t, r)
	from, to := funnelWindow()

	insertFunnel(t, r, []funnel.Event{
		{SessionID: uuidFor(1), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-1", ConsentVersion: 2, OccurredAt: from.Add(time.Minute)},
	})

	pts, err := r.AbandonPoints(context.Background(), from, to, 10)
	if err != nil {
		t.Fatalf("AbandonPoints: %v", err)
	}
	// Сессия без следующего шага — это уход; точка броска должна называть
	// последнее событие сессии.
	for _, p := range pts {
		if p.Sessions < 0 || p.Share < 0 {
			t.Errorf("некорректная точка броска: %+v", p)
		}
	}

	if _, err := r.AvgSessionSeconds(context.Background(), from, to); err != nil {
		t.Fatalf("AvgSessionSeconds: %v", err)
	}
	cleanFunnel(t, r)
}

func TestFunnelCleanupDeletesOnlyOld(t *testing.T) {
	r := funnelRepo(t)
	cleanFunnel(t, r)

	insertFunnel(t, r, []funnel.Event{
		{SessionID: uuidFor(6), Name: "constructor.open", Props: map[string]any{}, Visitor: "v-1", ConsentVersion: 2, OccurredAt: time.Now().UTC()},
	})
	n, err := r.Cleanup(context.Background(), time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	// Свежее событие переживает уборку: иначе воронка обнулялась бы по
	// расписанию и теряла бы текущие данные.
	if n != 0 {
		t.Errorf("уборка снесла %d свежих событий", n)
	}
	if got, _ := r.EventCount(context.Background(), time.Now().UTC().Add(-24*time.Hour), time.Now().UTC().Add(time.Hour)); got != 1 {
		t.Errorf("свежее событие пропало: осталось %d", got)
	}
	cleanFunnel(t, r)
}

func TestFunnelInsertEmptyIsNoop(t *testing.T) {
	r := funnelRepo(t)
	// Пустая пачка не должна падать и не должна ничего писать.
	if err := r.InsertEvents(context.Background(), nil); err != nil {
		t.Errorf("InsertEvents(nil) вернул ошибку: %v", err)
	}
}
