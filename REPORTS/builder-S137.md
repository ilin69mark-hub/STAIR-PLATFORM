# S-137: Test-hygiene — детерминированный повторный прогон на shared stair_test

**Вердикт:** DONE — корень найден и воспроизведён (dirty-10), фикс только в тестовом хелпере `TestMain` (прод-миграции не тронуты), полный сьют зелёный **дважды подряд без DROP/CREATE между прогонами**. Коммит в `dev/swarm/ws-s132b-client` (не запушен).

## Корень (воспроизведено)
- `stair_configurations` создаётся в **000001** (init), а **000010** добавляет `revision INTEGER NOT NULL DEFAULT 1` + `CREATE UNIQUE INDEX (project_id, revision)` (семантика: down-010 легально удаляет колонку/индекс).
- Внутри пакета `internal/infrastructure/database` файлы исполняются по алфавиту: `migrate_extra_test.go` идёт **до** `migrate_test.go`. Его `TestMigrateToVersionIntegration` делает `MigrateToVersion(1)` — down до версии 1, при котором таблицы 000001 (включая `stair_configurations`) **не дропаются, а строки выживают**. Затем `Migrate up` заново применяет 2..24, и на 000010 `CREATE UNIQUE INDEX (project_id, revision)` падает: всем выжившим строкам проставляется `revision=1`, и любой проект с ≥2 конфигурациями даёт duplicate → golang-migrate оставляет `dirty version 10`.
- Свежая БД проходила: в прогоне №1 к моменту `TestMigrateToVersionIntegration` конфигураций ещё нет/мало; строки-дубликаты накапливаются в repo-тестах, которые идут **после** (`order/project/sso/testimonial`) и переживают прогон №1 окончанием сьта → повторный прогон №2 красный.
- Воспроизведение (до фикса): DROP/CREATE → прогон пакета ok → повторный прогон `Dirty database version 10` (виновник — `TestMigrateToVersionIntegration`; после — каскад из-за устоявшегося dirty); третий прогон: 61 FAIL/0 PASS. Улика в БД: проект `9719dfc9` с 2 строками `stair_configurations`, колонка `revision` отсутствует (транзакция up-010 откатилась целиком).

## Фикс (только тест-хелпер, прод-миграции НЕ тронуты)
`internal/infrastructure/database/testmain_test.go` — новый `TestMain` (пакет database):
1. **TestMain-хук**: при заданном `STAIR_TEST_DATABASE_URL` до первого теста выполняется полный детерминированный reset — `down до 0` + `up до latest` тем же мигратором, что и прод (`newMigrateInstance`/`schema_migrations`); строки и dirty-флаги предыдущих прогонов удаляются, схема всегда свежая. Дополнительно `SELECT to_regclass(...)` — свежая БД (DROP/CREATE) без `schema_migrations` пропускает down-фазу (иначе golang-migrate считает отсутствие таблицы версий ошибкой).
2. **Serial-guard (advisory lock, `pg_try_advisory_lock(0x53743133)`)**: lock удерживается **на весь прогон** (закрытие соединения после `m.Run()`, а не внутри reset) — параллельные `go test ./...` против shared БД получают понятную ошибку вместо порчи данных; после процесса lock снимается сам с закрытием соединения.
3. **Force-unlock dirty в хелпере**: если прошлый прогон оборвался на середине миграции — `m.Force(фактическая_dirty_версия)` (был риск заforce-нуть на latest: версии выше dirty никогда не применялись). Если после force `Down()` всё равно не прошёл (полусобранная схема) — страховка `DROP SCHEMA public CASCADE; CREATE SCHEMA public` + свежий up-инстанс (кэш версий старого после DROP устарел). Это только тестовая БД.

## Гейты
- Пакет `database`: DROP/CREATE → прогон ok → 2-й прогон ok → 3-й прогон ok (без сбросов между).
- Полный сьют `STAIR_TEST_DATABASE_URL=postgres://stair:stair@127.0.0.1:5432/stair_test?sslmode=disable go test -race -p 1 -count=1 -timeout 25m ./...` (timeout 25m — конвенция CI/репо, дефолтный 10m режет `engine/variation` под загрузкой машины ~30 load):
  - **Прогон 1: 58/58 ok, rc=0.**
  - **Прогон 2 (без DROP/CREATE между): 58/58 ok, rc=0.**
- БД после прогонов: `schema_migrations` version=24, dirty=false.
- `go vet ./...` — 0 замечаний; `go build ./internal/... ./cmd/...` — ok; golangci-lint `./internal/infrastructure/database/...` — 0 issues; gofmt — изменённые файлы чистые.

## Побочные наблюдения
- Зависания/таймаута нет: `TestGeneratedRoomFit500` (`engine/variation`, race) — тяжёлый CPU-тест, на этой машине 757s; в полном прогоне с дефолтным `-timeout 10m` краснел, с `-timeout 25m` — зелёный. Нагрузка машины внешняя (node-сборка чужого проекта + 3 opencode, load ~30).
- Пре-существующее отклонение вне scope: `internal/transport/websocket/subscribe_auth_test.go` (коммит 95ac92a, S-132c) — нет финального перевода строки (`gofmt -d`). Не трогал (чужой коммит); поправить chore-задачей.