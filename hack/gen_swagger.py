#!/usr/bin/env python3
"""Актуальная OpenAPI-спецификация STAIR Platform, генерируемая из кода роутера.

Гарантия соответствия: internal/transport/http/swagger_sync_test.go проверяет,
что набор `paths` в этой спецификации совпадает с маршрутами, зарегистрированными
в internal/transport/http/router.go (метод+путь). При расхождении тест падает,
а правка выполняется командами:

    python3 hack/gen_swagger.py                             # проверить дифф
    python3 hack/gen_swagger.py --write                     # перегенерировать

Запускается без внешних зависимостей (только stdlib). Источники:
- маршруты: регэксп по .Handle(HANDLER)("...") в транспортном пакете http;
- описания: map SUMMARIES ниже (редактируется вручную при добавлении роутов).
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ROUTER_DIR = ROOT / "internal" / "transport" / "http"
OUT_SERVED = ROOT / "internal" / "transport" / "http" / "swagger" / "swagger.yaml"
OUT_DOCS = ROOT / "docs" / "openapi" / "swagger.yaml"

# Роуты, не входящие в REST-спецификацию (инфраструктура/мета/веб-сокет).
NON_REST = {"/", "/ws", "/swagger", "/docs/openapi/swagger.yaml", "/n/swagger.yaml"}

# Публичные (без bearerAuth) и webhook-маршруты.
PUBLIC = {
    ("GET", "/health"),
    ("GET", "/ready"),
    ("GET", "/metrics"),
    ("GET", "/auth/sso"),
    ("GET", "/auth/sso/callback"),
    ("GET", "/auth/sso/config"),
    ("POST", "/auth/login"),
    ("POST", "/auth/register"),
    ("GET", "/public/testimonials"),
    ("POST", "/public/orders"),
    ("POST", "/public/stairs:quote"),
    ("POST", "/public/stairs:validate"),
    ("POST", "/payments/webhook"),
    ("POST", "/payments/stripe/webhook"),
}

# Краткие описания (summary) по маршрутам. Fallback собирается из пути.
SUMMARIES = {
    ("POST", "/auth/register"): "Зарегистрировать пользователя",
    ("POST", "/auth/login"): "Войти в систему (сессионная cookie)",
    ("POST", "/auth/logout"): "Выйти из системы",
    ("GET", "/auth/me"): "Текущий пользователь",
    ("GET", "/auth/sso"): "Начать SSO-вход (redirect на провайдера)",
    ("GET", "/auth/sso/callback"): "Callback SSO-провайдера",
    ("GET", "/auth/sso/config"): "Настройки SSO для фронтенда",
    ("GET", "/projects"): "Список проектов",
    ("POST", "/projects"): "Создать проект",
    ("GET", "/projects/{id}"): "Проект по ID",
    ("POST", "/projects/{id}/calculate"): "Рассчитать проект (сквозной конвейер)",
    ("POST", "/projects/{id}/preview"): "Каркас решения для предпросмотра",
    ("POST", "/projects/{id}/optimize"): "Оптимизация вальсы-нормативов",
    ("POST", "/projects/{id}/review"): "Запрос ревью проекта",
    ("GET", "/projects/{id}/reviews"): "Ревью и подписи проекта",
    ("POST", "/projects/{id}/reviews/{reviewID}/sign-off"): "Подпись ревью",
    ("POST", "/projects/{id}/reviews/{reviewID}/changes"): "Правки по ревью",
    ("GET", "/projects/{id}/approvals"): "Согласования проекта",
    ("GET", "/projects/{id}/configurations"): "Конфигурации проекта",
    ("GET", "/projects/{id}/configurations/{configID}"): "Конфигурация проекта",
    ("GET", "/projects/{id}/configurations/{configID}/approval"): "Статус согласования конфигурации",
    ("POST", "/projects/{id}/configurations/{configID}/approve"): "Согласовать конфигурацию",
    ("POST", "/projects/{id}/configurations/{configID}/restore"): "Восстановить конфигурацию",
    ("GET", "/projects/{id}/comments"): "Комментарии проекта",
    ("POST", "/projects/{id}/comments"): "Добавить комментарий",
    ("DELETE", "/projects/{id}/comments/{commentID}"): "Удалить комментарий",
    ("GET", "/projects/{id}/members"): "Участники проекта",
    ("POST", "/projects/{id}/members"): "Пригласить участника",
    ("PATCH", "/projects/{id}/members/{userID}"): "Изменить роль участника",
    ("DELETE", "/projects/{id}/members/{userID}"): "Удалить участника",
    ("GET", "/projects/{id}/audit"): "Аудит действий по проекту",
    ("GET", "/projects/{id}/export"): "Экспорт проекта (JSON)",
    ("GET", "/projects/{id}/export/cad"): "Экспорт CAD (DWG)",
    ("POST", "/projects/{id}/export/cad/store"): "Записать CAD-чертёж в хранилище",
    ("POST", "/projects/{id}/checkout"): "Оформить заказ/оплату проекта",
    ("GET", "/projects/{id}/payments"): "Платежи проекта",
    ("POST", "/projects/{id}/quote-send"): "Отправить КП клиенту",
    ("POST", "/projects/{id}/order-send"): "Передать проект в производство",
    ("POST", "/projects/{id}/crm-sync"): "Синхронизировать проект с CRM",
    ("POST", "/stairs:calculate"): "Рассчитать лестницу по параметрам",
    ("POST", "/stairs:calculate/async"): "Фоновый расчёт (задача)",
    ("POST", "/stairs:optimize"): "Оптимизировать лестницу",
    ("POST", "/stairs:validate"): "Валидация входных параметров",
    ("POST", "/public/stairs:quote"): "Публичный расчёт стоимости",
    ("POST", "/public/stairs:validate"): "Публичная валидация параметров",
    ("GET", "/orders"): "Заказы пользователя",
    ("POST", "/orders"): "Создать заказ",
    ("GET", "/payments/{id}"): "Платёж по ID",
    ("POST", "/payments/webhook"): "Webhook платёжного провайдера",
    ("POST", "/payments/stripe/webhook"): "Webhook Stripe",
    ("GET", "/admin/orders"): "Заказы (админ)",
    ("PATCH", "/admin/orders/{id}/status"): "Сменить статус заказа",
    ("GET", "/admin/overview"): "Обзор метрик АП (сводка)",
    ("GET", "/admin/settings"): "Настройки (админ)",
    ("PUT", "/admin/settings"): "Обновить настройки",
    ("GET", "/admin/users"): "Пользователи (админ)",
    ("PATCH", "/admin/users/{id}"): "Изменить пользователя (роль/блокировка)",
    ("GET", "/admin/analytics/projects"): "Аналитика: проекты",
    ("GET", "/admin/analytics/usage"): "Аналитика: использование",
    ("GET", "/admin/analytics/manufacturing"): "Аналитика: производство",
    ("GET", "/admin/analytics/cost"): "Аналитика: стоимость",
    ("GET", "/admin/api-keys"): "API-ключи (админ)",
    ("POST", "/admin/api-keys"): "Создать API-ключ",
    ("DELETE", "/admin/api-keys/{id}"): "Отозвать API-ключ",
    ("GET", "/admin/testimonials"): "Отзывы (админ)",
    ("POST", "/admin/testimonials"): "Добавить отзыв (админ)",
    ("PATCH", "/admin/testimonials/{id}"): "Обновить отзыв (модерация)",
    ("DELETE", "/admin/testimonials/{id}"): "Удалить отзыв",
    ("GET", "/admin/export"): "Экспорт данных (админ)",
    ("GET", "/public/testimonials"): "Публичные отзывы",
    ("POST", "/public/orders"): "Публичная заявка на расчёт",
    ("GET", "/audit"): "Журнал аудита",
    ("POST", "/audit"): "Запись в журнал аудита",
    ("GET", "/assistant/{kind}"): "Ответ ассистента по разделу конфигурации",
    ("GET", "/jobs/{id}"): "Статус асинхронной задачи",
    ("GET", "/integrations/endpoints"): "Эндпоинты интеграций",
    ("POST", "/integrations/endpoints"): "Зарегистрировать endpoint",
    ("DELETE", "/integrations/endpoints/{id}"): "Удалить endpoint",
    ("GET", "/storage/{key}"): "Загрузка файла из хранилища",
    ("DELETE", "/storage/{key}"): "Удалить файл из хранилища",
    ("GET", "/health"): "Живучесть сервиса",
    ("GET", "/ready"): "Готовность (независимые проверки)",
    ("GET", "/metrics"): "Prometheus-метрики",
}

# RPC-маршруты с ':' в пути нельзя отразить в OpenAPI paths — документируем
# как расширение (стандартный приём для действий вида /resource:verb).
RPC = [
    "POST /stairs:calculate",
    "POST /stairs:calculate/async",
    "POST /stairs:optimize",
    "POST /stairs:validate",
    "POST /public/stairs:quote",
    "POST /public/stairs:validate",
]


def routes() -> list[tuple[str, str]]:
    """(method, path) из router.go: все Handle-регистрации, без /api/v1."""
    pat = re.compile(r'\.Handle(Func)?\(("(?:(?:POST|GET|PUT|PATCH|DELETE|OPTIONS)\s+)?[^"]+")')
    out = []
    for f in ROUTER_DIR.glob("*.go"):
        if f.name.endswith("_test.go"):
            continue
        for m in pat.finditer(f.read_text(encoding="utf-8")):
            raw = m.group(2).strip('"')
            if " " in raw:
                met, path = raw.split(" ", 1)
            else:
                met, path = "GET", raw
            if path.startswith("/api/v1"):
                path = path[len("/api/v1"):]
            path = path.replace("{key...}", "{key}")
            if path in NON_REST or ":" in path:
                continue
            out.append((met, path))
    return sorted(set(out), key=lambda mp: (mp[1], mp[0]))


def _tag(path: str) -> str:
    seg = path.strip("/").split("/", 1)[0]
    aliases = {
        "stairs": "stairs",
        "public": "stairs",
        "auth": "auth",
        "storage": "projects",
    }
    return aliases.get(seg, seg if seg in {
        "projects", "orders", "payments", "admin", "audit", "assistant",
        "jobs", "integrations", "health", "ready", "metrics",
    } else "other")


def _summary(m: str, p: str) -> str:
    if (m, p) in SUMMARIES:
        return SUMMARIES[(m, p)]
    segs = p.strip("/").split("/")
    verb = {"GET": "Получить", "POST": "Создать", "PUT": "Обновить",
            "PATCH": "Изменить", "DELETE": "Удалить"}.get(m, m)
    return f"{verb} {segs[-1]}"


def default_responses(m: str) -> str:
    ok = {"GET": "200", "POST": "201", "PUT": "200", "PATCH": "200",
          "DELETE": "200"}[m]
    resp = f'''        {ok}:
          description: OK
'''
    for code, desc in (("400", "Invalid request"), ("401", "Unauthorized"),
                       ("403", "Forbidden"), ("422", "Validation error"),
                       ("500", "Internal error")):
        if code == "401" and m in ("GET", "DELETE", "PATCH", "PUT"):
            continue
        if code == "403" and m in ("POST",):
            continue
        resp += f'''        {code}:
          description: {desc}
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
'''
    return resp


def render() -> str:
    out: list[str] = []
    add = out.append
    add('openapi: "3.0.3"')
    add("info:")
    add('  title: "STAIR Platform API"')
    add('  description: "API for stair configuration, pricing, and manufacturing"')
    add('  version: "1.0.0"')
    add("")
    add("servers:")
    add('  - url: "http://localhost:8080"')
    add('    description: "Local development"')
    add('  - url: "https://api.stairplatform.com"')
    add('    description: "Production"')
    add("")
    add("tags:")
    for tag, desc in [
        ("auth", "Authentication & registration"),
        ("projects", "Project management"),
        ("stairs", "Stair configurations"),
        ("orders", "Order management"),
        ("payments", "Payment processing"),
        ("admin", "Administration"),
        ("audit", "Audit log"),
        ("analytics", "Usage analytics"),
        ("assistant", "Configuration assistant"),
        ("jobs", "Async jobs"),
        ("integrations", "Integrations"),
        ("health", "Health checks"),
        ("metrics", "Prometheus metrics"),
    ]:
        add(f'  - name: "{tag}"')
        add(f'    description: "{desc}"')
    add("")
    add("paths:")
    by_path: dict[str, list[tuple[str, str]]] = {}
    for m, p in routes():
        by_path.setdefault(p, []).append((m, p))
    for path in sorted(by_path):
        add(f"  {path}:")
        for m, _ in sorted(by_path[path], key=lambda x: x[0]):
            pub = (m, path) in PUBLIC
            add(f"    {m.lower()}:")
            add(f'      tags: ["{_tag(path)}"]')
            add(f'      summary: "{_summary(m, path)}"')
            if not pub:
                add("      security:")
                add("        - bearerAuth: []")
            if m in ("POST", "PUT", "PATCH"):
                add("      requestBody:")
                add("        required: true")
                add("        content:")
                add("          application/json:")
                add("            schema:")
                add("              type: object")
            add("      responses:")
            add(default_responses(m))
    add("")
    add('  # Нестандартные RPC-маршруты с ":" в пути (см. x-rpc-routes) '
        "не могут быть ключами paths в OpenAPI 3.")
    add("x-rpc-routes:")
    for r in RPC:
        add(f"  - \"{r}\"")
    add("")
    add("components:")
    add("  securitySchemes:")
    add("    bearerAuth:")
    add("      type: http")
    add('      scheme: "bearer"')
    add('      description: "Токен из ответа /auth/login или API-ключ"')
    add("  schemas:")
    add("    Error:")
    add("      type: object")
    add("      required: [error]")
    add("      properties:")
    add("        error:")
    add("          type: string")
    add("          description: Описание ошибки")
    add("        request_id:")
    add("          type: string")
    add("          description: Идентификатор запроса для поддержки")
    add("    User:")
    add("      type: object")
    add("      properties:")
    add("        id:")
    add("          type: string")
    add("          format: uuid")
    add("        email:")
    add("          type: string")
    add("          format: email")
    add("        name:")
    add("          type: string")
    return "\n".join(out) + "\n"


def main() -> int:
    content = render()
    if "--write" in sys.argv[1:]:
        for out in (OUT_SERVED, OUT_DOCS):
            out.parent.mkdir(parents=True, exist_ok=True)
            out.write_text(content, encoding="utf-8")
        print(f"written: {OUT_SERVED} ({len(content.splitlines())} lines)")
        return 0
    changed = [str(p) for p in (OUT_SERVED, OUT_DOCS) if p.read_text(encoding="utf-8") != content]
    if changed:
        print("drift in: " + ", ".join(changed))
        print("run: python3 hack/gen_swagger.py --write")
        return 1
    print("in sync")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())