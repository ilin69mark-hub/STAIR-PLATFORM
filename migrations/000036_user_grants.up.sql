-- 000036_user_grants — точечные выдачи прав сотрудникам (EDR-0041 §5).
--
-- Зачем: права приходили из роли, а роль — одна на всех. «Админ» значил
-- «всё, включая финансы», поэтому руководителю продаж нельзя было дать
-- заказы, не дав ещё и возвраты, а менеджеру — одну цифру, не дав всей
-- финансовой части. Владелец теперь открывает страницу сотрудника и
-- выдаёт конкретные метрики построчно: «видеть» / «менять», а для
-- действий (возврат, выгрузка, приглашение) — один переключатель.
--
-- Принципы, из которых таблица и вытекает:
--   1. Метрика — одна строка на сотрудника. Два уровня (read/write) или
--      один (action) — различает реестр capability.go, в БД это уровень.
--      'write' в БД означает «и видеть, и менять»: выдать write без read
--      нельзя (сервис не даст), поэтому хранить их двумя строками — значит
--      хранить противоречие.
--   2. Пустой набор строк — законное состояние («не дал ничего»), но чтобы
--      отличить его от «настройки ещё не трогали», есть флаг
--      users.grants_initialized. Пока флаг сброшен, действует роль — это
--      переходный путь, чтобы не переписывать 28 файлов тестов разом и не
--      менять поведение существующих сотрудников без пересадки.
--   3. Право выдавать (staff.grants) в этой таблице не хранится ни у кого,
--      кроме владельца: право выдавать другим нельзя получить по правилу
--      «нельзя выдать больше, чем имеешь сам». Владелец получает полный
--      набор программно, а не строкой в таблице.
--   4. Отзывы о выдаче — только через audit_events (grant.go пишет
--      ActionUserGrantsChanged), чтобы по таблице нельзя было понять,
--      кто и когда расширил доступ.

CREATE TABLE IF NOT EXISTS user_grants (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    capability TEXT NOT NULL,
    level TEXT NOT NULL,
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, capability),
    CONSTRAINT user_grants_level_check CHECK (level IN ('read', 'write', 'action'))
);

COMMENT ON TABLE user_grants IS
    'Точечные выдачи прав сотруднику (EDR-0041). Активны, только если users.grants_initialized = true; иначе действует роль.';
COMMENT ON COLUMN user_grants.level IS
    'read — видеть, write — видеть и менять, action — выполнить действие (возврат, выгрузка, приглашение).';

CREATE INDEX IF NOT EXISTS idx_user_grants_user ON public.user_grants (user_id);

-- Флаг «права настроены вручную». False = действует роль (переходный
-- период), true = источник прав — эта таблица, даже если она пуста.
ALTER TABLE users ADD COLUMN IF NOT EXISTS grants_initialized BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN users.grants_initialized IS
    'true — права сотрудника заданы точечно в user_grants; false — действует роль (переходный период).';

-- Мягкое удаление сотрудника. Отдельная от status: «заблокирован» —
-- временно не пускаем в систему, «удалён» — человека больше нет, его
-- проекты и заказы остаются в истории. Явную метку времени удаляем не
-- из user_grants — она удаляется каскадом вместе с пользователем.
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_by UUID REFERENCES users(id) ON DELETE SET NULL;

COMMENT ON COLUMN users.deleted_at IS
    'Мягкое удаление: строка остаётся (проекты, заказы, история), но пользователь скрыт из списков и не может войти.';

-- Роли становятся шаблонами прав по умолчанию, а не источником (EDR-0041 §3.2).
-- Появляются две: owner — владелец, sales — руководитель отдела продаж.
-- Старая admin остаётся и по-прежнему даёт всё: сузить её набор значило бы
-- молча отнять доступ у действующих администраторов.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('user', 'admin', 'sales', 'owner'));

-- Частичный индекс: последнего владельца нельзя удалить/разжаловать, но
-- проверять это проще в сервисе. Индекс помогает найти владельцев дешево.
CREATE INDEX IF NOT EXISTS idx_users_role_owner ON public.users (role) WHERE role = 'owner';

-- Удалённые не должны попадать ни в один выбор пользователей.
CREATE INDEX IF NOT EXISTS idx_users_tenant_active
    ON public.users (tenant_id)
    WHERE deleted_at IS NULL AND status = 'active';