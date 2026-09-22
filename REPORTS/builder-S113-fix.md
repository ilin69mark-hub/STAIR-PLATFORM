# builder — S-113-fix: dummy-bcrypt (тайминг-энумерация) + docker-compose hardening

**Задача:** закрыть подтверждённую в S-113 уязвимость LOGIN-TIMING-ENUMERATION
(dummy-bcrypt) и hardening docker-compose (bind 127.0.0.1) — обе рекомендации
из отчёта `REPORTS/builder-S113.md`.

**Выполнено:** координатором (self-execute).

---

## 1. LOGIN-TIMING-ENUMERATION — фикс dummy-bcrypt

**Проблема:** `Login` для неизвестного email возвращал `ErrInvalidCreds`
мгновенно (~0.16 мкс), для известного — после bcrypt-сравнения (~50 мс).
Разница ~324 000× позволяла перечислять зарегистрированные email.

**Фикс** (`internal/application/auth/service.go`):
- Ветка `ErrNotFound` теперь выполняет `dummyBcryptCompare(password)` —
  bcrypt-сравнение с фиксированным хешем (cost 10) перед возвратом
  `ErrInvalidCreds`. Порядок зеркалит известную ветку: bcrypt → record.
- Хеш генерируется **лениво** (`sync.Once`) — конструктор `NewService` не
  платит ~50 мс; fallback-хеш на невозможную ошибку генерации.
- Константы `dummyBcryptPassword` / `dummyBcryptFallbackHash`.

**Замер после фикса** (`go test -bench=BenchmarkLoginTiming -benchtime=30x`):

| Путь | До | После |
|---|---|---|
| Известный email | 50.8 мс | 50.7 мс |
| Неизвестный email | 0.157 мкс | **52.6 мс** |

Разница ~4% (шум) — тайминг-канал закрыт.

**Тесты:**
- `TestLoginTimingEqualized` — нижняя граница (≥10 мс: bcrypt выполнен) +
  верхняя (≤ 5×known+20 мс: ветки сравнялись). PASS (0.70s).
- Существующие `TestLoginUnknownEmail` и др. — PASS (поведение не изменилось:
  по-прежнему `ErrInvalidCreds`).

**Побочный эффект:** каждый login с неизвестным email теперь стоит ~50 мс CPU —
приемлемо (та же цена, что у известных; есть login rate-limit).

## 2. docker-compose hardening — bind 127.0.0.1

**Проблема (S-113, кандидат 3):** `deployments/docker-compose.yml` публиковал
`8080:8080` на все интерфейсы; через docker-proxy RemoteAddr = bridge-шлюз
(172.17.x.x) ∈ allowedCIDRs `InternalOnlyMiddleware` → `/metrics`, `/swagger`,
`/debug/pprof` были достижимы с хоста и из LAN.

**Фикс:** `"127.0.0.1:8080:8080"` + комментарий (S-113). Прод-доступ — через
nginx/ingress (проксируется только `/api/`), там менять ничего не нужно.

---

## Гейты (локально)

| Гейт | Результат |
|---|---|
| gofmt -l | пусто |
| go vet ./... | 0 |
| go build ./... | ok |
| golangci-lint run | 0 issues |
| docker compose config --quiet | ok |
| auth-тесты (Login*) | 6/6 PASS |
| Бенчмарк после фикса | known 50.7 мс vs unknown 52.6 мс |
| **Полный прогон** `go test -race -p 1 ./...` + БД | **58/58 ok, 0 FAIL** |

## Остаточные наблюдения (не блокируют)

- Ветка «disabled user» (`ErrUserDisabled`) отвечает без bcrypt — но она и
  возвращает другой код ошибки, т.е. состояние раскрывается явно, а не через
  тайминг. Если требуется скрывать и это — отдельная задача (унифицировать
  ответы).
- `Register` (ErrEmailExists) — отдельный эндпоинт, аудитом не флагировался.

**Вердикт:** тайминг-энумерация закрыта (dummy-bcrypt, разница ~4% вместо
324 000×), docker-compose больше не светит internal-роуты в LAN. 58/58, lint 0.

**Длительность:** ~25 мин (self-execute).