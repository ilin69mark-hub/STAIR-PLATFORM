# STAIR PLATFORM — Frontend (MVP-09)

React + Vite + TypeScript. Тонкий клиент (FE-0000): бизнес-логика и
расчёты выполняет Go-бэкенд; фронтенд вызывает REST API v1.

## Разработка

```bash
npm install
npm run dev
```

Dev-сервер на `http://localhost:5173`, проксирует `/api` на Go-бэкенд
(`http://localhost:8080`, переопределяется `STAIR_API_PROXY_URL`).
Бэкенд должен быть запущен с `STAIR_DATABASE_URL`:

```bash
STAIR_DATABASE_URL="postgres://stair:stair@localhost:5433/stair_platform?sslmode=disable" \
  STAIR_HTTP_ADDR=:8080 go run ./cmd/api
```

## Команды

- `npm run dev` — dev-сервер с HMR
- `npm run build` — production-сборка в `dist/`
- `npm run lint` — oxlint
- `npm run preview` — предпросмотр собранного `dist/`

## Структура

- `src/api/` — API-клиент и типы (FE-0011)
- `src/components/` — страницы и панели результата
- `src/lib/` — форматирование и конфигурация формы
