# STAIR-PLATFORM FORENSIC AUDIT

Дата: 2026-09-24 · Ветка: `dev/swarm/s141-fixes` @ `111188f` · Режим: read-only (production-код не менялся; временные пробы удалены)
Метод: 8 проходов по фактическому коду + эмпирические пробы (временные тесты, удалены после), полный прогон с race, ревью инвариантов, кросс-слойные сверки.

---

## 1. Repository Map

| Область | Факт |
|---|---|
| Backend Go | 486 `.go`, слои: `transport` → `application` → `domain`/`engine`/`geometry` → `infrastructure` |
| Frontend | `frontend/` (admin, 432 юнит-теста), `frontend-store/` (store, 2100), `frontend/shared` |
| Миграции | 26 пар up/down, 56 FK/CHECK, `migrations/seeds` |
| Сборка/деплой | `cmd/api`, `cmd/worker`, `cmd/ai-backfill`, 3 Dockerfile, compose, Helm-чарт, Terraform (EKS 1.35) |
| Нагрузка | `k6/scripts/smoke.js` (1 сценарий) |
| CI | `ci.yml` (lint, arch, backend -race+coverage 85, 2 фронта, e2e, alerting, k6, security, build) + `cd.yml` |
| Зависимости | 260 Go-модулей, root npm workspaces |

## 2. Executed Checks

| Проверка | Результат |
|---|---|
| `go build ./...` | OK |
| `go vet ./...` | чисто |
| `gofmt -l` | чисто |
| `go test -race -p 1 ./...` | **61/61 ok, 0 FAIL** (~11 мин, лог `/tmp/audit-race.log`) |
| admin unit | 432/432 (8.6s) |
| store unit | 2100/2100 (14.4s) |
| Пробы: `flight:"diagonal"`, `approach_space_mm:500` | **500 Internal** (вместо 422) |
| Пробы: h=1e9/1e12, NaN, −width | корректная блокировка (200+valid:false / 400) |
| Проба: 4 конкурентных `SaveCalculationWithConfig` | **2–3 из 4 падают** (unique violation), 3/3 прогона |
| `hack/check_audit_exceptions.py` | 11/11 парных |

---

## 3. Findings

### DB-001 — Конкурентное сохранение конфигурации теряет запись
**Severity:** HIGH · **Status:** CONFIRMED · **Layer:** Database/Concurrency
**Location:** `internal/infrastructure/database/project_repo.go:409-447` (`SaveCalculationWithConfig`), `migrations/000010_configuration_versioning.up.sql:8`

**Problem:** Номер ревизии вычисляется как `(SELECT COALESCE(MAX(revision),0)+1 ...)` внутри транзакции, но **без блокировки** строки проекта (`FOR UPDATE` / advisory lock отсутствуют), при этом на `(project_id, revision)` стоит UNIQUE-индекс.

**Evidence (воспроизведено 3/3):**
```
concurrent SaveCalculationWithConfig (4 goroutine, 1 проект):
  run1: ok=2 fail=2  duplicate key ... "stair_configurations_project_revision_idx" (SQLSTATE 23505)
  run2: ok=2 fail=2  то же
  run3: ok=1 fail=3  то же
```

**Reproduction:** два пользователя (или один UI + ретрай) одновременно сохраняют конфигурацию одного проекта.
**Impact:** потеря сохранения у одного из пользователей + 500 в логах; в e2e-сценарии S-137 этот же механизм уже ронял миграции (dirty-10) — то есть класс дефекта уже проявлялся в тестах и был «закрыт» в тест-хелпере, а не в коде.
**Root Cause:** read-then-write без сериализации (classic lost update), компенсированный только UNIQUE-constraint, который превращает гонку в ошибку.
**Fix:** `SELECT ... FROM projects WHERE id=$1 FOR UPDATE` в начале транзакции (или `pg_advisory_xact_lock(hashtext(project_id))`); либо `INSERT ... SELECT COALESCE(MAX...)+1 ... ON CONFLICT DO NOTHING` с ретраем.
**Regression Test:** `TestConcurrentConfigurationRevision` — N=8 параллельных сохранений одного проекта, ожидание: 8 успехов и 8 уникальных ревизий.

---

