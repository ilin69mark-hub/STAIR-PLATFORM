# builder — S-115: WS same-origin дефолт достижим через пустой env

**Задача:** S-115 [P3] (SHOULD из ревью PR #50) — `envStringSlice` при пустых
`STAIR_WS_ORIGINS`/`STAIR_CORS_ORIGINS` фолбэчил на `[http://localhost:3000]`
→ same-host прод (nginx, фронт+API на одном хосте) отклонял собственный
легитимный WS. Безопасный дефолт (same-origin-only) жил только в unit-тестах.

**Выполнено:** координатором (self-execute; Task-dispatch отменён 3-й раз в
сессии — fallback по протоколу §9.1; отменённый Task успел оставить рабочий
diff, проверен и закоммичен).

## Изменения

- `cmd/api/main.go` — новая `wsAllowedOrigins(corsOrigins)`:
  - `STAIR_WS_ORIGINS` задан явно → его список (exact/`"*"`/`"*.domain"`);
  - иначе `STAIR_CORS_ORIGINS` задан явно → CORS-список (fallback, S-112);
  - иначе `nil` → OriginChecker = «только same-origin» (безопасный дефолт).
  - `wsOrigins := wsAllowedOrigins(corsOrigins)` вместо
    `envStringSlice("STAIR_WS_ORIGINS", corsOrigins)`.
- `cmd/api/ws_origins_test.go` (новый) — 2 теста, 7 сабтестов:
  `TestWSAllowedOrigins` (both-unset→nil, CORS-fallback, WS-wins,
  effectively-empty→nil) + `TestWSOriginsEnvToOriginPolicy` (end-to-end через
  OriginChecker: same-origin allowed / cross-origin rejected / port-mismatch
  rejected / exact+wildcard / non-browser без Origin allowed).
- `.env.example` — документация дефолта + EOF newline (был без неё).

## Гейты (локально)

| Гейт | Результат |
|---|---|
| gofmt -l | пусто |
| go vet ./... | 0 |
| go build ./... | ok |
| golangci-lint run | 0 issues |
| go test -count=1 (новые тесты) | 7/7 PASS |
| **Полный прогон** `go test -race -p 1 ./...` + БД (postgres:16) | **58/58 ok, 0 FAIL** |

## Решения

- `envStringSlice` НЕ менялся глобально (общий хелпер, используется CORS и
  другими конфигами) — поведение «пусто = дефолт» для CORS сохранено
  (HTTP-CORS не трогаем), WS получил отдельную функцию с nil-семантикой.
- Same-origin дефолт теперь реально достижим: пустые оба env → nil →
  OriginChecker разрешает браузерный same-origin, отклоняет кросс-ориджин.

**Вердикт:** S-115 реализован — пустые WS/CORS env дают same-origin-only
вместо localhost-фолбэка; 58/58 локально (race+БД), lint 0.

**Длительность:** ~15 мин (включая 2 отменённых Task-диспатча).

**Нюансы:** Task-dispatch отменяется в этой сессии систематически (3/3
попытки) — координатор перешёл на self-execute.

**Рекомендации:** S-116 (admin-cookie на WS) — следующая карточка; после неё
счётчик задач = 2/20, CI не требуется.