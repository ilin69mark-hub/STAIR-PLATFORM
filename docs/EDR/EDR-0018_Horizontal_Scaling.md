# STAIR PLATFORM

**Document:** EDR-0018_Horizontal_Scaling.md

**ID:** EDR-0018

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Scale

---

# 1. Purpose

Документ фиксирует горизонтальное масштабирование API (Phase H, H1) —
перевод сервиса в stateless-режим, допускающий запуск N реплик за
балансировщиком/load-balancer'ом с общим состоянием в PostgreSQL и Redis.
Скоуп H1:

1. **Readiness-проба** `GET /ready` — разграничение liveness и readiness:
   `/health` отвечает всегда, `/ready` — только когда подключения к
   зависимостям (БД, Redis) работоспособны. Балансировщик снимает реплику
   из ротации, пока `/ready` не вернёт 200.
2. **Stateless-аудит** — подтвердить, что инстанс API не хранит локального
   состояния, зависящего от реплики: сессии/полномочия/API-ключи в БД,
   rate-limit — общий Redis (memory — только fallback).
3. **Конфигурация инстанса** — `STAIR_INSTANCE_ID` (идентификация реплики
   в логах/метриках) и `STAIR_SHUTDOWN_TIMEOUT` (drain при graceful
   shutdown для плавного вывода реплики из ротации).

Реализация — как и фаза G, офлайн-безопасна (net/http, pgxpool, go-redis
уже в проекте), без новых внешних зависимостей.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase H — Scale, Horizontal Scaling)
- SEC-0003 (Identity and Authentication)
- SEC-0002 (Session Management)
- EDR-0014 (Advanced Security)
- API-0006 (Operational API: health/ready/metrics endpoints)
- ADR-0006 (Layered Architecture)
- BE-0005 (Infrastructure: PostgreSQL via pgxpool)

---

# 3. Model

## 3.1 Разграничение liveness и readiness

| Endpoint | Роль | Поведение |
|----------|------|-----------|
| `GET /health` | Liveness (k8s `livenessProbe`, LB health) | Всегда 200, без проверок зависимостей |
| `GET /ready` | Readiness (k8s `readinessProbe`, LB draining) | 200, если пробы БД и Redis успешны; 503 с деталями |

Liveness не должен зависеть от внешних систем (иначе недоступность БД
«убивает» реплику и k8s перезапускает её в цикле). Readiness, наоборот,
обязан отражать готовность принять трафик: если БД/Redis недоступны,
реплика не готова и снимается из ротации, но продолжает жить.

## 3.2 Readiness-пробы

Readiness проверяет две стратегические зависимости:

- **PostgreSQL**: `SELECT 1` через существующий пул `pgxpool.Pool` (Ping не
  всегда достаточно — `SELECT 1` выполняет реальный запрос).
- **Redis**: `PING` через клиент `go-redis` (если Redis сконфигурирован;
  при его отсутствии проба Redis пропускается — Redis необязательная
  зависимость, см. fallback EDR-0014).

Пробы выполняются с коротким таймаутом (по умолчанию 2с) и параллельно.

## 3.3 Статус ответа `/ready`

```json
200 OK
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

При недоступности хотя бы одной обязательной зависимости:

```json
503 Service Unavailable
{
  "status": "unavailable",
  "checks": {
    "database": "ok",
    "redis": "error: connection refused"
  }
}
```

`400`/`501` не используются; 503 — единственный код «не готов».

## 3.4 Stateless-аудит

Текущая архитектура уже stateless на уровне реплики:

- **Сессии** — в таблице `sessions` (БД, общая для всех реплик).
- **Полномочия/пользователи** — в БД.
- **API-ключи** — в БД (служебные токены, EDR-0016).
- **Rate-limit login/register** — распределённый через Redis
  (`ratelimit:{ip}`); memory-реализация — только fallback при недоступности
  Redis (EDR-0014 §4.2) и не влияет на распределённую согласованность в
  штатном режиме.
- **SSO state / PKCE** — в таблице `sso_states` (БД).

Инстанс не держит in-memory состояния, от которого зависят запросы
пользователей. Глобальные пакетные переменные (`cookieSecure`,
`maxBodyBytes`, лимитеры) — это конфигурация роутера, одинаковая на всех
репликах (инициализируется из одного окружения).

## 3.5 Конфигурация инстанса

| Env | Deфолт | Описание |
|-----|--------|----------|
| `STAIR_INSTANCE_ID` | `""` | Идентификатор реплики (например `api-1`); прокидывается в лог/метрики. Пусто — не логируется |
| `STAIR_SHUTDOWN_TIMEOUT` | `10s` | Таймаут graceful shutdown (drain) |

`STAIR_INSTANCE_ID` добавляется в structured log на старте и в каждый
лог-строку (через `slog`-контекст или поле при старте). Для H4 используется
как label метрик.

---

# 4. Invariants

```
1. /health никогда не зависит от БД/Redis (liveness). Всегда 200.
2. /ready возвращает 503, если недоступна БД (обязательная) или
   сконфигурированный Redis.
3. Пробы обязаны иметь короткий таймаут (<= 2s), чтобы /ready сам не
   зависал.
4. Реплики API stateless: всё состояние пользователей — в shared БД/Redis;
   локальные memory-структуры — только конфигурация или fallback.
5. STAIR_INSTANCE_ID, если задан, появляется в логах инстанса.
```

---

# 5. Schema

Без изменений схемы БД (миграция не требуется). Readiness использует
существующие подключения.

---

# 6. API

| Method | Path | Auth | Result |
|--------|------|------|--------|
| GET | `/ready` | public | 200 `{"status":"ok","checks":{...}}` или 503 |

`/ready` и `/health` — публичные, не требуют аутентификации и не подпадают
под rate-limit (иначе мониторинг/балансировщик сам блокируется — инвариант
H4).

---

# 7. Tests

- Transport: `/ready` 200, когда пробы успешны (mock-пров probe → ok);
  `/ready` 503, когда database-probe не удаётся (mock → error), redis-probe
  «пропускается», когда Redis не сконфигурирован; `/health` всегда 200 даже
  когда probe ошибается.
- Probe unit: `dbProbe` `SELECT 1` / `redisProbe` `PING` на реальном
  подключении (integration_test с БД) и через интерфейс (mock).

---

# 8. Acceptance Criteria

- `GET /ready` отвечает 200 при доступных БД/Redis и 503 при недоступной БД;
  `/health` всегда 200.
- `/ready` не блокирует балансировщик/мониторинг (имеет короткий таймаут,
  публичен).
- Инстанс запускается с `STAIR_INSTANCE_ID`, идентифицирующим реплику в
  логах.
- `STAIR_SHUTDOWN_TIMEOUT` управляет длительностью graceful shutdown.
- EDR-0018 помечен APPROVED; ROADMAP Phase H Horizontal Scaling CLOSED.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализовано (H1); APPROVED |

---

APPROVED