### API-001 — Ошибка валидации входа возвращается как 500
**Severity:** MEDIUM · **Status:** CONFIRMED · **Layer:** API/Cross-layer
**Location:** `internal/application/stair/service.go:159-164`, `internal/transport/http/handler.go:40-58` (`mapStairError`), `internal/domain/engineering/stair.go:219-229`

**Problem:** `buildConfiguration` → `StairConfiguration.Validate()` возвращает обычные `fmt.Errorf`. `mapStairError` маппит в 422 только `*solver.InputError`; всё остальное → 500. Итог: часть невалидного ввода честно отдаёт `200 + valid:false`, а часть — 500.

**Evidence (прогон через реальный пайплайн):**
```
unknown_flight  (flight:"diagonal")  → 500 internal_error   (0ms)
approach_500    (approach_space_mm:500) → 500 internal_error
negative_width                        → 200 valid:false blocking (GEO-WIDTH)
step_2700mm                           → 200 valid:false blocking (GEO-TREAD-POSITIVE)
```
**Reproduction:** `POST /api/v1/stairs:calculate` c валидной сессией+CSRF и любым из двух тел выше.
**Impact:** ложные 5xx в Sentry/алертах, дезориентация клиента (не понятно, чинить ввод или ждать), потенциальный расчёт лимита ошибок.
**Root Cause:** два разных механизма ввода-ошибок (`solver.InputError` vs domain `error`) без общего такта.
**Fix:** обернуть доменные ошибки в `InputError` (или добавить `errors.As`-маппинг по префиксу), дополнительно валидировать enum `flight` на границе.
**Regression Test:** табличный тест «невалидный вход → 422/200, но не 500» на calculate/validate/optimize/assistant.

---

### DOM-001 — Enum `flight` не валидируется ни на одном слое
**Severity:** MEDIUM · **Status:** CONFIRMED · **Layer:** Domain/Geometry
**Location:** `internal/domain/engineering/stair.go:170-279` (нет `FlightType.Valid()`), `internal/application/stair/service.go:325-380` (`default:` → straight), `frontend/shared/src/types.ts:95` (`flight: string`), `docs/openapi/swagger.yaml` (enum отсутствует)

**Problem:** `Validate()` проверяет `Flight == ""`, но не проверяет принадлежность множеству `{straight,l_shape,u_shape,spiral}`. В `checkedSolve` ветка `default:` трактует **любой** неизвестный тип как прямой марш.

**Evidence:** проба `StairConfiguration{Flight:"diagonal"}.Validate()` → `err == nil`; API → 500 (см. API-001).
**Impact:** (а) 500 вместо 422; (б) если геометрия не споткнётся — пользователь получит **прямую лестницу вместо заказанной конфигурации** (тихая порча расчёта); (в) фронт типизирует `flight: string`, спек не описывает enum — защиты нет ни на одном слое.
**Fix:** `FlightType.Valid()` + вызов в `Validate()`; заменить `default:` на явную ошибку; union-тип на фронте; enum в OpenAPI.
**Regression Test:** `Validate` на 4 валидных + 3 невалидных значениях; API-тест `flight:"diagonal"` → 422.

---

### DB-002 — Применение платёжного события не в транзакции
**Severity:** MEDIUM · **Status:** CONFIRMED · **Layer:** Database/Consistency
**Location:** `internal/application/payments/service.go:173-188`

**Problem:** `UpdateStatus` и `AppendEvent` — два независимых вызова репозитория, `WithTx` (`internal/infrastructure/database/tx.go`) не используется.
**Reproduction:** сбой БД/контекста между вызовами → интент `paid`, а журнала `payment_events` нет. Повтор Stripe приходит как «дубликат» → reconcile видит целевой статус → **событие не журналируется никогда** (S-145 №3 self-healing закрывает статус, но не аудит-трейл).
**Impact:** дыра в финансовом аудит-трейте; при споре «была ли оплата» доказательства нет.
**Fix:** обернуть пару вызовов в `WithTx` (или `Repository.ApplyEventTx`).
**Regression Test:** репо-фейк, роняющий `AppendEvent`, проверяет откат статуса.

---

