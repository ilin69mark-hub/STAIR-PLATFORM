# `21_ROADMAP/11_POST_MVP_ROADMAP.md`

```markdown
# STAIR PLATFORM

Document: 11_POST_MVP_ROADMAP.md

ID: ROADMAP-0011

Status: APPROVED

---

# Purpose

Определяет направления развития после MVP.

---

# Phase A — Product Expansion

Advanced Stair Types — **CLOSED 2026-08-14**: L-образный (EDR-0005), П-образный (EDR-0006), спиральный (EDR-0007) марши реализованы end-to-end (solver → geometry → manufacturing → API → frontend → e2e).

Advanced Geometry — **CLOSED 2026-08-14**: параметрическая генерация твёрдотельных моделей всех типов маршей, включая спираль с центральной колонной (веерные проступи, сегментация дуг).

Advanced Manufacturing — **CLOSED 2026-08-14**: разложение на детали (косоуры, ступени, подступенки, площадки, колонна), BOM, раскрой и стоимостной конвейер для всех типов маршей.

Advanced Pricing — **CLOSED 2026-08-14**: полный ценовой конвейер (материал → станок → труд → накладные → маржа → НДС) для всех типов маршей.

---

# Phase B — Engineering Expansion

Advanced Optimization

Performance Optimization

Parallel Execution

Distributed Processing

---

# Phase C — Collaboration

Multi-user Projects — **CLOSED 2026-08-15**: реализовано end-to-end (C1, EDR-0008) — владение проектами и роли owner/editor/viewer.

Project Sharing — **CLOSED 2026-08-15**: реализовано end-to-end (C2) — приглашение участников по email и просмотр «shared with me».

Comments — **CLOSED 2026-08-15**: реализовано end-to-end (C3, EDR-0009) — ветки обсуждений на проектах.

Review — **CLOSED 2026-08-15**: реализовано end-to-end (C4, EDR-0010) — запрос ревью, подпись, запрос изменений.

Approval — **CLOSED 2026-08-15**: реализовано end-to-end (C5, EDR-0011) — согласование конфигураций с историей решений.

Versioning — **CLOSED 2026-08-15**: реализовано end-to-end (C6, EDR-0012) — ревизии конфигураций, текущая версия, восстановление.

---

# Phase D — AI

AI Design Assistant

AI Engineering Assistant

AI Manufacturing Assistant

AI Pricing Assistant

---

# Phase E — Integrations

CAD — **DONE 2026-08-15**: реализовано E1, EDR-0022 — CAD-экспорт сетки
конфигурации в форматы DXF (R12 ASCII, 3DFACE), STL (ASCII) и SVG
(2D-проекция) на чистой stdlib; `internal/infrastructure/cad` (pure writers),
`project.Service.ExportCAD` (детерминированный пересчёт mesh из сохранённой
конфигурации), `GET /api/v1/projects/{id}/export/cad?format=dxf|stl|svg`,
unit-тесты writers и endpoint (ADR-0006/DEV-0009).

ERP — **DONE 2026-08-15**: реализовано E2, EDR-0023 — webhook-платформа
deliveries (HMAC-SHA256 `X-Stair-Signature`, replay-защита по timestamp) +
интеграция ERP: таблицы `integration_endpoints`/`integration_events`
(миграция 000014), `internal/infrastructure/integrations` (pure client +
verify, stdlib-only), `internal/application/integrations` (service с
регистрацией эндпоинтов и постановкой событий в очередь),
`internal/infrastructure/database/integration_repo.go`,
worker-обработчик `erp.quote_send` (доставка + статусы
pending/delivered/failed/dlq), админ-эндпоинты
`/api/v1/integrations/endpoints` и `POST /api/v1/projects/{id}/quote-send`
(привилегия `integration.manage`, wire-тесты + DB-интеграционные тесты).

CRM — **DONE 2026-08-15**: реализовано E3, EDR-0024 — синхронизация
метаданных проекта в CRM на webhook-каркасе E2 (kind `crm`): событие
`crm.project_sync` (канонический документ проекта: project_id, name,
description, status, owner_id, created/updated_at), эндпоинт
`POST /api/v1/projects/{id}/crm-sync` (owner/editor, 202/403/404/422),
сервисный `SyncProject` (общий `enqueue` вынесен из `SendQuote`), общий
воркер-обработчик `deliverEvent` для `erp.quote_send` и `crm.project_sync`
(диспетчер реестра выведен на обе ветки), unit/worker/transport-тесты.
Без новой миграции (таблицы 000014 поддерживают kind=crm).

Manufacturing — **DONE 2026-08-15**: реализовано E4, EDR-0025 — передача
производственного заказа в MES на webhook-каркасе E2/E3 (kind `mes`):
событие `mes.order_send` (канонический документ заказа: parts/bom/cut_list/
nesting в snake_case), эндпоинт `POST /api/v1/projects/{id}/order-send`
(owner/editor, 202/403/404/422 no_endpoint/no_manufacturing), сервисный
`SendManufacturingOrder` через общий `enqueue`, доставка через общий
`deliverEvent` (JobOrderSend на ту же ветку), unit/worker/transport-тесты.
Без новой миграции.

Storage — **DONE 2026-08-15**: реализовано E5, EDR-0026 — абстракция
объектного хранилища `internal/infrastructure/storage` (ObjectStore
Put/Get/Delete, ValidationKey, фабрика по `STAIR_STORAGE_BACKEND`), два
бэкенда: filesystem (корень `STAIR_STORAGE_DIR`, защита от выхода за root)
и S3-совместимый с подписью AWS SigV4 на чистой stdlib (path-style для
MinIO, `STAIR_S3_*`); прикладной сервис `internal/application/storage`
(tenant-скоуп ключей, SaveExport/Load/Delete); транспорт:
`POST /api/v1/projects/{id}/export/cad/store` (экспорт E1 в хранилище),
`GET/DELETE /api/v1/storage/{key...}` (скоуп tenant);
unit-тесты SigV4 (эталон AWS), fs, s3 (httptest), app и транспорта.

Payments — **DONE 2026-08-15**: реализовано E6, EDR-0027 — приём платежей
за проект через внешний PSP (mock-эмулятор на чистой stdlib): checkout-
сессии (`POST /api/v1/projects/{id}/checkout`, возвращает checkout_url),
входящий webhook с подтверждением оплаты (`POST /api/v1/payments/webhook`,
HMAC-SHA256 верификация — та же трубка, что EDR-0023 §3.1/§3.2), модель
`payment_intents`/`payment_events` (миграция 000015), статусы
pending/paid/failed/refunded, журнал событий webhook для аудита; список
платежей и статус интента (`GET .../payments`, `GET /api/v1/payments/{id}`);
unit/DB/транспорт-тесты + E2E-проверка подписанным webhook. **Phase E —
полностью CLOSED (E1–E6).**

---

# Phase F — Analytics

Usage Analytics — **DONE 2026-08-15** (F1, EDR-0028): агрегация метрик tenant по
существующим таблицам без новой миграции — пользователи, проекты, расчёты
(`calculations ⋈ projects` по tenant), аудит-события (входы/экспорты) и оплаченные
интенты; право `analytics.read` (RBAC, RoleAdmin); `GET /api/v1/admin/analytics/usage`
с `from`/`to` (YYYY-MM-DD | RFC3339) и `granularity` (day/week/month, серия
непрерывная через `generate_series`); панель «Аналитика использования» в
AdminPanel (итоги + таблица по бакетам, переключатель гранулярности);
unit/DB/транспорт-тесты.

Project Analytics — **DONE 2026-08-15** (F2, EDR-0029): агрегаты tenant по
проектам (всего/созданные в окне, распределение по статусам, проекты с
расчётом и валидные, конфигурации/расчёты/комментарии) и сводка по каждому
проекту (статус, владелец, конфигурации, расчёты, валидность последнего
расчёта, комментарии, участники); `GET /api/v1/admin/analytics/projects`
(та же трубка `analytics.read`/`queryTime`); панель «Проекты» в AdminPanel;
unit/DB/транспорт-тесты.

Manufacturing Analytics — **DONE 2026-08-15** (F3, EDR-0030): JSONB-агрегаты
из снапшотов расчётов (`calculations.result->'manufacturing'`) — детали, BOM,
карта раскроя, листы, площади (детали/листы/отходы), средняя утилизация,
распределение по материалам; серии по бакетам day/week/month (непрерывные,
`generate_series`); `GET /api/v1/admin/analytics/manufacturing` (та же трубка
`analytics.read`/`queryTime`/гранулярности); панель «Производство» в
AdminPanel; unit/DB/транспорт-тесты.

Cost Analytics — **DONE 2026-08-15** (F4, EDR-0031): JSONB-агрегаты из
снапшотов расчётов (`calculations.result->'pricing'`) — себестоимость
(материал/машина/труд/накладные), цепочка цены (прибыль/налог/итоговая
цена), средняя итоговая цена и валюта; серии по бакетам day/week/month
(непрерывные, `generate_series`); `GET /api/v1/admin/analytics/cost` (та же
трубка `analytics.read`/`queryTime`/гранулярности); панель «Стоимость» в
AdminPanel; unit/DB/транспорт-тесты. **Phase F — полностью CLOSED
(F1–F4).**

---

# Phase G — Enterprise

Advanced Security — **CLOSED 2026-08-15**: реализовано end-to-end (G2, EDR-0014) — ротация сессий, rate limiting login/register (Redis с fallback на память), проверка Origin/Referer поверх double-submit CSRF.

Audit — **CLOSED 2026-08-15**: реализовано end-to-end (G1, EDR-0013) — персистентный журнал событий безопасности с записью из auth/project сервисов, API чтения (проект + admin) и панелью «Аудит».

SSO — **CLOSED 2026-08-15**: реализовано end-to-end (G5, EDR-0017) — OIDC Authorization Code flow с PKCE на стандартной библиотеке (Discovery, token exchange, JWKS RS256-верификация id_token), provisioning по email (автосоздание/привязка в default tenant), единый вход в общую session-сессию, кнопка «Войти через …» на AuthPage, аудит sso.login/sso.login_denied/sso.linked. Phase G — **полностью CLOSED**.

Enterprise Controls — **CLOSED 2026-08-15**: реализовано end-to-end (G4, EDR-0016) — admin-панель (пользователи: роль/статус, политики безопасности per-tenant, экспорт данных JSON/CSV), API-ключи (service-токены со scope-правами, Bearer-доступ), аудит всех admin-действий.

Advanced Permissions — **CLOSED 2026-08-15**: реализовано end-to-end (G3, EDR-0015) — permission-модель (RBAC) в auth (admin/user) и project (owner/editor/viewer) с единым механизмом HasPermission вместо CanEdit/CanManage и inline-сравнений; admin-эндпоинты управления пользователями (список и смена роли, tenant-скоуп, запрет смены собственной роли).

---

# Phase H — Scale

Horizontal Scaling — **CLOSED 2026-08-15**: реализовано (H1, EDR-0018) — разграничение liveness/readiness, эндпоинт `/ready` (SELECT 1 по БД + Redis PING, 200/503), пробы с коротким таймаутом, конфигурация инстанса `STAIR_INSTANCE_ID`/`STAIR_SHUTDOWN_TIMEOUT`, stateless-аудит (состояние в shared БД/Redis).

Regional Deployment — **CLOSED 2026-08-15**: реализовано (H2, EDR-0019) — региональный конфиг-слой `STAIR_REGION` (отражается в `/health` и логах), документация топологии multi-region (blue/green, canary, LB-пробы `/health`+`/ready`), пример docker-compose `deployments/multi-region.example.yml`, развёртывание (README в `deployments/`).

Distributed Infrastructure — **CLOSED 2026-08-15**: реализовано (H3, EDR-0020) — система фоновых заданий: JobQueue (распределённая на Redis List + in-memory fallback), процесс воркера `cmd/worker` (BRPOP-потребитель, retry/backoff, max-attempts, graceful shutdown, периодический таймер очистки), очистка истёкших сессий, sso_states и аудит-ретенции (новые repo-методы DeleteExpiredSessions/DeleteExpiredSsoStates/DeleteBefore).

Advanced Observability — **CLOSED 2026-08-15**: реализовано (H4, EDR-0021) — собственный реестр метрик на чистой stdlib (Counter/Histogram/Gauge на sync/atomic, Prometheus text-формат), публичный эндпоинт `/metrics` (HTTP-метрики: count/duration; runtime: goroutines/mem/uptime; labels region/node), канонизация path в label ({id}), расширенные HTTP-логи (remote_ip, user_agent). Phase H — **полностью CLOSED**.

---

# Prioritization

Каждая post-MVP capability должна пройти:

Value Assessment

Technical Assessment

Risk Assessment

Operational Assessment

---

APPROVED
``` id="h2kzqk"

---
