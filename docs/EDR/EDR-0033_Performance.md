# STAIR PLATFORM

**Document:** EDR-0033_Performance.md

**ID:** EDR-0033

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Engineering

---

# 1. Purpose

Документ фиксирует Performance Optimization (Phase B, B2) — второй элемент
Engineering Expansion bounded context (BC-002). Цель — сделать сквозной
конвейер и оптимизацию (EDR-0032) предсказуемо быстрыми и отменяемыми:

1. **Контекст отмены** — `context.Context` прокидывается от HTTP-слоя через
   прикладные сервисы (`Calculate`, `Optimize`) в движки: поиск по сетке
   (EDR-0032) и Geometry Engine проверяют `ctx.Err()` и останавливаются при
   отмене/дедлайне (обрыв клиента, shutdown, таймаут). Это инвариант
   наблюдаемости и доступности: долгий поиск не должен висеть после обрыва.
2. **Latency-метрики** — гистограммы времени конвейера и оптимизации с
   лейблами `flight`/`target` в `/metrics` (расширение EDR-0021): база для
   SLO и профилирования горячих точек.
3. **Профилирование** — `net/http/pprof` на `/debug/pprof/*` по
   `STAIR_PPROF_ENABLED` (по умолчанию выключено): CPU/memory/allocs
   профили в проде без изменения маршрутов API.
4. **Триангуляция O(n²)** — ear clipping с предвычисленным списком reflex-
   вершин вместо полного сканирования всех вершин в `isEar` (в худшем
   случае было O(n³)): главный CPU-хотспот конвейера (EM-06). Кеш
   триангуляций (TessellationCache) уже устранил 4× повтор — здесь
   оптимизируется сам примитив.

Горячая точка подтверждена бенчмарком: полный конвейер ~416µs
(BenchmarkCalculatePipeline, Apple M4), ядро — триангуляция граней.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase B — Engineering Expansion: Performance Optimization)
- EDR-0021 (метрики Prometheus text-формата; этот документ расширяет его
  реестром прикладного слоя)
- EDR-0032 (Advanced Optimization — поиск по сетке, получает ctx)
- ADR-0003 (детерминизм: отмена не меняет результат при отсутствии отмены)
- `internal/geometry/triangulate.go` (ear clipping), `bench_test.go`
- `cmd/api/main.go` (pprof-маршруты)

---

# 3. Model

## 3.1 Контекст отмены через движки

Сигнатуры меняются только там, где есть циклы/дорогие стадии:

- `optimization.Search(ctx, eval, opt) Result` — проверка `ctx.Err()` на
  каждой итерации сетки; при отмене возвращает `Result{Cancelled: true}`
  (порядок обхода и отбор лучшего не меняются — детерминизм ADR-0003).
- `geometry.Generate(ctx, cfg)` — `errgroup.WithContext(ctx)`: глобальная
  отмена, если любой воркер вернул ошибку или контекст отменён; воркеры
  проверяют `ctx.Err()` перед обработкой тела. Лимит пула —
  `runtime.GOMAXPROCS(0)` (как сейчас).
- `stair.Service.Calculate(ctx, cfg, opts)` и `Optimize(ctx, ...)` —
  проверка `ctx.Err()` между стадиями конвейера (solver → geometry →
  manufacturing → cost → price); при отмене возвращают ошибку
  `context.Canceled`/`context.DeadlineExceeded` (обёрнутая).

Решатели (solver) — быстрые чисто-функциональные формулы, контекст им не
передаётся: отмена обрабатывается на границах стадий, где реальные циклы.

## 3.2 Latency-метрики

Новый реестр прикладного слоя `stair.ServiceMetricsReg`
(`internal/application/stair/metrics.go`) — расширение EDR-0021:

| Метрика | Тип | Лейблы | Смысл |
|---|---|---|---|
| `stair_calculate_duration_seconds` | histogram | `flight`, `valid` | время полного конвейера |
| `stair_optimize_duration_seconds` | histogram | `flight`, `target` | время поиска оптимума |

Бакеты по умолчанию (см. `normBuckets`). Наблюдение — в начале/конце
`Calculate`/`Optimize` (включая blocking-валидацию и отмену). `/metrics`
пишет оба реестра (HTTP + прикладной), чтобы не плодить эндпоинты.

## 3.3 Профилирование (pprof)

`cmd/api/main.go`: при `STAIR_PPROF_ENABLED=true` регистрируются
`/debug/pprof/` маршруты `net/http/pprof` (index, cmdline, profile, symbol,
trace, goroutine, heap, allocs, threadcreate, block, mutex) поверх
основного роутера. По умолчанию выключено — без поверхностного расхода в
продакшене. Доступ не ограничивается auth: включать только во внутреннем
контуре/локально.

## 3.4 Триангуляция O(n²)

Ear clipping с reflex-списком (Кл. теорема: если треугольник (a,b,c)
CCW-контура содержит вершину, то он содержит reflex-вершину). `isEar`
проверяет только reflex-вершины вместо всех; reflex-статус пересчитывается
только для двух соседей `a` и `c` после среза уха. Сложность в худшем
случае O(n²) вместо O(n³). Выход (набор треугольников) детерминирован и
идентичен предыдущему для невырожденных полигонов.

---

# 4. Security

- pprof выключен по умолчанию; включается явно через `STAIR_PPROF_ENABLED`
  (внутренний контур/локально). Документировано в §3.3.
- Метрики не содержат данных запросов/тенантов (только лейблы
  flight/target/valid — низкая кардинальность, инвариант EDR-0021).
- Контекст отмены не расширяет поверхность атак: это штатный механизм
  net/http (request ctx).

---

# 5. Acceptance

1. `go build ./...`, `go vet ./...`, `gofmt -w` — чисто.
2. Триангуляция: существующие юнит-тесты проходят без изменений выходных
   данных; бенчмарк `BenchmarkTriangulate` не деградирует.
3. Отмена: юнит-тест `Search` с отменённым ctx возвращает
   `Result{Cancelled: true}`; `Calculate`/`Optimize` возвращают
   `context.Canceled` при отменённом ctx.
4. Метрики: после `Calculate` в `/metrics` появляется
   `stair_calculate_duration_seconds_count` с лейблами `flight`, `valid`.
5. pprof: при `STAIR_PPROF_ENABLED=true` `GET /debug/pprof/` отвечает 200;
   при выключенном — 404.
6. ROADMAP: «Performance Optimization — DONE 2026-08-15».