### DOC-001 — Документация аутентификации описывает несуществующие механизмы
**Severity:** MEDIUM · **Status:** CONFIRMED · **Layer:** Documentation
**Location:** `docs/09_API/04_AUTHENTICATION.md:50-60` («Supported Methods: OAuth 2.1, OpenID Connect, JWT Access Token, Refresh Token, Service Token, mTLS», «Token Structure»)

**Problem:** в коде есть только opaque-сессии (crypto/rand) + SSO/OIDC-вход + admin API-ключи. OAuth 2.1, refresh-токены, service token, JWT **не реализованы**. Пакет `internal/infrastructure/jwt` (74+ строки, GenerateTokenPair/RefreshTokens) — **мёртвый код**: ни одного импорта вне самого пакета и его тестов.
**Impact:** интегратор/ревьюер, читая доки, будет искать refresh-flow, revocation, mTLS — их нет. Пакет JWT создаёт ложное впечатление, что JWT-механика работает (и что её нужно искать в коде).
**Fix:** переписать раздел по факту; удалить jwt-пакет + зависимость `golang-jwt/jwt/v4` (v4 устарел — актуальна v5) или пометить как explicitly-disabled с TODO.
**Regression Test:** CI-скрипт «docs/API claims ⊆ implemented» (ручной ревью-лист).

---

### DOC-002 — OpenAPI-спека — карта маршрутов, а не контракт
**Severity:** MEDIUM · **Status:** CONFIRMED · **Layer:** Documentation/API
**Location:** `docs/openapi/swagger.yaml` (2893 строки), генератор `hack/gen_swagger.py`

**Problem:** спека генерируется скриптом-сканером `mux.Handle(...)`: в ней есть пути/методы/коды, но почти нет схем запросов (`properties:` — 2 на весь файл), нет enum для `flight`, нет `Content-Type`-контрактов для большинства POST. Ключевой параметр продукта (`flight`) в спеке **не встречается ни разу**.
**Impact:** клиент не сгенерирует валидный SDK; интегратор не знает контракт; расхождение с фактической валидацией (API-001) невозможно заметить по спеке.
**Fix:** описать requestBody для 4 основных эндпоинтов (calculate/validate/optimize/assistant) с enum и required; генератор оставить как sync-инструмент путей, схемы — вручную/из аннотаций.

---

### TEST-001 — Coverage-гейт обходится снятием переменной БД
**Severity:** LOW · **Status:** CONFIRMED · **Layer:** CI/Testing
**Location:** `scripts/coverage-check.sh:26-33`

**Problem:** если `STAIR_TEST_DATABASE_URL` не задан, пакет `internal/infrastructure/database` **исключается из подсчёта**, и гейт 85% проходит на заниженной базе. В CI переменная задана (гейт честный), но локальный «зелёный» результат означает меньше, чем кажется, и скрипт не предупреждает об исключении.
**Fix:** печатать явное предупреждение с исключёнными пакетами и требовать `--allow-db-skip` флагом; в CI — запретить skip.
**Regression Test:** `scripts/coverage-check.sh` на пустом профиле без БД → не должен молча проходить.

---

### INF-001 — Мусор в репозитории и пустые пакеты-заготовки
**Severity:** INFO (уточнено 2026-09-24 после проверки) · **Status:** CONFIRMED · **Layer:** Repo hygiene
**Location:** `scripts/debug_spiral.go`, `scripts/debug_spiral2.go`, `api/.gitkeep`, `pkg/.gitkeep`, `bin/stair-api`

**Problem:** отладочные скрипты помечены `//go:build ignore` (то есть это явные dev-утилиты, а не случайный мусор — вина снижена с LOW до INFO); `api/` и `pkg/` пустые; в рабочем дереве лежит 26 МБ бинарник `bin/stair-api` (в git не отслеживается).
**Fix:** перенести debug-скрипты в `hack/` или удалить; удалить пустые `api/`, `pkg/`; `bin/` в `.gitignore`.

---

## 4. Attack Matrix (перепроверено в этом аудите)

