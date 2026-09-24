# S-150 + S-151 — Red-team «не требует человека» (2026-09-24)

## Вердикт

**Обе закрыты кодом.** Из 6 красных флагов red-team 2 закрыты полностью
(«платное бесплатно», SSRF/IMDS), 4 требуют человека → `docs/SECURITY_BACKLOG.md`.
Коммиты: `fca3525` (S-150), `c55d3fc` (S-151). Self-execute, режим build-and-plan.

## S-150 — Цена checkout из серверного каталога (red-team #3, 🔴→🟢)

**Было:** `POST /api/v1/projects/{id}/checkout` принимал `amount_minor` из тела;
`CreateCheckout` проверял только `> 0`. Атака: checkout на 1 копейку → полная
услуга за 1 копейку.

**Стало:**
- `application/payments/tier.go` — `Catalog` (server-side price authority):
  `Tier{ID, AmountMinor, Currency}`, `Resolve` (неизвестный/пустой → `ErrInvalid`),
  `List`; `DefaultCatalog()`: basic=90 000 RUB, pro=180 000 RUB; `WithCatalog`.
- `Service.CreateCheckout(ctx, tenant, project, user, tierID)` — сумма и валюта
  ТОЛЬКО из каталога; параметра суммы в сигнатуре больше нет.
- `transport/http`: тело `{"tier_id":"basic"}`; пустой tier → 422; неизвестный → 422.
  Старые `amount_minor`/`currency` в теле игнорируются (цены не задают).
- Swagger перегенерирован `hack/gen_swagger.py --write` (оба файла).
- Тесты: `TestCreateCheckoutPriceFromCatalog` (провайдеру уходит цена каталога),
  `TestCreateCheckoutUnknownTierRejected` (пустой/free/`../../etc/passwd` → ErrInvalid),
  `TestCheckoutPriceNotClientControlled` (HTTP: `amount_minor:1` в теле → 201 с
  `amount_minor:90000`; без tier_id → 422), `TestCreateCheckoutNormalizesCatalogCurrency`.

**Просадка:** фронты не вызывали checkout (поиск пуст) — UI-правки не нужны.

## S-151 — Fail-closed окружение + IMSS-защита allowlist (red-team #4, 🟡→🟢)

**Было:** `STAIR_ENVIRONMENT=""` или опечатка (`prd`, `prodction`) молча
выключали все прод-гарды (S1-2 шифрование, Stripe-guard, SSRF-loopback в воркере).
Allowlist (`STAIR_WEBHOOK_ALLOW_HOSTS`) полностью обходил классификацию IP,
включая link-local — теоретический путь к IMDS 169.254.169.254.

**Стало:**
- `infrastructure/envguard`: `Known` (dev/test/staging/production/prod), `Validate`
  (пустое/неизвестное → ошибка со списком), `IsProduction`. api и worker при старте:
  `Validate(os.Getenv(...))` → `os.Exit(1)` с error-логом.
- `integrations.classifyIP(ip, allowPrivate)`: link-local блокируется ВСЕГДА,
  allowlist открывает loopback+RFC1918 (внутренние сервисы) — регрессия S-104 не
  сломана (`TestSSRFAllowlistOverridesBlock` зелёный). `resolveDialIP` и
  `validateTarget` больше не имеют early-return на allowlist: хост всегда
  резолвится и пинится (anti-rebinding сохранён).
- Тест-ловушка `TestSSRFAllowlistNeverUnlocksLinkLocal`: allowlist-хост на
  169.254.169.254 → блок в preflight и на dial; internal.erp.local на 10.1.2.3 → ок.

## Гейты

- `gofmt` чист; пакеты payments/integrations/envguard/transport/http — ok.
- Полный `go test -race -p 1 ./...` — в фоне, результат дописать координатору.
- `python3 hack/check_audit_exceptions.py` — 11/11 парных (реестр не тронут).
