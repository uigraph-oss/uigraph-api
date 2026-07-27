-- Findings can cite evaluation runs as evidence alongside training runs.
-- Mirrors ml_finding_runs from 0060_ml_studio.sql, plus the reverse-lookup
-- index that table is missing.

CREATE TABLE ml_finding_evaluations (
    finding_id    UUID NOT NULL REFERENCES ml_findings(id)    ON DELETE CASCADE,
    evaluation_id UUID NOT NULL REFERENCES ml_evaluations(id) ON DELETE CASCADE,
    PRIMARY KEY (finding_id, evaluation_id)
);

CREATE INDEX idx_ml_finding_evaluations_eval ON ml_finding_evaluations(evaluation_id);
