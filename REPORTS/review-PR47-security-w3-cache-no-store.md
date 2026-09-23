# Review — PR #47 (S-103 W3, S-107 cache/no-store)

- reviewer: @reviewer (self-execute; Task-dispatch ⚙️ BLOCKED_INFRA)
- branch: `dev/swarm/security-w3-cache-no-store` → `main`
- commit: ced1a3c (3 файла, +77/−2)
- verdict: **APPROVED**

## Diff

- `internal/transport/http/cache.go` — `CacheMiddleware`: `if policy.Private && isIdentityBearerRequest(r) { policy = CacheNoCache }`.
- `internal/transport/http/router.go` — удалён `/api/v1/projects/` из `cachePolicies` (+ поясняющий комментарий).
- `internal/transport/http/cache_test.go` — 3 новых теста.

## Проверка

- **Корректность фикса.** `private` действительно не защищает от браузерного кеша; downgrade в `no-store` для identity-запросов закрывает класс. Скоуп на `Private` точен: не задевает `CacheLong`/`CacheImmutable` (публичные ассеты), что подтверждено тестом с cookie. `isIdentityBearerRequest` переиспользован из `responsecache.go` — единая семантика identity (session/csrf + admin-суффиксы + Bearer).
- **Защита в глубину.** Identity-downgrade (класс) + удаление префикса (конкретный путь, в т.ч. анонимный 401). Лишнего не удалено; инертные префиксы (`configs/assortments/materials/profiles/stairs` — роутов нет) намеренно не тронуты.
- **Порядок middleware.** `CacheMiddleware` снаружи, заголовок ставится до `next.ServeHTTP` — не перетирается хендлером.
- **Тесты.** Позитив (authed → no-store), негатив-скоуп (anon → policy сохраняется), регресс ассетов (immutable с cookie), роутер-level (реальный `NewRouter`, 200 + no-store). Достаточно.
- **Идиомы/стиль.** Комментарии по-русски в стиле файла, gofmt/vet чисто.

## Замечания

Нет блокирующих. Прежние 6 lint-замечаний не относятся к диффу. Требуется merge-команда человека (D1).
