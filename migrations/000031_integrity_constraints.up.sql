-- DB-001 (forensic 2026-09-26): пробелы целостности в схеме.
--
-- Проверено по всем 26 таблицам. Реальные пробелы (остальное уже покрыто
-- FK/CHECK/UNIQUE в 000001–000030):
--
--  1. integration_endpoints: FindEndpointByKind возвращает `ORDER BY
--     created_at LIMIT 1`, то есть сервис ПРЕДПОЛАГАЕТ «один активный
--     эндпоинт на вид», но БД разрешала сколько угодно. Второй ERP-эндпоинт
--     создавался без ошибки, а webhook-доставка молча уходила в более
--     старый — пользователь считал новый активным, и доставка в CRM/
--     производство не работала. Фикс: UNIQUE (tenant_id, kind).
--
--  2. projects.status: TEXT DEFAULT 'draft' БЕЗ CHECK, хотя тот же паттерн
--     реализован для users.role/status, project_members.role,
--     project_reviews.decision, orders.status, calc_jobs.status,
--     payment_intents.status, integration_endpoints.kind. Опечатка в коде
--     сохранялась бы как «новая» статусная строка и ломала бы фильтры.
--     Список взят из констант домена (project/entity.go:102-105), а не
--     «по смыслу»: draft → in_review → approved | changes_requested.
--
--  3. stair_configurations.flight: TEXT БЕЗ CHECK. Тип марша — ключевой
--     параметр, по нему выбираются геометрия и расчёт; мусорное значение
--     проходило в БД и всплывало в виде «лестница не строится».
--
--  4. stair_configurations: геометрические величины без CHECK > 0.
--     Ноль/отрицательные ширина, высота, высота ступени, проступь, просвет
--     и толщины не строятся, но занимали строки ревизий.
--
--  5. payment_intents.amount_minor: BIGINT NOT NULL БЕЗ CHECK >= 0.
--     Для сравнения, store_rates.price_per_kg_rub такой CHECK имеет.
--     Отрицательная сумма платежа — это деньги, выданные поRefund сверх
--     оплаты, и она молча портила бы сверки.
--
--  6. integration_events.attempts: счётчик попыток доставки без CHECK >= 0
--     (инкремент, но ограничения не было).
--
-- Ограничения добавляются идемпотентно (DROP IF EXISTS + ADD), как в 000029.
-- CHECK-ограничения на CHECK-совместимых колонках безопасны для новых строк.
-- Если на legacy-данных ограничение не выполняется, миграция упадёт с
-- указанием конкретного ограничения — это осознанный «громкий» отказ:
-- молча чинить денежные/геометрические данные нельзя.

-- 1) Один эндпоинт на вид интеграции в пределах tenant.
ALTER TABLE integration_endpoints
    DROP CONSTRAINT IF EXISTS integration_endpoints_tenant_kind_key;
ALTER TABLE integration_endpoints
    ADD CONSTRAINT integration_endpoints_tenant_kind_key UNIQUE (tenant_id, kind);

-- 2) Статус проекта: ровно четыре значения из project/entity.go.
ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_status_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_status_check
    CHECK (status IN ('draft', 'in_review', 'approved', 'changes_requested'));

-- 3+4) Тип марша и геометрия конфигурации.
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_flight_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_flight_check
    CHECK (flight IN ('straight', 'l_shape', 'u_shape', 'spiral'));

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_positive_geometry;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_positive_geometry CHECK (
        width_mm             > 0
    AND height_mm            > 0
    AND step_height_mm       > 0
    AND stringer_thickness_mm > 0
    AND step_thickness_mm     > 0
    -- clearance_mm, railing_height_mm и comfort_step_mm допускают 0 — это
    -- легальные доменные значения, а не «битые» данные (CRITICAL-01,
    -- 2026-09-26). Строгое > 0 по всем трём было неверно на трёх уровнях:
    --
    --   * comfort_step_mm = 0 — сентинел «взять solver.DefaultComfortStep»
    --     (application/stair/service.go:76, application/stair/optimize.go:266);
    --   * clearance_mm = 0 — «нет ограничения по просвету», так заведена
    --     штатная демо-конфигурация (migrations/seeds/000001_demo_data.up.sql),
    --     из-за чего `make seed` падал с 23514;
    --   * railing_height_mm = 0 — перил нет: движок это штатно терпит
    --     (engine/geometry/railing.go:53, RailingNone в
    --     domain/engineering/stair.go:35).
    --
    -- Транспорт кладёт значения прямиком из запроса (transport/http/dto.go:660-661),
    -- а project.Service.Calculate сохраняет конфигурацию независимо от
    -- результата валидации (application/project/service.go:295), поэтому строгий
    -- CHECK превращал обычный запрос без необязательных полей в ошибку БД.
    AND clearance_mm         >= 0
    AND railing_height_mm    >= 0
    AND comfort_step_mm      >= 0
    AND revision             >= 1
    );

-- 5) Сумма платежа неотрицательна.
ALTER TABLE payment_intents
    DROP CONSTRAINT IF EXISTS payment_intents_amount_check;
ALTER TABLE payment_intents
    ADD CONSTRAINT payment_intents_amount_check CHECK (amount_minor >= 0);

-- 6) Счётчик попыток доставки неотрицателен.
ALTER TABLE integration_events
    DROP CONSTRAINT IF EXISTS integration_events_attempts_check;
ALTER TABLE integration_events
    ADD CONSTRAINT integration_events_attempts_check CHECK (attempts >= 0);

COMMENT ON COLUMN integration_endpoints.kind IS
    'Вид интеграции; не более одного активного эндпоинта на вид в пределах tenant (UNIQUE (tenant_id, kind)) — по нему же работает FindEndpointByKind';
