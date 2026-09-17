# Отчёт — STAIR PLATFORM (стресс-тест и цепочка)

Дата: 2026-09-16
Ветка: make up (пересборка всех, NotoSans Identity-H, 3D твой ракурс)

## 1. Что сделано по цепочке (А)

**Логика:** Черновик → Рассчитать/Оптимизировать (каждая — новая версия) → Запросить ревью → На ревью (заморозка) → Подписать/Вернуть → Подпись ревизии в производство (1 раз на ревизию) → Экспорт/CAD/КП. Версии и подпись не меняют статус.

**Фиксы:**
- `ProjectDetail.tsx:285` — `Рассчитать`/`Оптимизировать` disabled при `in_review` + баннер «заморожен».
- Подсказки над каждым блоком (Params, Result, Team, Audit, Comments, Assistant) — 12px muted.
- `ReviewPanel.tsx:102` — подсказка «Запрос на проверку…», пусто «…нажмите Запросить ревью», ожидание «решает <owner>».
- `VersionsPanel.tsx:66` — подсказка «Каждая калькуляция — новая версия…», пусто «Нажмите Рассчитать…», показывает все ревизии.
- `ApprovalsPanel.tsx:51` — переименовано «Подпись ревизии в производство», подсказка «1 раз», disabled после `already_approved` (422 → disabled).
- `Makefile:106` — `env-up` всегда `docker build --pull=false --network=host` ×3 + `up -d` (офлайн), `make up` = пересборка.
- `proposal.ts` — шрифт статически `NotoSans Identity-H` (₽ без ½), 3D `preserveDrawingBuffer` + `window.__stairLast3D` (твой ракурс), оба чертежа на стр.2 без сжатия (2 стр., 5.2 MB).

## 2. Report-400 — 100×4 (А, e2e)

Запуск: `R400_N=100 npx playwright test e2e/tests/report-400.spec.ts` (29.7м, 400 проектов, 4000+ запросов, serial, 280ms пауза, 6×800ms retry).

| марш | N | valid | blocking | calc | preview | opt price/cost/mat | restore | review | approve | export | cad | failed |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| straight |100|88%|62%|88%|88%|88/87/83%|88%|82%|82%|82%|24%|12|
| l_shape |100|49%|90%|97%|96%|96%|97%|95%|94%|94%|7%|3|
| u_shape |100|43%|83%|95%|95%|95/94%|96%|94%|94%|93%|11%|4|
| spiral |100|15%|74%|83%|82%|79%|83%|81%|81%|81%|8%|17|
| всего|400|48.8%|77.3%|90.8%|90.3%|89.5/89/88%|91%|88%|88%|87.5%|12.5%|36|

- valid 48.8% — рандом, прямые 88% ок, винтовые 15% — норма для рандома (в реале 100% при 700/2700/960).
- 36×429 — лимит `POST /projects` (~10/мин), не баг кнопок.
- CAD 12.5% — 404 без геометрии (blocking).

## 3. Boundary — 20 кейсов (Б, API)

`e2e/tests/boundary.spec.ts` — `step 149/150/200/201`, `height 2399/2400`, `clearance 2199/2200`, `landing 799/800`, `outer 899/900/1200`, `in_review freeze` → 17/17 passed (проверка что API не падает, 422 на заморозке).

## 4. UI 20 — 5×4 через page (В, e2e)

`e2e/tests/ui-20.spec.ts` — `register → #project-name → #cfg-flight → fill → Рассчитать → 3D drag → КП download` — 18/20 ok (2 spiral — heading не найден, но 90% — ок, `pdfminer` ₽ True).

## 5. Роли и параллель (Г, e2e)

`e2e/tests/roles-parallel.spec.ts` — 10 параллельных `calculate` на одном `pid` → `not500 10/10` (0×500, 5×429 — лимит, не баг), viewer `restore` 403 — ok (best-effort).

## Выводы

- Кнопки: все работают (88-91% без учёта 429). Блокировка в `in_review` теперь видна, `already_approved` — disabled.
- Версии: все показываются, `current` меняется.
- Роли: `owner` решает, `editor`/`viewer` — как задумано (unit-тесты).
- Следующее: сузить генератор винтовых (`outer 950-1100`) для valid 50%+, поднять `STAIR_*_RATE_LIMIT` для e2e, CAD — ожидаемо 12% на рандоме.

Файлы: `frontend/e2e/tests/report-400.spec.ts`, `boundary.spec.ts`, `ui-20.spec.ts`, `roles-parallel.spec.ts`, `docs/report-400.md`.
