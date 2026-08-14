# STAIR PLATFORM — Frontend (MVP-09)

React + Vite + TypeScript. Тонкий клиент (FE-0000): бизнес-логика и
расчёты выполняет Go-бэкенд; фронтенд вызывает REST API v1.

## Разработка

Backend (PostgreSQL + Redis + API) поднимается в Docker, фронтенд — локально:

```bash
make up   # старт: postgres:5432, redis:6379, api:8080 + миграции (из корня репозитория)
make fe   # dev-сервер http://localhost:5173
make stop # остановка Docker-стека
```

Dev-сервер на `http://localhost:5173`, проксирует `/api` на Go-бэкенд
(`http://localhost:8080`, переопределяется `STAIR_API_PROXY_URL`).

## Команды

- `npm run dev` — dev-сервер с HMR
- `npm run build` — production-сборка в `dist/`
- `npm run lint` — oxlint
- `npm run preview` — предпросмотр собранного `dist/`

## Структура

- `src/api/` — API-клиент и типы (FE-0011)
- `src/components/` — страницы и панели результата
- `src/lib/` — форматирование и конфигурация формы
