# S-121: nginx security-заголовки + WS-проксирование

**Вердикт:** DONE — оба конфига обновлены, `nginx -t` syntax ok, живые заголовки проверены curl, закоммичено в `dev/swarm/ws-s115-s116`.

## Что сделано
- `deployments/nginx/store.conf` / `admin.conf`
  - `server_tokens off` — `Server: nginx` без версии.
  - CSP (`always`): `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img/connect` + `data: blob:`, `connect-src 'self' ws: wss:`, `frame-ancestors 'self'` (store) / `'none'` (admin), `base-uri/form-action 'self'`.
  - `X-Frame-Options: SAMEORIGIN` (store) / `DENY` (admin, кликджекинг привилегированного UI), `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` — все с `always` (есть и на 502 прокси).
  - `location /ws` — прокси на `api:8080` с `Upgrade/Connection`, `proxy_http_version 1.1`, `proxy_read_timeout 86400`; префикс покрывает `/ws` и `/ws/admin` (S-119/S-132).
  - `/assets/`: продублированы nosniff+Referrer-Policy (квирк nginx — `add_header` в location перекрывает server-уровень), коммент в файле.
- `deployments/docker-compose.yml` — api env `STAIR_TRUSTED_PROXIES: "172.16.0.0/12"` (связка S-120: nginx XFF доверяется из Docker-bridge).

## Гейты (deploy-зона, Go-код не тронут — `git diff --name-only` только conf/yml/md)
- `nginx:1.27-alpine nginx -t` (тот же базовый образ, что в Dockerfile) — syntax ok оба конфига (тест с `--add-host api:127.0.0.1`, т.к. upstream `api` резолвится только в compose-сети).
- Живой контейнер + `curl -I`: `/` 200 — все 4 заголовка; `/api/...` 502 — заголовки на месте (`always` работает); `Server: nginx` без версии; admin — `DENY` + `frame-ancestors 'none'`.
- `yaml.safe_load` compose — ok, значение на месте.
- Полный `go test` пропущен обоснованно: Go-файлы не изменены (DoD deploy-зоны — синтаксис + живые заголовки).

## Счётчик CI
Задача 10/20 после PR #51 (следующий CI-прогон после 20-й).
