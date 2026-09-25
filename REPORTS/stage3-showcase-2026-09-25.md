# Этап 3 «витрина» — отчёт (2026-09-25)

Ветка: `dev/swarm/s141-fixes` · не пушить без команды человека.
Вариант А (Next.js App Router), согласован ранее.

## Что сделано

| Шаг | Содержание | Файлы |
|---|---|---|
| 3.0 | Каркас витрины: `showcase/` на Next.js 16 (App Router, Turbopack), TypeScript strict, oxlint + vitest (как в остальных фронтах), реген типов `import.meta.env` под Vite-экосистему | `showcase/package.json`, `next.config.ts`, `tsconfig.json`, `vitest.config.ts`, `types/global.d.ts` |
| 3.1 | Прокси `/api/*` и `/static-assets/*` на Go-API (`STAIR_API_URL`), статический пререндер с revalidate | `showcase/next.config.ts`, `app/lib/api.ts` |
| 3.2 | **Публичный каталог материалов на API**: `GET /api/v1/public/materials` — код, плотность, диапазоны толщины и габаритов, ставка ₽/кг, ссылка на PBR-превью, финиши. Источник истины — бэкенд, а не копия во фронте | `internal/transport/http/public_materials.go` (+ тест), `router.go` |
| 3.3 | Главная: hero с живым 3D, «как работает», примеры расчётов, каталог с настоящими текстурами, FAQ, CTA | `showcase/app/page.tsx`, `app/globals.css`, `app/layout.tsx` |
| 3.4 | Страница материалов и постраничная карточка материала (плотность, толщины, лист, цена, финиши, CTA в калькулятор) | `app/materials/page.tsx`, `app/materials/[code]/page.tsx` |
| 3.5 | Страница примеров: модели считаются живым расчётом API, не картинки | `app/examples/page.tsx` |
| 3.6 | **Калькулятор-остров**: страница `/calculator` подключает общий с витриной-калькулятором `Constructor` через алиас `@store/*` (логика не копируется), с общим `AuthProvider` | `app/calculator/page.tsx`, `app/calculator.tsx` |
| 3.7 | SEO: `sitemap.xml` (включая постраницу каждого материала из API), `robots.txt`, metadata с OG | `app/sitemap.ts`, `app/robots.ts`, `app/layout.tsx` |

## Найденные и исправленные баги

1. **Раздача `/static-assets/*` отдавала 404 на все текстуры.** Обработчик получал путь с
   префиксом монтирования и искал файл в `<root>/static-assets/...`; юнит-тест передавал
   путь уже без префикса и маскировал баг. Исправлено: префикс срезается в обработчике,
   файл отдаётся через `http.ServeContent` (а не `http.FileServer`, который смотрит на
   исходный `r.URL.Path`). Добавлен сквозной тест через `ServeMux` с префиксом.
2. **Swagger-sync сломался на конкатенации пути** в роутере: регэксп вытаскивает маршруты
   из строковых литералов. Путь возвращён литералом, спека перегенерирована.

## Спираль (S-152)

Винтовой марш скрыт целиком: `SPIRAL_ENABLED=false` убирает его из UI витрины, магазина
и админки; публичные `:quote`/`:validate` отвечают 422 `flight_temporarily_disabled`; публичный
DTO не отдаёт спиральные варианты советника (иначе «Спасти расчёт» увела бы в 422).
Код движка, каталог листов/скоростей и тесты солвера сохранены — вернём с исправлением норм.

## Гейты

| Гейт | Результат |
|---|---|
| `go test -race -p 1 ./...` | 61/61 пакетов, 0 FAIL |
| `frontend` | 450/450 |
| `frontend-store` | 2111/2111 |
| `showcase` | tsc 0 ошибок, vitest 5/5, oxlint 0 замечаний, `next build` — 7 маршрутов |
| Playwright (store/room-fit/rails/seo/session) | 20/20 |
| Живая проверка витрины | `/`, `/materials`, `/examples`, `/calculator`, `/robots.txt`, `/sitemap.xml`, карточка материала — 200; калькулятор считает и рисует 3D (canvas создан, ошибок страницы нет) |

