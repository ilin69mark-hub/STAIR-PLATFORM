
---

# `19_INFRASTRUCTURE/27_PERFORMANCE_BASELINE.md`

```markdown
# STAIR PLATFORM

Document: 27_PERFORMANCE_BASELINE.md

ID: INFRA-0027

Status: APPROVED

---

# Purpose

Формальный performance baseline (S-134): воспроизводимые цифры, SLO-пороги
и процедура перезамера. Связано: 22_SLO_SLA (пороги), 21_ALERTING
(StairCalculateSlow/StairOptimizeSlow), k6/scripts/smoke.js (гейты CI).

---

# Go micro-baseline (S-134, Apple M4, 2026-09-22)

`go test ./internal/application/stair/ -bench BenchmarkCalculateFlights
-benchtime=1x` (полный конвейер Calculate, reference-конфиги):

| марш | ns/op | B/op | allocs/op |
|---|---|---|---|
| straight | 1 060 084 | 1 225 480 | 13 593 |
| lshape | 616 208 | 1 331 392 | 18 724 |
| ushape | 515 542 | 1 329 120 | 18 718 |
| spiral | 750 917 | 2 208 360 | 24 814 |

Optimize (детерминированный поиск, ComfortStepGrid 20):

| кейс | ns/op |
|---|---|
| OptimizePrice (straight) | 1 610 542 |
| OptimizeCost (lshape) | 17 453 250 |

Вывод: движок считает доли–десятки миллисекунд. Цифра «l_shape/u_shape
~16.5 с» из report-400 (2026-09-17) — среднее 17-кнопочного flow
(calc+preview+3×optimize+restore+review+approve+export+CAD+proposal+
comment+audit) на неизвестном железе, НЕ время Calculate.

---

# CPU-profile вердикт (S-134, lshape: 30×Calculate + Optimize 13–17×20)

pprof (500ms семплов): top — runtime (GC/планировщик); единственный
прикладной узел — TessellationCache.Face 2%. Алгоритмического хотспота
НЕТ — оптимизировать нечего. Тяжёлый хвост закрыт алертами
StairCalculateSlow (p95 > 2s) / StairOptimizeSlow (p95 > 10s), S-126.
Пересмотр — только по данным прод-трафика (/metrics), не по ощущениям.

---

# k6 baseline (CI load-smoke, k6/scripts/smoke.js)

Пороги (аборт при нарушении): http_req_failed rate<0.01;
http_req_duration p95<1000; validate p95<500.
Последний известный хороший прогон (S5-2): 34892 req, checks 100%,
validate p95 2.38ms, failed 0.
Локальный k6-запуск 2026-09-22 невозможен (нет сети к registry/brew в
среде) — baseline = CI-гейт PR #52 (Test Backend green включает smoke).

---

# Re-run процедура

1. Go: `go test ./internal/application/stair/ -bench . -benchtime=10x
   -count=1` (сверить с таблицей; регресс >2× — investigate).
2. k6: CI job load-smoke на каждый PR (автоматически).
3. Тяжёлый кейс: `S134_PROF=1` + pprof top (метод S-134, scratch-тест
   не коммитить).
```

---

# Acceptance Criteria

Цифры воспроизводимы из коробки; вердикт «хотспота нет» доказан профилем;
тяжёлый хвост покрыт алертами S-126; k6-гейты в CI.
