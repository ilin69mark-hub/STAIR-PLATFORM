package funnel

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ---- заглушка репозитория ----

type memRepo struct {
	inserted []Event
	fail     error
	// ответы на отчёт
	sessions int
	events   int
	steps    map[string]StepStat
	blockers []Blocker
	abandons []AbandonPoint
	avg      float64
	cleaned  int64
}

func (m *memRepo) InsertEvents(_ context.Context, events []Event) error {
	if m.fail != nil {
		return m.fail
	}
	m.inserted = append(m.inserted, events...)
	return nil
}
func (m *memRepo) SessionCount(context.Context, time.Time, time.Time) (int, error) {
	return m.sessions, nil
}
func (m *memRepo) EventCount(context.Context, time.Time, time.Time) (int, error) {
	return m.events, nil
}
func (m *memRepo) StepCounts(context.Context, time.Time, time.Time) (map[string]StepStat, error) {
	return m.steps, nil
}
func (m *memRepo) TopBlockers(context.Context, time.Time, time.Time, int) ([]Blocker, error) {
	return m.blockers, nil
}
func (m *memRepo) AbandonPoints(context.Context, time.Time, time.Time, int) ([]AbandonPoint, error) {
	return m.abandons, nil
}
func (m *memRepo) AvgSessionSeconds(context.Context, time.Time, time.Time) (float64, error) {
	return m.avg, nil
}
func (m *memRepo) Cleanup(context.Context, time.Time) (int64, error) {
	return m.cleaned, nil
}

const goodSession = "11111111-2222-3333-4444-555555555555"

func TestIngest_WritesAcceptedEvents(t *testing.T) {
	repo := &memRepo{}
	svc := NewService(repo, func() time.Time { return time.Unix(1700000000, 0).UTC() })

	res, err := svc.Ingest(context.Background(), goodSession, []Event{
		{Name: EventFunnelOpen, Path: "/#constructor"},
		{Name: EventBlockerField, Props: map[string]any{"reason": "widthMM"}},
	}, 1)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if res.Accepted != 2 || res.Dropped != 0 {
		t.Fatalf("accepted=%d dropped=%d, хочу 2/0", res.Accepted, res.Dropped)
	}
	if len(repo.inserted) != 2 {
		t.Fatalf("в репозиторий %d событий, хочу 2", len(repo.inserted))
	}
	if repo.inserted[0].SessionID != goodSession {
		t.Errorf("session_id не проставлен: %q", repo.inserted[0].SessionID)
	}
	if repo.inserted[0].ConsentVersion != 1 {
		t.Errorf("consent_version не проставлен: %d", repo.inserted[0].ConsentVersion)
	}
}

// Главная защита приватности: без актуального согласия не пишется НИЧЕГО,
// даже если пакет валиден.
func TestIngest_RequiresConsent(t *testing.T) {
	for _, version := range []int{0, -1, 2, 99} {
		repo := &memRepo{}
		svc := NewService(repo, nil)
		_, err := svc.Ingest(context.Background(), goodSession, []Event{{Name: EventPageView}}, version)
		if !errors.Is(err, ErrConsentRequired) {
			t.Errorf("consent_version=%d: ожидался ErrConsentRequired, получено %v", version, err)
		}
		if len(repo.inserted) != 0 {
			t.Errorf("consent_version=%d: записано %d событий без согласия", version, len(repo.inserted))
		}
	}
}

func TestIngest_DropsUnknownNameButKeepsRest(t *testing.T) {
	repo := &memRepo{}
	svc := NewService(repo, nil)
	res, err := svc.Ingest(context.Background(), goodSession, []Event{
		{Name: "totally.made_up"},
		{Name: "foo.bar.baz.qux"},
		{Name: EventPageView},
	}, 1)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	// Одно мусорное имя отбрасывается ПОТОЧНО: иначе сломанный клиент выбил бы
	// весь визит из отчёта и в воронке появился бы провал, которого не было.
	if res.Accepted != 1 || res.Dropped != 2 {
		t.Fatalf("accepted=%d dropped=%d, хочу 1/2", res.Accepted, res.Dropped)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].Name != EventPageView {
		t.Fatalf("записано не то: %+v", repo.inserted)
	}
}

