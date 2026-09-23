package database

// AIRepository — инфраструктурная реализация портов AI-слоя (S-135):
// RAG-корпус (ai_corpus_chunks) и conversation-memory (conversation_messages).
// Домен/application не зависят от этой реализации (DOM-0008): порты
// объявлены в internal/application/assistant.
//
// Два пути ретривера (решение S-135):
//   - pgvector: колонка embedding vector(1536) + HNSW-индекс. Доступна,
//     когда расширение vector установлено (прод-RDS, локальный
//     stair-pgvector-pg). Используется VectorSearch.
//   - FTS-fallback: tsvector/GIN по russian-конфигурации. Работает на
//     штатном postgres:16 (stair-test-pg, CI) БЕЗ pgvector. Используется
//     FTSSearch и векторным ретривером при недоступности pgvector.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	appast "stairplatform/internal/application/assistant"
)

// AIRepository объединяет порты RAG и памяти.
type AIRepository struct {
	pool      *pgxpool.Pool
	hasVector bool // расширение pgvector доступно на этом инстансе
}

// NewAIRepository создаёт репозиторий. hasVector определяется реально:
// запрос к pg_type один раз при инициализации.
func NewAIRepository(pool *pgxpool.Pool) *AIRepository {
	r := &AIRepository{pool: pool}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var has bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vector')").Scan(&has); err != nil {
		slog.Warn("ai_repo: pgvector availability check failed", "error", err)
		has = false
	}
	r.hasVector = has
	if has {
		slog.Info("ai_repo: pgvector available — vector path enabled")
	} else {
		slog.Info("ai_repo: pgvector NOT available — FTS fallback (tsvector/GIN)")
	}
	return r
}

// HasVector сообщает, доступен ли pgvector на этом инстансе.
func (r *AIRepository) HasVector() bool { return r.hasVector }

var (
	_ appast.ChunkStore  = (*AIRepository)(nil)
	_ appast.MemoryStore = (*AIRepository)(nil)
)

// ---------------------------------------------------------------------------
// RAG-корпус
// ---------------------------------------------------------------------------

// aiChunkRow — строка ai_corpus_chunks с вектором (nil в FTS-режиме).
type aiChunkRow struct {
	hash       string
	sourcePath string
	version    string
	docTitle   string
	index      int
	content    string
	vec        []float32 // nil — расширение недоступно / эмбеддинг не записан
}

const aiChunkCols = `chunk_hash, source_path, version, doc_title, chunk_index, content`

const aiChunkVectorExpr = `CASE WHEN embedding IS NULL THEN NULL ELSE embedding::text END`

func scanChunkRow(row pgx.Row) (aiChunkRow, error) {
	var c aiChunkRow
	var vecTxt *string
	err := row.Scan(&c.hash, &c.sourcePath, &c.version, &c.docTitle, &c.index, &c.content, &vecTxt)
	if err != nil {
		return aiChunkRow{}, err
	}
	if vecTxt != nil {
		c.vec, err = decodeVector(*vecTxt)
		if err != nil {
			return aiChunkRow{}, fmt.Errorf("ai_repo: decode embedding: %w", err)
		}
	}
	return c, nil
}

// encodeVector сериализует []float32 в текстовое представление pgvector
// "[0.1,0.2,...]" — короткое и без потерь (кратчайший round-trip float32).
func encodeVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.Grow(len(v)*2 + 8)
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// decodeVector разбирает текст pgvector в []float32.
func decodeVector(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("malformed vector literal %q", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, nil
	}
	parts := strings.Split(inner, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("malformed vector element %q: %w", p, err)
		}
		out = append(out, float32(f))
	}
	return out, nil
}

