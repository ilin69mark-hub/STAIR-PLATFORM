ALTER TABLE stair_configurations
    ADD COLUMN landing_width_mm DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN lower_step_count INTEGER NOT NULL DEFAULT 0;