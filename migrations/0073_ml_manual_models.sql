-- Allow models to be registered manually, in addition to MLflow sync. Mirrors
-- 0070_ml_manual_experiments_runs.sql: the existing UNIQUE (org_id, mlflow_id)
-- constraint is left untouched since Postgres treats NULL as distinct from
-- NULL, so manually registered rows (mlflow_id IS NULL) coexist fine
-- alongside synced rows.

ALTER TABLE ml_models ALTER COLUMN mlflow_id DROP NOT NULL;
ALTER TABLE ml_models ALTER COLUMN synced_at DROP NOT NULL;
ALTER TABLE ml_models ADD COLUMN origin TEXT NOT NULL DEFAULT 'mlflow' CHECK (origin IN ('mlflow','manual'));
