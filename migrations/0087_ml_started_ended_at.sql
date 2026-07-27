DELETE FROM ml_runs WHERE started_at IS NULL OR ended_at IS NULL;
ALTER TABLE ml_runs ALTER COLUMN started_at SET NOT NULL;
ALTER TABLE ml_runs ALTER COLUMN ended_at SET NOT NULL;

DELETE FROM ml_evaluations WHERE evaluated_at IS NULL;
ALTER TABLE ml_evaluations ADD COLUMN started_at TIMESTAMPTZ;
ALTER TABLE ml_evaluations ADD COLUMN ended_at TIMESTAMPTZ;
UPDATE ml_evaluations SET started_at = evaluated_at, ended_at = evaluated_at;
ALTER TABLE ml_evaluations ALTER COLUMN started_at SET NOT NULL;
ALTER TABLE ml_evaluations ALTER COLUMN ended_at SET NOT NULL;
ALTER TABLE ml_evaluations DROP COLUMN evaluated_at;
