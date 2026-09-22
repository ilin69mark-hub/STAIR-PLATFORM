# S-134: Perf baseline формализован, хотспота нет (доказано профилем)

**Вердикт:** DONE — baseline-док + вердикт вместо слепой «оптимизации», запушено в `dev/swarm/ws-s132b-client`.

## Замеры (M4, 2026-09-22)
- Calculate: straight 1.06ms, lshape 0.62ms, ushape 0.52ms, spiral 0.75ms.
- Optimize: price 1.6ms, cost/lshape 17.5ms.
- pprof (30×calc + optimize-сетка, lshape): top — runtime (GC), прикладное только TessellationCache.Face 2%. **Хотспота нет.**
- Разбор «16.5с»: report-400 — среднее 17-кнопочного flow на неизвестном железе, не время Calculate. Аудитная формулировка была неточна — зафиксировано в доке.

## Что сделано
- `docs/17_INFRASTRUCTURE/27_PERFORMANCE_BASELINE.md` (новый, INFRA-0027): таблицы, вердикт, k6-пороги + последний хороший прогон (S5-2: 34892 req, validate p95 2.38ms), re-run процедура.
- Тяжёлый хвост уже закрыт алертами S-126 (Calculate p95>2s, Optimize p95>10s) — дублировать нечего.

## Гейты
- Бенчи/профиль — живые прогоны (код не менялся → Go-сьют не гонялся; док-зона).
- Локальный k6 невозможен (сеть закрыта к registry/brew) — baseline опирается на CI-гейт PR #52 (smoke green). Честно зафиксировано.

## Счётчик CI
Задача 3/20 после PR #52.
