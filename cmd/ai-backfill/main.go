// Команда ai-backfill — идемпотентный backfill RAG-корпуса (S-135, AI-0007):
// чанкирует markdown-доки (docs/13_AI, ADR, EDR), считает эмбеддинги
// OpenAI-совместимым /embeddings (если STAIR_AI_EMBED_BASE_URL задан) и
// апсертит в ai_corpus_chunks. Идемпотентность — по sha256(content) чанка
// (uq_ai_corpus_chunks_hash_tenant): повторный запуск с тем же корпусом не
// создаёт дублей и не переписывает идентичные эмбеддинги.
//
// Примеры:
//
//	STAIR_DATABASE_URL="postgres://..." go run ./cmd/ai-backfill
//	STAIR_AI_EMBED_BASE_URL="https://openrouter.ai/api/v1" \
//	  STAIR_AI_EMBED_MODEL="openai/text-embedding-3-small" \
//	  STAIR_AI_EMBED_API_KEY="sk-or-..." \
//	  STAIR_DATABASE_URL="postgres://..." go run ./cmd/ai-backfill -version v1.2.0
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"stairplatform/internal/application/assistant"
	"stairplatform/internal/infrastructure/database"
)

func main() {
	corpus := flag.String("corpus", envOr("STAIR_AI_CORPUS_DIR", "docs/13_AI,docs/ADR,docs/EDR"),
		"каталоги корпуса через запятую (рекурсивный обход .md)")
	version := flag.String("version", envOr("STAIR_AI_CORPUS_VERSION", "dev"),
		"версия корпуса (git ref/тег/дата — попадает в метаданные чанков)")
	dryRun := flag.Bool("dry-run", false, "посчитать чанки и НЕ писать в БД")
	databaseURL := flag.String("database", os.Getenv("STAIR_DATABASE_URL"),
		"URL подключения к PostgreSQL")
	flag.Parse()

	if *databaseURL == "" {
		log.Fatal("STAIR_DATABASE_URL must be set (or -database)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 1. Сбор файлов корпуса.
	var files []string
	for _, dir := range strings.Split(*corpus, ",") {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(strings.ToLower(path), ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("walk corpus dir %s: %v", dir, err)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatalf("corpus is empty (dirs: %s)", *corpus)
	}

	// 2. Чанкинг. Целевой размер 500–800 токенов с overlap ~100 (S-135).
	chunker := assistant.NewMarkdownChunker(assistant.DefaultChunkOptions())
	chunks := make([]assistant.Chunk, 0, 64)
	for _, f := range files {
		// #nosec G304 -- пути корпуса задаёт оператор (вызов ком. строки/env);
		// утилита работает только с локальными markdown-доками репозитория.
		doc, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}
		chunks = append(chunks, chunker.Chunk(f, *version, string(doc))...)
	}
	log.Printf("corpus: %d files, %d chunks (version=%s)", len(files), len(chunks), *version)

	if *dryRun {
		log.Printf("dry-run: nothing written")
		return
	}

	// 3. Эмбеддинги (опционально): без STAIR_AI_EMBED_* — FTS-only backfill,
	// retriever работает по tsvector/GIN (fallback S-135).
	if base := os.Getenv("STAIR_AI_EMBED_BASE_URL"); base != "" {
		model := os.Getenv("STAIR_AI_EMBED_MODEL")
		if model == "" {
			log.Fatal("STAIR_AI_EMBED_MODEL must be set when STAIR_AI_EMBED_BASE_URL is set")
		}
		emb, err := assistant.NewOpenAIEmbedder(assistant.EmbedderConfig{
			BaseURL: base,
			APIKey:  os.Getenv("STAIR_AI_EMBED_API_KEY"),
			Model:   model,
		})
		if err != nil {
			log.Fatalf("embedder init: %v", err)
		}
		texts := make([]string, len(chunks))
		for i := range chunks {
			texts[i] = chunks[i].Content
		}
		log.Printf("embedding %d chunks (model=%q base=%q)...", len(texts), model, base) // #nosec G706 -- значения model/base приходят из конфигурации оператора (env), не из http-запросов
		vecs, err := emb.Embed(ctx, texts)
		if err != nil {
			log.Fatalf("embed failed: %v", err)
		}
		for i := range chunks {
			chunks[i].Embedding = vecs[i]
		}
	} else {
		log.Printf("STAIR_AI_EMBED_BASE_URL not set — FTS-only backfill (no embeddings)")
	}

	// 4. Backfill в БД.
	pool, err := database.Connect(ctx, database.DefaultConfig(*databaseURL))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	repo := database.NewAIRepository(pool)
	inserted, err := repo.BackfillChunks(ctx, chunks)
	if err != nil {
		log.Fatalf("backfill: %v", err)
	}
	log.Printf("backfill done: %d chunks upserted (%d new)", len(chunks), inserted) // #nosec G706 -- целочисленные счётчики, инъекция невозможна
	if !repo.HasVector() {
		log.Printf("NOTE: pgvector not available — embeddings skipped, FTS fallback in use")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
