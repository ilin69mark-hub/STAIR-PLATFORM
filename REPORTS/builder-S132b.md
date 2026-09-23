# S-132b: WS-клиент фронтов — shared + admin wiring

**Вердикт:** DONE — shared realtime-клиент, admin live-уведомления в ProjectDetail, 15 новых тестов, 419/419 unit, оба typecheck чистые, запушено в `dev/swarm/ws-s132b-client`.

## Что сделано
- `frontend/shared/src/api/realtime.ts` (новый) — framework-free WS-клиент: subscribe/unsubscribe с ack-wait (S-132a), автопереподключение (backoff+jitter, переподписка комнат), ping-keepalive (дедлайн сервера 60с), игнор pong/мусора, seam `createSocket` для тестов. Cookie-auth браузера, никакого ?token= (S-110).
- `frontend/shared/src/api/realtime.test.ts` — 8 тестов на FakeSocket (open/subscribe-ack/ack-timeout/event-routing/reconnect+resub/ping/unsubscribe/no-reconnect-after-disconnect).
- `frontend/src/lib/realtimeClient.ts` — синглтон /ws/admin (same-origin), fan-out по `pipeline:<configId>` из payload, изоляция сбойных хендлеров, no-op без WebSocket (jsdom/SSR).
- `frontend/src/lib/usePipelineRoom.ts` — хук: подписка на configurationId, notices (cap 5), live-флаг; `toNotice` маппит 3 типа событий + сырой fallback.
- `frontend/src/components/PipelineLiveLine.tsx` — строка «● live — Документ готов · spec · pdf» / «○ realtime offline» / null.
- `ProjectDetail.tsx` — wiring на `calculation.configuration_id`; `vite.config.ts` — dev-proxy /ws (ws:true, changeOrigin false) как зеркало nginx S-121.
- Тесты: `usePipelineRoom.test.tsx` (4), `PipelineLiveLine.test.tsx` (3).

## Гейты (frontend-зона, Go не тронут)
- unit admin: **47 файлов, 419 passed** (было 404 + 15 новых); ProjectDetail.test.tsx зелен без правок.
- `tsc -b` admin + store — 0 ошибок; oxlint новых файлов — чисто (set-state-in-effect ворнинг хука = прецедент кодобазы: CommentsPanel/AuthPage/AdminPanel).
- e2e WS осознанно пропущен: нет детерминированного триггера серверного события в e2e-окружении; follow-up при появлении WS-триггеров в API.

## Решение (скоуп)
- Store НЕ wired: анонимный флоу без engine configId — серверных событий для него нет; shared-клиент готов к переиспользованию. Зафиксировано, не скипнуто молча.
- Остаток S-132: S-132c (per-config authorization подписок).

## Счётчик CI
Задача 1/20 после PR #52.
