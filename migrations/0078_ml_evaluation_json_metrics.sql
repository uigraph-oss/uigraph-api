-- Evaluations now carry metrics and parameters as JSONB objects, matching the
-- shape already used by ml_runs. Drops the per-metric child table and the
-- per-step run metric points, which are no longer surfaced anywhere.

ALTER TABLE ml_evaluations ADD COLUMN IF NOT EXISTS parameters JSONB NOT NULL DEFAULT '{}';
ALTER TABLE ml_evaluations ADD COLUMN IF NOT EXISTS metrics    JSONB NOT NULL DEFAULT '{}';

UPDATE ml_evaluations e SET metrics = m.metrics
FROM (
    SELECT evaluation_id, jsonb_object_agg(name, value) AS metrics
    FROM ml_evaluation_metrics
    GROUP BY evaluation_id
) m
WHERE m.evaluation_id = e.id;

DROP TABLE ml_evaluation_metrics;
DROP TABLE ml_run_metric_points;
