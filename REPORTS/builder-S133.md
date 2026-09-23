# S-133: Payments покрытие 80% → 85%+ (100% / 91.3%)

**Вердикт:** DONE — оба пакета над порогом с запасом, полный сьют 58/58, запушено в `dev/swarm/ws-s132b-client`.

## Замеры (statements)
- `application/payments`: 80.0% → **100%** (+11: ListByProject, Get, ApplyVerifiedEvent, ветки ошибок).
- `infrastructure/payments`: 62.6% → **91.3%** (+~57: stripe_cb 0→~100, eventdedup 0→100, stripe httptest, webhook-ветки, mock/adapter).
- Остатки <100%: HandleStripeWebhook 81.6% (лог-ветки ошибок apply — покрыты частично), VerifyWebhookSignature 88.5%, makeRequest 92.9% — некритично, порог держится с запасом.

## Что добавлено (19 тест-кейсов, только stdlib + httptest + real test-redis)
- `application/payments/service_extra_test.go`: ListByProject (скоуп), Get (+not found/foreign tenant), ApplyVerifiedEvent paid/failed + 4 ошибки, HandleWebhook (nil verifier, bad JSON), CreateCheckout (provider/repo err), apply (update/append err). Фейки failRepo/failProvider (базовые не тронуты).
- `infrastructure/payments/stripe_cb_test.go` (новый): Name/Breaker, CreateCheckout ok/err, Verify+Parse ok/err — inner на httptest, настоящий CB.
- `infrastructure/payments/stripe_http_test.go` (новый): CreateCheckoutSession ok/non-200/bad-JSON, GetSession 4 статуса + 404.
- `infrastructure/payments/stripe_webhook_extra_test.go` (новый): failed/expired/unhandled-ветки, dedup-error, no-processor, mock Name/nil-ctx/zero-amount, adapter success-путь.
- `infrastructure/payments/eventdedup_test.go` (новый): key-формат + roundtrip fresh→replay→clear на реальном тестовом Redis (адреса 6380→6379, skip-прецедент queue).

## Гейты
- `go vet` + `gofmt` — чисто; пакеты ok (-race).
- Полный: `go test -race -p 1 -count=1 ./...` на свежей БД — **58 ok, EXIT 0**.

## Находка
- Тестовый Redis слушает хост-порт **6380** (не 6379 — там dev-стек); в CI redis нет вообще (скип по прецеденту). Записано в тест комментом.

## Счётчик CI
Задача 2/20 после PR #52.
