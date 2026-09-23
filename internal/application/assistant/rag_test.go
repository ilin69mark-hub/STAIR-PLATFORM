package assistant

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeChunkStore — in-memory ChunkStore для юнит-тестов ретриверов.
// Симулирует tenant-изоляцию: ищет только в чанках своего tenant + публичных.
type fakeChunkStore struct {
	chunks     []Chunk // все чанки (tenant_id пустых — публичные)
	tenantOf   func(c Chunk) string
	vecErr     error
	ftsErr     error
	calledVec  bool
	calledFTS  bool
	lastTenant string
	lastQuery  string
	lastVec    []float32
	lastTopK   int
}

func newFakeChunkStore(chunks []Chunk) *fakeChunkStore {
	return &fakeChunkStore{chunks: chunks}
}

func (f *fakeChunkStore) UpsertChunks(context.Context, []Chunk) (int, error) { return 0, nil }

func (f *fakeChunkStore) tenantMatch(tenantID, tenantOf string) bool {
	return tenantOf == "" || tenantOf == tenantID // публичные + свои
}

func (f *fakeChunkStore) VectorSearch(ctx context.Context, tenantID string, vec []float32, topK int) ([]Chunk, error) {
	f.calledVec = true
	f.lastTenant = tenantID
	f.lastVec = vec
	f.lastTopK = topK
	if f.vecErr != nil {
		return nil, f.vecErr
	}
	// Простейший «поиск»: чанки сортируются фифо — фейк не отвечает за качество.
	return f.filtered(tenantID, topK), nil
}

func (f *fakeChunkStore) FTSSearch(ctx context.Context, tenantID, query string, topK int) ([]Chunk, error) {
	f.calledFTS = true
	f.lastTenant = tenantID
	f.lastQuery = query
	f.lastTopK = topK
	if f.ftsErr != nil {
		return nil, f.ftsErr
	}
	got := f.filtered(tenantID, topK)
	// FTS-семантика: оставляем только чанки, содержащие слово запроса.
	if query != "" {
		kw := strings.Fields(query)[0]
		out := got[:0]
		for _, c := range got {
			if strings.Contains(strings.ToLower(c.Content), strings.ToLower(kw)) {
				out = append(out, c)
			}
		}
		got = out
	}
	return got, nil
}

func (f *fakeChunkStore) filtered(tenantID string, topK int) []Chunk {
	var out []Chunk
	for _, c := range f.chunks {
		tenantOf := ""
		if f.tenantOf != nil {
			tenantOf = f.tenantOf(c)
		}
		if f.tenantMatch(tenantID, tenantOf) {
			out = append(out, c)
		}
	}
	if len(out) > topK {
		out = out[:topK]
	}
	return out
}

// fakeEmbedder — возвращает вектор по числу вхождений query; err для сбоя.
type fakeEmbedder struct {
	err error
}

func (f fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = make([]float32, EmbeddingDim)
	}
	return out, nil
}

func pubChunk(i int, content, docTitle string) Chunk {
	return Chunk{
		Hash:       ComputeChunkHash(content),
		SourcePath: "docs/13_AI/test.md",
		Version:    "v1",
		DocTitle:   docTitle,
		Index:      i,
		Content:    content,
	}
}

func TestFTSRetrieverTenantIsolation(t *testing.T) {
	store := newFakeChunkStore([]Chunk{
		pubChunk(0, "Прямой марш рассчитывается просто", "Прямой"),
		pubChunk(1, "Винтовой марш требует радиуса", "Винтовой"),
	})
	// Тенантные чанки: tenantOf по хешу.
	tenantChunk := pubChunk(2, "Прямой марш для клиента А", "Прямой")
	tenantChunk.Hash = ComputeChunkHash(tenantChunk.Content)
	store.chunks = append(store.chunks, tenantChunk)
	store.tenantOf = func(c Chunk) string {
		if c.Index == 2 {
			return "tenant-B"
		}
		return ""
	}

	r := NewFTSRetriever(store)
	got, err := r.Retrieve(context.Background(), "tenant-A", "прямой", 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	for _, c := range got {
		if store.tenantOf(c) == "tenant-B" {
			t.Fatalf("tenant-B chunk leaked into tenant-A search: %q", c.Content)
		}
	}
	if store.lastTenant != "tenant-A" {
		t.Fatalf("lastTenant = %q", store.lastTenant)
	}
}

func TestVectorRetrieverUsesVectorPath(t *testing.T) {
	store := newFakeChunkStore([]Chunk{pubChunk(0, "текст", "Док")})
	emb := fakeEmbedder{}
	r := NewVectorRetriever(store, emb)
	if _, err := r.Retrieve(context.Background(), "t1", "запрос", 3); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !store.calledVec {
		t.Fatal("expected vector search to be called")
	}
	if store.lastVec == nil || len(store.lastVec) != EmbeddingDim {
		t.Fatalf("embedding not passed to store")
	}
}

func TestVectorRetrieverFallsBackToFTSOnEmbedError(t *testing.T) {
	store := newFakeChunkStore([]Chunk{pubChunk(0, "винтовая лестница занимает место", "Док")})
	r := NewVectorRetriever(store, fakeEmbedder{err: errors.New("api down")})
	got, err := r.Retrieve(context.Background(), "t1", "винтовая", 3)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !store.calledFTS {
		t.Fatal("expected FTS fallback on embedder error")
	}
	if len(got) != 1 {
		t.Fatalf("chunks = %d, want 1 (FTS match)", len(got))
	}
	if !strings.Contains(got[0].Content, "винтовая") {
		t.Fatalf("FTS returned irrelevant chunk: %q", got[0].Content)
	}
}

func TestVectorRetrieverFallsBackToFTSOnVectorError(t *testing.T) {
	store := newFakeChunkStore([]Chunk{pubChunk(0, "прямой марш", "Док")})
	store.vecErr = errors.New("pgvector down")
	r := NewVectorRetriever(store, fakeEmbedder{})
	got, err := r.Retrieve(context.Background(), "t1", "прямой", 3)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !store.calledVec || !store.calledFTS {
		t.Fatalf("vec=%v fts=%v, want both", store.calledVec, store.calledFTS)
	}
	if len(got) != 1 {
		t.Fatalf("chunks = %d, want 1", len(got))
	}
}

func TestRetrieverValidation(t *testing.T) {
	store := newFakeChunkStore(nil)
	r := NewFTSRetriever(store)
	if _, err := r.Retrieve(context.Background(), "", "q", 5); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty tenant err = %v, want ErrInvalid", err)
	}
	if _, err := r.Retrieve(context.Background(), "t1", "  ", 5); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty query err = %v, want ErrInvalid", err)
	}
	if _, err := r.Retrieve(context.Background(), "t1", "q", 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("topK=0 err = %v, want ErrInvalid", err)
	}
	if _, err := r.Retrieve(context.Background(), "t1", "q", 51); !errors.Is(err, ErrInvalid) {
		t.Fatalf("topK=51 err = %v, want ErrInvalid", err)
	}
}

