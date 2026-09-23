package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appast "stairplatform/internal/application/assistant"
	"stairplatform/internal/application/project"
)

// integrationAI — репозиторий AI против stair_test (skip без энва):
// применяет миграции и открывает пул на весь тест.
func integrationAI(t *testing.T) *AIRepository {
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
	return NewAIRepository(pool)
}

// pubChunkRow — короткий builder публичного чанка для тестов.
func pubChunkRow(hash, title, content string) appast.Chunk {
	return appast.Chunk{
		Hash:       hash,
		SourcePath: "docs/13_AI/test.md",
		Version:    "test",
		DocTitle:   title,
		Index:      0,
		Content:    content,
	}
}

// randWord — уникальный термин на прогон: изоляция данных тестов в общей
// stair_test (TestMain сбрасывает БД одним reset'ом на весь прогон).
func randWord() string {
	return fmt.Sprintf("w%x", time.Now().UnixNano())
}

func TestCorpusBackfillIdempotent(t *testing.T) {
	repo := integrationAI(t)
	ctx := context.Background()
	word := randWord()

	chunks := []appast.Chunk{
		pubChunkRow("h1", "Прямой марш", "Прямой марш: "+word+" рекомендация для малых высот."),
		pubChunkRow("h2", "Винтовой марш", "Винтовой марш требует радиуса."),
		pubChunkRow("h3", "Закрытый марш", "Закрытый марш экономит место."),
	}
	n1, err := repo.BackfillChunks(ctx, chunks)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n1 != 3 {
		t.Fatalf("inserted = %d, want 3", n1)
	}

	// Повторный backfill с тем же хешем не создаёт дублей, метаданные
	// обновляются (идемпотентность S-135).
	chunks[0].Content = "Обновлённое содержание " + word + "."
	n2, err := repo.BackfillChunks(ctx, chunks)
	if err != nil {
		t.Fatalf("backfill 2: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("second backfill inserted = %d, want 0 (idempotent)", n2)
	}

	all, err := repo.loadPublicChunks(ctx)
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("corpus size = %d, want 3", len(all))
	}
	if !strings.Contains(all["h1"].content, "Обновлённое") {
		t.Fatal("metadata/content not updated by upsert")
	}
}

func TestFTSSearchAndTenantIsolation(t *testing.T) {
	repo := integrationAI(t)
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenantA := testTenantID(t, pr)
	tenantB := createTestTenant(t, pr, "iso-"+randWord())
	word := randWord()

	if _, err := repo.BackfillChunks(ctx, []appast.Chunk{
		pubChunkRow("pub1", "Публичная база", "Публичный чанк про "+word+"."),
	}); err != nil {
		t.Fatalf("backfill pub: %v", err)
	}
	// Приватный чанк tenant A (прямая вставка, минуя backfill корпуса).
	if _, err := repo.pool.Exec(ctx,
		`INSERT INTO ai_corpus_chunks
		    (chunk_hash, source_path, version, doc_title, chunk_index, content, tenant_id)
		 VALUES ('privA', 'docs/users/a.md', 'v1', 'Приватный', 0, $1, $2)`,
		"Приватный чанк tenant A про "+word+".", tenantA); err != nil {
		t.Fatalf("insert private: %v", err)
	}

	// Tenant A видит публичный + свой.
	gotA, err := repo.FTSSearch(ctx, tenantA, word, 10)
	if err != nil {
		t.Fatalf("FTS A: %v", err)
	}
	if len(gotA) != 2 {
		t.Fatalf("tenant A results = %d, want 2 (public + own)", len(gotA))
	}
	// Tenant B видит только публичный (SEC-0005).
	gotB, err := repo.FTSSearch(ctx, tenantB, word, 10)
	if err != nil {
		t.Fatalf("FTS B: %v", err)
	}
	if len(gotB) != 1 || gotB[0].Hash != "pub1" {
		t.Fatalf("tenant B results = %+v, want only pub1", gotB)
	}
	// Пустой запрос / без совпадений.
	if res, err := repo.FTSSearch(ctx, tenantA, "   ", 5); err != nil || len(res) != 0 {
		t.Fatalf("blank query: res=%d err=%v", len(res), err)
	}
	if _, err := repo.FTSSearch(ctx, "", word, 5); err == nil {
		t.Fatal("FTS without tenant must fail (SEC-0005)")
	}
	// topK работает.
	gotTop1, err := repo.FTSSearch(ctx, tenantA, word, 1)
	if err != nil || len(gotTop1) != 1 {
		t.Fatalf("topK=1: n=%d err=%v", len(gotTop1), err)
	}
}

