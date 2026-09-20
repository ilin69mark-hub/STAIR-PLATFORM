# STAIR PLATFORM

**Document:** EDR-0040_Session_Management.md

**ID:** EDR-0040

**Status:** DRAFT

**Author:** Swarm (@swarm)

**Date:** 2026-09-19

**Category:** Frontend Architecture

---

# 1. Purpose

Документ фиксирует модель единой обработки истёкшей/невалидной сессии в
обоих фронтах (admin :5174, store :3000, shared-слой). Решает аудит-задачу
S3-2 из `docs/00_REVIEW/06_AUDIT_FIX_PLAN_S1_S5.md`: «нет единой обработки
401 → неявные фолбэки» → централизованный интерцептор 401 → редирект на
логин.

---

# 2. Related Artifacts

- 06_AUDIT_FIX_PLAN_S1_S5.md (S3-2)
- SEC-0003 (Cookie-based session + CSRF double-submit)
- FE-0011 (API Client, `frontend-shared` слой)
- Branch `dev/swarm/s3-2-401`, PR #21

---

# 3. Model

## 3.1 Признак истёкшей сессии

Истёкшая/невалидная сессия определяется как **401 на защищённом эндпоинте**.
Эндпоинты `/api/v1/auth/*` из правила исключены: там 401 = «неверные или
повторные креды» — сброс сессии на провале логина создал бы зацикливание
(провалился логин → сброс → снова логин).

Refresh-token: сессионная cookie (httpOnly) ротируется сервером
(`middleware_auth.go`, sliding-выдача новой cookie при валидном токене).
Клиентская «переброска refresh-токена» отдельно не нужна — это обязанность
серверной middleware, фронт лишь реагирует на факт 401.

## 3.2 Шина 401 (`frontend/shared/src/api/session.ts`)

Один владелец логики, админ и store не дублируют:

- `onUnauthorized(handler) → unsubscribe` — подписка; возвращает функцию
  отписки (cleanup для React strict-mode в dev).
- `fireUnauthorized()` — вызов всех подписчиков со snapshot-итерацией и
  try/catch на подписчика (один сбойный не роняет остальных).
- `isAuthEndpoint(url)` — фильтр `/api/v1/auth/*`.

## 3.3 Интерцептор в API-клиентах

Admin (`frontend/src/api/client.ts`) и store (`frontend-store/src/api/client.ts`):
после `fetch`, при `res.status === 401 && !isAuthEndpoint(url)` → ровно **один**
`fireUnauthorized()` (до чтения тела, не в блоке `!res.ok`, чтобы не вызывать
дважды). Мутирующие запросы по-прежнему идут с CSRF-заголовками.

## 3.4 Реакция UI

`AuthProvider` каждого приложения: `useEffect(() => onUnauthorized(clear), [clear])`.
`clear` — `useCallback` без deps (стабилен), поэтому подписка ровно одна.
`clear()` → `user=null` → App рендерит логин-форму (admin: AuthPage; store:
`#cabinet` → AuthForm). «Редирект на логин» реализован как переход в
`!user`-состояние, без навигации уровня роутера.

---

# 4. Consequences

## 4.1 Positive

- Единая точка разбора 401 в обоих фронтах; нет неявных фолбэков.
- Покрыто e2e (DoD S3-2): «сессия протухла → защищённый запрос → 401 →
  UI на логине» для admin и store (перезапись httpOnly cookie на мёртвую в
  контексте Playwright).
- Устойчиво к strict-mode (подписка не дублируется).

## 4.2 Negative

- 401 на `/auth/*`-эндпоинтах шину не триггерит (сознательный компромисс).
- Подписчик вне React (например, сервисный слой) должен сам отписаться.

---

# 5. Alternatives Considered

- Axios-интерцептор с глобальным `window.location` на 401: отклонён — жёсткая
  перезагрузка, потеря состояния, дубли логики в двух приложениях.
- Дублирование обработчика в каждом `request`/AuthContext: отклонено — риск
  расхождения (именно так и было до S3-2).

---

# 6. Open Questions

- (Нет блокеров.) Возможный follow-up: юнит-тест на `session.ts` (S-101) и
  анти-регрессия 4 admin-спеков из-за tabbed-UI (S-100).