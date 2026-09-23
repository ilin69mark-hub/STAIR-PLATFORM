package assistant

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

// fakeRetriever — детерминированный ретривер с записью вызовов.
type fakeRetriever struct {
	mu         sync.Mutex
	chunks     []Chunk
	err        error
	lastTenant string
	lastQuery  string
	lastTopK   int
}

func (f *fakeRetriever) Retrieve(_ context.Context, tenantID, query string, topK int) ([]Chunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastTenant = tenantID
	f.lastQuery = query
	f.lastTopK = topK
	if f.err != nil {
		return nil, f.err
	}
	return f.chunks, nil
}

// fakeMemory — in-memory MemoryStore.
type fakeMemory struct {
	mu        sync.Mutex
	msgs      []MemoryMessage
	appendErr error
	recentErr error
}

func (f *fakeMemory) AppendMessages(_ context.Context, msgs []MemoryMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appendErr != nil {
		return f.appendErr
	}
	f.msgs = append(f.msgs, msgs...)
	return nil
}

func (f *fakeMemory) RecentMessages(_ context.Context, _, _ string, limit int) ([]MemoryMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recentErr != nil {
		return nil, f.recentErr
	}
	out := make([]MemoryMessage, 0, limit)
	for i := len(f.msgs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, f.msgs[i])
	}
	return out, nil
}

func (f *fakeMemory) PruneMessages(context.Context, time.Time) (int64, error) { return 0, nil }

// auditSpy — in-memory audit.Repository.
type auditSpy struct {
	mu     sync.Mutex
	events []*audit.Event
}

func (a *auditSpy) Insert(_ context.Context, e *audit.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, e)
	return nil
}
func (a *auditSpy) ListByProject(context.Context, string, string) ([]*audit.Event, error) {
	return nil, nil
}
func (a *auditSpy) ListByTenant(context.Context, string) ([]*audit.Event, error) { return nil, nil }

func (a *auditSpy) last() *audit.Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.events) == 0 {
		return nil
	}
	return a.events[len(a.events)-1]
}

func TestAskWithRAG(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	retr := &fakeRetriever{chunks: []Chunk{
		pubChunk(0, "Прямой марш — рекомендация для малых высот.", "Прямой марш"),
		pubChunk(1, "Для винтовой лестницы нужен радиус.", "Винтовой марш"),
	}}
	mem := &fakeMemory{}
	spy := &auditSpy{}
	svc := NewService(calc, audit.NewService(spy)).
		WithRAG(retr, 5).
		WithMemory(mem)

	res, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config:    stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
		ProjectID: "p1",
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	// 1. Цитаты источников в Response.Notes.
	found := 0
	for _, n := range res.Response.Notes {
		if strings.HasPrefix(n, "[source: docs/13_AI/test.md, §") {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("notes citations = %d, want 2: %v", found, res.Response.Notes)
	}

	// 2. Ретривер получил tenant и topK, контекст донёс chunk content.
	retr.mu.Lock()
	tenant, topK, q := retr.lastTenant, retr.lastTopK, retr.lastQuery
	retr.mu.Unlock()
	if tenant != "t1" || topK != 5 {
		t.Fatalf("retriever tenant=%q topK=%d", tenant, topK)
	}
	if !strings.Contains(q, "kind=design") {
		t.Fatalf("query = %q, want kind=design summary", q)
	}

	// 3. Аудит: detail содержит использованные chunk ids (DoD S-135).
	ev := spy.last()
	if ev == nil {
		t.Fatal("no audit event")
	}
	if ev.Result != audit.ResultOK {
		t.Fatalf("audit result = %s", ev.Result)
	}
	for _, want := range []string{"rag_sources=", "[source: docs/13_AI/test.md, §0]", "[source: docs/13_AI/test.md, §1]"} {
		if !strings.Contains(ev.Detail, want) {
			t.Fatalf("audit detail missing %q: %s", want, ev.Detail)
		}
	}

	// 4. Память: диалог сохранился (user+assistant).
	mem.mu.Lock()
	n := len(mem.msgs)
	mem.mu.Unlock()
	if n != 2 {
		t.Fatalf("memory messages = %d, want 2", n)
	}
}

func TestAskMemoryHistoryInContext(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	mem := &fakeMemory{msgs: []MemoryMessage{
		{TenantID: "t1", ProjectID: "p1", Role: "user", Content: "Какой марш дешевле?", CreatedAt: time.Now()},
	}}
	retr := &fakeRetriever{}
	svc := NewService(calc, nil).WithRAG(retr, 3).WithMemory(mem)

	// Первичный бэкенд — spy, чтобы увидеть Prompt.Context.
	backend := &captureBackend{}
	svc = svc.WithPrimaryBackend(backend)

	if _, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config:       stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
		ProjectID:    "p1",
		HistoryLimit: 10,
	}); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !strings.Contains(backend.got.Context, "История диалога по проекту") {
		t.Fatalf("history not injected into context: %q", backend.got.Context)
	}
	if !strings.Contains(backend.got.Context, "Какой марш дешевле?") {
		t.Fatalf("history content missing: %q", backend.got.Context)
	}
}

// captureBackend — запоминает Prompt для ассертов.
type captureBackend struct{ got Prompt }

func (c *captureBackend) Infer(_ context.Context, p Prompt) (*Answer, error) {
	c.got = p
	return &Answer{Text: "ok"}, nil
}

func TestAskRAGFailureIsBestEffort(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	svc := NewService(calc, nil).WithRAG(&fakeRetriever{err: errBoom}, 5)
	res, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	})
	if err != nil {
		t.Fatalf("Ask must not fail on RAG error: %v", err)
	}
	if res.Response.Recommendation == "" {
		t.Fatal("recommendation must still be produced")
	}
}

var errBoom = errBoomT{}

type errBoomT struct{}

func (errBoomT) Error() string { return "boom" }

func TestAskMemoryFailureIsBestEffort(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	mem := &fakeMemory{recentErr: errBoom}
	svc := NewService(calc, nil).WithMemory(mem)
	if _, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config:    stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
		ProjectID: "p1",
	}); err != nil {
		t.Fatalf("Ask must not fail on memory load error: %v", err)
	}
}

func TestAskNoMemoryWithoutProject(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	mem := &fakeMemory{}
	svc := NewService(calc, nil).WithMemory(mem)
	if _, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config: stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
	}); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	mem.mu.Lock()
	defer mem.mu.Unlock()
	if len(mem.msgs) != 0 {
		t.Fatal("memory must not be touched without project scope")
	}
}

func TestAskHistoryLimitClamped(t *testing.T) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	// 60 сообщений в памяти; запрашиваем 1000 → cap 50.
	mem := &fakeMemory{}
	for i := 0; i < 60; i++ {
		mem.msgs = append(mem.msgs, MemoryMessage{Role: "user", Content: "m", CreatedAt: time.Now()})
	}
	backend := &captureBackend{}
	svc := NewService(calc, nil).WithMemory(mem).WithPrimaryBackend(backend)
	if _, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
		Config:       stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
		ProjectID:    "p1",
		HistoryLimit: 1000,
	}); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !strings.Contains(backend.got.Context, "последние 50") {
		t.Fatalf("history not capped to 50: %q", backend.got.Context)
	}
}
