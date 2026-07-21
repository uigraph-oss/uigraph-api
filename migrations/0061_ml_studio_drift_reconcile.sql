-- Reconcile ml_experiments, ml_runs and ml_artifacts with the schema defined in
-- 0060_ml_studio.sql. Some databases were provisioned from an earlier draft of
-- 0060 that carried extra columns (a model-scoped experiment design that was
-- dropped before 0060 was committed). Because 0060 is already recorded as
-- applied, the simplified definition never reached those databases, leaving
-- ml_experiments.model_id as a NOT NULL column with no default. That made every
-- experiment upsert from the MLflow syncer fail, which in turn blocked runs and
-- run series (their parent experiments never existed).
--
-- None of these columns are referenced by any application code, and all three
-- tables ingest from MLflow where the concepts do not exist. Dropping them
-- brings the live schema back in line with 0060.

ALTER TABLE ml_experiments DROP COLUMN IF EXISTS model_id;
ALTER TABLE ml_experiments DROP COLUMN IF EXISTS version_id;
ALTER TABLE ml_experiments DROP COLUMN IF EXISTS goal;
ALTER TABLE ml_experiments DROP COLUMN IF EXISTS hypothesis;
ALTER TABLE ml_experiments DROP COLUMN IF EXISTS owner;
ALTER TABLE ml_experiments DROP COLUMN IF EXISTS ended_at;

ALTER TABLE ml_runs DROP COLUMN IF EXISTS trigger;
ALTER TABLE ml_runs DROP COLUMN IF EXISTS environment;
ALTER TABLE ml_runs DROP COLUMN IF EXISTS model_arch;

ALTER TABLE ml_artifacts DROP COLUMN IF EXISTS description;
ALTER TABLE ml_artifacts DROP COLUMN IF EXISTS mlflow_created_at;
