-- An evaluation run belongs to the MLflow experiment it ran in, so experiment_id
-- is required alongside version_id. Rows synced before the column existed carry
-- no experiment and are removed; the next sync recreates them.

ALTER TABLE ml_evaluations ADD COLUMN IF NOT EXISTS experiment_id UUID REFERENCES ml_experiments(id) ON DELETE CASCADE;

DELETE FROM ml_evaluations WHERE experiment_id IS NULL;

ALTER TABLE ml_evaluations ALTER COLUMN experiment_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ml_evaluations_experiment ON ml_evaluations(experiment_id) WHERE deleted_at IS NULL;
