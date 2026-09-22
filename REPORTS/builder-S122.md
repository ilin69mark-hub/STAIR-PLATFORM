# S-122: docker-compose — bind 127.0.0.1 для postgres/redis

**Вердикт:** DONE — оба маппинга забиндены на localhost, compose config валиден, закоммичено в `dev/swarm/ws-s115-s116`.

## Что сделано
- `deployments/docker-compose.yml`
  - `postgres`: `"5432:5432"` → `"127.0.0.1:5432:5432"` (+коммент S-122).
  - `redis`: `"6379:6379"` → `"127.0.0.1:6379:6379"` (+коммент S-122).

## Гейты (deploy-зона, Go-код не тронут)
- `yaml.safe_load` — ok; `docker compose config` — ok, `host_ip: 127.0.0.1` у postgres/redis/api.
- Полный `go test` пропущен обоснованно: изменён только yml (DoD deploy-зоны).

## Не-цель (осознанно)
- `store` (`3000:80`) и `admin` (`5174:80`) оставлены на всех интерфейсах: это статика для LAN-превью, привилегированных эндпоинтов не несут; api-порт 8080 уже localhost-bound (S-113-fix), nginx ходит к api по container-сети. Если нужно — отдельной задачей.

## Счётчик CI
Задача 11/20 после PR #51 (следующий CI-прогон после 20-й).
