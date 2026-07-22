CREATE TABLE ml_evaluation_datasets (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    mlflow_id   TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    digest      TEXT        NOT NULL DEFAULT '',
    source      TEXT        NOT NULL DEFAULT '',
    source_type TEXT        NOT NULL DEFAULT '',
    row_count   BIGINT      NOT NULL DEFAULT 0,
    schema      JSONB       NOT NULL DEFAULT '[]',
    tags        JSONB       NOT NULL DEFAULT '{}',
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    UNIQUE (org_id, mlflow_id)
);
CREATE INDEX idx_ml_eval_datasets_org ON ml_evaluation_datasets(org_id) WHERE deleted_at IS NULL;

CREATE TABLE ml_experiment_evaluation_datasets (
    experiment_id UUID NOT NULL REFERENCES ml_experiments(id) ON DELETE CASCADE,
    dataset_id    UUID NOT NULL REFERENCES ml_evaluation_datasets(id) ON DELETE CASCADE,
    PRIMARY KEY (experiment_id, dataset_id)
);
CREATE INDEX idx_ml_exp_eval_ds_experiment ON ml_experiment_evaluation_datasets(experiment_id);