| Атака | Entry | Preconditions | Результат | Confirmed |
|---|---|---|---|---|
| IDOR чужого проекта | `GET /projects/{id}` | своя сессия | membership+tenant в SQL → 404/403 | ✅ защищено |
| IDOR чужой памяти | `POST /assistant/*` с чужим project_id | своя сессия | S-142 member-check → 403 | ✅ защищено |
| RAG другого тенанта | `POST /assistant/*` | валидный tenant | `tenant_id=$1 OR NULL` → изоляция | ✅ защищено |
| SSRF IMDS (169.254.169.254) | `POST /integrations/endpoints` → webhook | прод-конфиг | S-151: link-local блок всегда | ✅ защищено |
| Webhook replay | `POST /payments/stripe/webhook` | перехваченное событие | dedup+reconcile+guard → статус не откатывается | ✅ защищено |
| Подмена цены | `POST /checkout` | `amount_minor` в теле | S-150: серверный каталог | ✅ защищено |
| XSS | admin/store | ввод пользователя | `dangerouslySetInnerHTML` отсутствует | ✅ не найдено |
| CSRF на 4 POST | calculate/validate/optimize/assistant | браузер | S-144 `authMutating` | ✅ защищено |
| Ферма аккаунтов → LLM-бюджет | регистрация + Ask | дефолтный env | бюджет default-off → **открыто** (B-1) | ⚠️ открыто |
| Enum-мусор → 500 | `flight:"diagonal"` | любая сессия | 500 (API-001) | ❌ дефект |

## 5. Invariant Matrix

| Инвариант | Где | Обходим? | Тест есть? |
|---|---|---|---|
| Сумма ступеней × высота = подъём | solver.Solve (пересчёт h=H/n) | нет | да |
| Шаг ступени 150–200 мм | constraint GEO_STEP_HEIGHT | нет (проверено пробой 2700 мм → блок) | да |
| Шаг комфорта 600–640 | solver.Solve | нет (проба 700 → GEO-COMFORT-STEP) | да |
| Проступь > 0 | solver.Solve b=s−2h | нет (верхняя граница шага ловит) | да |
| R(спираль) > W | StairConfiguration.Validate | нет | да |
| **enum Flight** | **—** | **да (нет проверки)** | **нет** |
| **ревизия конфигурации уникальна под конкуренцией** | UNIQUE-индекс | **да (гонка → 5xx)** | **нет** |
| **статус+журнал платежа атомарны** | **—** | **да (нет tx)** | **нет** |
| изоляция tenant (projects) | SQL projectScope | нет | да |
| цена checkout из каталога | payments.Catalog | нет (S-150) | да |

## 6. API Security Matrix (выборка)

| Endpoint | Auth | Authz | Validation | Rate limit | IDOR | Результат |
|---|---|---|---|---|---|---|
| POST /stairs:calculate | session | — | частичная (enum → 500) | 200/мин user | — | ⚠️ |
| POST /assistant/{kind} | session+CSRF | member-check проекта | ok | 200/мин user | закрыт S-142 | ✅ |
| DELETE /assistant/memory | session+CSRF | member-check | ok | общий | закрыт | ✅ |
| POST /projects/{id}/checkout | session+CSRF | GetProject membership | tier enum | общий | закрыт | ✅ |
| POST /payments/stripe/webhook | подпись | — | len+HMAC | нет (!) | — | ⚠️ replay-safe по dedup, но без rate limit |
| POST /auth/register | публичный | — | email/пароль | 5/мин IP | — | ⚠️ без верификации email (B-2) |
| GET /metrics, /swagger | InternalOnly | CIDR (env) | — | — | — | ⚠️ E10-конфиг |
| GET /admin/users | session | permission users.list | — | — | — | ✅ |

## 7. Database Integrity Matrix (выборка)

| Entity | PK | FK | Unique | Check | Tx | Race |
|---|---|---|---|---|---|---|
| projects | id | tenant_id | — | status | create в tx | ок |
| project_members | — | project/user | (project,user) | role enum | — | ок |
| stair_configurations | id | project | (project,revision) | — | **tx без блокировки** | **DB-001** |
| payment_intents | id | tenant/project | provider_checkout | status | **нет tx для apply** | **DB-002** |
| conversation_messages | id | tenant/project | — | role | — | ок |
| ai_corpus_chunks | id | tenant nullable | (hash,tenant) | — | — | ок |

## 8. Test Audit