## Хвосты этапа 3 (закрыты 2026-09-25)

1. **Локализация каталога.** В MFG-0005 добавлено поле `NameRu` (витринное имя на
   русском) для всех 7 материалов; публичный DTO отдаёт `name_ru`. Витрина берёт
   русское имя из API, словарь фронта остался фолбэком. Инвариант-тест требует
   `NameRu` у каждого материала — новый код без перевода не пройдёт сборку.
2. **Стили конструктора.** `frontend-store/src/App.css` (1423 строки) разложен по
   секциям в `frontend-store/src/styles/`: base, landing, constructor, auth, cabinet,
   viewer, site. `App.css` теперь только импортирует их в исходном порядке (каскад
   не изменился), витрина тянет только `base` + `constructor` + `viewer` вместо
   всего файла.
3. **E2E витрины.** `showcase/playwright.config.ts` + `showcase/e2e/tests/showcase.spec.ts`
   — 6 тестов: главная с каталогом и 3D, отсутствие упоминаний спирали, таблица и
   карточка материала, калькулятор считает и рисует 3D, sitemap/robots, 404 материала.

## Финальные гейты (после хвостов)

| Гейт | Результат |
|---|---|
| `go test -race -p 1 ./...` | 61/61, 0 FAIL |
| `frontend` | 450/450 |
| `frontend-store` | 2111/2111 |
| `showcase` | vitest 6/6, tsc 0, oxlint 0 замечаний, build OK |
| e2e витрины | 6/6 |
| e2e витрины-калькулятора (store/rails/room-fit/seo/session) | 20/20 |

## Общий пакет `@shared/storefront` (2026-09-25)

Витрина и магазин делили код только через алиас `@store/*` — зависимость от
исходников соседнего приложения. Код перенесён в общий пакет:

| Было | Стало |
|---|---|
| `frontend-store/src/components/{Constructor,QuoteResult,quoteView,OrderForm,AuthForm}` | `frontend/shared/src/storefront/components/*` |
| `frontend-store/src/api/{client,store,auth}` | `frontend/shared/src/storefront/api/*` |
| `frontend-store/src/auth/{context,AuthContext,errors}` | `frontend/shared/src/storefront/auth/*` |
| `frontend-store/src/styles/{base,constructor,viewer}.css` | `frontend/shared/src/storefront/styles/*` |

В магазине остались тонкие фасады (`export * from '@shared/storefront/…'`), поэтому
его код и тесты не менялись; алиас `@store/*` из витрины удалён. Модульные тесты,
которые шпионили за фасадом (`api/auth.test.ts`, `api/store.test.ts`, `AuthForm.test.tsx`),
переведены на реальные модули пакета — иначе spy не перехватывал вызов.

Каскад стилей сохранён: `App.css` импортирует секции в прежнем порядке, три из них
теперь живут в общем пакете.

## Финальные гейты (после общего пакета)

| Гейт | Результат |
|---|---|
| `go test -race -p 1 ./...` | 61/61, 0 FAIL |
| `frontend` | 450/450 |
| `frontend-store` | 2111/2111 |
| `showcase` | vitest 6/6, tsc 0, oxlint 0 замечаний, build OK |
| e2e витрины | 6/6 |
| e2e магазина (store/rails/room-fit/seo/session) | 20/20 |

## Известные ограничения

- Локализация живёт в каталоге MFG-0005 (`NameRu`) и покрывает только материалы; тексты
  интерфейса по-прежнему в коде фронтендов — полноценного i18n на бэкенде нет.
- Общий пакет лежит в `frontend/shared/src/storefront` и подключается алиасом `@shared/*`
  в обоих приложениях; отдельного npm-пакета с публикацией в реестр нет — монорепо
  собирается из исходников, как и остальной shared-код.
- Vitest витрины покрывает чистые функции (подписи, список маршей); UI покрыт e2e.
- `STAIR_API_URL` и `SITE_URL` задаются окружением; локально — localhost:8080 и :5176.

