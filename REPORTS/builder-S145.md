# builder-S145 — Payments: crash-window dedup self-healing + guard переходов (S-141 №3, №4)

**Вердикт**: S-145 закрыт. Обе находки аудита устранены на backend (payments + database), два отдельных коммита: S-145 (этот отчёт) и S-146 (см. `REPORTS/builder-S146.md`).

## №3 [HIGH] — Потеря платежа при крахе между dedup-меткой и применением (CWE-367/703)

**Что было**: `stripe_webhook.go:79-97` — CheckAndMark ДО ApplyVerifiedEvent, defer Clear только на err-возврате. Паника/kill между ними → метка живёт 7 дней, Stripe-retry отбрасывался как дубликат (`duplicate event ignored`, `return nil`). Деньги списаны, доступ не выдан.

**Что сделано** (вариант (b) аудита — self-healing на дубликате, Redis+PG кросс-стор транзакции не вводились):

- `internal/infrastructure/payments/stripe_webhook.go`:
  - Новый опциональный интерфейс `IntentReader` (`GetIntentByProviderCheckout`) — type-assert `s.processor.(IntentReader)` с деградацией к старому поведению (молчаливый игнор дубликата), если процессор ридер не реализует. Реализуется `application.Service` (см. ниже) — в проде type-assert проходит автоматически, wiring в `cmd/api/main.go` не менялся.
  - Ветка дубликата вынесена в `handleDuplicate`: интент в целевом статусе события → лог `duplicate event ignored (intent in target status)` + `return nil` (чистый дубликат, точно как раньше); интент НЕ отражает событие (статус иной/нецелевой, интента нет) → лог `duplicate event reconciled` + `ApplyVerifiedEvent` (reconcile inline). Оба пути логируются с `event_id` + `checkout_id`.
  - Единый `applyEvent` для нового события и reconcile: тот же `defer Clear` только на err-пути (семантика не изменена, TTL 7 дней не тронут). Ошибка чтения интента при сверке → ошибка наружу, метка НЕ снимается (crash-window защита продолжает работать); ErrNotFound интента → применяем как обычно (Apply вернёт ErrNotFound, метка снимется, Stripe-retry повторит — событие не теряется молча).
  - `target` для сверки через `reconcileTarget`: completed→paid, failed/expired→failed (тот же маппинг, что в switch).
- `internal/application/payments/service.go`: добавлен публичный `Service.GetIntentByProviderCheckout` (делегирует в repo) — минимальное расширение, чтобы `*Service` реализовывал `IntentReader`.

**Тест-ловушка аудита** (`stripe_webhook_test.go`):
- `TestStripeWebhookDuplicateReconcilesAfterCrashWindow`: mark → «процесс убит» (CheckAndMark выполнен, Apply НЕ вызван, Clear не вызывался; метка предзаполнена в fakeDeduper) → повторный тот же event.id → **ApplyVerifiedEvent ВЫЗВАН** (reconcile), интент → paid; метка переживает успешный reconcile.
- `TestStripeWebhookDuplicateIgnoredWhenIntentInTargetStatus`: чистый дубликат (интент уже paid, повтор completed) → **Apply НЕ вызван повторно**, интент untouched.
- `TestStripeWebhookDuplicateReconcilesWhenIntentStatusDiffers`: failed-интент + повтор completed → reconcile (Apply вызван — решает guard №4).
- `TestStripeWebhookDuplicateDegradesWithoutReader`: ридер не реализован → старое поведение (игнор, без ошибки).
- `TestStripeWebhookDuplicateReconcileReadError`: ошибка чтения → ошибка наружу, Apply не вызван, метка не снята.
- `TestStripeWebhookDuplicateIntentMissingApplies`: интента нет → Apply вызывается, ErrNotFound наружу, метка снимается.
- Существующие `TestStripeWebhookDeduplicates` / `TestStripeWebhookDedupMarkerClearedOnFailure` проходят без изменений (деградация через recordingProcessor без ридера; Clear на err-пути сохранён).

## №4 [HIGH] — Регрессия статуса интента (CWE-20)