func TestIngest_RejectsOversizedBatch(t *testing.T) {
	repo := &memRepo{}
	svc := NewService(repo, nil)
	events := make([]Event, MaxBatchEvents+1)
	for i := range events {
		events[i] = Event{Name: EventPageView}
	}
	if _, err := svc.Ingest(context.Background(), goodSession, events, 1); !errors.Is(err, ErrBatchTooLarge) {
		t.Fatalf("ожидался ErrBatchTooLarge, получено %v", err)
	}
	if len(repo.inserted) != 0 {
		t.Fatalf("при отказе по размеру записано %d событий", len(repo.inserted))
	}
}

func TestIngest_RejectsBadSessionID(t *testing.T) {
	repo := &memRepo{}
	svc := NewService(repo, nil)
	for _, id := range []string{"", "не-uuid", "11111111-2222-3333-4444-55555555555", "ZZZZZZZZ-2222-3333-4444-555555555555"} {
		if _, err := svc.Ingest(context.Background(), id, []Event{{Name: EventPageView}}, 1); !errors.Is(err, ErrInvalidSession) {
			t.Errorf("session_id=%q: ожидался ErrInvalidSession, получено %v", id, err)
		}
	}
}

// Границы пропсов — это защита от утечки того, что человек ввёл в поле
// формы: без них в отчёт могло бы попасть произвольное значение.
func TestSanitizeProps(t *testing.T) {
	ok := []map[string]any{
		{},
		{"reason": "widthMM"},
		{"count": float64(3), "ok": true, "ratio": 0.5},
		{"code": 422},
		{"nothing": nil},
		{"long": strings.Repeat("я", MaxPropValueLen)},
	}
	for _, p := range ok {
		if _, good := sanitizeProps(p); !good {
			t.Errorf("ожидался пропуск %v, получено отказ", p)
		}
	}

	bad := []struct {
		name  string
		props map[string]any
	}{
		{"слишком много ключей", manyKeys(MaxPropCount + 1)},
		{"слишком длинное значение", map[string]any{"reason": strings.Repeat("я", MaxPropValueLen+1)}},
		{"слишком длинный ключ", map[string]any{strings.Repeat("k", MaxPropKeyLen+1): "v"}},
		{"пустой ключ", map[string]any{"  ": "v"}},
		{"вложенный объект", map[string]any{"cfg": map[string]any{"a": 1}}},
		{"массив", map[string]any{"list": []any{1, 2}}},
		{"переполнение бюджета", manyBig(12)},
	}
	for _, c := range bad {
		if _, good := sanitizeProps(c.props); good {
			t.Errorf("%s: пропущен, хочу отказ", c.name)
		}
	}
}

func manyKeys(n int) map[string]any {
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		m[string(rune('a'+i%26))+strings.Repeat("x", i)] = 1
	}
	return m
}

// manyBig — много значений, каждое в пределе MaxPropValueLen: по одному
// пропуску всё проходит (значение не длиннее предела), а суммарно они
// переполняют MaxPropBytes.
func manyBig(n int) map[string]any {
	m := map[string]any{}
	for i := 0; i < n; i++ {
		m["k"+strings.Repeat("z", i)] = strings.Repeat("я", MaxPropValueLen)
	}
	return m
}

// Клиентские часы не должны уводить событие за пределы окна отчёта.
func TestIngest_ClampsClientClock(t *testing.T) {
	repo := &memRepo{}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	svc := NewService(repo, func() time.Time { return now })

	far := now.Add(72 * time.Hour)
	past := now.Add(-72 * time.Hour)
	_, err := svc.Ingest(context.Background(), goodSession, []Event{
		{Name: EventPageView, OccurredAt: far},
		{Name: EventPageView, OccurredAt: past},
		{Name: EventPageView, OccurredAt: now.Add(-time.Minute)},
	}, 1)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	for i, e := range repo.inserted {
		if math.Abs(e.OccurredAt.Sub(now).Hours()) > 1 {
			t.Errorf("событие %d осталось с клиентским временем %v вместо %v", i, e.OccurredAt, now)
		}
	}
}

