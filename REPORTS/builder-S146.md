# builder-S146 — Stripe: парсинг подписей при ротации ключей (S-141 №9)

**Вердикт**: S-146 закрыт. Коммит отдельный от S-145 (`fix: S-146 ...`), затронут только `internal/infrastructure/payments`.

## Что было — CWE-754

`internal/infrastructure/payments/stripe.go:161-166` — `VerifyWebhookSignature` принимал заголовок только из ровно 2 частей (`len(parts) != 2` → reject). В окне ротации ключей Stripe присылает `t=<ts>,v1=<подпись-старым>,v1=<подпись-новым>` (несколько v1) — такой заголовок отвергался целиком, и **все** вебхуки падали в окно ротации (двойной reject: и формат, и подпись не совпадала с одним настроенным секретом).

## Что сделано

- `internal/infrastructure/payments/stripe.go` — `VerifyWebhookSignature` переписан:
  - жёсткое `len(parts) != 2` убрано (остался дешёвый sanity `len(parts) < 2` — t + хотя бы одна v1 физически = минимум 2 пары);
  - разбираются **все** пары `key=value`, собираются **все** значения `v1` в слайс; `timestamp` — из **первого** `t=`;
  - подпись принимается, если **ЛЮБАЯ** v1 совпадает с настроенным `webhookSecret` (сверка через `hmac.Equal`, expected считаем один раз);
  - поведение НЕ ослаблено: пустой/битый заголовок, отсутствие `t` или `v1`, несовпадение всех v1, нечисловой timestamp, просрочка tolerance-окна (5 мин) — по-прежнему reject.
- Существующие тесты формата подписи не сломаны (проверено):
  - `TestStripeProviderVerifyWebhookSignature` ("invalid_signature" → нет t/v1 → reject),
  - `TestStripeProviderVerifyWebhookSignatureFormat` ("t=123,v1=" → v1 пустая, hmac не совпадает → reject),
  - `TestStripeWebhookServiceHandleGoodSignature` / `HandleBadSignature` / `HandleStaleSignature` — без изменений PASS.

## Обязательные тесты-ловушки (новый `stripe_rotation_test.go`)

Подписи считаются реальным кодом теста: `signWithSigns` — HMAC-SHA256 от `"<ts>.<payload>"` (как Stripe), ts = `time.Now().Unix()` (в tolerance).
- `TestStripeVerifyWebhookSignatureRotation`: заголовок `t=<ts>,v1=<старым>,v1=<новым>` при настроенном **новом** секрете → **verified**; обратный порядок v1 → verified; при настроенном **старом** секрете → verified (совпала любая v1).
- `TestStripeVerifyWebhookSignatureRotationNoMatch`: обе v1 — чужими секретами → **reject**.
- `TestStripeVerifyWebhookSignatureRotationMalformed`: garbage / без t / без v1 / пустая v1 / одни запятые → **reject** (5 сабтестов).
- `TestStripeVerifyWebhookSignatureSingleV1`: одиночный v1 по-старому → **verified** (регресс обычного режима без ротации).

## Гейты (S-146)

- `gofmt -l internal/infrastructure/payments` — пусто; `go vet ./internal/infrastructure/payments` — 0.
- `go test -count=1 ./internal/infrastructure/payments ./internal/application/payments ./internal/infrastructure/database` — PASS (19.5s / 3.1s / 3.4s; database-интеграция с локальной stair-test-pg прогонялась полным race-прогоном ниже).
- `golangci-lint run ./internal/infrastructure/payments/...` — 0 issues (были G101 и QF1012/errcheck в новом тесте — закрыты nolint-комментарием фикстуры и `_ , _ = fmt.Fprintf` на hmac.Hash).
- Полный `go test -race -p 1` с `STAIR_TEST_DATABASE_URL` — зелёный (общий прогон на оба коммита, см. итог в `builder-S145.md`/сводку).

## Файлы

- `internal/infrastructure/payments/stripe.go` — multi-pair/multi-v1 parsing, any-match HMAC, первый t.
- `internal/infrastructure/payments/stripe_rotation_test.go` — 4 новых теста (ротация, no-match, malformed, single-v1 регресс).

## Решения/заметки

- Параметр `webhookSecret` один (как и было) — «прими если любая v1 совпадает» реализовано именно через сверку каждой v1 с ним; отдельный список доверенных секретов не вводился (Stripe-механика ротации: старые подписи продолжают валидироваться старым секретом, поэтому одного настроенного секрета достаточно).
- Пустой `t=` (первая пара t с пустым значением) → reject (timestamp == ""), «второй t» не используется — поведение «бери первый t» трактуется строго, как в задаче.