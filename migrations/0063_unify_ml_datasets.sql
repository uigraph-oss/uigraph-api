-- Unify datasets into one MLflow-native, experiment-scoped entity carrying a
-- content digest and a training/evaluation context. Drops the org-wide registry
-- shape of ml_datasets and the separate ml_evaluation_datasets concept. Dataset
-- data is re-synced from MLflow, so the tables are rebuilt outright.

ALTER TABLE ml_runs        DROP CONSTRAINT IF EXISTS ml_runs_dataset_fk;
ALTER TABLE ml_evaluations DROP CONSTRAINT IF EXISTS ml_evaluations_dataset_fk;
ALTER TABLE ml_runs        ADD COLUMN IF NOT EXISTS dataset_id UUID;
ALTER TABLE ml_evaluations ADD COLUMN IF NOT EXISTS dataset_id UUID;
UPDATE ml_runs        SET dataset_id = NULL;
UPDATE ml_evaluations SET dataset_id = NULL;

DROP TABLE IF EXISTS ml_experiment_evaluation_datasets;
DROP TABLE IF EXISTS ml_evaluation_datasets;
DROP TABLE IF EXISTS ml_datasets;

CREATE TABLE ml_datasets (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id        UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    experiment_id UUID        NOT NULL REFERENCES ml_experiments(id) ON DELETE CASCADE,
    mlflow_id     TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    digest        TEXT        NOT NULL DEFAULT '',
    source        TEXT        NOT NULL DEFAULT '',
    source_type   TEXT        NOT NULL DEFAULT '',
    context       TEXT        NOT NULL DEFAULT 'training' CHECK (context IN ('training','evaluation')),
    row_count     BIGINT      NOT NULL DEFAULT 0,
    schema        JSONB       NOT NULL DEFAULT '[]',
    tags          JSONB       NOT NULL DEFAULT '{}',
    synced_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID        NOT NULL,
    updated_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    UNIQUE (org_id, experiment_id, mlflow_id)
);
CREATE INDEX idx_ml_datasets_org_exp ON ml_datasets(org_id, experiment_id) WHERE deleted_at IS NULL;

ALTER TABLE ml_runs        ADD CONSTRAINT ml_runs_dataset_fk        FOREIGN KEY (dataset_id) REFERENCES ml_datasets(id) ON DELETE SET NULL;
ALTER TABLE ml_evaluations ADD CONSTRAINT ml_evaluations_dataset_fk FOREIGN KEY (dataset_id) REFERENCES ml_datasets(id) ON DELETE SET NULL;
