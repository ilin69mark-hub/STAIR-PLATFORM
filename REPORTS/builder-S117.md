# builder — S-117: Docker-сборка фронтов под root workspaces

**Задача:** S-117 [P1] — `store/admin.Dockerfile` делали `COPY frontend(-store)/package*.json && npm ci`,
но PR #36 (root npm workspaces) удалил per-app lockfile → `npm error EUSAGE` в CI
(run 35696977365 «Build and push Store» FAIL), сломаны `make env-up` и CD ghcr.io.

**Выполнено:** координатором (self-execute).

## Изменения

- `deployments/store.Dockerfile` — build stage на root workspaces: `COPY package.json
  package-lock.json` + оба workspace-манифеста → один `npm ci` в /app (hoisted
  /app/node_modules) → `COPY frontend/shared + frontend-store` → `npm run build`
  в /app/frontend-store. Убран костыль-симлинк node_modules (не нужен:
  shared резолвится подъёмом к /app/node_modules).
- `deployments/admin.Dockerfile` — аналогично: `COPY frontend ./frontend` (контекст
  уже подрезан .dockerignore до нужных файлов) → `npm run build` в /app/frontend.
- `.dockerignore` — убран re-include несуществующего `frontend/package-lock.json`;
  добавлены `**/node_modules` + `**/dist` (локальные артефакты не едут в контекст;
  соответствует исходному комментарию файла).

## Проверка (локально)

| Гейт | Результат |
|---|---|
| `docker build -f deployments/store.Dockerfile` | ✅ образ собран (npm ci по root lock + `tsc -b && vite build` прошли) |
| `docker build -f deployments/admin.Dockerfile` | ✅ образ собран |
| smoke: `ls /usr/share/nginx/html` в обоих образах | ✅ index.html + assets (+ robots.txt / favicon.svg) |
| Go-гейты | N/A (зона deploy, Go-код не тронут) |

Покрытие алиаса @shared: store `../frontend/shared/src` (tsc paths) и admin
`./shared/src` — оба присутствуют в образе на этапе build (проверено успешной сборкой).

**Вердикт:** S-117 закрыт — обе витрины собираются из root lockfile, `make env-up`
и CD-публикация разблокированы (CI-верификация — по правилу 20/1, счётчик 7/20).

**Длительность:** ~25 мин (self-execute).