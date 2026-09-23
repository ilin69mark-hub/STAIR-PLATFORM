# S-140-front — Sentry-интеграция фронтов (lazy, PII-скраб, ErrorBoundary)

**Статус:** ✅ КОД ГОТОВ (2026-09-23) — коммит в `dev/swarm/ws-s132b-client` (не запушен; backend-файлы @builder в коммит не входят)
**Зона:** frontend (admin `frontend/`, store `frontend-store/`, shared `frontend/shared/`)
**Ветка:** `dev/swarm/ws-s132b-client`

## Что сделано

### 1. Зависимости (root workspaces, единый lock)
- `@sentry/react@^10.75.2` — **совместим с React 19**: peerDependencies `^16.14.0 || 17.x || 18.x || 19.x` (проверено по установленному пакету, network есть).
- `@sentry/vite-plugin@^5.4.0` — devDependency обоих воркспейсов (engines node >= 18, у нас 22).
- Добавлены в оба `package.json` (frontend + frontend-store), lock обновлён один раз из root (`npm i -w` — единый `package-lock.json`).

### 2. Ленивая инициализация SDK — `frontend/shared/src/sentry/sentry.ts`
- `initSentry()` вызывается на старте обоих приложений; SDK **НЕ в стартовом чанке** — `import('@sentry/react')` выполняется:
  - по `requestIdleCallback(..., { timeout: 3000 })` (fallback `setTimeout(3000)`), **либо**
  - при первой пойманной ошибке (`captureError` → `ensureSdkLoaded` немедленно) — что раньше.
- До готовности SDK ошибки копятся в очереди (лимит 20, защита от роста), после init — `flushQueue()`.
- DSN из `import.meta.env.VITE_SENTRY_DSN`; пусто/отсутствует = `isSentryEnabled() === false`, весь модуль no-op, приложение работает как без Sentry (S-118 отложен).
- Идемпотентность: повторный `initSentry`/ошибки не порождают второй загрузки; провал init не роняет приложение (повторная попытка на следующей ошибке).
- **PII-скраб** (`beforeSend: scrubEvent` + `scrubValue`, глубина 6):
  - IP: `request.ip` и `user.ip_address` удаляются;
  - «парольные» ключи/значения-формы (`password`, `token`, `secret`, `authorization`, `api_key`, `credential`, …): значение удаляется целиком;
  - email → SHA-256 (lowercase), первые 12 hex (`hashEmail`); нет WebCrypto → email удаляется, не утекает.

### 3. ErrorBoundary — `frontend/shared/src/sentry/ErrorBoundary.tsx`
- Единый shared-компонент (классовый, `componentDidCatch`), ловит ошибки рендера subtree.
- Fallback по умолчанию на русском: заголовок «Что-то пошло не так», текст, **eventId** при наличии («Код ошибки: …») и кнопка **«Попробовать снова»** (сбрасывает `hasError` и перерисовывает children).
- Проп `fallback` сохранён для совместимости (store `QuoteResult` передаёт кастомный fallback 3D-вьювера).
- Wiring: обёрнут корень **admin** (`frontend/src/main.tsx`) и корень **store** (`frontend-store/src/main.tsx`) — `<ErrorBoundary><AuthProvider><App/></AuthProvider></ErrorBoundary>`.
- Заменён старый store-локал `frontend-store/src/ErrorBoundary.tsx` (удалён), все импорты переведены на `@shared/sentry/ErrorBoundary`.
- CSS fallback-стили `.sentry-boundary*` добавлены в оба `App.css` (переменные темы каждого приложения).

### 4. Sourcemaps и `@sentry/vite-plugin`
- `frontend/vite.config.ts` + `frontend-store/vite.config.ts`: `build.sourcemap: true` всегда.
- `sentryVitePlugin({ authToken, org, project, sourcemaps: { assets: ['./dist/assets/**'], filesToDeleteAfterUpload: './dist/assets/*.map' } })` — включается **только** когда заданы `SENTRY_AUTH_TOKEN` + `SENTRY_ORG` + `SENTRY_PROJECT` (все три; CI без секретов остаётся зелёным — плагин не входит в сборку). Конфиг — «выключатель»: реальную заливку карт человек подключит после S-118.
- `VITE_SENTRY_DSN` задокументирован в `frontend-store/.env.example`.