- 61 Go-пакет / 2532 фронт-теста зелёные, `-race` чистый — база крепкая.
- **Пробелы:** нет тестов на (а) конкурентную запись конфигурации, (б) неизвестный `flight`, (в) транзакционность платёжного apply, (г) «вход-ошибка ≠ 500» для 4 эндпоинтов, (д) property/fuzz на solver (только примеры: `generated_table_test.go`).
- **Test-the-tests (мутация):** `StairConfiguration.Validate()` без enum-проверки **проходит весь существующий сьют** — покрытие этого инварианта нулевое, при том что тесты `stair_extra_test.go` выглядят плотными. Аналогично: удаление `FOR UPDATE`-эквивалента не ломает ни один тест (DB-001 не ловится).
- Fuzz/property: готовых fuzz-тестов в проекте нет (только `bench_*`).

## 9. Documentation Audit

| Заявление | Факт |
|---|---|
| auth: OAuth 2.1 / JWT / Refresh / mTLS | ❌ не реализовано (DOC-001) |
| OpenAPI описывает API | ⚠️ только карта путей (DOC-002) |
| GraphQL 15 операций | ❌ 5 из 15, в прод не зарегистрирован (известно из S-131, docs помечены EXPERIMENTAL) |
| `STAIR_AI_DAILY_LLM_BUDGET` защищает расход | ⚠️ код есть, дефолт выключен (B-1) |
| CI = защита | ⚠️ нет govulncheck, нет npm audit, нет fuzz; coverage-гейт с БД-скипом (TEST-001) |

## 10. Master Findings Table

| ID | Sev | Status | Layer | Category | Finding | Evidence | Exploit | Impact |
|---|---|---|---|---|---|---|---|---|
| DB-001 | HIGH | CONFIRMED | DB | Concurrency | lost update ревизии → unique violation | 3/3 прогона, 4→2/1 успехов | правый клиент (без злого умысла) | потеря сохранения, 5xx |
| API-001 | MEDIUM | CONFIRMED | API | Error mapping | домен-валидация → 500 | пробы flight/approach | любой клиент | ложные 5xx, дезориентация |
| DOM-001 | MEDIUM | CONFIRMED | Domain | Enum/invariant | нет `Flight.Valid()`, `default:`=straight | проба err==nil; 500 | любой клиент | тихая порча расчёта |
| DB-002 | MEDIUM | CONFIRMED | DB | Transaction | платёж apply без tx | код + путь retry | сбой БД | потеря фин-аудита |
| DOC-001 | MEDIUM | CONFIRMED | Docs | Drift | auth-доки описывают OAuth/JWT/refresh | пакет jwt без импортов | — | обман интегратора |
| DOC-002 | MEDIUM | CONFIRMED | Docs/API | Contract | спек без схем/enum | 0 вхождений `flight` | — | нет SDK-контракта |
| TEST-001 | LOW | CONFIRMED | CI | Gate bypass | coverage-скипает БД-пакет | скрипт | локально | ложное «зелёное» |
| INF-001 | LOW | CONFIRMED | Repo | Hygiene | debug-скрипты, пустые api/pkg | ls | — | шум, ловушки |
| B-1 | MEDIUM | OPEN | Ops | Config | LLM-бюджет default-off | main.go | ферма акков | $ расход |
| B-2 | MEDIUM | OPEN | Auth | Abuse | нет верификации email | auth.go | ферма акков | обход лимитов |

## 11. Final Status (на момент аудита — до исправлений; см. раздел 12)

**VERIFIED (силами прогонов):** сборка/vet/gofmt/race-сьют 61/61; фронт 432+2100; math-инварианты solver (h, b, шаг, R>W) держатся на границах; tenant-изоляция projects/RAG/memory; CSRF/SSRF/IDOR/цены закрыты (S-141…S-151); graceful shutdown есть.

**BROKEN (воспроизведено):** DB-001 (гонка ревизий), API-001 (500 на невалидном вводе), DOM-001 (enum не валидируется), DB-002 (платёж без транзакции).

**AT RISK:** LLM-бюджет/email-верификация (нужен человек); покрытие падает при локальном скипе БД; live-LLM поведение не проверено (safety-eval только на детерминированном пути).

