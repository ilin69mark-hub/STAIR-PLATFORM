# Coverage Checkpoint — 2026-09-16 (закрытие на сегодня)

> Быстрый файл для продолжения. Сгенерирован после фаз 1-2 подъёма. Все команды — из корня `stair-platform/`.

> **Обновление 2026-09-17 (admin):** закрыты дыры админки — `src/api/admin.ts` и `src/api/analytics.ts` 0→100% (новые `admin.test.ts`, `analytics.test.ts`), `App.tsx` 0→95% (новый `App.test.tsx`, роутер: loading/логин/список/детали/админ/аудит/выход/ошибка). Frontend total stmts ~70→71.92%. Новый e2e `frontend/e2e/tests/admin-panel.spec.ts` (4 кейса: разделы, смена роли, политика, API-ключи) — админ создаётся через БД `docker exec` psql (SEC-0004). Заодно починен баг: `load()` в `AdminPanel` сбрасывал панель в скелетон после мутаций и терял одноразовый токен API-ключа → добавлен параметр `silent` (тихое обновление без скелетона).

> **Обновление 2026-09-17 (админ-детали):** дожато до 100% — `App.tsx` роутер (добавлен кейс создания проекта → onCreated(refresh+переход), не-ApiError ошибка списка; оставшаяся ветка line 28 `e instanceof ApiError ?`) и панели `PolicyPanel.tsx`, `OrdersPanel.tsx` (новые собственные `*.test.tsx`, presentational: все onChange/чекбоксы/save, empty-state, «—»-фолбэки разметки без размеров/цены, селект статуса). Итог: **314/314 теста**, frontend total stmts **73.37%**, branch 74.40%, lines 75.49%. Коммит тестов админки — `8885010` (не запушен).

> **Обновление 2026-09-17 (аудит/скроллспай):** `shared/src/api/audit.ts` 0→100% и `src/api/audit.ts` 75→100% (новые `audit.test.ts`), `src/lib/useScrollSpy.ts` 33→100% (stmts/lines/funcs; branch 85.7%) — новый `useScrollSpy.test.ts`. Готча окружения: jsdom-`localStorage` здесь без `getItem/setItem` — `logAction` из-за `catch{}` всё равно шлёт fetch; в тесте подставляем in-memory Storage через `vi.stubGlobal`.

