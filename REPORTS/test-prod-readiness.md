# Tester — Аудит тестового покрытия и качества тестов (готовность к продакшену)

- **Дата:** 2026-09-22
- **Ветка:** main @ 2d0aa20 (dirty: только untracked REPORTS/)
- **Команды:** go test -count=1 ./... · go test -cover ./internal/... · scripts/coverage-check.sh · golangci-lint run · gofmt -l · go vet · npm run lint/typecheck/test (frontend, frontend-store) · статика Playwright + gh run view
- **Окружение:** go1.26.6 darwin/arm64, golangci-lint 2.13.2, node workspaces (root), vitest 5.0.1

---

## Go

### Прогон без БД (`go test -count=1 ./...`) — PASS
- **60 пакетов ok, 0 failures**, 1 без тестов (cmd/migrate), общий ~4 мин (engine/variation ~101s).
- **Требуют STAIR_TEST_DATABASE_URL:** только `internal/infrastructure/database` — **67 из 75 тестов (89%) скипаются** без БД (репо-тесты на skip-наборе, `grep -c "--- SKIP"` = 67). Покрытие пакета без БД — 2.2%.
  - Остальные инфраструктурные пакеты (cache, queue, events, storage…) тестируются с моками — БД не нужна.
- Замечание: репо-правило роя требует локальный прогон `go test -race -p 1` с `STAIR_TEST_DATABASE_URL` после каждой задачи — т.е. полный сет 58/58 гоняется с БД в рабочем потоке; здесь — быстрый прогон без БД по ТЗ.

### Покрытие по ключевым пакетам (`go test -cover ./internal/...`)
| Пакет | Покрытие | Статус vs 85% |
|---|---|---|
| application/auth | 89.0% | ✅ |
| application/project | 89.8% | ✅ |
| application/order | 90.5% | ✅ |
| application/payments | **80.0%** | ⚠️ ниже порога |
| application/analytics | 91.5% | ✅ |
| application/integrations | **82.5%** | ⚠️ ниже порога |
| infrastructure/database | **2.2%** (без БД; 67/75 skip) | ⚠️ слепое пятно без БД |
| transport/http | 90.3% | ✅ |
| **ИТОГО (весь репо)** | **87.1%** | ✅ порог 85% достигнут (`scripts/coverage-check.sh 85` → exit 0) |

### Линт/анализ
- `golangci-lint run ./...` → **0 issues** (exit 0).
- `gofmt -l .` → чисто (0 файлов).
- `go vet ./...` → чисто (exit 0).

### Количество тестов
- Go-тестов: **1795** (`go test -list '^Test' ./...`), бенчмарков: **18**.
- Тестовых файлов (_test.go, без frontend/node_modules): **229** (топ: transport/http 46, infrastructure/database 17, geometry 13).

---

## Frontend

### frontend (admin)
- `npm run lint` (oxlint 1.83): **exit 0** — 0 errors, warnings react(set-state-in-effect) в 5 файлах (не блокируют).
- `npm run typecheck` (tsc -b): **exit 0**.
- `npm test` (vitest): **404/404 passed** (44 файла, 5.66s).
- Coverage (отчёт frontend/coverage, 2026-09-21): statements 86.4%, lines 88.6%, branches 80.6%, functions 88.1% — пороги vitest.config.ts (lines 80 / stmts 80 / funcs 75 / branches 70) ✅.

### frontend-store
- `npm run lint` (oxlint 1.75): **exit 0** — warnings no-unused-vars (rails-matrix.spec.ts:108), set-state-in-effect (Cabinet.tsx:51), exhaustive-deps (Constructor.tsx:288).
- `npm run typecheck` (tsc -b): **exit 0**.
- `npm test` (vitest): **2100/2100 passed** (13 файлов, 7.22s).
- Coverage (frontend-store/coverage, 2026-09-21): statements 85.3%, lines 86.9%, branches 82.7%, functions 87.2% — пороги (lines 80 / stmts 80 / funcs 75 / branches 80) ✅ (частные файлы ~70% branches — medium, глобально ок).

---

## E2E (Playwright) — готовность

