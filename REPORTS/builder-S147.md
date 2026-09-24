# S-147 — Лимитер EXPIRE-DEL + WS-лимиты (S-141 №5, №10, день 5)

## Вердикт

**S-147 ЗАКРЫТ.** Обе находки устранены: EXPIRE-сбой лечится DEL (нет вечного 429),
fail-open виден в метриках; WS-коннекты под cap'ами per-user/per-IP, входящий флуд
дропает соединение. Тесты-ловушки аудита написаны и зелёные (×5 повторов + race).

## №5 — Fail-open + проглоченный EXPIRE (HIGH, CWE-307/399)

Файлы: `internal/transport/http/rate_limiter_redis.go`,
`internal/infrastructure/security/ratelimit.go`,
`internal/transport/http/ratelimiter_redis_test.go`. Коммит `b703831`.

- EXPIRE-ошибка больше не проглатывается: ключ удаляется (`Del`), окно начинается
  заново — вечного 429 нет. Удаление видно в `rate_limit_expire_cleanup_total`.
- Fail-open (INCR err → allow) оставлен осознанно (инвариант EDR-0014 §4.2: лимитер
  не роняет аутентификацию), но теперь с метрикой `rate_limit_fail_open_total{reason}`
  (обе — в существующем `security.RateLimitRegistry`, уже wired в `/metrics`).
- `*redis.Client` заменён узким интерфейсом `redisCounter` (Incr/Expire/Del) ради
  fault-инъекции без новых зависимостей.
- Тесты: `TestRedisLimiterExpireFailureDeletesKey` (INCR ok + EXPIRE err → DEL=1),
  `TestRedisLimiterRedisDownFailOpen`, `TestRedisLimiterNormalCounting`.

## №10 — WS без лимитов (MEDIUM, CWE-400)

Файлы: `internal/transport/websocket/websocket.go`,
`internal/transport/websocket/websocket_limits_test.go`. Коммит `2d852cf`.

- Hub: `maxPerUser=32` / `maxPerIP=128` (константы + `HubOption`, `NewHub()` обратно
  совместим). Проверка в ветке `register` цикла `Run`: сверх cap — закрыть `send` +
  `conn`, не регистрировать. TOCTOU проверка→регистрация документирован (cap жмёт
  порядки флуда, не семафор).
- `Client.ip` из `RemoteAddr` (только host; XFF не читаем — симметрично EDR-0014/S-112);
  `ClientCountByIP` зеркалит `ClientCountByUser`.
- readPump: `allowMessage()` — max 200 входящих/мин; превышение → break (defer:
  unregister + close). Close-кадры из `Run`/`readPump` НЕ пишутся осознанно:
  единственный writer — `writePump` (конкурентный `WriteMessage` — паника gorilla,
  поймана тестами при `-count=3`, исправлено).
- Тесты: `TestHubConnCapPerUser`, `TestHubConnCapPerIP`, `TestClientMessageFlood`,
  `TestHandleWebSocketConnCapReject` (live: httptest + Dialer, 3-й upgrade закрыт,
  в хабе 2). Стабильность: `-count=5` green, race green.

## Гейты

- `gofmt` чист, `go vet` 0 по затронутым пакетам.
- Пакеты `transport/websocket`, `transport/http` — ok, в т.ч. под `-race`.
- Полный `go test -race -p 1 ./...` — в фоне, результат дописать координатору.

## Остаток

- Алерт на `rate_limit_fail_open_total` (Prometheus) — инфра, день 7 / S-149 scope.
- Роутер `/ws` оставлен без `limitRate`-обёртки осознанно: cap в хабе — прямой фикс;
  HTTP-rate на handshake — опциональный follow-up.