**Что было**: `payment_repo.go:117-127` — UpdateStatus без guards; поздний `checkout.session.expired` после `completed` переворачивал оплаченный интент (paid→failed), `paid_at` оставался (COALESCE) — учёт расходился.

**Реальные значения статусов в БД** (из `internal/application/payments`): `pending`, `paid`, `failed`, `refunded` (не `succeeded` — resolveStatus маппит `succeeded`/`paid` → `StatusPaid="paid"`, `failed`/`declined` → `StatusFailed="failed"`). Терминальные: **paid, failed, refunded**.

**Что сделано**:
- `internal/application/payments/entity.go`: `TerminalStatuses()` (экспорт) + `isTerminalStatus()` — единый источник правды о терминальных статусах.
- `internal/infrastructure/database/payment_repo.go`: SQL-guard в WHERE —
  `AND NOT (status IN (paid,failed,refunded) AND status <> $1)` —
  терминальный статус нельзя перезаписать другим статусом; повтор того же статуса (`paid→paid`, «succeeded→succeeded») разрешён. Литерал списка собран из `payments.TerminalStatuses()` на init (строки не дублируются).
  `RowsAffected == 0` → `slog.Warn("payments: intent status transition rejected (terminal status)", intent_id, from → to)` + отдельные warn для not-found/ошибки чтения. Метрик в payments нет — только лог, ничего не выдумано.
- Отклонённый переход = не ошибка (0 строк): сервис продолжает, событие попадает в журнал `payment_events` (аудит), статус/paid_at не трогаются.

**Тесты**:
- PG-интеграционный `payment_repo_test.go`: `TestPaymentUpdateStatus` переписан — pending→paid (paid_at), затем paid→failed (поздний expired) → **статус остался paid**, **paid_at не едет** (равен значению до перехода). Новый `TestPaymentUpdateStatusTerminalGuard`: paid→paid повтор разрешён, pending→failed разрешён, failed→paid отклонён.
- Unit application: `TestApplyVerifiedEventRespectsTerminalGuard` (`service_extra_test.go`, guardRepo-фейк с семантикой PG-guard и копиями при чтении как реальный scanIntent): succeeded→paid, поздний failed → не ошибка, статус paid, paid_at не двигается, оба события в журнале, повтор paid→paid разрешён.

## Гейты (S-145)

- `gofmt -l` — пусто; `go vet` (3 затронутых пакета) — 0; `go build ./...` — ок.
- `go test -count=1 ./internal/application/payments ./internal/infrastructure/payments ./internal/infrastructure/database` (с `STAIR_TEST_DATABASE_URL`) — PASS (app 0.13s, infra-payments 12s, database 2.3s).
- `golangci-lint run` по 3 затронутым пакетам — 0 issues.
- Полный `go test -race -p 1` с БД — в итоговом прогоне (см. конец S-146, один общий прогон на оба коммита).

## Файлы

- `internal/infrastructure/payments/stripe_webhook.go` — IntentReader, handleDuplicate (reconcile), applyEvent (единый defer Clear), reconcileTarget.
- `internal/infrastructure/payments/stripe_webhook_test.go` — readerProcessor фейк + 6 новых тестов (ловушка crash-window + дубликаты).
- `internal/application/payments/entity.go` — TerminalStatuses()/isTerminalStatus().
- `internal/application/payments/service.go` — GetIntentByProviderCheckout (реализация IntentReader).
- `internal/application/payments/service_extra_test.go` — guardRepo + TestApplyVerifiedEventRespectsTerminalGuard.
- `internal/infrastructure/database/payment_repo.go` — SQL-guard + warn-лог.
- `internal/infrastructure/database/payment_repo_test.go` — обновлённый + новый guard-тест.

## Решения/заметки

- Новое событие + processor==nil: метка остаётся (err==nil → Clear не вызывается) — поведение сохранено как было.
- Гонка «два параллельных дубликата, оба читают pending до коммита первого» может дать повторное применение (журнал получит 2 записи, статус всё равно paid — учёт не расходится): это врождённая цена варианта (b) без распределённой транзакции; аудит требовал именно self-healing, а не полную атомарность.
- Redis+PG кросс-стор транзакция не вводилась (осознанно, вариант (b) по аудиту).