> **Обновление 2026-09-17 (store: отзывы/заказы, scopes API-ключей):** закрыт приоритет №1 — Go-тесты `internal/transport/http/{testimonials,orders}_test.go`: расширены `fakeTestimonialService`/`fakeOrderService` (дикие ошибки по операциям), добавлены `failingTenantAuth` + роутер-хелперы с встроенным аудитом; ~20 новых кейсов (list/create/update/delete отзывов: 400/404/422/500/audit/403, orders: admin list error, updateStatus 400/404/422/500/audit/GetError, консультация tenant/service 500). Итог: **testimonials.go все хендлеры 100%**, orders admin list + updateStatus 100%, `handleCreateConsultation` 90%, http 81.5→**83.6%**. Приоритет №2 — e2e admin-panel **7 кейсов**: + заказы (смена статуса `select[aria-label="Статус заказа {id}"]` new→confirmed; заказ сидится в beforeAll через `INSERT ... SELECT t.id,u.id ... JOIN tenants WHERE u.email=... RETURNING id` с `jsonb_build_object` — двойные кавычки внутри `-c "..."` ломают шелл) + отзывы CRUD (черновик/публикация/скрытие/удаление, notice'ы «Отзыв добавлен...», «Отзыв опубликован на лендинге», «Отзыв удалён») + ключ с неизвестным scope отклоняется. Приоритет №3 — валидация scopes в `handleCreateApiKey`: добавлены `auth.AllPermissions()`/`IsKnown()` (`entity.go`), 422 на неизвестный scope (фронт уже тримил пробелы). ВАЖНО: e2e использовал СТАРЫЙ бинарник контейнера — после правок Go требуется `docker compose -f deployments/docker-compose.yml build api && up -d api`.

## 1. Где остановились (факт, с БД)

```bash
# Go total
STAIR_TEST_DATABASE_URL=postgres://stair:stair@127.0.0.1:5432/stair_test?sslmode=disable \
  go test -p 1 ./... -coverprofile=/tmp/coverage.out
go tool cover -func=/tmp/coverage.out | grep total:
# → total: 79.9% (старт 73.3% с БД, +6.6pp; без БД 67.8% → ~70.2%)
./scripts/coverage-check.sh 85  # → FAIL -5.1pp

# Per-package (p1, с БД)
# validation 100%, audit 100%, order 90.5%, jwt 91.5%, cad 91.7%, pricing 85.5%,
# health 70.8%, geometry 86.3%, solver 90%, database 72.6%→73.x%, http 81.4%, websocket 92.9%, graphql 58.6%

# Frontend
npm --prefix frontend run test:coverage       # 70.09% stmts / 72.23% lines (30 файлов 252 теста) — было 69.66%/71.82%
npm --prefix frontend-store run test:coverage # 87.68% / 88.84% (12 файлов 95 тестов) — было 80.03%/81.13%
# Пороги vitest 50/50/40/40 — оба PASS

# Все зелёные
STAIR_TEST_DATABASE_URL=... go test -p 1 ./...  # 55 ok 0 FAIL
npm --prefix frontend run test -- --run        # 30/252
npm --prefix frontend-store run test -- --run  # 12/95
```

**Гейт:** `scripts/coverage-check.sh:16` `THRESHOLD=85` (исключает `database` без `STAIR_TEST_DATABASE_URL`), `vitest.config.ts:26` / `frontend-store/vitest.config.ts:20`.

## 2. Что сделано (21 Go + 10 Frontend новых файлов)

**Go S/M:** `validation/email_test.go` (0→100%), `audit/context_test.go` (57→100%), `order/consultation_test.go` (57→90%), `engineering/factory_test.go` (68→71%), `jwt/jwt_extra_test.go` (51→91%), `pricing/events_test.go` (71→85%), `cad/cnc_test.go` (35→91%), `stair/errors_extra_test.go` (67→71%), `health/checker_extra_test.go` (50→70%), `payments/verifier_test.go`, `cmd/api/env_test.go` (11→37%), `http: dedup_test.go/internalonly_test.go/deprecation_test.go/errorlog_test.go/ratelimiter_redis_test.go/routetimeout_extra_test.go/pagination_extra_test.go/usererror_extra_test.go` (72→81.4% via subagents), `geometry/topology_extra_test.go/vec_extra_test.go` (83→86.3%), `solver/input_extra_test.go`, `database/extra_coverage_test.go + approvals_extra + migrate_extra + database_extra_test.go` (65→72.6%), `graphql/subscription_extra_test.go` (52→58.6%), `domain/document/document_extra2_test.go` (69→73%), `websocket` 39→92.9% (subagent `websocket_extra_test.go`).

**Frontend S:** `shared/VariationPicker.test.tsx` (0→100%), `shared/scheme-annot.test.ts` (37→100%), `src/lib/export-extra.test.ts` (16→92%), `src/api/auth.test.ts` + `frontend-store/src/api/auth.test.ts`, `src/components/AuditPage.test.tsx` (0→100%), `frontend-store/src/components/AuthForm.test.tsx` (35→93%), `frontend-store/src/api/client.test.ts` (35→~85% via копия `frontend/src/api/client.test.ts:125`), `src/api/projects.test.ts` (20→~60%), `src/lib/proposal.ts` экспортированы `rub`/`flightLabel` + `proposal-helpers.test.ts` (0.8→3.7%).

**Инфра:** `docker compose -f deployments/docker-compose.yml up -d` (postgres:16 healthy, redis:7), `stair_test` пересоздаётся `DROP/CREATE` перед полным прогоном — иначе `Dirty database version 17` при параллельном `go test ./...` (решение: `-p 1`).

## 3. Что осталось до 85% Go (-5.1pp ≈ 750 строк)

| Пакет | Сейчас | Цель | Эффект total | Файл:строка | Приём |
|---|---|---|---|---|---|
| `transport/http:81.4%` | 81.4→85% | +1.0pp | `projects.go:1013` `approve`/`rate_limiter_redis.go:60` miniredis | `httptest` + `fakeProjectService` (`projects_test.go:22`) |
| `database:72.6%` →80% | +0.8pp | `project_repo.go:763` `ListTenantProjects` уже покрыт, осталось `migrate.go:GetMigrationVersion` (уже `migrate_extra_test.go`) | PG `STAIR_TEST_DATABASE_URL` |
| `graphql:58.6%` →75% | +0.5pp | `handler.go:18` `execute*` | `MockProjectRepository` (`mock_repo_test.go:114`) |
| `application/stair:71.7%`, `jobs:69.4%`, `engine/graph:75%` | +1pp | `stair/service.go:592` `manufacturingBlocked` | `fakeRepo` table-driven |
| `websocket` уже 92.9% — done |

**Frontend до 85% admin:** `src/App.tsx:0% (14-111)` root router, `src/lib/proposal.ts:3.7% (11-409)` `generateProposalPdf` `jspdf`+`three` мок (`vi.mock`), `src/api/projects.ts` уже 60% — добить loops. Store уже 87% — готов.

## 4. Как продолжить (команды)

```bash
# 1. Поднять БД если упала
docker compose -f deployments/docker-compose.yml up -d
docker exec stair-platform-postgres psql -U stair -d postgres -c "CREATE DATABASE stair_test OWNER stair" # если нет

# 2. Полный прогон с БД (сериализованно, иначе dirty)
docker exec stair-platform-postgres psql -U stair -d postgres -c "DROP DATABASE stair_test; CREATE DATABASE stair_test OWNER stair"
STAIR_TEST_DATABASE_URL=postgres://stair:stair@127.0.0.1:5432/stair_test?sslmode=disable go test -p 1 ./... -coverprofile=/tmp/coverage.out
go tool cover -func=/tmp/coverage.out | grep total:
go tool cover -html=/tmp/coverage.out -o /tmp/coverage.html && open /tmp/coverage.html

# 3. Топ дыр
go tool cover -func=/tmp/coverage.out | grep "0.0%" | cut -d: -f1 | sort | uniq -c | sort -rn | head -20

# 4. Frontend
npm --prefix frontend run test:coverage
npm --prefix frontend-store run test:coverage
```

## 5. Риски

- Параллельный `go test ./...` без `-p 1` даёт `Dirty database version 17` (order_repo_test dirty down) — всегда `-p 1` с БД.
- `frontend/src/lib/proposal.ts` `rub`/`flightLabel` экспортированы для теста — не ломает API, но вернуть в private если не нужно.
- `opencode` ключ ротирован `~/.config/opencode/opencode.json:112` на `sk-or-v1-853...` (50/день free) — не коммитить.

## 6. Следующий чекпойнт

Цель: Go 85% с БД, Frontend admin 82-85%. План фаз 2-3 в этом файле §3. После достижения — `./scripts/coverage-check.sh 85` должен стать PASS.
