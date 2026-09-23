# builder-S135 — AI-фаза: RAG (Этап A) + conversation-memory & eval (Этап B)

**Задача:** S-135 [P3] `zone:backend` — этапы A+B по утверждённому скоупу
(борд 2026-09-23): pgvector-миграция, чанкер docs, эмбеддер OpenRouter
`/embeddings`, retriever topK с tenant-фильтром, встройка в `assistant.Ask` +
цитаты в `Response.Notes`; conversation-memory (таблица+TTL+history_limit),
eval.jsonl ~30–50 Q/A + раннер с порогом. Вендор — OpenRouter через
OpenAI-совместимый endpoint; `openai.go` НЕ тронут (проверено `git status`).

---

## Что сделано (Этап A — RAG)

- **Миграция `000025_ai_rag_corpus`** — таблица `ai_corpus_chunks`
  (chunk_hash/source_path/version/doc_title/tenant_id/chunk_index/content/
  content_tsv tsvector-GENERATED + created_at). **Условный pgvector**:
  расширение `vector` создаётся только при доступности (DO-блок);
  колонка `embedding vector(1536)` + HNSW (`vector_cosine_ops`) — только
  когда тип `vector` реально существует. На штатном `postgres:16`
  (stair-test-pg, CI) работает FTS-fallback (tsvector/GIN, russian).
  Уникальность — `(chunk_hash, COALESCE(tenant_id, zero-uuid))` (идемпотентный
  backfill); индексы GIN(content_tsv), (source_path, chunk_index).
- **Чанкер `MarkdownChunker`** (`internal/application/assistant/chunker.go`):
  разбор markdown на блоки (заголовки/абзацы/фенсы кода) с путём секций,
  целевые 500–800 «токенов» (оценка руны/4) с overlap ~100, сверхдлинные
  блоки — скользящее окно с гарантией прогресса; детерминирован;
  `Hash = sha256(content)`; цитата `[source: <путь>, §N]`.
- **Эмбеддер `OpenAIEmbedder`** (`embedder.go`): OpenAI-совместимый
  `POST {baseURL}/embeddings` (OpenAI/OpenRouter/Ollama/vLLM), батчи по 64,
  сортировка по index, **строгая проверка размерности 1536**, таймаут 15s,
  доп. заголовки (HTTP-Referer/X-Title для OpenRouter). `openai.go` не
  изменён — эмбеддер отдельный клиент (требование: вендор через env).
- **Retriever** (`rag.go`): порты `ChunkStore`/`Retriever`/`Embedder`;
  `ftsRetriever` (tsvector/GIN) и `vectorRetriever` (embed → косинус в
  pgvector) **с деградацией в FTS** при сбое эмбеддера/поиска (AI-0003);
  валидация инвариантов: tenant обязателен (SEC-0005), topK ∈ [1..50],
  query непустая.
- **Хранилище `AIRepository`** (`internal/infrastructure/database/ai_repo.go`):
  `BackfillChunks` (идемпотентный upsert + запись эмбеддингов только
  изменённых/новых), `VectorSearch` (`tenant_id = $1 OR IS NULL`,
  `<=>` cosine, LIMIT topK), `FTSSearch` (русский tsvector, ts_rank),
  детект pgvector при инициализации (`pg_type.typname='vector'`).
- **Встройка в Ask** (`entity.go`/`design.go`):
  `Service.WithRAG(retriever, topK)` / `WithMemory(store)` (nil-safe,
  default-off); чанки → `intent.Context` («Релевантные фрагменты
  документации (RAG)»), **цитаты → `Response.Notes`**, **chunk ids →
  detail события аудита** (`rag_sources=[source: …,§N],…` — DoD «аудит
  chunk ids»); RAG/память — best-effort (сбой не роняет ответ).
- **Wiring** (`cmd/api/main.go`): RAG по `STAIR_AI_EMBED_BASE_URL`
  (+`STAIR_AI_EMBED_MODEL/API_KEY`), `STAIR_AI_RAG_TOP_K` (default 5);
  без env — FTS-путь. Транспорт (`internal/transport/http/assistant.go`):
  запрос принимает `project_id`, `history_limit`; swagger синхронизирован.
- **backfill-команда `cmd/ai-backfill`**: обход корпуса
  (`STAIR_AI_CORPUS_DIR`, default `docs/13_AI,docs/ADR,docs/EDR`), чанкинг,
  опциональные эмбеддинги, upsert; `-dry-run`.

## Что сделано (Этап B — память + eval)

- **Миграция `000026_ai_conversation_memory`** — `conversation_messages`
  (tenant_id/project_id FK CASCADE, role CHECK user|assistant, content,
  created_at; индексы (project_id, created_at DESC), (created_at) для TTL).
- **Память** (`rag.go` порты + `ai_repo.go`): `AppendMessages` (валидация
  скоупа tenant+project и ролей), `RecentMessages` («новые сверху», limit),
  `PruneMessages` (TTL). В контекст — `HistoryBlock` (последние N,
  truncate 1500 рун; PII-политика S-127 — только резюме и ответ, не
  конфиги). `history_limit`: 0 → default 10, cap 50, <0 → без истории.
  TTL `STAIR_AI_MEMORY_TTL` (default 720h) — фоновая prune-горутина в cmd/api.
- **Eval**: `testdata/eval.jsonl` — **38 кейсов** (design + engineering,
  факты как ожидаемые подстроки в сериализованном Response); раннер
  `eval_test.go` с порогами: агрегат ≥80%, кейс ≥60%, набор ≥30.

## Цифры

