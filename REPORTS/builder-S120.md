# S-120: Rate-limit за L7-прокси — XFF только от trusted-CIDR

**Вердикт:** DONE — реализовано, 3 новых регресс-теста зеленые, полный локальный прогон 58/58 (race+БД), lint 0, gofmt чист, закоммичено в `dev/swarm/ws-s115-s116`.

## Что сделано
- `internal/transport/http/middleware_auth.go`
  - `Config.TrustedProxies string` — CSV из CIDR/одиночных IP (`STAIR_TRUSTED_PROXIES`).
  - `parseTrustedProxies(csv)` — разбор CIDR + одиночных IP (v4→/32, v6→/128); невалид пропускается; пусто → nil (XFF не доверяется никому).
  - `rateLimitIP(r, trusted)` — ключ per-IP лимитера: XFF (самый левый) только если прямой пир (RemoteAddr) входит в trusted; иначе RemoteAddr (поведение S-112); битый XFF → fallback RemoteAddr.
  - `limitRate(l, trusted, next)` — новая сигнатура, все 8 вызовов обновлены.
- `internal/transport/http/router.go` — `trusted := parseTrustedProxies(cfg.TrustedProxies)`; проброшен в register/login/quote/validate/orders + 3 SSO-маршрута (S-109).
- `cmd/api/main.go` — `TrustedProxies: os.Getenv("STAIR_TRUSTED_PROXIES")`; gofmt-фикс блока Config.
- `internal/transport/http/trusted_proxies_test.go` (новый) — `TestParseTrustedProxies`, `TestRateLimitIP`, `TestLimitRateTrustedProxy` (per-client бюджеты за одним прокси + анти-спуфинг при прямом доступе).

## Гейты
- `go test -run 'TestParseTrustedProxies|TestRateLimitIP|TestLimitRateTrustedProxy' -race` — 3/3 PASS.
- Полный: `STAIR_TEST_DATABASE_URL=... go test -race -p 1 -count=1 ./...` — **58 ok, 0 FAIL** (после сброса БД, см. ниже).
- `go vet` — чисто; `golangci-lint run` на затронутых пакетах — 0 issues; `gofmt -l` — чисто.

## Замечание по окружению (не S-120)
- Локальный `stair-test-pg`: пароль `stair` (в CI — `testpassword`).
- Два параллельных прогона сьюта против одной БД оставили `Dirty database version 17` → лечится `DROP/CREATE DATABASE stair_test` через `docker exec stair-test-pg psql`. Полные прогоны — только серийно.

## Счётчик CI
Задача 9/20 после PR #51 (следующий CI-прогон после 20-й).
