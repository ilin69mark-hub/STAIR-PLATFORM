# STAIR PLATFORM

**Document:** EDR-0014_Advanced_Security.md

**ID:** EDR-0014

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Enterprise

---

# 1. Purpose

Документ фиксирует Advanced Security (Phase G, Security): усиление
аутентификации и транспортной защиты поверх SEC-0003. Скоуп G2:
сессионная ротация (session rotation), rate limiting с распределённым
хранилищем (Redis, fallback на память) для login/register, и проверка
Origin/Referer поверх double-submit CSRF.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase G — Enterprise, Advanced Security)
- SEC-0003 (Identity and Authentication)
- SEC-0004 (Authorization Model)
- SEC-0005 (Tenant Isolation)
- ADR-0006 (Layered Architecture)
- EDR-0013 (Audit Log — журнал событий безопасности)

---

# 3. Model

## 3.1 Session rotation

Opaque session-токен (64 hex символа) ротируется при аутентификации,
если сессия «старая»: возраст сессии > половины TTL. Новая сессия
создаётся с новым токеном, старая удаляется; транспорт обновляет
session-cookie. Защита от session fixation и уменьшение окна действия
скопированного токена.

| Шаг | Описание |
|-----|----------|
| Authenticate | если age > TTL/2 → новый токен, старая сессия удаляется |
| Transport | при ротации выдаётся новый session-cookie (httpOnly) |

Инвариант: в любой момент существует не более одной активной сессии
на токен; ротация не увеличивает число активных сессий.

## 3.2 Rate limiting

Ограничение числа попыток входа и регистрации с одного IP за окно.
Лимитер — стратегия с интерфейсом `Allow(ctx, key) bool`:

- `memory` — sliding-window в памяти процесса (fallback).
- `redis` — счётчик через Redis (INCR + EXPIRE), распределённый.

При недоступности Redis происходит fallback на memory (запись в лог),
чтобы не ронять аутентификацию при сбое хранилища.

| Эндпоинт | Лимит по умолчанию | Окно |
|----------|--------------------|------|
| POST /auth/login | 10 | 1 мин |
| POST /auth/register | 5 | 1 мин |

Ответ при превышении — 429 `rate_limited`.

## 3.3 CSRF Origin/Referer

Поверх double-submit (заголовок X-CSRF-Token == csrf-cookie) проверяется
источник запроса:

- если заголовок Origin присутствует — его host должен совпадать с
  host запроса (r.Host) или входить в доверенный список;
- иначе проверяется Referer;
- при отсутствии обоих (не-браузерный клиент) — пропуск.

Недопустимый источник — 403 `csrf`.

---

# 4. Invariants

```
1. Ротация сессии не создаёт дополнительные активные сессии.
2. Лимитер падает на memory при недоступности Redis (не блокирует вход).
3. Origin/Referer проверяется только при наличии заголовка (совместимость
   с API-клиентами); double-submit CSRF остаётся обязательным.
4. Лимиты по умолчанию настраиваются через конфигурацию HTTP-слоя.
5. Ротация и отказ в доступе фиксируются в аудите (EDR-0013).
```

---

# 5. API

Изменений внешнего API нет: поведение усиливается (429, 403 на
cross-origin мутацию), новые ответы документируются здесь.

| Ответ | Когда |
|-------|-------|
| 429 | превышение лимита login/register |
| 403 `csrf` | несовпадение токена или недопустимый Origin/Referer |

---

# 6. Tests

- Auth service: ротация при старой сессии, отсутствие ротации для свежей,
  старая сессия удалена после ротации.
- Limiter: интерфейс + memory (окно сбрасывается), redis (через реальный
  Redis в integration, fallback при недоступности).
- Transport: 429 на login/register после лимита; CSRF Origin/Referer
  (совпадение, несовпадение, отсутствие заголовка).
- Integration: ротация выставляет новый session-cookie, старый токен
  перестаёт работать.

---

# 7. Acceptance Criteria

- Session rotation реализована в application/auth и transport (cookie
  обновляется при ротации).
- Rate limiting применяется к login и register; Redis-стратегия с
  fallback на memory.
- CSRF проверяет Origin/Referer поверх double-submit.
- Лимиты и флаги конфигурируемы (Config HTTP-слоя, env в main).
- EDR-0014 помечен APPROVED; ROADMAP Phase G Advanced Security CLOSED.

---

# 8. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализация end-to-end; status APPROVED |

---

APPROVED