### 5. Бюджет бандла (S4-1 порог: entry gzip < 300KB)
| приложение | entry gzip | Sentry (отдельный lazy-чанк) | рост entry к S4-1 |
|---|---|---|---|
| admin | **109.84 KB** | `esm-*.js` 156.17 KB gzip | +3.4 KB (было 106.4) |
| store | **87.45 KB** | `esm-*.js` 156.14 KB gzip | ~0 |

Порог держится; в entry попадают только строки CSS-классов ErrorBoundary (проверено: `@sentry` runtime-кода в entry нет, `import('./esm-*.js')` — динамический). SDK грузится только при реальном DSN.

## Гейты

- `tsc -b`: **0 ошибок** — frontend ✅, frontend-store ✅ (включая vite.config.ts с плагином).
- `vitest run`:
  - frontend: **432 passed** (было 419; **+13 новых**: 7 — lazy-init/queue/PII в `sentry.test.ts`, 6 — ErrorBoundary в `ErrorBoundary.test.tsx`), 49 файлов;
  - frontend-store: **2100 passed** (без регрессий).
- Coverage frontend (пороги 80/70/75/80): Statements 86.41, Branches 79.43, Functions 88.46, Lines 88.48 — ✅; новый `sentry.ts` покрыт 86.7% lines.
- `oxlint`: 0 новых замечаний в изменённых файлах (существующие warning'и старых файлов не трогаю).
- e2e: **НЕ гонялись** (нет DSN — по условию задачи; поведение без DSN = прежнее, покрыто юнитами).

## Файлы

- `frontend/package.json`, `frontend-store/package.json`, `package-lock.json` — зависимости @sentry/react + @sentry/vite-plugin.
- `frontend/shared/src/sentry/sentry.ts` (+ `sentry.test.ts`) — lazy-инит, очередь, PII-скраб.
- `frontend/shared/src/sentry/ErrorBoundary.tsx` (+ `ErrorBoundary.test.tsx`) — общий boundary.
- `frontend/src/main.tsx`, `frontend-store/src/main.tsx` — wiring boundary + `initSentry()` в корни.
- `frontend/vite.config.ts`, `frontend-store/vite.config.ts` — sourcemap + sentryVitePlugin (по токену).
- `frontend/src/App.css`, `frontend-store/src/App.css` — стили fallback.
- `frontend-store/src/components/QuoteResult.tsx` — импорт из shared; `frontend-store/src/ErrorBoundary.tsx` — удалён.
- `frontend-store/.env.example` — документирован `VITE_SENTRY_DSN`.

## Решения / заметки

- `hashEmail` тестируется с известным вектором SHA-256; в jsdom-тестах `crypto.subtle` подменён на Node WebCrypto (jsdom — «небезопасный контекст»).
- В `sentry.ts` — `import type` только для типов из @sentry/react (никакого runtime-импорта в стартовый чанк).
- Shared-слой менялся → типизация проверена в **обоих** приложениях (frontend + frontend-store), т.к. оба потребляют `frontend/shared`.

## Блокеры / остаток

- Реальный DSN — за человеком (S-118): код работает с пустым `VITE_SENTRY_DSN` (skip-init).
- Заливка sourcemap в Sentry — за человеком: задать `SENTRY_AUTH_TOKEN`/`SENTRY_ORG`/`SENTRY_PROJECT` в CI (локально можно через env build).
- Backend-часть S-140 (Go SDK init + скраб) — @builder, файлы не трогал (cmd/, internal/, go.mod, go.sum).

**Вердикт:** ✅ S-140-front готов — SDK лениво (не в стартовом чанке), Entry gzip 109.84/87.45 KB < 300 KB, PII-скраб + ErrorBoundary в обоих приложениях, tsc 0/0, unit 432+2100 c +13 новыми тестами, e2e не гонялись (нет DSN). Секреты/DSN — позже человеком.