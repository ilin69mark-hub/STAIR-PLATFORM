# S-132a: WS subscribe-протокол (backend) — комнаты ожили

**Вердикт:** DONE — клиентские subscribe/unsubscribe + ack, allowlist комнат, 7 новых тестов зелены, полный прогон 58/58, запушено в `dev/swarm/ws-s115-s116`.

## Что сделано (`internal/transport/websocket/`)
- `websocket.go`: типы `subscribe/unsubscribe` (client→server) + `subscribed/unsubscribed` (ack); `validPipelineRoom` — только `pipeline:<uuid>` (UUID-regex, остальное игнор); `handleClientMessage` выделен из readPump (тестируемость без conn); неизвестные типы — лог; server-события по-прежнему игнорятся от клиентов.
- Безопасность: знание UUID = capability (S-132a); per-config authorization — follow-up S-132c (записан на борд). Role-гейт /ws/admin (S-119) без изменений.
- `subscribe_test.go` (новый, 7 тестов): валидация комнат, join+ack, игнор мусора (0 комнат), broadcast только подписчику, leave+ack, игнор server-типов, ping→pong.

## Гейты
- Новые тесты 7/7 PASS (-race); `go vet` + `gofmt` чисто.
- Полный: `go test -race -p 1 -count=1 ./...` на свежей БД — **58 ok, 0 FAIL**.

## Остаток S-132 (следующие)
- S-132b: фронтенд shared WS-клиент + wiring admin/store (переподключение, JoinRoom UI).
- S-132c: per-config authorization подписок (проверка владения конфигом).

## Счётчик CI
Задача 19/20 после PR #51.
