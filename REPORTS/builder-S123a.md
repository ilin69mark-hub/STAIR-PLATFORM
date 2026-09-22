# S-123a: апгрейд базовых образов (SHA-пины GHA — отдельно pre-CI)

**Вердикт:** DONE (частично) — runtime-образы обновлены, все 3 образа собираются и дымят зелено, запушено в `dev/swarm/ws-s115-s116`. SHA-pinning GHA вынесен в pre-CI задачу (см. ниже).

## Что сделано
- `deployments/Dockerfile`: `alpine:3.20` → `alpine:3.22` (актуальный stable).
- `deployments/store.Dockerfile` / `admin.Dockerfile`: `nginx:1.27-alpine` → `nginx:1.29-alpine` (актуальный stable).
- Без изменений (уже актуальны): `golang:1.26-alpine` (= toolchain go1.26.6), `node:22-alpine` (LTS до 2027), `postgres:16`, `redis:7` (их мажорные апгрейды — отдельные рискованные задачи, не часть S-123).

## Гейты
- `docker build`: api (go build внутри, alpine 3.22) ✅, store ✅, admin ✅ (npm-слои из кэша S-117).
- Smoke: `/bin/api` стартует на 3.22 (fail-fast `STAIR_DATABASE_URL is not set` — поведение S-104, значит бинарь исполняется); store/admin: `index.html`+`assets` на месте, S-121 CSP/DENY в bundled conf.
- `nginx:1.29-alpine nginx -t` с S-121 конфигом — successful (совместимость подтверждена).
- Go-гейты неприменимы (только Dockerfile, Go-код не тронут).

## Решение: сплит S-123 (записано на борд)
- (a) образы — сделано здесь, валидируется локальной сборкой ✅.
- (b) SHA-pinning ~20 `uses:` в ci.yml/cd.yml — ОТЛОЖЕН к pre-CI задаче: пины валидируются только реальным CI-раном, а правило роя — 1 CI на 20 задач; пинить сейчас = непроверенный дифф с высоким blast radius на все джобы. Новая подзадача создана на борде (S-123b).

## Счётчик CI
Задача 12/20 после PR #51 (следующий CI-прогон после 20-й).
