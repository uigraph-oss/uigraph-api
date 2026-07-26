-- Tags on experiments, mirroring ml_models.tags. Seeded from MLflow experiment
-- tags on first sync and editable from ML Studio afterwards; later syncs leave
-- the column alone so manual edits are not clobbered.

ALTER TABLE ml_experiments ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
