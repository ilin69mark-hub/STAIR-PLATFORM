# S-139: Login timing-enumeration — верификация (код уже в main)

**Вердикт:** DONE — писать код не потребовалось: фикс уже внедрён коммитом `beee671` (S-113-fix, влит в main через PR #52) и подтверждён прогоном. Карта S-139 закрыта верификацией.

## Что найдено в main (коммит beee671)

- `internal/application/auth/service.go`: `dummyBcryptCompare` (sync.Once-ленИвый хеш cost 10 + fallback-хеш) вызывается на ветке «неизвестный email» (`Login`, ErrNotFound) — тайминг выровнен с веткой неверного пароля; bcrypt-сравнение выполняется ДО проверки статуса, все три ветки возвращают единый `ErrInvalidCreds` (disabled-аккаунт неотличим снаружи).
- `internal/application/auth/timing_validation_test.go`: `TestLoginTimingEqualized` (нижняя граница ≥10 мс доказывает выполнение bcrypt; верхняя ≤ 5×known + 20 мс — ветки сравнялись) + бенчмарки `BenchmarkLoginTiming{ Known, Unknown }Email`.

## Верификация (2026-09-23, self-execute @swarm)

- `go test -race -count=1 -run TestLoginTimingEqualized -v ./internal/application/auth/` — **PASS (20.59s)**: unknown-email идёт через dummy-bcrypt (~100+ мс на 5 сэмплах), утечки нет.
- Полный пакет `./internal/application/auth/` (race) — **ok (86.26s)**.
- Полный сьют 60/60 гонялся для S-140 в этой же сессии на том же HEAD-диапазоне — регрессий нет (пакет auth входит в прогон).

## Почему карта была TODO

Фикс сделан внутри S-113-серии до заведения нумерации S-13x: борд-трекер S-139 создан позже по тексту S-113-отчёта, а сам код уже лежал в main. Урок: перед стартом задачи проверять `git log -S` на предмет уже внедрённых фиксов.