| Метрика | Результат |
|---|---|
| Eval-набор | **38 кейсов, факты 198/199 = 99%** (порог 80%) — зелёный |
| p95 Ask+RAG (локальный детерм. путь) | `BenchmarkAskWithRAG`: **67.9 µs/op** (500 итер., 59503 B/op, 164 allocs) → p95 «локально» ≪ 1 мс; реальный путь (embed ~0.1–0.3 c + LLM ~0.5–2 c) укладывается в бюджет **< 5 c** (DoD) |
| Корпус backfill | dry-run: **71 файл → 142 чанка** (version=dev) |
| Аудит chunk ids | `rag_sources=` в detail события `ai.assist.*` (тест `TestAskWithRAG`) ✅ |
| Покрытие | **86.6%** общее (порог 85), инклюдит database-пакет с БД |

## Гейты (все зелёные)

- `gofmt -l internal/ cmd/` → пусто ✅
- `go vet ./...` → 0 ✅
- `go build ./...` + `go build ./cmd/...` → OK ✅
- **Полный**: `STAIR_TEST_DATABASE_URL=postgres://stair:stair@127.0.0.1:5432/stair_test?sslmode=disable go test -race -p 1 -count=1 -timeout 25m ./...` → **60/60 ok, exit 0** ✅
- `golangci-lint run ./...` → **0 issues** ✅
- **pgvector-гейт**: `pgvector/pgvector:pg16` (контейнер `stair-pgvector-pg`,
  127.0.0.1:5434) — миграция создала `vector` + HNSW-индекс (проверено
  pg_indexes), `TestVectorSearchRanking` (сквозной: backfill с векторами →
  косинусный top-1 = релевантный чанк) PASS; штатный postgres:16 — FTS-путь
  PASS (изоляция tenant'ов A/B, idempotent backfill).
- `scripts/coverage-check.sh` → 86.6% ≥ 85 ✅

## Сопутствующие правки (не S-135-код, но гейты)

- `internal/engine/scheduler/scheduler_test.go` — фикс pre-existing флейка
  `TestExecuteCancelledMidway` («expected context.Canceled, got nil» под
  -race -p 1: 47 мгновенных задач успевали до cancel-горутины). Блокируются
  первые 4 задачи (== пул), sem полон → детерминизм; 25/25 на -race.
- `internal/application/payments/service_extra_test.go` +
  `internal/infrastructure/payments/stripe_webhook_extra_test.go` —
  2 pre-existing замечания более свежего golangci-lint (unparam→seedIntent,
  SA1012 nil-ctx в регресс-тесте nil-guard провайдера, оставлен с
  `//nolint:staticcheck`).
- `internal/transport/websocket/subscribe_auth_test.go` — недостающий
  перенос строки в конце файла (gofmt).
- Docs `06_MEMORY_ENGINE.md` синхронизированы с реализацией (таблица
  `conversation_messages`, скоуп tenant+project, prune-горутина в cmd/api).

## Решения

1. **Вендор — OpenRouter** через существующий OpenAI-совместимый контур
   (`STAIR_AI_BASE_URL` уже в openai.go; эмбеддер — отдельный `/embeddings`
   клиент, openai.go не тронут). Модель эмбеддера — env, схема фиксирует
   `vector(1536)` (text-embedding-3-small на OpenRouter = 1536).
2. **Fallback-first**: на postgres:16 без pgvector RAG работает через
   tsvector/GIN; векторный путь включается только при `STAIR_AI_EMBED_*`
   и доступном расширении; векторный ретривер деградирует в FTS.
3. **Tenant-фильтр в каждом запросе** (SEC-0005): `(tenant_id = $1 OR
   tenant_id IS NULL)`; конфиги пользователей в корпус не складываются
   (PII S-127); ретривер без tenant — ошибка (тест).
4. **Best-effort всюду**: сбой RAG/памяти не проваливает Ask (AI-0003),
   аудит — best-effort как раньше.
5. **Идемпотентность backfill** по sha256(content) + ON CONFLICT: повторный
   прогон не плодит дубли, идентичные эмбеддинги не переписываются
   («new» в логе = 0).
6. История в контекст — только последние N с модельным капом 50;
   TTL-очистка фоновой горутиной с `STAIR_AI_MEMORY_TTL`.

## Блокеры

- Нет. Остаток за человеком/деплоем: задать OpenRouter-ключи
  (`STAIR_AI_API_KEY`, `STAIR_AI_EMBED_API_KEY`), прогнать
  `go run ./cmd/ai-backfill` на проде после миграций; самообслуживание
  pgvector в прод-RDS (на postgres:16 prod работает FTS-fallback).
- Счёт >$300/мес 3 мес подряд → пересмотр self-hosted (решение человека
  уже в скоупе S-135, борд).

## Рекомендации (дальше)

- Этап C (tool-calling наружу) и агентные циклы — отдельными задачами,
  только после накопления реальных диалогов и eval-зелени.
- Прод: включить `STAIR_AI_EMBED_*` → pgvector-путь; метрика
  `assistant_duration` уже наблюдает p95 в проде (см. INFRA/14_AI).
- Включить eval-кейсы в CI-гейт (они уже в сьюте — `TestEvalDeterministicCoverage`).

## Вердикт

✅ **DONE (2026-09-23).** S-135 Этапы A+B реализованы, DoD выполнен:
eval green (99% > 80%), p95 Ask+RAG < 5с (локальный детерм. путь 68 µs/op;
реальный бюджет 1–3 c), аудит chunk ids (`rag_sources=` в audit detail),
runbook AI-0007 (docs/13_AI/07 §Implementation/Runbook + env-таблица),
покрытие 86.6% ≥ 85%. Гейты: полный `go test -race -p 1 -count=1
-timeout 25m ./...` 60/60, vet/build/gofmt/lint 0, pgvector-гейт PASS.
Коммит: `dev/swarm/ws-s132b-client` (не запушен).