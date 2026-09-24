package assistant

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// RAG-ретривер и порты памяти (S-135, AI-0007/AI-0006).
//
// Слои: application/assistant определяет порты (ChunkStore, MemoryStore,
// Embedder) и ретривер; infrastructure/database реализует хранилища
// (DOM-0008). Tenant-фильтр обязателен в каждом запросе к корпусу, даже
// когда в индексе только публичные чанки (SEC-0005): конфиги пользователей
// в индекс не складываются (PII-политика S-127).

// ChunkStore — порт доступа к корпусу чанков (реализует AIRepository).
type ChunkStore interface {
	// UpsertChunks — идемпотентный backfill: чанк с тем же
	// (chunk_hash, tenant) обновляет метаданные/эмбеддинг, дубли не создаёт.
	UpsertChunks(ctx context.Context, chunks []Chunk) (int, error)
	// VectorSearch — поиск по косинусному сходству эмбеддингов (pgvector).
	VectorSearch(ctx context.Context, tenantID string, vec []float32, topK int) ([]Chunk, error)
	// FTSSearch — полнотекстовый поиск tsvector/GIN (fallback-путь).
	FTSSearch(ctx context.Context, tenantID, query string, topK int) ([]Chunk, error)
}

// Retriever — выборка релевантных чанков корпуса для контекста ассистента.
type Retriever interface {
	Retrieve(ctx context.Context, tenantID, query string, topK int) ([]Chunk, error)
}

// ftsRetriever — FTS-ретривер (tsvector/GIN). Используется, когда эмбеддер
// не сконфигурирован (STAIR_AI_EMBED_*) или pgvector недоступен: это
// временный путь «до pgvector в прод-RDS» (S-135 решение).
type ftsRetriever struct {
	store ChunkStore
}

// NewFTSRetriever создаёт FTS-ретривер поверх хранилища.
func NewFTSRetriever(store ChunkStore) Retriever {
	return ftsRetriever{store: store}
}

func (r ftsRetriever) Retrieve(ctx context.Context, tenantID, query string, topK int) ([]Chunk, error) {
	if err := validateTenantAndQuery(tenantID, query, topK); err != nil {
		return nil, err
	}
	return r.store.FTSSearch(ctx, tenantID, query, topK)
}

// vectorRetriever — pgvector-ретривер: query → эмбеддинг → косинусный поиск.
// При сбое эмбеддера/поиска — деградация в FTS (не проваливаем ответ).
type vectorRetriever struct {
	store    ChunkStore
	embedder Embedder
}

// NewVectorRetriever создаёт векторный ретривер с FTS-fallback.
func NewVectorRetriever(store ChunkStore, embedder Embedder) Retriever {
	return vectorRetriever{store: store, embedder: embedder}
}

func (r vectorRetriever) Retrieve(ctx context.Context, tenantID, query string, topK int) ([]Chunk, error) {
	if err := validateTenantAndQuery(tenantID, query, topK); err != nil {
		return nil, err
	}
	vecs, err := r.embedder.Embed(ctx, []string{query})
	if err != nil {
		slog.WarnContext(ctx, "assistant: RAG embed failed, falling back to FTS", "err", err)
		return r.store.FTSSearch(ctx, tenantID, query, topK)
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	chunks, err := r.store.VectorSearch(ctx, tenantID, vecs[0], topK)
	if err != nil {
		slog.WarnContext(ctx, "assistant: RAG vector search failed, falling back to FTS", "err", err)
		return r.store.FTSSearch(ctx, tenantID, query, topK)
	}
	return chunks, nil
}

// validateTenantAndQuery — обязательные инварианты ретривера: tenant не может
// быть пустым (SEC-0005), topK в 1..50, query непустая (иначе нет смысла).
func validateTenantAndQuery(tenantID, query string, topK int) error {
	if tenantID == "" {
		return fmt.Errorf("%w: RAG retriever requires tenant", ErrInvalid)
	}
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("%w: RAG retriever requires non-empty query", ErrInvalid)
	}
	if topK < 1 || topK > 50 {
		return fmt.Errorf("%w: RAG topK out of range [1..50]: %d", ErrInvalid, topK)
	}
	return nil
}

// ChunkContext — текст чанков для контекста модели + цитаты для Notes.
// Возвращает: (контекстный блок, цитаты [source: путь, §N]).
func ChunkContext(chunks []Chunk) (string, []string) {
	if len(chunks) == 0 {
		return "", nil
	}
	var b strings.Builder
	cites := make([]string, 0, len(chunks))
	for i, c := range chunks {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		fmt.Fprintf(&b, "[%s]\n%s", c.Citation(), c.Content)
		cites = append(cites, c.Citation())
	}
	return b.String(), cites
}

// MemoryMessage — одно сообщение conversation-memory (AI-0006).
type MemoryMessage struct {
	ID        int64
	TenantID  string
	ProjectID string
	Role      string // "user" | "assistant"
	Content   string
	CreatedAt time.Time
}

// MemoryStore — порт conversation-memory (реализует AIRepository).
type MemoryStore interface {
	// AppendMessages дописывает сообщения (роль/содержимое/скоуп tenant+project).
	AppendMessages(ctx context.Context, msgs []MemoryMessage) error
	// RecentMessages возвращает последние limit сообщений проекта
	// (новые сверху; контекст строится в обратном порядке).
	RecentMessages(ctx context.Context, tenantID, projectID string, limit int) ([]MemoryMessage, error)
	// PruneMessages удаляет сообщения старше cutoff; возвращает число удалённых.
	PruneMessages(ctx context.Context, cutoff time.Time) (int64, error)
}

// Лимиты истории диалога (S-135): default 10 последних сообщений, cap 50.
const (
	DefaultHistoryLimit = 10
	MaxHistoryLimit     = 50
)

// ProjectAuthorizer — порт проверки членства в проекте (S-142, IDOR-фикс
// S-141 №1). Conversation-memory скоупится по projectID, который приходит
// из тела запроса, поэтому перед чтением (RecentMessages) и записью
// (AppendMessages) сервис обязан убедиться, что вызывающий — член проекта
// (EDR-0008); иначе — ErrForbidden. Реализует application/project.Service
// (инверсия зависимостей: assistant не импортирует project).
// Чужой/несуществующий проект — (false, nil); сбой проверки — (false, err).
type ProjectAuthorizer interface {
	IsMember(ctx context.Context, tenantID, userID, projectID string) (bool, error)
}

// clampHistoryLimit нормализует запрошенный history_limit:
// 0 → DefaultHistoryLimit; > Max → Max; отрицательные → 0 (без истории).
func clampHistoryLimit(n int) int {
	switch {
	case n < 0:
		return 0
	case n == 0:
		return DefaultHistoryLimit
	case n > MaxHistoryLimit:
		return MaxHistoryLimit
	default:
		return n
	}
}

// HistoryBlock форматирует сообщения истории для Prompt.Context
// (только последние N; PII-правило S-127: старые сообщения в контекст
// не попадают, из БД их вычищает TTL-prune).
func HistoryBlock(msgs []MemoryMessage) string {
	if len(msgs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("История диалога по проекту (последние ")
	b.WriteString(itoa(len(msgs)))
	b.WriteString("):\n")
	// msgs идут «новые сверху» — для нарратива разворачиваем в хронологию.
	for i := len(msgs) - 1; i >= 0; i-- {
		content := truncateRunes(msgs[i].Content, 1500)
		fmt.Fprintf(&b, "[%s] %s\n", msgs[i].Role, content)
	}
	return b.String()
}

// truncateRunes обрезает строку до n рун (защита размера промпта).
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