func vecEqual(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ConvertToChunk преобразует строку БД в прикладной Chunk (включая вектор,
// когда pgvector доступен).
func (c aiChunkRow) ConvertToChunk() appast.Chunk {
	return appast.Chunk{
		Hash:       c.hash,
		SourcePath: c.sourcePath,
		Version:    c.version,
		DocTitle:   c.docTitle,
		Index:      c.index,
		Content:    c.content,
		Embedding:  c.vec,
	}
}

// BackfillChunks — идемпотентный backfill корпуса (S-135): чанк с тем же
// (chunk_hash, tenant) обновляет метаданные, дубли не создаются; эмбеддинг
// записывается либо копированием уже сохранённого равного вектора, либо
// новым из input. Возвращает число НОВЫХ вставленных чанков.
func (r *AIRepository) BackfillChunks(ctx context.Context, chunks []appast.Chunk) (int, error) {
	if len(chunks) == 0 {
		return 0, nil
	}
	existing, err := r.loadPublicChunks(ctx)
	if err != nil {
		return 0, err
	}
	inserted := 0
	for _, c := range chunks {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO ai_corpus_chunks
			    (chunk_hash, source_path, version, doc_title, chunk_index, content)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (chunk_hash, COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid))
			 DO UPDATE SET
			    source_path = EXCLUDED.source_path,
			    version     = EXCLUDED.version,
			    doc_title   = EXCLUDED.doc_title,
			    chunk_index = EXCLUDED.chunk_index,
			    content     = EXCLUDED.content`,
			c.Hash, c.SourcePath, c.Version, c.DocTitle, c.Index, c.Content); err != nil {
			return inserted, fmt.Errorf("ai_repo: backfill upsert %s: %w", c.SourcePath, err)
		}
		if isNewHash(existing, c.Hash) {
			inserted++
		}
	}

	// Шаг 2: эмбеддинги (только затронутые/новые; равные существующие не трогаем).
	if r.hasVector {
		for _, c := range chunks {
			if len(c.Embedding) == 0 {
				continue
			}
			if old, ok := existing[c.Hash]; ok && old.vec != nil && vecEqual(old.vec, c.Embedding) {
				continue // уже сохранён идентичный вектор — идемпотентно
			}
			if _, err := r.pool.Exec(ctx,
				`UPDATE ai_corpus_chunks
				    SET embedding = $2::text::vector
				  WHERE chunk_hash = $1 AND tenant_id IS NULL`,
				c.Hash, encodeVector(c.Embedding)); err != nil {
				return inserted, fmt.Errorf("ai_repo: backfill embedding %s: %w", c.Hash, err)
			}
		}
	}
	return inserted, nil
}

// isNewHash оценивает, существовал ли хеш до backfill (для статистики
// числа вставленных чанков в логах/отчёте).
func isNewHash(existing map[string]aiChunkRow, hash string) bool {
	_, ok := existing[hash]
	return !ok
}

// loadPublicChunks загружает все публичные чанки (tenant_id IS NULL) с
// векторами (nil, если pgvector недоступен).
func (r *AIRepository) loadPublicChunks(ctx context.Context) (map[string]aiChunkRow, error) {
	vecExpr := "NULL::text"
	if r.hasVector {
		vecExpr = aiChunkVectorExpr
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+aiChunkCols+`, `+vecExpr+` FROM ai_corpus_chunks WHERE tenant_id IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("ai_repo: load corpus: %w", err)
	}
	defer rows.Close()
	out := make(map[string]aiChunkRow, 64)
	for rows.Next() {
		c, err := scanChunkRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ai_repo: scan corpus: %w", err)
		}
		out[c.hash] = c
	}
	return out, rows.Err()
}

// UpsertChunks — публичный интерфейс backfill (ChunkStore): тот же
// идемпотентный механизм; синоним BackfillChunks.
func (r *AIRepository) UpsertChunks(ctx context.Context, chunks []appast.Chunk) (int, error) {
	return r.BackfillChunks(ctx, chunks)
}

