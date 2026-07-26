-- Tags on experiments, mirroring ml_models.tags. Synced from MLflow experiment
-- tags: every sync replaces the column with what MLflow currently reports.

ALTER TABLE ml_experiments ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
