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

ERP

CRM

Manufacturing

Storage

Payments

---

# Phase F — Analytics

Usage Analytics

Project Analytics

Manufacturing Analytics

Cost Analytics

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