func TestChunkContextAndCitations(t *testing.T) {
	chunks := []Chunk{
		pubChunk(0, "первый чанк", "H1"),
		pubChunk(1, "второй чанк", "H1"),
	}
	block, cites := ChunkContext(chunks)
	if block == "" {
		t.Fatal("empty context block")
	}
	if len(cites) != 2 {
		t.Fatalf("cites = %d, want 2", len(cites))
	}
	if !strings.Contains(cites[0], "[source: docs/13_AI/test.md, §0]") {
		t.Fatalf("cite[0] = %q", cites[0])
	}
	if !strings.Contains(block, "второй чанк") {
		t.Fatalf("block missing chunk content: %q", block)
	}
	if b, c := ChunkContext(nil); b != "" || c != nil {
		t.Fatal("empty input -> empty output")
	}
}

func TestHistoryBlock(t *testing.T) {
	msgs := []MemoryMessage{
		{Role: "user", Content: "первый вопрос", CreatedAt: time.Now()},
		{Role: "assistant", Content: "первый ответ", CreatedAt: time.Now()},
	}
	block := HistoryBlock(msgs)
	for _, want := range []string{"[user] первый вопрос", "[assistant] первый ответ", "История диалога"} {
		if !strings.Contains(block, want) {
			t.Fatalf("block missing %q: %q", want, block)
		}
	}
	if HistoryBlock(nil) != "" {
		t.Fatal("empty history -> empty block")
	}
}

func TestHistoryBlockTruncatesLongContent(t *testing.T) {
	long := strings.Repeat("длинный текст ", 500)
	block := HistoryBlock([]MemoryMessage{{Role: "user", Content: long}})
	// len() считает БАЙТЫ; кириллица — 2 байта на руну.
	if l := len([]rune(block)); l > 2000 {
		t.Fatalf("block too long: %d runes", l)
	}
	if !strings.Contains(block, "…") {
		t.Fatal("expected truncation ellipsis")
	}
}

func TestClampHistoryLimit(t *testing.T) {
	if clampHistoryLimit(0) != DefaultHistoryLimit {
		t.Fatalf("0 -> %d, want %d", clampHistoryLimit(0), DefaultHistoryLimit)
	}
	if clampHistoryLimit(-1) != 0 {
		t.Fatalf("-1 -> %d, want 0", clampHistoryLimit(-1))
	}
	if clampHistoryLimit(7) != 7 {
		t.Fatalf("7 -> %d", clampHistoryLimit(7))
	}
	if clampHistoryLimit(500) != MaxHistoryLimit {
		t.Fatalf("500 -> %d, want %d", clampHistoryLimit(500), MaxHistoryLimit)
	}
}

func TestHistoryLimitAndProjectScopeExtraction(t *testing.T) {
	dr := DesignRequest{HistoryLimit: -3, ProjectID: "p1"}
	if historyLimitOf(dr) != 0 {
		t.Fatalf("design negative limit -> %d", historyLimitOf(dr))
	}
	ar := AnalysisRequest{HistoryLimit: 5, ProjectID: "p2"}
	if historyLimitOf(ar) != 5 || projectIDOf(ar) != "p2" {
		t.Fatalf("analysis scope wrong: limit=%d project=%q", historyLimitOf(ar), projectIDOf(ar))
	}
	if projectIDOf(dr) != "p1" {
		t.Fatalf("design project scope wrong")
	}
	if projectIDOf(struct{}{}) != "" {
		t.Fatal("unknown request -> empty scope")
	}
}
