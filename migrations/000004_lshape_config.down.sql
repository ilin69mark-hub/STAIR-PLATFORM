ALTER TABLE stair_configurations
    DROP COLUMN IF EXISTS landing_width_mm,
    DROP COLUMN IF EXISTS lower_step_count;