func TestVectorSearchGuards(t *testing.T) {
	repo := integrationAI(t)
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenant := testTenantID(t, pr)

	// Без tenant — ошибка безопасности (проверка раньше dim/hasVector).
	_, err := repo.VectorSearch(ctx, "", make([]float32, appast.EmbeddingDim), 5)
	if err == nil || !strings.Contains(err.Error(), "tenant") {
		t.Fatalf("vector search without tenant: err=%v, want tenant error", err)
	}

	if !repo.HasVector() {
		// Штатный postgres:16 без pgvector: корректный по размеру вектор
		// обязан вернуть «pgvector not available».
		_, err := repo.VectorSearch(ctx, tenant, make([]float32, appast.EmbeddingDim), 5)
		if err == nil || !strings.Contains(err.Error(), "pgvector") {
			t.Fatalf("expected pgvector-not-available error, got %v", err)
		}
		return
	}

	// pgvector есть: неверная размерность отклоняется.
	_, err = repo.VectorSearch(ctx, tenant, []float32{1, 2, 3}, 5)
	if err == nil || !strings.Contains(err.Error(), "dim") {
		t.Fatalf("wrong dim: err=%v, want dim error", err)
	}
}

// TestVectorSearchRanking — сквозной векторный путь (pgvector-гейт S-135):
// backfill с эмбеддингами → косинусный поиск возвращает релевантный чанк.
// Пропускается на postgres:16 без расширения vector (FTS-fallback-инстанс).
func TestVectorSearchRanking(t *testing.T) {
	repo := integrationAI(t)
	if !repo.HasVector() {
		t.Skip("pgvector not available; vector path not testable here")
	}
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenant := testTenantID(t, pr)

	// Псевдовекторы с детерминированным «сходством»: vX близок к eX.
	eA := deterministicVec(1)
	eB := deterministicVec(2)
	chunks := []appast.Chunk{
		{
			Hash: "vecA", SourcePath: "docs/pg.md", Version: "v", DocTitle: "A",
			Index: 0, Content: "Чанк A: прямой марш.", Embedding: eA,
		},
		{
			Hash: "vecB", SourcePath: "docs/pg.md", Version: "v", DocTitle: "B",
			Index: 1, Content: "Чанк B: винтовая лестница.", Embedding: eB,
		},
	}
	if _, err := repo.BackfillChunks(ctx, chunks); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Запрос со «смыслом» A — первым обязан идти чанк A (§0).
	got, err := repo.VectorSearch(ctx, tenant, eA, 2)
	if err != nil {
		t.Fatalf("vector search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("results = %d, want 2", len(got))
	}
	if got[0].Hash != "vecA" {
		t.Fatalf("top-1 = %q (content %q), want vecA", got[0].Hash, got[0].Content)
	}
	if len(got[0].Embedding) != appast.EmbeddingDim {
		t.Fatalf("chunk embedding not returned: %d dims", len(got[0].Embedding))
	}
}

// deterministicVec строит нормализованный вектор-«сигнатуру» (sum |v|=1):
// косинусное сходство вектора с самим собой (и его вариантом) = 1.
func deterministicVec(seed int) []float32 {
	v := make([]float32, appast.EmbeddingDim)
	var sum float64
	for i := range v {
		x := float32(seed*31 + i*17)
		v[i] = x
		sum += float64(x * x)
	}
	norm := float32(1 / sqrt64(sum))
	for i := range v {
		v[i] *= norm
	}
	return v
}

func sqrt64(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Ньютон-итерация без внешних зависимостей (точность >> float32).
	z := x
	for i := 0; i < 64; i++ {
		z = (z + x/z) / 2
		if z*z-x < 1e-12 {
			break
		}
	}
	return z
}

// TestMemoryAppendRecentPrune — сквозной цикл conversation-memory:
// append → recent (limit, порядок, изоляция) → prune по TTL.
func TestMemoryAppendRecentPrune(t *testing.T) {
	repo := integrationAI(t)
	ctx := context.Background()
	pr := NewProjectRepository(repo.pool)
	tenant := testTenantID(t, pr)
	owner := testOwnerID(t, pr, tenant)
	p := &project.Project{Name: "Память", Description: "тест", Status: project.StatusDraft}
	if err := pr.CreateProject(ctx, tenant, owner, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	old := time.Now().UTC().Add(-48 * time.Hour)
	recent := time.Now().UTC().Add(-time.Hour)
	msgs := []appast.MemoryMessage{
		{TenantID: tenant, ProjectID: p.ID, Role: "user", Content: "старое", CreatedAt: old},
		{TenantID: tenant, ProjectID: p.ID, Role: "assistant", Content: "старый ответ", CreatedAt: old.Add(time.Minute)},
		{TenantID: tenant, ProjectID: p.ID, Role: "user", Content: "новое", CreatedAt: recent},
	}
	if err := repo.AppendMessages(ctx, msgs); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Валидация скоупа и ролей.
	if err := repo.AppendMessages(ctx, []appast.MemoryMessage{{Role: "user", Content: "x"}}); err == nil {
		t.Fatal("append without tenant+project must fail")
	}
	if err := repo.AppendMessages(ctx, []appast.MemoryMessage{
		{TenantID: tenant, ProjectID: p.ID, Role: "admin", Content: "x"}}); err == nil {
		t.Fatal("append with invalid role must fail")
	}

	// Recent: новые сверху, limit работает.
	got, err := repo.RecentMessages(ctx, tenant, p.ID, 2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 2 || got[0].Content != "новое" || got[1].Content != "старый ответ" {
		t.Fatalf("recent(2) = %+v", got)
	}

	// Изоляция по tenant.
	foreign := createTestTenant(t, pr, "mem-"+randWord())
	if other, err := repo.RecentMessages(ctx, foreign, p.ID, 5); err != nil || len(other) != 0 {
		t.Fatalf("foreign tenant recent: n=%d err=%v", len(other), err)
	}

	// Prune старше суток: удаляются 2 старых.
	n, err := repo.PruneMessages(ctx, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 2 {
		t.Fatalf("pruned = %d, want 2", n)
	}
	left, err := repo.RecentMessages(ctx, tenant, p.ID, 10)
	if err != nil || len(left) != 1 || left[0].Content != "новое" {
		t.Fatalf("after prune: %+v err=%v", left, err)
	}
}

// --- Unit-тесты векторных хелперов (без БД) ---

func TestEncodeDecodeVector(t *testing.T) {
	in := []float32{0.5, -1.25, 3, 0, 1e-3}
	enc := encodeVector(in)
	if !strings.HasPrefix(enc, "[") || !strings.HasSuffix(enc, "]") {
		t.Fatalf("encode: %q", enc)
	}
	out, err := decodeVector(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !vecEqual(in, out) {
		t.Fatalf("round-trip: %v != %v", out, in)
	}
	if _, err := decodeVector("not a vector"); err == nil {
		t.Fatal("malformed literal must fail")
	}
	v, err := decodeVector("[]")
	if err != nil || v != nil {
		t.Fatalf("empty vector: %v %v", v, err)
	}
	if encodeVector(nil) != "[]" {
		t.Fatalf("encode nil: %q", encodeVector(nil))
	}
}

func TestVecEqual(t *testing.T) {
	if !vecEqual(nil, nil) {
		t.Fatal("nil==nil expected")
	}
	if vecEqual([]float32{1}, []float32{1, 2}) {
		t.Fatal("len mismatch")
	}
	if vecEqual([]float32{1, 2}, []float32{1, 3}) {
		t.Fatal("value mismatch")
	}
}

// TestConvertToChunk — строка БД → прикладной Chunk (включая вектор).
func TestConvertToChunk(t *testing.T) {
	c := (aiChunkRow{hash: "h", sourcePath: "s.md", version: "v", docTitle: "t", index: 3, content: "c", vec: []float32{1, 2}}).ConvertToChunk()
	if c.Hash != "h" || c.SourcePath != "s.md" || c.Index != 3 || len(c.Embedding) != 2 {
		t.Fatalf("convert: %+v", c)
	}
}
