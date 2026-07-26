-- Allow experiments and runs to be entered manually, in addition to MLflow sync.
-- The existing UNIQUE (org_id, mlflow_id) constraints are left untouched: Postgres
-- treats NULL as distinct from NULL in unique constraints, so multiple manually
-- created rows (mlflow_id IS NULL) coexist fine alongside synced rows.

ALTER TABLE ml_experiments ALTER COLUMN mlflow_id DROP NOT NULL;
ALTER TABLE ml_experiments ALTER COLUMN synced_at DROP NOT NULL;
ALTER TABLE ml_experiments ADD COLUMN source TEXT NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual'));
ALTER TABLE ml_experiments ADD COLUMN deleted_by UUID;

ALTER TABLE ml_runs ALTER COLUMN mlflow_id DROP NOT NULL;
ALTER TABLE ml_runs ALTER COLUMN synced_at DROP NOT NULL;
ALTER TABLE ml_runs ADD COLUMN source TEXT NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual'));
ALTER TABLE ml_runs ADD COLUMN deleted_by UUID;
