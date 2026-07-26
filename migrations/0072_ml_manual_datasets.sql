-- Allow datasets to be logged manually against an experiment, in addition to
-- MLflow sync. Mirrors 0070_ml_manual_experiments_runs.sql: the existing
-- UNIQUE (org_id, experiment_id, mlflow_id) constraint is left untouched since
-- Postgres treats NULL as distinct from NULL, so manually logged rows
-- (mlflow_id IS NULL) coexist fine alongside synced rows.

ALTER TABLE ml_datasets ALTER COLUMN mlflow_id DROP NOT NULL;
ALTER TABLE ml_datasets ALTER COLUMN synced_at DROP NOT NULL;
ALTER TABLE ml_datasets ADD COLUMN origin TEXT NOT NULL DEFAULT 'mlflow' CHECK (origin IN ('mlflow','manual'));
ALTER TABLE ml_datasets ADD COLUMN deleted_by UUID;
