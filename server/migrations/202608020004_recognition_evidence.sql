-- +goose Up
ALTER TABLE recognition_jobs
    ADD COLUMN ocr_text text NOT NULL DEFAULT '' CHECK (char_length(ocr_text) <= 30000),
    ADD COLUMN ocr_provider text NOT NULL DEFAULT '' CHECK (char_length(ocr_provider) <= 120),
    ADD COLUMN ocr_model text NOT NULL DEFAULT '' CHECK (char_length(ocr_model) <= 160),
    ADD COLUMN ocr_duration_ms integer CHECK (ocr_duration_ms BETWEEN 0 AND 600000),
    ADD COLUMN ocr_completed_at timestamptz,
    ADD COLUMN trace jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE recognition_jobs
    DROP COLUMN IF EXISTS trace,
    DROP COLUMN IF EXISTS ocr_completed_at,
    DROP COLUMN IF EXISTS ocr_duration_ms,
    DROP COLUMN IF EXISTS ocr_model,
    DROP COLUMN IF EXISTS ocr_provider,
    DROP COLUMN IF EXISTS ocr_text;
