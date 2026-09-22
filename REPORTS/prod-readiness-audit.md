# Аудит готовности к продакшену — STAIR PLATFORM

**Дата:** 2026-09-22 · **Ветка:** main @ d3319e3 + dev/swarm/ws-s115-s116 (7 коммитов, не смёржены)
**Метод:** параллельный аудит (security-auditor + tester) + ручной анализ архитектуры, доков, деплоя, roadmap.

---

## 1. Резюме проекта

**STAIR PLATFORM** — B2B-платформа проектирования и производства лестниц:
конструктор (4 типа маршей: straight/l_shape/u_shape/spiral) → расчёт →
оптимизация → геометрия → производственные данные (BOM/материалы) →
детерминированное ценообразование → экспорт (CAD/КП/PDF) → ревью/подпись
ревизий → заказ.

**Стек:** Go 1.24 (DDD: domain/application/infrastructure/transport) + PostgreSQL 16 +
Redis 7 + React 18/Vite/TS (админка `frontend/`, витрина `frontend-store/`) +
Three.js (3D-визуализация) + Stripe (оплата) + AI-ассистенты (4 вида) +
Helm/K8s + nginx + GitHub Actions (CI/CD).

**Масштаб кода:** ~60 Go-пакетов, 1795 Go-тестов, 2504 фронтенд-теста,
229 тестовых файлов, 210 коммитов, 51 PR.

---

## 2. Готовность по областям

| Область | Оценка | Статус |
|---|---|---|
| **Функциональность (MVP-цепочка)** | **90%** | Черновик→расчёт→оптимизация→ревью→подпись→экспорт→заказ работает end-to-end; e2e-матрица 400 проектов (report-400): кнопки 100%, 0 failed; 3D-рендер, КП (PDF 2 стр.), CAD-экспорт |
| **Архитектура** | **85%** | DDD-слои чистые (import-направление соблюдено), ADR/EDR покрывают ключевые решения; ⚠️ GraphQL — заглушка (5/15 операций, НЕ зарегистрирован в проде); WS-канал инертен (rooms не используются) |
| **Безопасность** | **8.5/10** | 0 CRITICAL/HIGH; 1 MEDIUM (rate-limit за прокси), 5 LOW, 3 INFO; gitleaks чисто (210 коммитов), govulncheck 0, npm audit 0; весь security-бэклог S-104…S-116 закрыт |
| **Тестирование** | **PASS** | Go 60/60 пакетов, покрытие **87.1% ≥ 85%**; admin 404/404 (86.4%), store 2100/2100 (85.3%); e2e 8/8 шардов зелёные; lint 0 issues; ⚠️ payments 80%, database 2.2% без БД |
| **Документация** | **95%** | 21 раздел docs/ (00-20), 28 roadmap-доков, ADR/EDR, OpenAPI swagger.yaml, манифесты по каждой области |
| **Деплой** | **55%** | Helm-чарт (secure defaults, ClusterIP), docker-compose (hardened), nginx, CI/CD; 🟥 **S-117: Docker-сборка фронтов сломана** (npm ci без lockfile); нет ingress-шаблона; EKS 1.28 EOL; CD-секреты не заданы |
| **Observability** | **40%** | Логи (JSON), метрики (/metrics), трейсинг (OTLP→Jaeger/Tempo) в коде есть; НЕТ развёрнутого мониторинга/алертов/error-tracking в проде |
| **Operations** | **20%** | НЕТ incident-процедур, rollback-доков, on-call, backup-верификации в проде (скрипты db-backup/restore есть, но не проверены на проде) |
| **Performance** | **60%** | k6 smoke.js есть; report-400: avg 1.1-17 с на марш (l_shape/u_shape ~16.5 с — расчёт тяжёлый); НЕТ формального baseline/SLO-верификации |

---

## 3. Общая готовность к продакшену: **~75%**

**MVP-функциональность готова (~90%)**, security и тесты — на уровне прода.
**Не хватает до запуска:** фикс Docker-сборки (S-117), CD-секреты, прод-инфраструктура
(мониторинг/алерты/бэкапы), ops-процедуры, закрытие MEDIUM/LOW-хвостов.

---

## 4. Блокеры продакшена (по приоритету)