- **Конфиги:** оба есть — `frontend/playwright.config.ts` (admin, webServer Vite :5173 + Go API :8080 с БД), `frontend-store/playwright.config.ts` (store, :5175, workers 4). CI: retries 2, workers 1, html/blob-репортеры.
- **Спеки:** admin — 13 файлов (CI запускает 12, report-400 исключён: 400 проектов), ~21 test(); store — 6 файлов (CI: 5 + quote-matrix-generated отдельной матрицей), ~12 test() + **матрица 2000 сгенерированных** (8 шардов × 250); Go e2e `tests/e2e` — 5 файлов, 7 тестов.
- **Последний прогон (CI, main @ d3319e3, 2026-09-22):** E2E Admin ✅, E2E Store ✅, E2E Tests (Go) ✅, E2E matrix **8/8 шардов** ✅. Т.е. e2e реально исполняются в CI на каждый merge.
- Локально не запускались (по ТЗ — долго): оценка готовности высокая: конфиги, спеки, CI-оркестрация и свежий зелёный прогон есть.

---

## Регрессии

### 🟥 Store-образ Docker не собирается на main (зона deploy) — БЛОКЕР прод-деплоя фронтов
- CI run **35696977365** (merge PR #51, 2026-09-22): джоба **"Build and push Store" — failure**; "Build and push Admin" — skipped следом. Все 19 тестовых джоб — зелёные.
- Симптом: `npm error code EUSAGE` — "The \`npm ci\` command can only install with an existing package-lock.json".
- Причина: PR #36 (root npm workspaces, 2026-09-21) удалил `frontend/package-lock.json` и `frontend-store/package-lock.json` (единый root-лок), а `deployments/store.Dockerfile:9-10` и `deployments/admin.Dockerfile:9-10` по-прежнему делают `COPY frontend/package*.json ./ && npm ci` — lock-файла в контексте нет → EUSAGE. `.dockerignore` содержит re-include `!frontend/package-lock.json` для уже несуществующих файлов.
- Влияние: `make env-up`/`make up` для frontend-контейнеров и CI docker-publish (теги ghcr.io) сломаны; API-образ (deployments/Dockerfile) собирается нормально.
- Фикс — зона deploy (не мой зон: отчёт, без правок): копировать root `package-lock.json` или `npm ci --workspaces` в обоих Dockerfile.

### Известные флейки (не воспроизведены в этом прогоне)
- `roles-parallel/admin-panel:86` — только при параллельном R400-стрессе (429-дедуп), изолированно 8/8 (борд S-100, 2026-09-20). В сегодняшнем прогоне не наблюдался.

---

## Пробелы покрытия (оценка)

1. **internal/infrastructure/database — 2.2% без БД** (67/75 скипов incl. auth/project/order/payment/analytics/integration/audit репозитории + миграции): самый большой слепой участок при дефолтном `go test ./...`. Покрывается только прогоном с `STAIR_TEST_DATABASE_URL` (репо-правило — после каждой задачи; CI: postgres service). Рекомендация: гейт БД-части в CI уже есть — держать.
2. **Ниже 85% по пакетам:** infrastructure/queue 56.7%, infrastructure/payments 62.6%, application/storage 66.7%, infrastructure/tracing 68.2%, infrastructure/health 70.8%, domain/document 73.1%, infrastructure/integrations 77.6%, application/assistant 79.4%, application/payments 80.0%, application/jobs 81.6%, application/stair 81.9%, engine/document 82.1%, application/integrations 82.5% — платёжные/очередные/интеграционные контуры и вспомогательная инфраструктура слабее домена/транспорта.
3. **Payments-контур в целом** (application 80.0% + infrastructure 62.6%) — самый важный бизнес-риск для проде: тоньше всего покрыты именно деньги.
4. **Frontend-store branches ~70% в части файлов** (git-дерево/3D-рендер) — глобально в пороге, но маржинально.
5. E2E не покрывают бесшовно: admin `report-400.spec.ts` выключен из CI (локально есть); payments/CRM-флоу в e2e отсутствуют.

---

## Вердикт: PASS — тестовое качество готово к продакшену (все тестовые гейты зелёные: Go 60/60 + 87.1% покрытия, lint 0, frontend 404+2100, e2e 8/8 шардов на main); НО заблокирован прод-деплой фронтов: Docker-сборка store/admin сломана root-workspaces миграцией (npm ci без lockfile) — регрессия зоны deploy, требует отдельного фикса перед CD.
