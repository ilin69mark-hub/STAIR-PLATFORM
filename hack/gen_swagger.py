#!/usr/bin/env python3
"""Актуальная OpenAPI-спецификация STAIR Platform, генерируемая из кода роутера.

Гарантия соответствия:
  1. internal/transport/http/swagger_sync_test.go — набор `paths` совпадает с
     маршрутами router.go (метод+путь);
  2. internal/transport/http/swagger_contract_test.go — контрактные элементы
     (base path в servers, схема ошибки, security, параметры) не «разъезжаются»;
  3. `make openapi-check` (или `python3 hack/gen_swagger.py`) — ни одна копия
     файла не отличается от сгенерированной.

Правка:
    python3 hack/gen_swagger.py            # проверить дифф (exit 1 при дрейфе)
    python3 hack/gen_swagger.py --write    # перегенерировать обе копии

Источники: роуты — регэксп по .Handle(...) в транспортном пакете; описания,
коды ответов и параметры — таблицы ниже (их нужно править вручную при
добавлении роутов). Никаких внешних зависимостей, только stdlib.

API-003/DOC-001 (2026-09-26): до этого регенерация ПОЛНОСТЬЮ ТЕРЯЛА контракт:
  * в `servers` не было base path /api/v1 → любой сгенерированный клиент
    получал 404 на 100% операций;
  * схема Error объявляла `error` строкой, а код отдаёт объект {code, message};
  * CSRF-контракт (X-CSRF-Token, cookie csrf) и session-cookie аутентификация
    не были описаны вовсе → клиент по спеке получал 403 csrf на всех ~40
    изменяющих запросах;
  * path-параметры не объявлялись ни для одного пути (нарушение OAS 3.0.3
    §4.8.10), обязательные query-параметры (?format, ?scope) отсутствовали;
  * не документированы ни поля конфигурации (flight, turn_kind, winder_count,
    railing*, comfort_step_mm, run_mm), ни реальные коды ответов (17 расхождений,
    в т.ч. 202 и 204).
Всё это исправлено ниже: спека генерируется из таблиц, а таблицы проверяются
тестами контракта.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ROUTER_DIR = ROOT / "internal" / "transport" / "http"
OUT_SERVED = ROOT / "internal" / "transport" / "http" / "swagger" / "swagger.yaml"
OUT_DOCS = ROOT / "docs" / "openapi" / "swagger.yaml"

# Базовый путь API. Пути в спецификации хранятся БЕЗ этого префикса, поэтому
# он обязан быть в servers.url, иначе клиент строит запросы от корня сервера.
# API-003: до фикса здесь был `http://localhost:8080` — 100% операций давали 404.
BASE_PATH = "/api/v1"

# Роуты, не входящие в REST-спецификацию (инфраструктура/мета/веб-сокет).
# Синхронизировано с nonRESTRoutes в internal/transport/http/swagger_sync_test.go.
NON_REST = {"/", "/ws", "/ws/admin", "/swagger", "/docs/openapi/swagger.yaml", "/n/swagger.yaml"}

# Публичные маршруты (без аутентификации): витрина, регистрация/вход, вебхуки
# провайдера, health. Синхронизировано с PUBLIC_ROUTES в swagger_contract_test.go.
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
    ("GET", "/public/materials"),
    ("GET", "/public/payment-tiers"),
    ("GET", "/public/store-settings"),
    ("POST", "/public/orders"),
    ("POST", "/public/stairs:quote"),
    ("POST", "/public/stairs:validate"),
    ("POST", "/payments/webhook"),
    ("POST", "/payments/stripe/webhook"),
}

# Реальные success-коды операций. API-003: генератор раньше выдавал 201 для
# каждого POST и 200 для каждого DELETE, хотя код возвращает 200/202/204
# (17 расхождений, ломающих строгих клиентов). Здесь перечислены отклонения от
# умолчания; остальное берётся из default_success_code().
SUCCESS_CODES = {
    ("POST", "/auth/login"): ("200", "Сессия создана, установлена session cookie"),
    ("POST", "/auth/logout"): ("204", "Сессия завершена"),
    ("POST", "/assistant/{kind}"): ("200", "Ответ консультанта по конфигурации"),
    ("POST", "/payments/webhook"): ("200", "Webhook принят"),
    ("POST", "/payments/stripe/webhook"): ("200", "Webhook принят"),
    ("POST", "/projects/{id}/calculate"): ("200", "Расчёт выполнен"),
    ("POST", "/projects/{id}/preview"): ("200", "Предпросмотр выполнен"),
    ("POST", "/projects/{id}/optimize"): ("200", "Оптимизация выполнена"),
    ("POST", "/projects/{id}/configurations/{configID}/restore"): ("200", "Ревизия восстановлена"),
    ("POST", "/projects/{id}/crm-sync"): ("202", "Задача синхронизации принята"),
    ("POST", "/projects/{id}/order-send"): ("202", "Задача отправки заказа принята"),
    ("POST", "/projects/{id}/quote-send"): ("202", "Задача отправки КП принята"),
    ("DELETE", "/admin/store/prices/{code}"): ("204", "Цена удалена"),
    ("DELETE", "/admin/testimonials/{id}"): ("204", "Отзыв удалён"),
    ("DELETE", "/integrations/endpoints/{id}"): ("204", "Эндпоинт удалён"),
    ("DELETE", "/projects/{id}/comments/{commentID}"): ("204", "Комментарий удалён"),
    ("DELETE", "/projects/{id}/members/{userID}"): ("204", "Участник удалён"),
    ("POST", "/projects/{id}/export/cad/store"): ("201", "Объект сохранён в Storage"),
    # API-004 (2026-09-26): расхождения статусов спека↔код, найденные сверкой
    # генератора с хендлерами. Код — источник истины, спека приведена в
    # соответствие (а не наоборот: 204 без тела для удаления ключа — это
    # осознанное решение обработчика).
    ("DELETE", "/admin/api-keys/{id}"): ("200", "Ключ отозван"),
    ("DELETE", "/assistant/memory"): ("200", "История очищена"),
    ("POST", "/admin/payments/{id}/refund"): ("200", "Возврат оформлен"),
    ("GET", "/auth/sso"): ("302", "Редирект на SSO-провайдера"),
    ("GET", "/auth/sso/callback"): ("302", "Редирект обратно в приложение"),
    ("POST", "/projects/{id}/members"): ("201", "Участник добавлен"),
}

# Обязательные query-параметры. API-003: ?format= и ?scope= обязательны в коде
# (пустое значение → 400 invalid_input / 422 invalid_scope), но в спеке не были
# объявлены — клиент по спеке получал ошибку.
REQUIRED_QUERY = {
    ("GET", "/projects/{id}/export/cad"): [
        ("format", "Формат выгрузки", ["dxf", "stl", "svg"]),
    ],
    ("POST", "/projects/{id}/export/cad/store"): [
        ("format", "Формат выгрузки", ["dxf", "stl", "svg"]),
    ],
    ("GET", "/admin/export"): [
        ("scope", "Область выгрузки", ["users", "projects", "audit"]),
    ],
}

# Необязательные query-параметры пагинации.
#
# API-003: пагинация обязательна в описании — без верхней границы page*per_page
# переполнял int32 и давал 500 (API-001: MaxPage=1_000_000, MaxPerPage=100).
#
# Список операций взят ИЗ КОДА: ParsePagination вызывается ровно в четырёх
# хендлерах (handleListProjects, handleListComments, handleListReviews,
# handleListApprovals). Раньше генератор добавлял page/per_page вообще всем
# 66 операциям подряд (включая POST /auth/login и PUT /admin/settings), что
# делало спеку врущую. Тест TestSwaggerPaginationOnlyWhereCodeHasIt сверяет
# этот список с местом вызова ParsePagination.
OPTIONAL_QUERY_PAGINATION = {
    ("page", "Номер страницы, начиная с 1", "integer", "1", "1000000"),
    ("per_page", "Размер страницы (максимум 100)", "integer", "1", "100"),
}

# Операции с пагинацией: (метод, путь) → (handler, router path)
PAGINATED_OPERATIONS = {
    ("GET", "/projects"): "handleListProjects",
    ("GET", "/projects/{id}/comments"): "handleListComments",
    ("GET", "/projects/{id}/reviews"): "handleListReviews",
    ("GET", "/projects/{id}/approvals"): "handleListApprovals",
}

# Схема тела расчёта конфигурации: поля calculateRequest (internal/transport/
# http/dto.go). API-003: ни одно поле не было документировано — интегратор не
# знал даже про `flight`. Здесь перечислены с типами и перечислителями.
CONFIG_INPUT_PROPERTIES: list[tuple[str, str, str, str | None, str]] = [
    # (json-имя, тип, формат/описание, enum|None, required?)
    ("width_mm", "number", "double", None, "true"),
    ("height_mm", "number", "double", None, "true"),
    ("flight", "string", None, ["straight", "l_shape", "u_shape", "spiral"], "true"),
    ("material", "string", None, ["STEEL-S235", "WOOD-OAK",
                                  "WOOD-WALNUT", "WOOD-ASH", "WOOD-SOFT"], ""),
    ("step_height_mm", "number", "double", None, "true"),
    ("stringer_thickness_mm", "number", "double", None, "true"),
    ("step_thickness_mm", "number", "double", None, "true"),
    ("riser", "boolean", None, None, "true"),
    ("clearance_mm", "number", "double", None, "true"),
    ("railing_height_mm", "number", "double", None, "true"),
    ("comfort_step_mm", "number", "double", None, ""),
    ("landing_width_mm", "number", "double", None, ""),
    ("landing_depth_mm", "number", "double", None, ""),
    ("room_width_mm", "number", "double", None, ""),
    ("room_length_mm", "number", "double", None, ""),
    ("approach_space_mm", "number", "double", None, ""),
    ("lower_step_count", "integer", "int32", None, ""),
    ("outer_radius_mm", "number", "double", None, ""),
    # DOM-001: тип поворота и число поворотных ступеней.
    ("turn_kind", "string", None, ["platform", "winder"], ""),
    ("winder_count", "integer", "int32", None, ""),
    # DOM-003/CONF-RAILING: стороны перил и направления.
    ("railing", "string", None, ["none", "left", "right", "both"], ""),
    ("railing_lower", "string", None, ["none", "left", "right", "both"], ""),
    ("railing_landing", "string", None, ["none", "left", "right", "both"], ""),
    ("railing_upper", "string", None, ["none", "left", "right", "both"], ""),
    ("direction", "string", None, ["left", "right"], ""),
    ("spiral_direction", "string", None, ["cw", "ccw"], ""),
    # SEC-001: клиентские ставки запрещены, поле оставлено для явного отказа.
    ("rates", "object", "Не принимается: цена считается только по серверным ставкам "
                       "(SEC-001). Любое значение → 422 rates_not_allowed.", None, ""),
]

# Мутирующие методы: для них обязателен CSRF-токен (EDR-0016 §4, requireCSRF).
MUTATING_METHODS = ("POST", "PUT", "PATCH", "DELETE")

# Тела, для которых используется схема конфигурации.
CONFIG_REQUEST_PATHS = {
    ("POST", "/stairs:calculate"),
    ("POST", "/public/stairs:quote"),
    ("POST", "/public/stairs:validate"),
    ("POST", "/projects/{id}/calculate"),
    ("POST", "/projects/{id}/preview"),
    ("POST", "/projects/{id}/optimize"),
    ("POST", "/assistant/{kind}"),
}

# Операции без тела запроса, несмотря на метод (webhooks, выход).
NO_BODY = {
    ("POST", "/auth/logout"),
    ("POST", "/payments/webhook"),
    ("POST", "/payments/stripe/webhook"),
    ("POST", "/admin/payments/{id}/refund"),
}

# Ответы со схемой вместо пустого «OK».
RESPONSE_SCHEMAS = {
    ("GET", "/auth/me"): "SubjectMe",
    ("POST", "/auth/login"): "Session",
    ("GET", "/public/materials"): "MaterialList",
    ("GET", "/public/payment-tiers"): "PaymentTierList",
    ("GET", "/admin/payments"): "AdminPaymentList",
    ("POST", "/admin/payments/{id}/refund"): "AdminPayment",
}

# Схемы, на которые ссылаются ответы, но у которых нет отдельного $ref-контейнера.
INLINE_RESPONSE_SCHEMAS = {
    "MaterialList": 'type: array\nitems:\n  $ref: "#/components/schemas/Material"',
    "PaymentTierList": 'type: array\nitems:\n  $ref: "#/components/schemas/PaymentTier"',
    "AdminPaymentList": 'type: array\nitems:\n  $ref: "#/components/schemas/AdminPayment"',
}

# Краткие описания (summary) по маршрутам. Fallback собирается из пути.
SUMMARIES: dict[tuple[str, str], str] = {
    ("POST", "/auth/register"): "Зарегистрировать пользователя",
    ("POST", "/auth/login"): "Войти в систему (сессионная cookie)",
    ("POST", "/auth/logout"): "Выйти из системы",
    ("GET", "/auth/me"): "Текущий пользователь",
    ("GET", "/auth/sso"): "Начать SSO-вход (redirect на провайдера)",
    ("GET", "/auth/sso/callback"): "Callback SSO-провайдера",
    ("GET", "/auth/sso/config"): "Настройки SSO для фронтенда",
    ("GET", "/projects"): "Список проектов",
    ("POST", "/projects"): "Создать проект",
    ("GET", "/projects/{id}"): "Получить проект",
    ("DELETE", "/projects/{id}"): "Удалить проект",
    ("POST", "/projects/{id}/calculate"): "Рассчитать конфигурацию проекта",
    ("POST", "/projects/{id}/preview"): "Предпросмотр без сохранения",
    ("POST", "/projects/{id}/optimize"): "Подобрать оптимальную конфигурацию",
    ("GET", "/projects/{id}/configurations"): "История ревизий конфигурации",
    ("POST", "/projects/{id}/configurations/{configID}/restore"):
        "Восстановить ревизию конфигурации",
    ("GET", "/projects/{id}/export/cad"): "Выгрузить чертёж (DXF/STL/SVG)",
    ("POST", "/projects/{id}/export/cad/store"): "Выгрузить чертёж и сохранить в Storage",
    ("GET", "/admin/payments"): "Список платежей (admin)",
    ("POST", "/admin/payments/{id}/refund"): "Оформить возврат платежа",
    ("GET", "/admin/export"): "Выгрузка данных (CSV/JSON)",
    ("POST", "/public/stairs:quote"): "Публичный расчёт стоимости",
    ("POST", "/public/stairs:validate"): "Публичная валидация конфигурации",
    ("GET", "/health"): "Проверка живости",
    ("GET", "/ready"): "Проверка готовности",
    ("GET", "/metrics"): "Метрики Prometheus",
}

# RPC-маршруты с ":" в пути (gRPC-стиль): ключами paths в OpenAPI 3 быть не могут.
RPC = [
    "POST /stairs:calculate",
    "POST /stairs:calculate/async",
    "POST /stairs:optimize",
    "POST /stairs:validate",
    "POST /public/stairs:quote",
    "POST /public/stairs:validate",
    # Приём событий воронки витрины (миграция 000035). Без аутентификации,
    # но ТОЛЬКО после согласия: без consent_version сервер ничего не пишет.
    "POST /public/analytics:events",
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
            if path.startswith(BASE_PATH):
                path = path[len(BASE_PATH):]
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


def default_success_code(m: str) -> str:
    return {"GET": "200", "POST": "201", "PUT": "200", "PATCH": "200",
            "DELETE": "204"}.get(m, "200")


def is_paginated(m: str, path: str) -> bool:
    """Пагинация объявляется ровно там, где код её разбирает."""
    return (m, path) in PAGINATED_OPERATIONS


def yaml_str(v: str) -> str:
    """Кавычки для YAML: текст с ': ' или спецсимволами иначе ломает разбор."""
    return '"' + v.replace("\\", "\\\\").replace('"', '\\"') + '"'


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
    # API-003: base path обязателен — пути в спецификации хранятся без /api/v1.
    add(f'  - url: "http://localhost:8080{BASE_PATH}"')
    add('    description: "Local development"')
    add(f'  - url: "https://api.stairplatform.com{BASE_PATH}"')
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
    by_path: dict[str, list[str]] = {}
    for m, p in routes():
        by_path.setdefault(p, []).append(m)
    for path in sorted(by_path):
        add(f"  {path}:")
        for m in sorted(by_path[path]):
            pub = (m, path) in PUBLIC
            add(f"    {m.lower()}:")
            add(f'      tags: ["{_tag(path)}"]')
            add(f'      summary: "{_summary(m, path)}"')
            if not pub:
                # Сессионная cookie ИЛИ bearer-токен (EDR-0016). OR-семантика:
                # список элементов security = «любой из».
                add("      security:")
                add("        - bearerAuth: []")
                add("        - cookieAuth: []")
                # API-004 (2026-09-26): схема csrfToken была объявлена, но на
                # неё не ссылалась НИ ОДНА операция, хотя requireCSRF
                # обязателен на всех authMutating-маршрутах. Клиент, следующий
                # спеке, получал 403 csrf на каждом изменяющем запросе.
                # Теперь csrfToken — обязательная часть security для мутирующих
                # операций (AND к bearer/cookie): без cookie-сессии Bearer-ключу
                # CSRF не нужен, но double-submit обязателен для браузера.
                if m in MUTATING_METHODS:
                    add("        - csrfToken: []")
            # --- параметры -------------------------------------------------
            params: list[str] = []
            for ph in re.findall(r"\{([A-Za-z_][A-Za-z0-9_]*)\}", path):
                params.append(
                    "        - name: " + ph + "\n"
                    "          in: path\n"
                    "          required: true\n"
                    "          description: " + yaml_str("Идентификатор " + ph) + "\n"
                    "          schema:\n"
                    "            type: string"
                )
            for qname, qdesc, qenum in REQUIRED_QUERY.get((m, path), []):
                params.append(
                    "        - name: " + qname + "\n"
                    "          in: query\n"
                    "          required: true\n"
                    "          description: " + yaml_str(qdesc) + "\n"
                    "          schema:\n"
                    "            type: string\n"
                    "            enum: [" + ", ".join(qenum) + "]"
                )
            if (m, path) not in REQUIRED_QUERY and is_paginated(m, path):
                for qname, qdesc, qtype, qmin, qmax in OPTIONAL_QUERY_PAGINATION:
                    params.append(
                        "        - name: " + qname + "\n"
                        "          in: query\n"
                        "          required: false\n"
                        "          description: " + yaml_str(qdesc) + "\n"
                        "          schema:\n"
                        "            type: " + qtype + "\n"
                        "            minimum: " + qmin + "\n"
                        "            maximum: " + qmax
                    )
            if params:
                add("      parameters:")
                out.extend(params)
            # --- тело запроса ---------------------------------------------
            if m in ("POST", "PUT", "PATCH") and (m, path) not in NO_BODY:
                add("      requestBody:")
                if (m, path) in CONFIG_REQUEST_PATHS:
                    add("        required: true")
                    add("        content:")
                    add("          application/json:")
                    add("            schema:")
                    add('              $ref: "#/components/schemas/StairConfigInput"')
                else:
                    add("        required: true")
                    add("        content:")
                    add("          application/json:")
                    add("            schema:")
                    add("              type: object")
            elif m in ("POST", "PUT", "PATCH"):
                add("      requestBody:")
                add("        required: false")
            # --- ответы -----------------------------------------------------
            code, desc = SUCCESS_CODES.get((m, path), (default_success_code(m), "OK"))
            add("      responses:")
            add(f"        {code}:")
            add(f"          description: {yaml_str(desc)}")
            schema = RESPONSE_SCHEMAS.get((m, path))
            if schema:
                add("          content:")
                add("            application/json:")
                add("              schema:")
                add(f'                $ref: "#/components/schemas/{schema}"')
            else:
                add("          content:")
                add("            application/json:")
                add("              schema:")
                add("                type: object")
            errs: list[tuple[str, str]] = []
            if not pub:
                # 401 обязателен для защищённых операций: раньше генератор
                # выбрасывал его для GET/DELETE/PATCH/PUT, из-за чего клиент
                # не обрабатывал «сессия истекла».
                errs.append(("401", "Unauthorized"))
                if m not in ("GET", "HEAD"):
                    errs.append(("403", "Forbidden (в т.ч. CSRF-токен)"))
                errs.append(("429", "Rate limit exceeded"))
            errs.append(("400", "Invalid request"))
            errs.append(("404", "Not found"))
            errs.append(("422", "Validation error"))
            errs.append(("500", "Internal error"))
            seen: set[str] = set()
            for ecode, edesc in errs:
                if ecode in seen:
                    continue
                seen.add(ecode)
                add(f"        {ecode}:")
                add(f"          description: {yaml_str(edesc)}")
                add("          content:")
                add("            application/json:")
                add("              schema:")
                add('                $ref: "#/components/schemas/Error"')
    add("")
    add('  # Нестандартные RPC-маршруты с ":" в пути (см. x-rpc-routes) '
        "не могут быть ключами paths в OpenAPI 3.")
    add("x-rpc-routes:")
    for r in RPC:
        add(f'  - "{r}"')
    add("")
    add("components:")
    add("  securitySchemes:")
    add("    bearerAuth:")
    add("      type: http")
    add('      scheme: "bearer"')
    add('      description: "API-ключ (Authorization: Bearer) — EDR-0016"')
    add("    cookieAuth:")
    add("      type: apiKey")
    add("      in: cookie")
    add("      name: session")
    add('      description: "Сессионная cookie (httpOnly, SameSite=Strict); '
        'для admin — session_admin"')
    add("    csrfToken:")
    add("      type: apiKey")
    add("      in: header")
    add("      name: X-CSRF-Token")
    add('      description: "Double-submit CSRF: значение cookie csrf (csrf_admin) '
        'должно быть продублировано в этом заголовке для всех изменяющих запросов"')
    add("  headers:")
    add("    X-CSRF-Token:")
    add('      description: "CSRF-токен (double-submit, cookie csrf / csrf_admin)"')
    add("      schema:")
    add("        type: string")
    add("  schemas:")
    # --- Error: код отдаёт ОБЪЕКТ {code, message[, request_id]}, а не строку.
    add("    Error:")
    add("      type: object")
    add("      required: [error]")
    add("      properties:")
    add("        error:")
    add("          type: object")
    add("          required: [code, message]")
    add("          properties:")
    add("            code:")
    add("              type: string")
    add("              description: " + yaml_str(
        "Машинный код ошибки (not_found, forbidden, validation_error, invalid_input, "
        "rates_not_allowed, csrf, internal и др.)"))
    add("            message:")
    add("              type: string")
    add("              description: Человекочитаемое сообщение")
    add("            request_id:")
    add("              type: string")
    add("              description: " + yaml_str("Идентификатор запроса (только для 5xx)"))
    add("        request_id:")
    add("          type: string")
    add("          description: " + yaml_str("Идентификатор запроса для поддержки"))
    # --- StairConfigInput: реальные поля calculateRequest.
    add("    StairConfigInput:")
    add("      type: object")
    required_props = [n for n, _t, _f, _e, req in CONFIG_INPUT_PROPERTIES if req == "true"]
    if required_props:
        add("      required: [" + ", ".join(required_props) + "]")
    add("      properties:")
    for name, typ, fmt, enum, _req in CONFIG_INPUT_PROPERTIES:
        add(f"        {name}:")
        add(f"          type: {typ}")
        if fmt and typ in ("number", "integer"):
            add(f"          format: {fmt}")
        if enum:
            add("          enum: [" + ", ".join(enum) + "]")
        if fmt and typ == "object":
            add(f"          description: {yaml_str(fmt)}")
    # --- User / Session.
    # User — по факту userDTO (auth.go:145): id, email, name, role, tenant_id.
    # Раньше здесь был status в required, которого в ответе нет, и не было
    # tenant_id, который есть: клиент по спеке получал status === undefined.
    add("    User:")
    add("      type: object")
    add("      required: [id, email, role]")
    add("      properties:")
    add("        id:")
    add("          type: string")
    add("          format: uuid")
    add("        email:")
    add("          type: string")
    add("          format: email")
    add("        name:")
    add("          type: string")
    add("        role:")
    add("          type: string")
    add("          enum: [user, admin]")
    add("        tenant_id:")
    add("          type: string")
    add("          format: uuid")
    # SubjectMe — ответ GET /auth/me (SEC-005): subject_type различает
    # сессионного пользователя и API-ключ, у них разные формы ответа.
    add("    SubjectMe:")
    add("      type: object")
    add("      required: [subject_type]")
    add("      properties:")
    add("        subject_type:")
    add("          type: string")
    add("          enum: [user, api_key]")
    add("        user:")
    add('          $ref: "#/components/schemas/User"')
    add("        api_key:")
    add('          $ref: "#/components/schemas/ApiKey"')
    add("    ApiKey:")
    add("      type: object")
    add("      required: [id, name]")
    add("      properties:")
    add("        id:")
    add("          type: string")
    add("          format: uuid")
    add("        name:")
    add("          type: string")
    add("        scopes:")
    add("          type: array")
    add("          items:")
    add("            type: string")
    add("        created_at:")
    add("          type: string")
    add("          format: date-time")
    add("        last_used_at:")
    add("          type: string")
    add("          format: date-time")
    # Session — по факту authResponse (auth.go:154): user + token в теле.
    # Раньше здесь был required [user, csrf_token], причём csrf_token нет НИ В
    # ОДНОМ ответе API (nonce живёт только в cookie csrf/csrf_admin) — клиент
    # по спеке получал csrf_token === undefined и получал 403 на всех
    # изменяющих запросах. Теперь CSRF описан как securityScheme csrfToken.
    add("    Session:")
    add("      type: object")
    add("      required: [user]")
    add("      properties:")
    add("        user:")
    add('          $ref: "#/components/schemas/User"')
    add("        token:")
    add("          type: string")
    add("          description: " + yaml_str(
        "Сессионный токен. Дублирует httpOnly cookie session (session_admin) и " +
        "нужен только небраузерным клиентам; CSRF-токен берётся из cookie csrf " +
        "и дублируется в заголовке X-CSRF-Token"))
    add("    Material:")
    add("      type: object")
    add("      required: [code]")
    add("      properties:")
    add("        code:")
    add("          type: string")
    add("        name:")
    add("          type: string")
    add("        name_ru:")
    add("          type: string")
    add("        category:")
    add("          type: string")
    add("        density:")
    add("          type: number")
    add("        min_thickness:")
    add("          type: number")
    add("        max_thickness:")
    add("          type: number")
    add("        max_width_mm:")
    add("          type: number")
    add("        max_height_mm:")
    add("          type: number")
    add("    PaymentTier:")
    add("      type: object")
    add("      required: [id, price_minor, currency]")
    add("      properties:")
    add("        id:")
    add("          type: string")
    add("        title:")
    add("          type: string")
    add("        price_minor:")
    add("          type: integer")
    add("          format: int64")
    add("        currency:")
    add("          type: string")
    add("        features:")
    add("          type: array")
    add("          items:")
    add("            type: string")
    # AdminPayment — единственная схема, совпадавшая с кодом; до API-003 она была
    # добавлена в файлы руками и терялась при каждой регенерации.
    add("    AdminPayment:")
    add("      type: object")
    add("      required: [id, amount_minor, currency, status, provider, created_at]")
    add("      properties:")
    for fld, ftype, ffmt, enum in [
        ("id", "string", "uuid", None),
        ("tier_id", "string", None, None),
        ("project_id", "string", "uuid", None),
        ("user_id", "string", "uuid", None),
        ("amount_minor", "integer", "int64", None),
        ("currency", "string", None, None),
        ("status", "string", None, ["pending", "paid", "failed", "refunded"]),
        ("provider", "string", None, None),
        ("created_at", "string", "date-time", None),
        ("paid_at", "string", "date-time", None),
    ]:
        add(f"        {fld}:")
        add(f"          type: {ftype}")
        if ffmt:
            add(f"          format: {ffmt}")
        if enum:
            add("          enum: [" + ", ".join(enum) + "]")
    for name, body in INLINE_RESPONSE_SCHEMAS.items():
        add(f"    {name}:")
        for line in body.split("\n"):
            add("      " + line)
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
