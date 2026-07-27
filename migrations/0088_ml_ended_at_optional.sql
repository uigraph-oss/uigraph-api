ALTER TABLE ml_runs        ALTER COLUMN ended_at DROP NOT NULL;
ALTER TABLE ml_evaluations ALTER COLUMN ended_at DROP NOT NULL;
ALTER TABLE ml_experiments DROP COLUMN started_at;