**UNPROVEN:** численная устойчивость под экстремальными входами (нет fuzz); поведение при падении БД в середине платёжной операции; реальная производительность под k6 в этой ветке (сценарий один); корректность L/U-геометрии для TurnWinder на нестандартных данных.

**NOT AUDITED:** Terraform-apply к AWS, реальный EKS, staging-топология, S3/файловое хранилище в проде, нагрузочное тестирование (k6 не запускался — нужен стенд), аудит фронт-бандла на утечки, pen-test внешнего периметра.

## 12. Исправления (проход «бери в работу», 2026-09-24)

| ID | Статус | Коммит |
|---|---|---|
| DB-001 | ✅ FIXED — `SELECT ... FOR UPDATE` по строке проекта в той же транзакции + конкурентный тест (8 вставок, 3 прогона) | `d5a9242` |
| **DB-003 (новый, найден при фиксе DB-001)** | ✅ FIXED — up-010: колонка `revision` добавляется без default и заполняется `row_number()` по проекту. Прежний `ADD COLUMN ... NOT NULL DEFAULT 1` проставлял 1 всем строкам → проекты с ≥2 конфигурациями ломали `CREATE UNIQUE INDEX` (тот самый «dirty 10» из S-137) и делали rollback невозможным | `fc35b45` |
| API-001 | ✅ FIXED — `configInputError` получил generic-fallback (любая доменная ошибка → blocking-валидация, а не 500); guard толщины перед геометрией (толщина 0 больше не роняет конвейер) | `18b3996` |
| DOM-001 | ✅ FIXED — `FlightType.Valid()` + проверка в `Validate()`; `default:` в `checkedSolve` больше не трактует мусор как прямой марш; union-тип `FlightType` во фронтовом контракте | `5da3054` |
| DB-002 | ✅ FIXED — `payments.EventApplier` + `ApplyVerifiedEventTx` в одной транзакции; тест доказывает откат статуса при падении вставки в журнал | `17dab28` |
| DOC-001 | ✅ FIXED — `04_AUTHENTICATION.md` переписан по факту (opaque-сессии + OIDC + API-ключи); OAuth/JWT/refresh/mTLS помечены как нереализованные | `b96afd7` |
| TEST-001 | ✅ FIXED — coverage-гейт падает без `STAIR_TEST_DATABASE_URL`, скип только через явный `ALLOW_DB_SKIP=1` | `bdf3d8e` |
| DOC-002 | ⏳ PARTIAL — спек по-прежнему генерируется скриптом-сканером; схемы запросов и enum требуют расширения генератора (отдельная задача, иначе ручные правки затираются) | — |
| INF-001 | ℹ️ уточнён до INFO: debug-скрипты помечены `//go:build ignore` (явные dev-утилиты, не мусор) | — |
| **DOM-002 (новый, найден при фиксе API-001)** | 🔴 OPEN — `buildConfiguration` (`internal/application/stair/service.go:538`) оставляет `StepCount: 1`, поэтому условие `StepCount > 1` в `StairConfiguration.Validate()` (`internal/domain/engineering/stair.go:233`) никогда не выполняется: **валидация L/U-маршей (ширина площадки, lower_step_count, поворотные ступени) недостижима из API**. Пробный фикс (`n = round(H/h)`) меняет приоритет валидации и ломает 3 теста с «богатыми» числовыми сообщениями — нужен отдельный проход: сначала выровнять тексты доменных ошибок с constraint-слоем, потом включать инвариант | — |

---

## 13. Final Conclusion

Система существенно крепче среднего (состояние на момент аудита; исправления — раздел 12): 61 пакет под race, 2532 фронт-теста, нормативный движок с реальными инвариантами, закрытые IDOR/SSRF/CSRF/ценовые/replay-атаки. Но аудит нашёл **четыре функциональных дефекта с воспроизведением** (гонка ревизий конфигурации — HIGH; 500 на невалидном вводе, непроверяемый enum `flight`, нет транзакции в платёжном apply — MEDIUM) и **две документационные лжи** (auth-механизмы, которые не существуют; OpenAPI как карта маршрутов). Все четыре дефекта находятся на границах слоёв — ровно там, где «фронт проверил, API нет, домен молчит, БД падает».
