ALTER TABLE ml_runs        ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE ml_evaluations ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE ml_datasets ADD COLUMN tags_arr TEXT[] NOT NULL DEFAULT '{}';
UPDATE ml_datasets SET tags_arr = COALESCE((
    SELECT array_agg(CASE WHEN value = '' THEN key ELSE key || ': ' || value END ORDER BY key)
    FROM jsonb_each_text(tags)
), '{}');
ALTER TABLE ml_datasets DROP COLUMN tags;
ALTER TABLE ml_datasets RENAME COLUMN tags_arr TO tags;

CREATE INDEX ml_models_tags_idx      ON ml_models      USING GIN (tags);
CREATE INDEX ml_experiments_tags_idx ON ml_experiments USING GIN (tags);
CREATE INDEX ml_datasets_tags_idx    ON ml_datasets    USING GIN (tags);
CREATE INDEX ml_runs_tags_idx        ON ml_runs        USING GIN (tags);
CREATE INDEX ml_evaluations_tags_idx ON ml_evaluations USING GIN (tags);
