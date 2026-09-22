# S-113 — runtime-валидация 3 security-кандидатов (run-1 NEEDS-VALIDATION)

**Задача:** S-113 [P3] — валидировать три кандидата из
`~/security-audit-skill/stair-platform/run-1/NEEDS-VALIDATION.md`, чья
децизивная проверка была заблокирована политикой «sandboxed-source-and-local-only».

**Выполнено:** координатором (self-execute). Локальные части планов валидации
выполнены полностью; deployment-части требуют staging/человека (см. ниже).

---

## 1. LOGIN-TIMING-ENUMERATION — ✅ ПОДТВЕРЖДЁН (реальная уязвимость)

**Замер** (`internal/application/auth/timing_validation_test.go`, `go test -bench=BenchmarkLoginTiming -benchtime=30x`):

| Путь | Латентность | |
|---|---|---|
| Известный email + неверный пароль (bcrypt compare, service.go:199) | **50 828 080 ns/op (50.8 мс)** | bcrypt cost 10 |
| Неизвестный email (мгновенный ErrInvalidCreds, service.go:191) | **156.9 ns/op (0.157 мкс)** | |

**Разница: ~324 000×.** Гипотеза аудита подтверждена: по времени ответа
`POST /api/v1/auth/login` можно перечислять зарегистрированные email.

**Рекомендация (фикс — отдельная задача):** для неизвестного email выполнять
«dummy» bcrypt-сравнение с фиксированным хешем (или всегда гонять bcrypt на
случайном хеше при ErrNotFound), чтобы обе ветки были ~равны по времени.
Ориентир: `bcrypt.GenerateFromPassword` при регистрации уже есть — можно
закешировать dummy-хеш в Service.

## 2. WEBHOOK-MAPPED-LINKLOCAL-BYPASS — ✅ УЖЕ ИСПРАВЛЕН (S-104), добавлен регресс-тест

**Проверка:** `cmd/worker/main.go` — продакшен-детект нормализован в S-104
(PR #45): `isProductionEnv()` = EqualFold(`production`|`prod`). Значит
`STAIR_ENVIRONMENT=prod` корректно включает SSRF-политику (AllowLoopback=false).

**Изменения:**
- Вынесена тестируемая `webhookPolicy(environment, allowHostsCSV)` (была
  инлайн-логика в main()); `isProductionEnv()` переиспользуется и для
  требования STAIR_SECRETS_KEY.
- Новые тесты `cmd/worker/main_test.go`:
  - `TestWebhookPolicyProductionBlocksLoopback` — env ∈ {production, prod, PROD,
    Production, PrOd} → AllowLoopback=false и доставка на `http://127.0.0.1:9999`
    блокируется (Send → SSRF-ошибка). 5/5 PASS.
  - `TestWebhookPolicyDevAllowsLoopback` — dev-окружения сохраняют loopback. 4/4 PASS.
  - `TestWebhookPolicyAllowHosts` — allowlist применяется независимо от env. PASS.

**Вердикт:** bypass-сценарий из аудита (env=prod отключает SSRF-политику)
**невоспроизводим** на текущем коде; регресс-тест закрепляет поведение.

## 3. INTERNAL-ONLY-PROXY-BYPASS — ⚠️ ЗАВИСИТ ОТ ТОПОЛОГИИ (dev-экспозиция в docker-compose)

**Ревью деплой-конфигов:**
- `internal/transport/http/internal_only.go` — решение только по `RemoteAddr`
  (X-Forwarded-For не доверяется) — корректно, спуфинг невозможен.
- **Helm/K8s (prod-путь):** `deploy/helm/stair-platform/values.yaml:58` —
  Service `type: ClusterIP`, порт 8080; ingress-шаблона НЕТ (ingress.enabled —
  мёртвая конфигурация). Внешняя достижимость /metrics, /swagger, /debug/pprof
  через K8s — **отсутствует**. ✅ PASS.
- **nginx (store.conf/admin.conf):** проксируется только `location /api/` —
  внутренние роуты через nginx недостижимы. ✅ PASS.
- **docker-compose (dev/staging-путь):** `deployments/docker-compose.yml:58-59`
  публикует `8080:8080` напрямую. RemoteAddr при обращении через docker-proxy =
  bridge-шлюз (172.17.x.x) → **входит в allowedCIDRs (172.16.0.0/12)** →
  /metrics, /swagger, /debug/pprof достижимы с хоста (и из LAN, если файрвол
  хоста открыт). ⚠️ **dev-экспозиция подтверждена.**

**Рекомендация (hardening, отдельная задача):** в docker-compose привязать
`127.0.0.1:8080:8080` (только localhost) — dev-топология не должна светить
внутренние роуты в LAN. В проде (Helm) менять ничего не нужно.

---

## Гейты (локально)

| Гейт | Результат |
|---|---|
| gofmt -l | пусто |
| go vet ./... | 0 |
| go build ./... | ok |
| golangci-lint run | 0 issues |
| Новые тесты worker (3) | 3/3 PASS |
| Бенчмарк таймингов | known 50.8 мс vs unknown 0.157 мкс |
| **Полный прогон** `go test -race -p 1 ./...` + БД | **58/58 ok, 0 FAIL** |

## Что осталось (требует человека/staging)

1. **LOGIN-TIMING**: фикс dummy-bcrypt (рекомендация выше) — отдельная задача.
2. **INTERNAL-ONLY**: на staging/prod — `ss -ltnp` + правила файрвола/LB для
   порта API; curl `/metrics` и `/swagger/` с внешней сети (ожидание 403).
3. **WEBHOOK**: на staging worker с `STAIR_ENVIRONMENT=prod` — webhook на
   127.0.0.1 должен отклоняться (ожидание: SSRF-ошибка в логах).

**Вердикт:** S-113 локальная валидация завершена — (1) тайминг-энумерация
подтверждена (324 000×, нужен фикс), (2) SSRF-байпас уже исправлен S-104 и
закреплён регресс-тестом, (3) internal-only в проде (Helm) недостижим, в
docker-compose dev — экспозиция (рекомендован bind 127.0.0.1). 58/58, lint 0.

**Длительность:** ~35 мин (self-execute).