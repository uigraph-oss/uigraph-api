-- Allow artifacts to be attached to a run manually (uploaded, linked, or hand
-- entered), in addition to MLflow sync. Mirrors 0070_ml_manual_experiments_runs.sql:
-- the existing UNIQUE (org_id, mlflow_id) constraint is left untouched since
-- Postgres treats NULL as distinct from NULL, so manually added rows
-- (mlflow_id IS NULL) coexist fine alongside synced rows.

ALTER TABLE ml_artifacts ALTER COLUMN mlflow_id DROP NOT NULL;
ALTER TABLE ml_artifacts ALTER COLUMN synced_at DROP NOT NULL;
ALTER TABLE ml_artifacts ADD COLUMN source TEXT NOT NULL DEFAULT 'mlflow' CHECK (source IN ('mlflow','manual'));
ALTER TABLE ml_artifacts ADD COLUMN deleted_by UUID;

-- Type becomes free text and is validated at the application layer: manually
-- added artifacts (PDFs, dashboards, links) do not fit the six MLflow values.
ALTER TABLE ml_artifacts DROP CONSTRAINT ml_artifacts_type_check;

-- Storage columns for uploaded artifacts. storage_key is internal and is never
-- returned to clients; reads presign a short-lived GET URL from it instead.
ALTER TABLE ml_artifacts ADD COLUMN storage_key TEXT NOT NULL DEFAULT '';
ALTER TABLE ml_artifacts ADD COLUMN mime_type TEXT NOT NULL DEFAULT '';
ALTER TABLE ml_artifacts ADD COLUMN size_bytes BIGINT;
