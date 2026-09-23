# Ревью PR #44 — vitest vmThreads pool + seam onExport

**Вердикт:** APPROVED
**Ветка:** dev/swarm/vitest-speed → main
**Проверено:** локально (CI ⚙️ BLOCKED_INFRA — billing)

## Изменения
1. `frontend/vitest.config.ts` — `pool: 'vmThreads'`.
2. `frontend-store/vitest.config.ts` — `threads` → `vmThreads`.
3. `AdminPanel.tsx` — опциональный prop `onExport?: (url: string) => void`, default — `window.location.href` (поведение не меняется).
4. `AdminPanel.test.tsx` — тест экспорта через `onExport` вместо мока `window.location`.

## Верификация

| Проверка | Результат |
|---|---|
| frontend `vitest run` | 44 файла / **404 passed**, стабильно ×2 (2.65s / 3.22s; было 5.41s) |
| frontend-store `vitest run` | 13 файлов / **2100 passed**, 3.70s |
| frontend coverage | stmt 86.43 / branch 80.6 / func 88.09 / lines 88.6 (пороги сходятся) |
| frontend-store coverage | stmt 85.32 / branch 87.19 / func 82.66 / lines 86.86 (пороги сходятся) |
| typecheck / lint / build | без ошибок (warnings pre-existing) |

## Зачем
Vitest 5 после #43 предупреждал: jsdom-окружение создаётся на каждый тест-файл (44 создания, 60% времени). `vmThreads` сохраняет изоляцию между файлами (в отличие от `isolate: false` — без риска флейков от shared state), при этом окружение инсталлируется один раз на воркер.

## Тонкость
Под `vmThreads` свойство `window.location` jsdom не переопределяется (`Cannot redefine property: location`) — нельзя ни `vi.spyOn(window,'location','get')`, ни `vi.stubGlobal('location')`. Поэтому для теста экспорта добавлен seam `onExport`. Прочие тесты `window.location` не мокают — регрессий нет.

## Вывод
Merge безопасен. Прогоны стабильны, покрытие в порогах, поведение компонента не изменилось.