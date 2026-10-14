-- Откат кода материала STEEL-C245 → STEEL-S235.
--
-- Обратная замена значений и комментария. Каталог MFG-0005 откатывается тем
-- же коммитом, что и миграция, поэтому после отката приложение снова видит
-- старый код в данных.
UPDATE stair_configurations
    SET material_code = 'STEEL-S235'
    WHERE material_code = 'STEEL-C245';

UPDATE stair_configurations
    SET tread_material_code = 'STEEL-S235'
    WHERE tread_material_code = 'STEEL-C245';

UPDATE store_rates
    SET material_code = 'STEEL-S235'
    WHERE material_code = 'STEEL-C245';

COMMENT ON COLUMN stair_configurations.material_code IS
    'Код материала каталога MFG-0005 (напр. STEEL-S235). NULL = автоназначение по толщине';