// VectorSearch — косинусный поиск по эмбеддингам (HNSW/seqscan, pgvector).
// Ошибка: пустой tenant (SEC-0005 — скоуп проверяется независимо от
// доступности расширения), pgvector недоступен, v != EmbeddingDim.
func (r *AIRepository) VectorSearch(ctx context.Context, tenantID string, vec []float32, topK int) ([]appast.Chunk, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("ai_repo: vector search requires tenant (SEC-0005)")
	}
	if !r.hasVector {
		return nil, fmt.Errorf("ai_repo: pgvector not available on this instance")
	}
	if len(vec) != appast.EmbeddingDim {
		return nil, fmt.Errorf("ai_repo: vector dim %d != %d", len(vec), appast.EmbeddingDim)
	}
	if topK < 1 {
		topK = 1
	}
	if topK > 50 {
		topK = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+aiChunkCols+`, `+aiChunkVectorExpr+`
		   FROM ai_corpus_chunks
		  WHERE (tenant_id = $1 OR tenant_id IS NULL)
		  ORDER BY embedding <=> $2::text::vector
		  LIMIT $3`,
		tenantID, encodeVector(vec), topK)
	if err != nil {
		return nil, fmt.Errorf("ai_repo: vector search: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

// FTSSearch — полнотекстовый поиск по content_tsv (tsvector/GIN), с русской
// конфигурацией. Tenant-фильтр: tenant_id = $1 (свой) или NULL (публичный).
// Идёт первым путём, когда pgvector недоступен; fallback для vectorRetriever.
func (r *AIRepository) FTSSearch(ctx context.Context, tenantID, query string, topK int) ([]appast.Chunk, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("ai_repo: FTS search requires tenant (SEC-0005)")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if topK < 1 {
		topK = 1
	}
	if topK > 50 {
		topK = 50
	}
	vecExpr := "NULL::text"
	if r.hasVector {
		vecExpr = aiChunkVectorExpr
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+aiChunkCols+`, `+vecExpr+`
		   FROM ai_corpus_chunks
		  WHERE (tenant_id = $1 OR tenant_id IS NULL)
		    AND content_tsv @@ plainto_tsquery('russian', $2)
		  ORDER BY ts_rank(content_tsv, plainto_tsquery('russian', $2)) DESC,
		           chunk_index ASC
		  LIMIT $3`,
		tenantID, query, topK)
	if err != nil {
		return nil, fmt.Errorf("ai_repo: FTS search: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

func scanChunks(rows pgx.Rows) ([]appast.Chunk, error) {
	out := make([]appast.Chunk, 0, 16)
	for rows.Next() {
		c, err := scanChunkRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c.ConvertToChunk())
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Conversation memory (AI-0006)
// ---------------------------------------------------------------------------

// AppendMessages — дописывает сообщения в рамках tenant+project (S-135).
func (r *AIRepository) AppendMessages(ctx context.Context, msgs []appast.MemoryMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	for _, m := range msgs {
		if m.TenantID == "" || m.ProjectID == "" {
			return fmt.Errorf("ai_repo: memory message requires tenant+project scope")
		}
		if m.Role != "user" && m.Role != "assistant" {
			return fmt.Errorf("ai_repo: invalid memory role %q", m.Role)
		}
		ts := m.CreatedAt
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO conversation_messages (tenant_id, project_id, role, content, created_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			m.TenantID, m.ProjectID, m.Role, m.Content, ts); err != nil {
			return fmt.Errorf("ai_repo: append memory: %w", err)
		}
	}
	return nil
}

// RecentMessages — последние limit сообщений проекта (новые сверху).
func (r *AIRepository) RecentMessages(ctx context.Context, tenantID, projectID string, limit int) ([]appast.MemoryMessage, error) {
	if limit <= 0 {
		return nil, nil
	}
	if limit > appast.MaxHistoryLimit {
		limit = appast.MaxHistoryLimit
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, project_id, role, content, created_at
		   FROM conversation_messages
		  WHERE tenant_id = $1 AND project_id = $2
		  ORDER BY created_at DESC, id DESC
		  LIMIT $3`,
		tenantID, projectID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("ai_repo: recent messages: %w", err)
	}
	defer rows.Close()
	out := make([]appast.MemoryMessage, 0, limit)
	for rows.Next() {
		var m appast.MemoryMessage
		if err := rows.Scan(&m.ID, &m.TenantID, &m.ProjectID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("ai_repo: scan memory: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PruneMessages — удаляет сообщения старше cutoff (TTL STAIR_AI_MEMORY_TTL).
func (r *AIRepository) PruneMessages(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM conversation_messages WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("ai_repo: prune memory: %w", err)
	}
	return tag.RowsAffected(), nil
}