func TestSanitize_TruncatesLongText(t *testing.T) {
	svc := NewService(&memRepo{}, nil)
	ev, ok := svc.sanitize(Event{
		Name: EventPageView,
		Path: strings.Repeat("p", 500),
	}, goodSession, 1, time.Now())
	if !ok {
		t.Fatal("событие должно пройти")
	}
	if len(ev.Path) != MaxPathLen {
		t.Errorf("path длиной %d, хочу %d", len(ev.Path), MaxPathLen)
	}
}

func TestReport_BuildsFunnel(t *testing.T) {
	repo := &memRepo{
		sessions: 100,
		events:   500,
		steps: map[string]StepStat{
			EventSessionStart: {Sessions: 100, AvgSecond: 0},
			EventPageView:     {Sessions: 90},
			EventFunnelOpen:   {Sessions: 60},
			EventCTAQuote:     {Sessions: 30},
		},
		blockers: []Blocker{{Event: EventBlockerField, Reason: "widthMM", Count: 25}},
		abandons: []AbandonPoint{{LastEvent: EventFunnelOpen, Sessions: 20, AvgSec: 42}},
		avg:      61.5,
	}
	svc := NewService(repo, nil)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 30)

	rep, err := svc.Report(context.Background(), from, to)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if rep.Sessions != 100 || rep.Events != 500 {
		t.Errorf("сессии/события: %d/%d, хочу 100/500", rep.Sessions, rep.Events)
	}
	// Шаги идут в порядке StepOrder, а не по алфавиту: иначе blocker.* и
	// cta.* разъезжаются с реальным движением по конструктору.
	wantOrder := []string{EventSessionStart, EventPageView, EventFunnelOpen, EventCTAQuote}
	if len(rep.Steps) != len(wantOrder) {
		t.Fatalf("шагов %d, хочу %d", len(rep.Steps), len(wantOrder))
	}
	for i, name := range wantOrder {
		if rep.Steps[i].Name != name {
			t.Errorf("шаг %d = %q, хочу %q", i, rep.Steps[i].Name, name)
		}
	}
	if rep.Steps[0].Share != 1.0 {
		t.Errorf("доля первого шага %v, хочу 1", rep.Steps[0].Share)
	}
	if rep.Steps[1].Share != 0.9 {
		t.Errorf("доля второго шага %v, хочу 0.9", rep.Steps[1].Share)
	}
	if rep.Blockers[0].Reason != "widthMM" || rep.Blockers[0].Count != 25 {
		t.Errorf("блокеры не доехали: %+v", rep.Blockers[0])
	}
	if rep.Abandons[0].LastEvent != EventFunnelOpen {
		t.Errorf("точки оттока: %+v", rep.Abandons[0])
	}
	if rep.AvgSecAll != 61.5 {
		t.Errorf("средняя длительность %v, хочу 61.5", rep.AvgSecAll)
	}
}

func TestReport_RejectsInvertedRange(t *testing.T) {
	svc := NewService(&memRepo{}, nil)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.Report(context.Background(), to.AddDate(0, 0, 1), to); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("ожидался ErrInvalidRange, получено %v", err)
	}
}

func TestReport_ZeroSessionsNoDivByZero(t *testing.T) {
	svc := NewService(&memRepo{}, nil)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rep, err := svc.Report(context.Background(), from, from.AddDate(0, 0, 7))
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if len(rep.Steps) != 0 || len(rep.Blockers) != 0 || len(rep.Abandons) != 0 {
		t.Errorf("пустой отчёт должен быть пустым: %+v", rep)
	}
	if rep.AvgSecAll != 0 {
		t.Errorf("средняя длительность %v, хочу 0", rep.AvgSecAll)
	}
}

