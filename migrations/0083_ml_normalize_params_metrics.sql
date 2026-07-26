CREATE TABLE IF NOT EXISTS ml_runs_params (
    id      UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    run_id  UUID NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    value   TEXT NOT NULL,
    UNIQUE (run_id, key)
);
CREATE INDEX IF NOT EXISTS idx_ml_runs_params_run ON ml_runs_params(run_id);

CREATE TABLE IF NOT EXISTS ml_runs_metrics (
    id      UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    run_id  UUID NOT NULL REFERENCES ml_runs(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    value   DOUBLE PRECISION NOT NULL,
    UNIQUE (run_id, key)
);
CREATE INDEX IF NOT EXISTS idx_ml_runs_metrics_run ON ml_runs_metrics(run_id);

CREATE TABLE IF NOT EXISTS ml_evaluations_params (
    id      UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    eval_id UUID NOT NULL REFERENCES ml_evaluations(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    value   TEXT NOT NULL,
    UNIQUE (eval_id, key)
);
CREATE INDEX IF NOT EXISTS idx_ml_evaluations_params_eval ON ml_evaluations_params(eval_id);

CREATE TABLE IF NOT EXISTS ml_evaluations_metrics (
    id      UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    eval_id UUID NOT NULL REFERENCES ml_evaluations(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    value   DOUBLE PRECISION NOT NULL,
    UNIQUE (eval_id, key)
);
CREATE INDEX IF NOT EXISTS idx_ml_evaluations_metrics_eval ON ml_evaluations_metrics(eval_id);

ALTER TABLE ml_runs DROP COLUMN IF EXISTS parameters, DROP COLUMN IF EXISTS metrics;
ALTER TABLE ml_evaluations DROP COLUMN IF EXISTS parameters, DROP COLUMN IF EXISTS metrics;
