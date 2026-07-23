-- Reconcile ml_models with the schema defined in 0060_ml_studio.sql, the same
-- way 0061 did for ml_experiments, ml_runs and ml_artifacts. Some databases
-- were provisioned from an earlier draft of 0060 that carried extra columns
-- (a per-model owner/status design that was dropped before 0060 was committed).
-- Because 0060 is already recorded as applied, the simplified definition never
-- reached those databases, leaving ml_models with stale owner and status
-- columns. Neither is referenced by any application code, and ml_models ingests
-- from MLflow where the concepts do not exist. Dropping them brings the live
-- schema back in line with 0060.

ALTER TABLE ml_models DROP COLUMN IF EXISTS owner;
ALTER TABLE ml_models DROP COLUMN IF EXISTS status;