| # | Блокер | Severity | Зона |
|---|---|---|---|
| 1 | **S-117: Docker-сборка фронтов сломана** — `store/admin.Dockerfile` делает `npm ci` без `package-lock.json` (root-workspaces миграция PR #36 удалила lockfile). Ломает `make env-up` и CD ghcr.io. CI run 35696977365: «Build and push Store» FAIL | 🔴 CRITICAL | deploy |
| 2 | **CD-секреты не заданы** — `STAIR_SECRETS_KEY` (64 hex) + AWS (ROLE_TO_ASSUME и др.) в GitHub Actions secrets; без них CD не деплоит | 🔴 CRITICAL | ops/человек |
| 3 | **WS role-гейт** — `/ws/admin` принимает любую валидную `session_admin` cookie без проверки роли; сегодня канал инертен (данных не раскрывает), но перед активацией realtime обязателен `hasPermission` | 🟠 HIGH (forward) | backend |
| 4 | **Rate-limit за L7-прокси** — per-IP лимитер (RemoteAddr-only) за nginx/ALB = общий бюджет на всех; lockout-DoS легитимных | 🟡 MEDIUM | backend |
| 5 | **EKS 1.28 EOL** — terraform production/main.tf:94 | 🟡 MEDIUM | infra |
| 6 | **nginx: нет security-заголовков, нет WS-проксирования** (store.conf/admin.conf) | 🟡 LOW | deploy |
| 7 | **Базовые образы aging** (alpine:3.20, node:22, nginx:1.27) + GHA на mutable-тегах | 🟡 LOW | deploy/ci |
| 8 | **docker-compose: postgres/redis на всех интерфейсах** (5432/6379) | 🟡 LOW | deploy |
| 9 | **Прод-инфраструктура**: мониторинг, алерты, error-tracking, backup-верификация, incident/rollback-процедуры | 🟠 HIGH (ops) | ops |

---

## 5. План доработок (приоритизированный)

### Фаза A — Разблокировать деплой (1-2 дня)
1. **S-117** — починить Docker-сборку: `npm ci` → `npm ci --workspaces` (или `npm install` с root-lockfile) в store/admin.Dockerfile; проверить `make env-up` + CD-публикацию.
2. Задать CD-секреты в GitHub Actions (STAIR_SECRETS_KEY, AWS_*).
3. Прогнать полный CI (правило 20/1: счётчик 6/20 — можно и раньше, т.к. это CI/деплой-зона).

### Фаза B — Закрыть security-хвосты (2-3 дня)
4. Rate-limit: доверять XFF только от закреплённых proxy-CIDR + поднять бюджеты за прокси.
5. nginx: security-заголовки (CSP/X-Frame-Options/X-Content-Type-Options), `server_tokens off`, проксирование /ws,/ws/admin (Upgrade/Connection).
6. docker-compose: bind 127.0.0.1 для postgres/redis.
7. WS role-гейт (`hasPermission`) на /ws/admin — до активации realtime.
8. Апгрейд базовых образов + SHA-pinning GHA (dependabot уже есть).

### Фаза C — Прод-инфраструктура (3-5 дней)
9. Ingress-шаблон для Helm (или явное решение: nginx + ClusterIP).
10. Мониторинг: Prometheus + Grafana (метрики уже отдаются), алерты (SLO: p95 login < 300 мс, calc < 30 с).
11. Error-tracking (Sentry) + трейсинг (Jaeger/Tempo — код готов).
12. Backup/restore: верифицировать db-backup.sh на staging, cron + retention.
13. EKS-апгрейд (1.28 → 1.30+) или решение по инфраструктуре.

### Фаза D — Ops-готовность (2-3 дня)
14. Incident-процедура, rollback-процедура (Helm rollback + миграции), on-call, контакты.
15. Production Launch Checklist (docs/19_ROADMAP/18) — пройти по пунктам и отметить.
16. Smoke-тесты после деплоя (health checks, /metrics, ключевые e2e).

### Фаза E — Функциональные долги (пост-MVP, не блокируют)
17. GraphQL: реализовать 15 операций или удалить из схемы/доков (сейчас заглушка).
18. WS realtime: подключить фронтенды (admin → /ws/admin, store → /ws) + role-гейт.
19. Payments-покрытие 80% → 85%+ (денежный контур — самый тонкий).
20. Performance: формальный baseline (k6) + оптимизация l_shape/u_shape расчёта (~16.5 с).
21. AI-фаза (Phase 9): ассистенты есть (D1-D4), но RAG/агентный фреймворк — пост-MVP.

---

## 6. Вердикт

**Проект готов к продакшену на ~75%.** MVP-функциональность, безопасность
(8.5/10) и тесты (87.1% покрытие, e2e зелёные) — на уровне прода. **Прямо сейчас
деплой невозможен из-за S-117 (сломанная Docker-сборка фронтов) и отсутствия
CD-секретов** — это 1-2 дня работы. После Фаз A+B (≈1 неделя) проект можно
выкатывать на staging; Фазы C+D (прод-инфраструктура и ops) — обязательны до
полноценного production launch по собственному чеклисту проекта.

**Рекомендация:** начать с S-117 (критический блокер), параллельно запросить у
человека CD-секреты; затем закрыть MEDIUM/LOW security-хвосты; staging-деплой →
верификация S-113 deployment-частей → прод.