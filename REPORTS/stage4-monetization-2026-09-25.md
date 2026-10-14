# Этап 4 «монетизация» — отчёт (2026-09-25)

Ветка: `dev/swarm/s141-fixes` · не пушить без команды человека.

## Решения (подтверждены человеком)

- **Модель:** фиксированные услуги из серверного каталога (без депозита за саму
  лестницу). Тарифы: «Выезд инженера и замер» — 900 ₽, «Проект и рабочая
  документация» — 1 800 ₽.
- **Оплата:** редирект на страницу платёжного провайдера (PSP), webhook возвращает
  статус.
- **Спираль** отключена (S-152), вернётся отдельным обновлением сайта.

## Что сделано

| Шаг | Содержание | Файлы |
|---|---|---|
| 4.0 | Каталог услуг с витринными подписями (`Tier.Title/Description`) и публичный прайс `GET /api/v1/public/payment-tiers`: id, подпись, описание, сумма в копейках и рублях, кэш 5 минут | `internal/application/payments/tier.go`, `internal/transport/http/public_payment_tiers.go` (+ тест) |
| 4.1 | Оплата услуги без проекта: миграция `tier_id` в `payment_intents`, `CreateServiceCheckout`, `ListByUser`, ручки `POST /api/v1/public/services/checkout` (auth) и `GET /api/v1/payments/mine` | `migrations/000027_payment_intents_tier.{up,down}.sql`, `payments/{entity,service}.go`, `database/payment_repo.go`, `public_service_checkout.go` (+ тесты) |
| 4.2 | **CSRF allowlist источников** — без него оплата с витрины невозможна: Next-прокси подменяет Host, а проверка требовала совпадения Origin с Host. `STAIR_CSRF_ALLOWED_ORIGINS` (список, без wildcard), по умолчанию — только свой host | `middleware_auth.go` (`csrfOriginAllowed`, `Config.CSRFAllowedOrigins`), `router.go`, `cmd/api/main.go`, `csrf_origin_test.go` |
| 4.3 | Витрина: страница `/services` и блок на главной — каталог с серверными ценами виден гостю, кнопка «Войти и оплатить» раскрывает форму входа, после входа «Оплатить» создаёт платёж и уводит на PSP | `showcase/app/services-pay.tsx`, `app/services/page.tsx`, `app/page.tsx`, `app/layout.tsx` |
| 4.4 | Кабинет: раздел «Мои покупки» — услуга, сумма, статус (оплачено/ожидает/ошибка/возврат), дата | `frontend-store/src/components/Cabinet.tsx` (+ тест) |
| 4.5 | Клиентский слой `paymentsApi.tiers/checkout/mine` в общем пакете, `StorefrontProviders` — одна сессия на конструктор и услуги | `frontend/shared/src/storefront/api/store.ts`, `StorefrontProviders.tsx` |
| 4.6 | E2E витрины: сид e2e-пользователя в global setup (общая с Go-тестами БД чистится) + два сценария оплаты | `showcase/e2e/global-setup.ts`, `showcase/e2e/tests/showcase.spec.ts` |

## Гарантии по деньгам

- **Сумму задаёт сервер.** Клиент присылает только `tier_id`; сумма берётся из
  каталога (S-150). Тест проверяет, что интент создан с ценой каталога даже если
  клиент пытается иначе (тело суммы не содержит вовсе).
- **Покупка принадлежит пользователю.** Интент без проекта, привязан к `user_id`;
  `GET /payments/mine` отдаёт только свои покупки (тест на чужого пользователя).
- **Провал не съедает деньги.** Неизвестный тариф → 422 (ошибка ввода), а не 500.

## Гейты

| Гейт | Результат |
|---|---|
| `go test -race -p 1 ./...` | 61/61, 0 FAIL |
| `frontend` | 450/450 |
| `frontend-store` | 2112/2112 |
| `showcase` | vitest 6/6, tsc 0, oxlint 0 замечаний, build OK |
| e2e витрины | 8/8 (включая оплату услуги) |
| e2e магазина (store/rails/room-fit/seo/session) | 20/20 |

## Что осталось за рамками этапа

- Настоящий PSP: локально и в e2e работает мок-провайдер; для Stripe нужен
  `STAIR_STRIPE_SECRET_KEY` и прод-ключи (страница оплаты открывается по
  `checkout_url`, который вернул сервер).
- Спасибо/квитанция на email после оплаты (интеграции EDR-0023 умеют, но шаблон и
  событие не настроены).
- Возврат средств в интерфейсе (статус `refunded` есть, кнопки нет).
- Промо-цены и скидки на услуги: каталог задаётся кодом (`DefaultCatalog`), env-
  переопределение цен (`STAIR_PAYMENT_TIERS`) в комментарии упоминается, но не
  реализовано.