func TestIngest_PropagatesRepositoryError(t *testing.T) {
	repo := &memRepo{fail: errors.New("db down")}
	svc := NewService(repo, nil)
	if _, err := svc.Ingest(context.Background(), goodSession, []Event{{Name: EventPageView}}, 1); err == nil {
		t.Fatal("ошибка репозитория должна пробрасываться, а не теряться")
	}
}

func TestIngest_EmptyBatchIsNoop(t *testing.T) {
	repo := &memRepo{}
	svc := NewService(repo, nil)
	res, err := svc.Ingest(context.Background(), goodSession, nil, 1)
	if err != nil || res.Accepted != 0 {
		t.Fatalf("res=%+v err=%v, хочу noop", res, err)
	}
	if len(repo.inserted) != 0 {
		t.Fatal("пустая пачка не должна писать")
	}
}

// TestMigrationEventNamesMatchCatalog: CHECK в миграции 000035 описывает форму
// имени, а каталог — разрешённые значения. Расхождение означало бы, что либо
// миграция отвергает валидное событие, либо сервис принимает то, чего в БД
// не может быть. Тест роняет сборку при расхождении.
func TestMigrationEventNamesMatchCatalog(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000035_web_events.up.sql"))
	if err != nil {
		t.Fatalf("читать миграцию: %v", err)
	}
	sql := string(raw)
	re := regexp.MustCompile(`CHECK \(name ~ '([^']+)'\)`)
	m := re.FindStringSubmatch(sql)
	if m == nil {
		t.Fatal("в миграции нет CHECK на имя события")
	}
	dbRe := regexp.MustCompile("^" + strings.ReplaceAll(m[1], `\d`, `\d`) + "$")

	for _, name := range AllEvents() {
		if !dbRe.MatchString(name) {
			t.Errorf("имя %q не проходит CHECK миграции (%s)", name, m[1])
		}
	}
	for _, name := range AllEvents() {
		if !KnownEvent(name) {
			t.Errorf("KnownEvent(%q) = false, хотя имя в каталоге", name)
		}
	}
	if KnownEvent("some.random_name") {
		t.Error("KnownEvent принял имя вне каталога")
	}
	if KnownEvent("") || KnownEvent("A.B") {
		t.Error("KnownEvent принял пустое или некорректное имя")
	}
}

// Границы из entity.go обязаны совпадать с CHECK миграции, иначе сервис
// отсечёт событие, а БД его пропустила бы (или наоборот).
func TestMigrationBoundsMatchEntity(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000035_web_events.up.sql"))
	if err != nil {
		t.Fatalf("читать миграцию: %v", err)
	}
	sql := string(raw)
	bounds := map[string]int{
		"length(path) <= 128":   MaxPathLen,
		"length(visitor) <= 32": 32,
		"length(browser) <= 32": MaxBrowserLen,
	}
	for needle, want := range bounds {
		if !strings.Contains(sql, needle) {
			t.Errorf("в миграции нет ограничения %q", needle)
			continue
		}
		var got int
		if _, err := fmtSscan(needle, &got); err != nil {
			t.Errorf("разбор %q: %v", needle, err)
			continue
		}
		if got != want {
			t.Errorf("%q: миграция %d, код %d", needle, got, want)
		}
	}
	if !strings.Contains(sql, "consent_version > 0") {
		t.Error("в миграции нет CHECK consent_version > 0")
	}
}

func fmtSscan(needle string, out *int) (int, error) {
	i := strings.Index(needle, "<=")
	if i < 0 {
		return 0, errors.New("нет <=")
	}
	n, err := strconvAtoi(strings.TrimSpace(needle[i+2:]))
	if err != nil {
		return 0, err
	}
	*out = n
	return n, nil
}

func strconvAtoi(s string) (int, error) {
	if s == "" {
		return 0, errors.New("пустое число")
	}
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("не число")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
