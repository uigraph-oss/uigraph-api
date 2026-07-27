-- 0084_ml_manual_evaluations.sql originally added this column as `origin`, then
-- was edited in place to `source` after it had already been applied. The migrator
-- keys on filename, so databases that ran the earlier version still have `origin`
-- while the Go store reads and writes `source`. Rename them into agreement.
--
-- Guarded so it is a no-op on databases that ran the current 0084 and already
-- have `source`. `ml_models` and `ml_datasets` keep their own `origin` columns —
-- only evaluations are renamed here.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_evaluations' AND column_name = 'origin'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_evaluations' AND column_name = 'source'
    ) THEN
        ALTER TABLE ml_evaluations RENAME COLUMN origin TO source;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'ml_evaluations'::regclass AND conname = 'ml_evaluations_origin_check'
    ) THEN
        ALTER TABLE ml_evaluations RENAME CONSTRAINT ml_evaluations_origin_check TO ml_evaluations_source_check;
    END IF;
END $$;
